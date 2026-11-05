package logcontent

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tidwall/gjson"
)

const (
	// maxAssembleChoices is how many OpenAI choices are followed.
	maxAssembleChoices = 8
	// assembleBlockCost is charged per block against the 1 MiB budget, so a
	// stream of empty blocks cannot grow without bound.
	assembleBlockCost = 64
)

// AssembledStream is the assistant message rebuilt from a stream.
type AssembledStream struct {
	Messages []ViewMessage
	// Terminal is set when the stream ended in a way its format defines.
	Terminal bool
	// Truncated is set when a frame or the message exceeded its limit.
	Truncated bool
}

// asmMessage is one message under construction: a choice, or the whole answer.
type asmMessage struct {
	blocks []ViewBlock
	// texts and args hold the growing Text and Arguments of each block, so
	// many small deltas do not copy the whole string every time.
	texts []*strings.Builder
	args  []*strings.Builder
	// byKey maps a format-specific key (block index, output index) to a block.
	byKey map[string]int
	// lastSummary is the last reasoning summary index seen per reasoning item.
	lastSummary map[string]int64
}

func newAsmMessage() *asmMessage {
	return &asmMessage{byKey: map[string]int{}, lastSummary: map[string]int64{}}
}

// add appends a block and returns its index.
func (m *asmMessage) add(block ViewBlock) int {
	text, args := &strings.Builder{}, &strings.Builder{}
	text.WriteString(block.Text)
	args.WriteString(block.Arguments)
	block.Text, block.Arguments = "", ""
	m.blocks = append(m.blocks, block)
	m.texts = append(m.texts, text)
	m.args = append(m.args, args)
	return len(m.blocks) - 1
}

// StreamAssembler rebuilds the message of a stream from its raw bytes, in any
// chunking. It is not safe for concurrent use.
type StreamAssembler struct {
	format string

	pending  []byte // incomplete line (SSE) or incomplete element (array)
	dropping bool   // the current line, frame or element is over the limit
	frame    []byte // data lines of the frame being read
	frameLen int

	mode       int // 0 undecided, 1 SSE, 2 JSON array
	depth      int
	inString   bool
	escaped    bool
	flushed    bool
	collecting bool
	messages   map[int]*asmMessage
	replaced   []ViewMessage
	used       int
	terminal   bool
	truncated  bool
}

const (
	modeUndecided = iota
	modeSSE
	modeArray
	modeIgnore
)

// NewStreamAssembler returns an assembler for one of the Format constants.
func NewStreamAssembler(format string) *StreamAssembler {
	return &StreamAssembler{format: format, messages: map[int]*asmMessage{}}
}

// Write feeds the next bytes of the stream.
func (s *StreamAssembler) Write(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	s.flushed = false
	if s.mode == modeUndecided {
		trimmed := bytes.TrimLeft(chunk, " \t\r\n\xef\xbb\xbf")
		if len(trimmed) == 0 {
			return
		}
		s.mode = modeSSE
		if s.format == FormatGemini && trimmed[0] == '[' {
			s.mode = modeArray
		}
		chunk = trimmed
	}
	switch s.mode {
	case modeArray:
		s.writeArray(chunk)
	case modeSSE:
		s.writeSSE(chunk)
	}
}

// Result returns what was assembled so far. A frame still being read when the
// stream stopped counts only if it is complete.
func (s *StreamAssembler) Result() AssembledStream {
	if !s.flushed {
		s.flushed = true
		if s.mode == modeSSE {
			if len(s.pending) > 0 && !s.dropping {
				s.line(s.pending)
			}
			s.pending = s.pending[:0]
			s.dropping = false
			s.endFrame()
		}
	}
	return AssembledStream{Messages: s.build(), Terminal: s.terminal, Truncated: s.truncated}
}

func (s *StreamAssembler) writeSSE(chunk []byte) {
	for len(chunk) > 0 {
		end := bytes.IndexByte(chunk, '\n')
		if end < 0 {
			s.keepPending(chunk)
			return
		}
		part := chunk[:end]
		chunk = chunk[end+1:]
		if s.dropping {
			s.dropping = false
			s.pending = s.pending[:0]
			continue
		}
		if len(s.pending) > 0 {
			s.keepPending(part)
			if s.dropping {
				s.dropping = false
				s.pending = s.pending[:0]
				continue
			}
			part = s.pending
		}
		s.line(part)
		s.pending = s.pending[:0]
	}
}

func (s *StreamAssembler) keepPending(part []byte) {
	if s.dropping {
		return
	}
	if len(s.pending)+len(part) > maxStoredBody {
		s.dropping = true
		s.truncated = true
		s.pending = s.pending[:0]
		return
	}
	s.pending = append(s.pending, part...)
}

