package logcontent

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/tidwall/gjson"
)

// Formats of a captured call, as the view reports them.
const (
	FormatOpenAI    = "openai"
	FormatResponses = "openai_responses"
	FormatClaude    = "claude"
	FormatGemini    = "gemini"
	FormatEmbedding = "embedding"
	FormatRerank    = "rerank"
)

const (
	roleSystem    = "system"
	roleDeveloper = "developer"
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"

	sourceRequest  = "request"
	sourceResponse = "response"

	blockText       = "text"
	blockThinking   = "thinking"
	blockToolCall   = "tool_call"
	blockToolResult = "tool_result"
	blockMedia      = "media"
	blockError      = "error"

	reasonNotCaptured = "not_captured"
	reasonUnavailable = "unavailable"
)

// base64MarkerPattern matches the whole text of a marker written by the sanitizer.
var base64MarkerPattern = regexp.MustCompile(`^\[base64 (\S+) (\d+) bytes omitted\]$`)

// ViewBlock is one piece of a message.
type ViewBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Id        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// ViewMessage is one turn of the conversation, from the request or the response.
type ViewMessage struct {
	Role   string      `json:"role"`
	Source string      `json:"source"`
	Blocks []ViewBlock `json:"blocks"`
}

// ViewTool is a tool the request offered; Called counts calls in the response.
type ViewTool struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Schema      common.RawMessage `json:"schema,omitempty"`
	Called      int               `json:"called"`
}

// BodyView is a stored body with how it relates to the original.
type BodyView struct {
	Text          string `json:"text"`
	OriginalBytes int64  `json:"original_bytes"`
	Truncated     bool   `json:"truncated"`
	HeadOnly      bool   `json:"head_only"`
	Undecodable   bool   `json:"undecodable"`
	MediaOmitted  int    `json:"media_omitted"`
}

// AttemptView is one try against a provider. It is only for the admin route.
type AttemptView struct {
	Attempt            int      `json:"attempt"`
	ChannelId          int      `json:"channel_id"`
	ChannelName        string   `json:"channel_name"`
	MultiKeyIndex      int      `json:"multi_key_index"`
	StatusCode         int      `json:"status_code"`
	ErrorMessage       string   `json:"error_message"`
	StartedAtMs        int64    `json:"started_at_ms"`
	DurationMs         int64    `json:"duration_ms"`
	Method             string   `json:"method"`
	Path               string   `json:"path"`
	RequestUnavailable bool     `json:"request_unavailable"`
	Request            BodyView `json:"request"`
	Response           BodyView `json:"response"`
}

// ConversationView holds the messages of the call.
type ConversationView struct {
	Available         bool          `json:"available"`
	ResponseAvailable bool          `json:"response_available"`
	Messages          []ViewMessage `json:"messages"`
}

// ClientView holds the bodies exchanged with the client.
type ClientView struct {
	Request  BodyView `json:"request"`
	Response BodyView `json:"response"`
}

// ContentView is the data of GET .../content/:request_id.
type ContentView struct {
	RequestId      string           `json:"request_id"`
	Available      bool             `json:"available"`
	Reason         string           `json:"reason"`
	Format         string           `json:"format"`
	IsStream       bool             `json:"is_stream"`
	Incomplete     bool             `json:"incomplete"`
	HttpStatus     int              `json:"http_status"`
	Conversation   ConversationView `json:"conversation"`
	Tools          []ViewTool       `json:"tools"`
	ToolsAvailable *int             `json:"tools_available"`
	ToolCalls      *int             `json:"tool_calls"`
	Client         ClientView       `json:"client"`
	Attempts       []AttemptView    `json:"attempts,omitempty"`
}

// viewBuilder collects messages. Blocks of one source message that share a role
// go into one message; a tool result in a user turn starts a tool message.
type viewBuilder struct {
	source string
	msgs   []ViewMessage
	open   bool
}

// split ends the current message, so the next block starts a new one.
func (b *viewBuilder) split() {
	b.open = false
}

func (b *viewBuilder) add(role string, blocks ...ViewBlock) {
	for _, block := range blocks {
		blockRole := role
		if block.Type == blockToolResult {
			blockRole = roleTool
		}
		last := len(b.msgs) - 1
		if b.open && last >= 0 && b.msgs[last].Role == blockRole {
			b.msgs[last].Blocks = append(b.msgs[last].Blocks, block)
			continue
		}
		b.msgs = append(b.msgs, ViewMessage{Role: blockRole, Source: b.source, Blocks: []ViewBlock{block}})
		b.open = true
	}
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case roleSystem:
		return roleSystem
	case roleDeveloper:
		return roleDeveloper
	case roleAssistant, "model":
		return roleAssistant
	case roleTool, "function":
		return roleTool
	default:
		return roleUser
	}
}