// line handles one complete SSE line.
func (s *StreamAssembler) line(raw []byte) {
	raw = bytes.TrimSuffix(raw, []byte("\r"))
	switch {
	case len(raw) == 0:
		s.endFrame()
	case raw[0] == ':':
	case bytes.HasPrefix(raw, []byte("data:")):
		data := bytes.TrimPrefix(raw[5:], []byte(" "))
		if s.frameLen+len(data)+1 > maxStoredBody {
			s.frame = nil
			s.frameLen = maxStoredBody + 1
			s.truncated = true
			return
		}
		if s.frameLen > 0 {
			s.frame = append(s.frame, '\n')
		}
		s.frame = append(s.frame, data...)
		s.frameLen = len(s.frame) + 1
	case raw[0] == '{':
		// Newline-delimited JSON without the data: prefix.
		s.endFrame()
		s.handle(raw)
	}
	// event:, id:, retry: and anything else carry nothing we keep.
}

func (s *StreamAssembler) endFrame() {
	frame := s.frame
	over := s.frameLen > maxStoredBody
	s.frame = nil
	s.frameLen = 0
	if over || len(frame) == 0 {
		return
	}
	s.handle(frame)
}

func (s *StreamAssembler) writeArray(chunk []byte) {
	for _, c := range chunk {
		if s.inString {
			if s.escaped {
				s.escaped = false
			} else if c == '\\' {
				s.escaped = true
			} else if c == '"' {
				s.inString = false
			}
			s.push(c)
			continue
		}
		switch c {
		case '"':
			s.inString = true
		case '{', '[':
			s.depth++
			if s.depth > maxJSONDepth {
				// Not something a model returns: stop following the stream.
				s.truncated = true
				s.mode = modeIgnore
				s.pending = nil
				return
			}
			if s.depth == 2 && c == '{' && !s.collecting {
				s.collecting = true
				s.dropping = false
				s.pending = s.pending[:0]
			}
		}
		s.push(c)
		if c == '}' || c == ']' {
			s.depth--
			if s.collecting && s.depth == 1 {
				s.collecting = false
				if !s.dropping {
					s.handle(s.pending)
				}
				s.pending = s.pending[:0]
				s.dropping = false
			}
		}
	}
}

// push keeps a byte of the array element being read.
func (s *StreamAssembler) push(c byte) {
	if !s.collecting || s.dropping {
		return
	}
	if len(s.pending) >= maxStoredBody {
		s.dropping = true
		s.truncated = true
		s.pending = s.pending[:0]
		return
	}
	s.pending = append(s.pending, c)
}

// handle processes one complete frame body.
func (s *StreamAssembler) handle(frame []byte) {
	frame = bytes.TrimSpace(frame)
	if len(frame) == 0 {
		return
	}
	if string(frame) == "[DONE]" {
		s.terminal = true
		return
	}
	if !isParsableJSON(frame) {
		return
	}
	root := gjson.ParseBytes(frame)
	if !root.IsObject() {
		return
	}
	switch s.format {
	case FormatOpenAI:
		s.openAI(root)
	case FormatResponses:
		s.responses(root)
	case FormatClaude:
		s.claude(root)
	case FormatGemini:
		s.gemini(root)
	}
}

// message returns the message for a choice or candidate index.
func (s *StreamAssembler) message(index int) *asmMessage {
	m := s.messages[index]
	if m == nil {
		m = newAsmMessage()
		s.messages[index] = m
	}
	return m
}

// take returns the part of text that still fits the message budget, copied so
// that it does not keep the whole frame it was read from alive.
func (s *StreamAssembler) take(text string) string {
	room := maxAssembled - s.used
	if room <= 0 {
		if text != "" {
			s.truncated = true
		}
		return ""
	}
	if len(text) > room {
		end := room
		for end > 0 && !utf8.RuneStart(text[end]) {
			end--
		}
		text = text[:end]
		s.truncated = true
		s.used = maxAssembled
		return strings.Clone(text)
	}
	s.used += len(text)
	return strings.Clone(text)
}

// open starts a block, or returns -1 when the budget is spent.
func (s *StreamAssembler) open(m *asmMessage, block ViewBlock) int {
	if s.used+assembleBlockCost > maxAssembled {
		s.truncated = true
		s.used = maxAssembled
		return -1
	}
	s.used += assembleBlockCost
	block.MediaType = strings.Clone(cleanMediaType(block.MediaType))
	s.used += len(block.MediaType)
	block.Text = s.take(block.Text)
	block.Arguments = s.take(block.Arguments)
	block.Id = s.take(block.Id)
	block.Name = s.take(block.Name)
	return m.add(block)
}