// textOrMedia turns a sanitizer marker into a media block and any other
// non-empty string into a text block.
func textOrMedia(text string) []ViewBlock {
	if text == "" {
		return nil
	}
	if match := base64MarkerPattern.FindStringSubmatch(text); match != nil {
		size, _ := strconv.ParseInt(match[2], 10, 64)
		return []ViewBlock{{Type: blockMedia, MediaType: match[1], SizeBytes: size}}
	}
	return []ViewBlock{{Type: blockText, Text: text}}
}

// mediaBlock describes inline data. A marker carries its own type and size; a
// short value that was kept is measured.
func mediaBlock(mediaType, data string) []ViewBlock {
	if match := base64MarkerPattern.FindStringSubmatch(data); match != nil {
		blocks := textOrMedia(data)
		if match[1] == "unknown" && mediaType != "" {
			blocks[0].MediaType = mediaType
		}
		return blocks
	}
	if data == "" {
		return nil
	}
	return []ViewBlock{{Type: blockMedia, MediaType: mediaType, SizeBytes: int64(len(data) * 3 / 4)}}
}

// rawString returns a JSON string as its value and an object or array as raw
// JSON, which is how tool arguments and results arrive.
func rawString(value gjson.Result) string {
	switch value.Type {
	case gjson.String:
		return value.Str
	case gjson.JSON:
		return value.Raw
	default:
		return ""
	}
}

// firstOf returns the first of the named fields that exists, for formats that
// accept both camelCase and snake_case.
func firstOf(node gjson.Result, names ...string) gjson.Result {
	for _, name := range names {
		if value := node.Get(name); value.Exists() {
			return value
		}
	}
	return gjson.Result{}
}

// errorBlock reads the error object every format puts in a failed response.
func errorBlock(root gjson.Result) (ViewBlock, bool) {
	failure := root.Get("error")
	switch {
	case failure.Type == gjson.String && failure.Str != "":
		return ViewBlock{Type: blockError, Text: failure.Str}, true
	case failure.IsObject():
		text := failure.Get("message").String()
		if text == "" {
			text = failure.Raw
		}
		return ViewBlock{Type: blockError, Text: text}, true
	}
	return ViewBlock{}, false
}

// newViewTool returns false for a tool without a name.
func newViewTool(name, description string, schema gjson.Result) (ViewTool, bool) {
	if name == "" {
		return ViewTool{}, false
	}
	tool := ViewTool{Name: name, Description: description}
	if schema.IsObject() {
		tool.Schema = common.RawMessage(schema.Raw)
	}
	return tool, true
}

// parseRoot returns the JSON object of a body, or false for anything else.
func parseRoot(body string) (gjson.Result, bool) {
	if !isParsableJSON([]byte(body)) {
		return gjson.Result{}, false
	}
	root := gjson.Parse(body)
	return root, root.IsObject()
}

// ParseRequestView reads the messages and tools of a request body. It never
// fails: what cannot be read is left out.
func ParseRequestView(format string, body string) (msgs []ViewMessage, tools []ViewTool) {
	root, ok := parseRoot(body)
	if !ok {
		return nil, nil
	}
	builder := &viewBuilder{source: sourceRequest}
	switch format {
	case FormatOpenAI:
		tools = openAIRequest(builder, root)
	case FormatResponses:
		tools = responsesRequest(builder, root)
	case FormatClaude:
		tools = claudeRequest(builder, root)
	case FormatGemini:
		tools = geminiRequest(builder, root)
	case FormatEmbedding, FormatRerank:
		embeddingRequest(builder, format, root)
	}
	return builder.msgs, tools
}

// ParseResponseView reads the messages of a response body, or its error.
func ParseResponseView(format string, body string) []ViewMessage {
	root, ok := parseRoot(body)
	if !ok {
		return nil
	}
	builder := &viewBuilder{source: sourceResponse}
	if block, failed := errorBlock(root); failed {
		builder.add(roleAssistant, block)
		return builder.msgs
	}
	switch format {
	case FormatOpenAI:
		openAIResponse(builder, root)
	case FormatResponses:
		responsesItems(builder, root.Get("output"))
	case FormatClaude:
		builder.add(roleAssistant, contentBlocks(root.Get("content"))...)
	case FormatGemini:
		geminiResponse(builder, root)
	}
	return builder.msgs
}

// decodeStoredBody unpacks a gzip blob into a BodyView. A damaged blob gives an
// undecodable body, not an error for the whole call.
func decodeStoredBody(blob []byte, meta model.BodyMeta) BodyView {
	view := BodyView{
		OriginalBytes: meta.OriginalBytes,
		Truncated:     meta.Truncated,
		HeadOnly:      meta.HeadOnly,
		Undecodable:   meta.Undecodable,
		MediaOmitted:  meta.MediaOmitted,
	}
	if len(blob) == 0 {
		return view
	}
	text, err := decodeContent(blob, "gzip")
	if err != nil {
		view.Undecodable = true
		return view
	}
	view.Text = strings.ToValidUTF8(string(text), "")
	return view
}