// appendTo adds text to the text or thinking block stored under key, creating
// it when needed.
func (s *StreamAssembler) appendTo(m *asmMessage, key, kind, text string) {
	if text == "" {
		return
	}
	i, ok := m.byKey[key]
	if !ok {
		i = s.open(m, ViewBlock{Type: kind})
		if i < 0 {
			return
		}
		m.byKey[key] = i
	}
	m.texts[i].WriteString(s.take(text))
}

// appendRun adds text to the last block when it has the same type, otherwise it
// starts a new block. It serves formats whose frames carry no block index.
func (s *StreamAssembler) appendRun(m *asmMessage, kind, text string) {
	if text == "" {
		return
	}
	if last := len(m.blocks) - 1; last >= 0 && m.blocks[last].Type == kind {
		m.texts[last].WriteString(s.take(text))
		return
	}
	s.open(m, ViewBlock{Type: kind, Text: text})
}

// toolBlock returns the tool call stored under key, creating it when needed.
func (s *StreamAssembler) toolBlock(m *asmMessage, key string) int {
	i, ok := m.byKey[key]
	if !ok {
		i = s.open(m, ViewBlock{Type: blockToolCall})
		if i < 0 {
			return -1
		}
		m.byKey[key] = i
	}
	return i
}

func (s *StreamAssembler) setTool(m *asmMessage, i int, id, name string) {
	if i < 0 {
		return
	}
	if id != "" && m.blocks[i].Id == "" {
		m.blocks[i].Id = s.take(id)
	}
	if name != "" && m.blocks[i].Name == "" {
		m.blocks[i].Name = s.take(name)
	}
}

func (s *StreamAssembler) addError(m *asmMessage, root gjson.Result) bool {
	block, failed := errorBlock(root)
	if !failed {
		return false
	}
	if text := strings.TrimSpace(block.Text); text == "" || text == "{}" {
		return false
	}
	s.terminal = true
	s.open(m, block)
	return true
}

func (s *StreamAssembler) openAI(root gjson.Result) {
	if s.addError(s.message(0), root) {
		return
	}
	for position, choice := range root.Get("choices").Array() {
		index := position
		if v := choice.Get("index"); v.Type == gjson.Number {
			index = int(v.Int())
		}
		if index < 0 || index >= maxAssembleChoices {
			continue
		}
		m := s.message(index)
		delta := choice.Get("delta")
		s.appendRun(m, blockThinking, delta.Get("reasoning_content").String())
		s.appendRun(m, blockThinking, delta.Get("reasoning").String())
		s.appendRun(m, blockText, delta.Get("content").String())
		s.appendRun(m, blockText, delta.Get("refusal").String())
		s.appendRun(m, blockText, choice.Get("text").String())
		for at, call := range delta.Get("tool_calls").Array() {
			slot := int64(at)
			if v := call.Get("index"); v.Type == gjson.Number {
				slot = v.Int()
			}
			i := s.toolBlock(m, "tool:"+strconvInt(slot))
			s.setTool(m, i, call.Get("id").String(), call.Get("function.name").String())
			if i >= 0 {
				m.args[i].WriteString(s.take(call.Get("function.arguments").String()))
			}
		}
		if legacy := delta.Get("function_call"); legacy.IsObject() {
			i := s.toolBlock(m, "function_call")
			s.setTool(m, i, "", legacy.Get("name").String())
			if i >= 0 {
				m.args[i].WriteString(s.take(legacy.Get("arguments").String()))
			}
		}
		if choice.Get("finish_reason").Type == gjson.String {
			s.terminal = true
		}
	}
}

func (s *StreamAssembler) responses(root gjson.Result) {
	m := s.message(0)
	kind := root.Get("type").String()
	oi := strconvInt(root.Get("output_index").Int())
	switch kind {
	case "response.output_text.delta", "response.refusal.delta":
		s.appendTo(m, "t:"+oi+":"+strconvInt(root.Get("content_index").Int()), blockText, root.Get("delta").String())
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		key := "r:" + oi
		if kind == "response.reasoning_summary_text.delta" {
			summary := root.Get("summary_index").Int()
			if last, seen := m.lastSummary[key]; seen && last != summary {
				if i, ok := m.byKey[key]; ok {
					m.texts[i].WriteString(s.take("\n"))
				}
			}
		}
		s.appendTo(m, key, blockThinking, root.Get("delta").String())
		if _, ok := m.byKey[key]; ok && kind == "response.reasoning_summary_text.delta" {
			m.lastSummary[key] = root.Get("summary_index").Int()
		}
	case "response.output_item.added":
		item := root.Get("item")
		if item.Get("type").String() == "function_call" {
			i := s.toolBlock(m, "f:"+oi)
			id := item.Get("call_id").String()
			if id == "" {
				id = item.Get("id").String()
			}
			s.setTool(m, i, id, item.Get("name").String())
			if i >= 0 {
				m.args[i].WriteString(s.take(item.Get("arguments").String()))
			}
		}
	case "response.function_call_arguments.delta":
		if i := s.toolBlock(m, "f:"+oi); i >= 0 {
			m.args[i].WriteString(s.take(root.Get("delta").String()))
		}
	case "response.completed", "response.failed", "response.incomplete":
		s.terminal = true
		response := root.Get("response")
		if kind == "response.completed" && response.Get("output").IsArray() && len(response.Get("output").Array()) > 0 {
			if msgs := ParseResponseView(FormatResponses, response.Raw); len(msgs) > 0 {
				s.replaced = msgs
			}
		}
		if s.replaced == nil && kind == "response.failed" {
			s.addError(m, response)
		}
	case "error":
		if message := root.Get("message").String(); message != "" {
			s.terminal = true
			s.open(m, ViewBlock{Type: blockError, Text: message})
		} else {
			s.addError(m, root)
		}
	}
}

func (s *StreamAssembler) claude(root gjson.Result) {
	m := s.message(0)
	key := "b:" + strconvInt(root.Get("index").Int())
	switch root.Get("type").String() {
	case "content_block_start":
		block := root.Get("content_block")
		switch block.Get("type").String() {
		case "text":
			s.appendTo(m, key, blockText, block.Get("text").String())
			s.ensureBlock(m, key, blockText)
		case "thinking":
			s.appendTo(m, key, blockThinking, block.Get("thinking").String())
			s.ensureBlock(m, key, blockThinking)
		case "tool_use":
			i := s.toolBlock(m, key)
			s.setTool(m, i, block.Get("id").String(), block.Get("name").String())
		}
	case "content_block_delta":
		delta := root.Get("delta")
		switch delta.Get("type").String() {
		case "text_delta":
			s.appendTo(m, key, blockText, delta.Get("text").String())
		case "thinking_delta":
			s.appendTo(m, key, blockThinking, delta.Get("thinking").String())
		case "input_json_delta":
			if i, ok := m.byKey[key]; ok && m.blocks[i].Type == blockToolCall {
				m.args[i].WriteString(s.take(delta.Get("partial_json").String()))
			}
		}
	case "message_stop":
		s.terminal = true
	case "error":
		s.addError(m, root)
	}
}

// ensureBlock makes sure key maps to a block, so an empty start still holds the place.
func (s *StreamAssembler) ensureBlock(m *asmMessage, key, kind string) {
	if _, ok := m.byKey[key]; ok {
		return
	}
	if i := s.open(m, ViewBlock{Type: kind}); i >= 0 {
		m.byKey[key] = i
	}
}

func (s *StreamAssembler) gemini(root gjson.Result) {
	m := s.message(0)
	if s.addError(m, root) {
		return
	}
	candidate := root.Get("candidates.0")
	for _, block := range geminiParts(candidate.Get("content.parts")) {
		switch block.Type {
		case blockText, blockThinking:
			s.appendRun(m, block.Type, block.Text)
		default:
			s.open(m, block)
		}
	}
	if candidate.Get("finishReason").String() != "" {
		s.terminal = true
	}
}

// build returns the messages in choice order, without empty blocks.
func (s *StreamAssembler) build() []ViewMessage {
	if s.replaced != nil {
		out := make([]ViewMessage, len(s.replaced))
		copy(out, s.replaced)
		return out
	}
	out := []ViewMessage{}
	for index := 0; index < maxAssembleChoices; index++ {
		m := s.messages[index]
		if m == nil {
			continue
		}
		blocks := make([]ViewBlock, 0, len(m.blocks))
		for at, block := range m.blocks {
			block.Text, block.Arguments = m.texts[at].String(), m.args[at].String()
			if block.Type == blockError && block.Text == "" {
				continue
			}
			if (block.Type == blockText || block.Type == blockThinking) && block.Text == "" {
				continue
			}
			if block.Type == blockToolCall && block.Arguments == "" && s.format == FormatClaude {
				block.Arguments = "{}"
			}
			blocks = append(blocks, block)
		}
		if len(blocks) == 0 {
			continue
		}
		out = append(out, ViewMessage{Role: roleAssistant, Source: sourceResponse, Blocks: blocks})
	}
	return out
}

func strconvInt(n int64) string {
	return strconv.FormatInt(n, 10)
}