// decodeAssembled reads the messages rebuilt from a stream.
func decodeAssembled(blob []byte) ([]ViewMessage, bool) {
	if len(blob) == 0 {
		return nil, false
	}
	text, err := decodeContent(blob, "gzip")
	if err != nil {
		return nil, false
	}
	var msgs []ViewMessage
	if err := common.Unmarshal(text, &msgs); err != nil {
		return nil, false
	}
	for i := range msgs {
		msgs[i].Source = sourceResponse
		if msgs[i].Blocks == nil {
			msgs[i].Blocks = []ViewBlock{}
		}
	}
	return msgs, len(msgs) > 0
}

func parseLogContentMeta(raw string) model.LogContentMeta {
	meta := model.LogContentMeta{MultiKeyIndex: -1}
	if raw != "" {
		_ = common.UnmarshalJsonStr(raw, &meta)
	}
	return meta
}

// BuildContentView assembles the view of a call from its stored attempts.
// Attempt 0 is the exchange with the client. Attempts are only included when
// includeAttempts is set, which only the admin route does.
func BuildContentView(rows []*model.LogContent, includeAttempts bool, channelNames map[int]string) ContentView {
	view := ContentView{
		Conversation: ConversationView{Messages: []ViewMessage{}},
		Tools:        []ViewTool{},
	}
	var client *model.LogContent
	for _, row := range rows {
		if row == nil {
			continue
		}
		if view.RequestId == "" {
			view.RequestId = row.RequestId
		}
		switch {
		case row.Attempt == 0:
			client = row
		case includeAttempts:
			view.Attempts = append(view.Attempts, buildAttemptView(row, channelNames))
		}
	}
	if client == nil {
		if view.RequestId != "" {
			view.Reason = reasonUnavailable
		}
		return view
	}

	meta := parseLogContentMeta(client.Meta)
	view.Format = meta.RelayFormat
	view.IsStream = meta.IsStream
	view.Incomplete = meta.Incomplete
	view.HttpStatus = meta.HttpStatus
	if client.CaptureStatus == reasonNotCaptured {
		view.Reason = reasonNotCaptured
		return view
	}
	view.Available = true
	view.Client = ClientView{
		Request:  decodeStoredBody(client.RequestBody, meta.Request),
		Response: decodeStoredBody(client.ResponseBody, meta.Response),
	}
	buildConversation(&view, client, meta)
	return view
}

func buildConversation(view *ContentView, client *model.LogContent, meta model.LogContentMeta) {
	request := view.Client.Request
	if request.Text != "" && !request.HeadOnly && !request.Undecodable && !meta.RequestUnavailable {
		var msgs []ViewMessage
		msgs, view.Tools = ParseRequestView(view.Format, request.Text)
		view.Conversation.Available = true
		view.Conversation.Messages = append(view.Conversation.Messages, msgs...)
		available := len(view.Tools)
		view.ToolsAvailable = &available
	}
	if view.Tools == nil {
		view.Tools = []ViewTool{}
	}

	var response []ViewMessage
	if assembled, ok := decodeAssembled(client.AssembledBody); ok {
		response = assembled
	} else if body := view.Client.Response; !body.HeadOnly && !body.Undecodable &&
		view.Format != FormatEmbedding && view.Format != FormatRerank {
		response = ParseResponseView(view.Format, body.Text)
	}
	if len(response) == 0 {
		return
	}
	view.Conversation.ResponseAvailable = true
	view.Conversation.Messages = append(view.Conversation.Messages, response...)

	calls := make(map[string]int)
	total := 0
	for _, msg := range response {
		for _, block := range msg.Blocks {
			if block.Type == blockToolCall {
				calls[block.Name]++
				total++
			}
		}
	}
	for i := range view.Tools {
		view.Tools[i].Called = calls[view.Tools[i].Name]
	}
	view.ToolCalls = &total
}

func buildAttemptView(row *model.LogContent, channelNames map[int]string) AttemptView {
	meta := parseLogContentMeta(row.Meta)
	return AttemptView{
		Attempt:            row.Attempt,
		ChannelId:          meta.ChannelId,
		ChannelName:        channelNames[meta.ChannelId],
		MultiKeyIndex:      meta.MultiKeyIndex,
		StatusCode:         meta.HttpStatus,
		ErrorMessage:       meta.ErrorMessage,
		StartedAtMs:        meta.StartedAtMs,
		DurationMs:         meta.DurationMs,
		Method:             meta.Method,
		Path:               meta.Path,
		RequestUnavailable: meta.RequestUnavailable,
		Request:            decodeStoredBody(row.RequestBody, meta.Request),
		Response:           decodeStoredBody(row.ResponseBody, meta.Response),
	}
}
