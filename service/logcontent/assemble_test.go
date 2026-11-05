package logcontent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var streamFixtures = []struct{ name, format, stream, response string }{
	{"openai", FormatOpenAI, "openai_stream.sse", "openai_response.json"},
	{"responses", FormatResponses, "responses_stream.sse", "responses_response.json"},
	{"claude", FormatClaude, "claude_stream.sse", "claude_response.json"},
	{"gemini", FormatGemini, "gemini_stream.sse", "gemini_response.json"},
}

func readStream(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

func assembleChunks(format string, data []byte, size int) AssembledStream {
	a := NewStreamAssembler(format)
	for len(data) > 0 {
		n := min(size, len(data))
		a.Write(data[:n])
		data = data[n:]
	}
	return a.Result()
}

func TestAssembleGolden(t *testing.T) {
	for _, fx := range streamFixtures {
		t.Run(fx.name, func(t *testing.T) {
			got := assembleChunks(fx.format, readStream(t, fx.stream), 1<<20)
			want := ParseResponseView(fx.format, readTestdata(t, fx.response))
			require.NotEmpty(t, want)
			assert.Equal(t, want, got.Messages)
			assert.True(t, got.Terminal)
			assert.False(t, got.Truncated)
		})
	}
}

// A stream whose text is only in the deltas, so the golden test cannot pass by
// copying the final object.
func TestAssembleResponsesCompletedReplacesDeltas(t *testing.T) {
	stream := `data: {"type":"response.output_text.delta","output_index":0,"delta":"partial"}` + "\n\n" +
		`data: {"type":"response.completed","response":{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"final"}]}]}}` + "\n\n"
	got := assembleChunks(FormatResponses, []byte(stream), 1<<20)
	assert.True(t, got.Terminal)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, []ViewBlock{{Type: blockText, Text: "final"}}, got.Messages[0].Blocks)
}

func TestAssembleResponsesFailed(t *testing.T) {
	stream := `data: {"type":"response.output_text.delta","delta":"so far"}` + "\n\n" +
		`data: {"type":"response.failed","response":{"error":{"message":"boom"}}}` + "\n\n"
	got := assembleChunks(FormatResponses, []byte(stream), 1<<20)
	assert.True(t, got.Terminal)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, []ViewBlock{{Type: blockText, Text: "so far"}, {Type: blockError, Text: "boom"}}, got.Messages[0].Blocks)
}

func TestAssembleAnyChunkBoundary(t *testing.T) {
	crlf := func(data []byte) []byte { return []byte(strings.ReplaceAll(string(data), "\n", "\r\n")) }
	multibyte := []byte(`data: {"choices":[{"index":0,"delta":{"content":"é語😀 ok"}}]}` + "\r\n\r\n" +
		`data: {"choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}]}` + "\r\n\r\n")

	cases := map[string][]byte{"multibyte": multibyte}
	formats := map[string]string{"multibyte": FormatOpenAI}
	for _, fx := range streamFixtures {
		cases[fx.name] = readStream(t, fx.stream)
		cases[fx.name+" crlf"] = crlf(readStream(t, fx.stream))
		formats[fx.name] = fx.format
		formats[fx.name+" crlf"] = fx.format
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			want := assembleChunks(formats[name], data, 1<<20)
			require.NotEmpty(t, want.Messages)
			assert.True(t, want.Terminal)
			for _, size := range []int{1, 7, 4096} {
				assert.Equal(t, want, assembleChunks(formats[name], data, size), "chunk size %d", size)
			}
		})
	}
	got := assembleChunks(FormatOpenAI, multibyte, 1)
	assert.Equal(t, "é語😀 ok!", got.Messages[0].Blocks[0].Text)
}

func TestAssembleGeminiArrayBody(t *testing.T) {
	body := "[{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hel\"}]}}]},\r\n" +
		"{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"lo } ] \\\" é\"}]},\"finishReason\":\"STOP\"}]}]"
	want := []ViewMessage{{Role: roleAssistant, Source: sourceResponse, Blocks: []ViewBlock{{Type: blockText, Text: "Hello } ] \" é"}}}}
	for _, size := range []int{1, 5, 4096} {
		got := assembleChunks(FormatGemini, []byte("\n "+body), size)
		assert.Equal(t, want, got.Messages, "chunk size %d", size)
		assert.True(t, got.Terminal)
	}
}

func TestAssembleIgnoresKeepAlive(t *testing.T) {
	for _, fx := range streamFixtures {
		t.Run(fx.name, func(t *testing.T) {
			noise := ": PING\n\nevent: ping\n\nevent: ping\ndata: {\"type\":\"ping\"}\n\n"
			got := assembleChunks(fx.format, []byte(noise), 3)
			assert.Empty(t, got.Messages)
			assert.False(t, got.Terminal)

			with := assembleChunks(fx.format, append([]byte(noise), readStream(t, fx.stream)...), 1<<20)
			assert.Equal(t, assembleChunks(fx.format, readStream(t, fx.stream), 1<<20), with)
		})
	}
}

func TestAssembleInterruptedStream(t *testing.T) {
	data := readStream(t, "openai_stream.sse")
	cut := strings.Index(string(data), `"content":"It is`) + 20
	got := assembleChunks(FormatOpenAI, data[:cut], 5)
	assert.False(t, got.Terminal)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, "The tool said 21.", got.Messages[0].Blocks[0].Text)
	assert.Equal(t, "A cat on a sofa. ", got.Messages[0].Blocks[1].Text)

	// A last frame with no blank line after it still counts when it is whole.
	done := assembleChunks(FormatOpenAI, []byte("data: [DONE]"), 4)
	assert.True(t, done.Terminal)
}

func TestAssembleMessageLimit(t *testing.T) {
	for _, fx := range streamFixtures {
		t.Run(fx.name, func(t *testing.T) {
			delta := strings.Repeat("x", 30000)
			frames := map[string]string{
				FormatOpenAI:    `data: {"choices":[{"index":0,"delta":{"content":"` + delta + `"}}]}` + "\n\n",
				FormatResponses: `data: {"type":"response.output_text.delta","delta":"` + delta + `"}` + "\n\n",
				FormatClaude:    `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"` + delta + `"}}` + "\n\n",
				FormatGemini:    `data: {"candidates":[{"content":{"parts":[{"text":"` + delta + `"}]}}]}` + "\n\n",
			}
			a := NewStreamAssembler(fx.format)
			for i := 0; i < 100; i++ { // 3 MB
				a.Write([]byte(frames[fx.format]))
			}
			got := a.Result()
			assert.True(t, got.Truncated)
			total := 0
			for _, msg := range got.Messages {
				for _, block := range msg.Blocks {
					total += len(block.Text) + len(block.Arguments)
				}
			}
			assert.LessOrEqual(t, total, 1<<20)
			assert.Greater(t, total, 1<<19)
		})
	}
}

func TestAssembleOversizedFrameIsIgnored(t *testing.T) {
	big := `data: {"choices":[{"index":0,"delta":{"content":"` + strings.Repeat("y", 1<<20) + `"}}]}` + "\n\n"
	small := `data: {"choices":[{"index":0,"delta":{"content":"after"},"finish_reason":"stop"}]}` + "\n\n"
	for _, size := range []int{1 << 22, 4096} {
		got := assembleChunks(FormatOpenAI, []byte(big+small), size)
		assert.True(t, got.Truncated, "chunk size %d", size)
		assert.True(t, got.Terminal)
		require.Len(t, got.Messages, 1)
		assert.Equal(t, "after", got.Messages[0].Blocks[0].Text)
	}
	// A frame split over many data lines counts as one frame.
	var lines strings.Builder
	lines.WriteString("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"\n")
	for i := 0; i < 12; i++ {
		lines.WriteString("data: " + strings.Repeat("z", 100000) + "\n")
	}
	lines.WriteString("data: \"}}]}\n\n")
	multi := assembleChunks(FormatOpenAI, []byte(lines.String()+small), 1<<22)
	assert.True(t, multi.Truncated)
	require.Len(t, multi.Messages, 1)
	assert.Equal(t, "after", multi.Messages[0].Blocks[0].Text)

	// The same limit applies to an element of a Gemini array.
	array := `[{"candidates":[{"content":{"parts":[{"text":"` + strings.Repeat("y", 1<<20) + `"}]}}]},{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}]`
	got := assembleChunks(FormatGemini, []byte(array), 4096)
	assert.True(t, got.Truncated)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, "ok", got.Messages[0].Blocks[0].Text)
}

func TestAssembleToolCallAcrossFrames(t *testing.T) {
	args := []string{`{"ci`, `ty":"Li`, `sbon",`, `"unit"`, `:"c"}`}
	want := []ViewBlock{{Type: blockToolCall, Id: "call_1", Name: "weather", Arguments: `{"city":"Lisbon","unit":"c"}`}}

	build := func(frame func(i int, part string) string) string {
		var sb strings.Builder
		for i, part := range args {
			sb.WriteString("data: " + frame(i, part) + "\n\n")
		}
		return sb.String()
	}
	quote := func(s string) string {
		raw, _ := common.Marshal(s)
		return string(raw)
	}

	cases := map[string]struct{ format, stream string }{
		FormatOpenAI: {FormatOpenAI, build(func(i int, part string) string {
			head := ""
			if i == 0 {
				head = `"id":"call_1","type":"function","function":{"name":"weather","arguments":` + quote(part) + `}`
			} else {
				head = `"function":{"arguments":` + quote(part) + `}`
			}
			return `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,` + head + `}]}}]}`
		})},
		FormatResponses: {FormatResponses, `data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","call_id":"call_1","name":"weather","arguments":""}}` + "\n\n" +
			build(func(i int, part string) string {
				return `{"type":"response.function_call_arguments.delta","output_index":0,"delta":` + quote(part) + `}`
			})},
		FormatClaude: {FormatClaude, `data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_1","name":"weather","input":{}}}` + "\n\n" +
			build(func(i int, part string) string {
				return `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":` + quote(part) + `}}`
			})},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, size := range []int{1, 1 << 20} {
				got := assembleChunks(tc.format, []byte(tc.stream), size)
				require.Len(t, got.Messages, 1)
				assert.Equal(t, want, got.Messages[0].Blocks)
			}
		})
	}

	t.Run("two calls and text keep their order", func(t *testing.T) {
		stream := `data: {"choices":[{"index":0,"delta":{"content":"Let me check."}}]}` + "\n\n" +
			`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"a","function":{"name":"f","arguments":"{}"}},{"index":1,"id":"b","function":{"name":"g","arguments":"{"}}]}}]}` + "\n\n" +
			`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"}"}}]},"finish_reason":"tool_calls"}]}` + "\n\n"
		got := assembleChunks(FormatOpenAI, []byte(stream), 1<<20)
		require.Len(t, got.Messages, 1)
		assert.Equal(t, []ViewBlock{
			{Type: blockText, Text: "Let me check."},
			{Type: blockToolCall, Id: "a", Name: "f", Arguments: "{}"},
			{Type: blockToolCall, Id: "b", Name: "g", Arguments: "{}"},
		}, got.Messages[0].Blocks)
	})
}

func TestAssembleOpenAIChoices(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&sb, "data: {\"choices\":[{\"index\":%d,\"delta\":{\"content\":\"c%d\"}}]}\n\n", i, i)
	}
	got := assembleChunks(FormatOpenAI, []byte(sb.String()), 64)
	require.Len(t, got.Messages, 8)
	assert.Equal(t, "c0", got.Messages[0].Blocks[0].Text)
	assert.Equal(t, "c7", got.Messages[7].Blocks[0].Text)
}

func TestAssembleStreamError(t *testing.T) {
	stream := `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}` + "\n\n" +
		`data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}` + "\n\n"
	got := assembleChunks(FormatClaude, []byte(stream), 9)
	assert.True(t, got.Terminal)
	assert.Equal(t, []ViewBlock{{Type: blockText, Text: "hi"}, {Type: blockError, Text: "Overloaded"}}, got.Messages[0].Blocks)

	gem := assembleChunks(FormatGemini, []byte(`data: {"error":{"code":429,"message":"quota"}}`+"\n\n"), 1<<20)
	assert.True(t, gem.Terminal)
	assert.Equal(t, []ViewBlock{{Type: blockError, Text: "quota"}}, gem.Messages[0].Blocks)
}

func TestAssembleMalformedFrame(t *testing.T) {
	stream := `data: {"choices":[{"index":0,"delta":{"content":"one "}}]}` + "\n\n" +
		"data: {nope\n\n" +
		"data: \n\n" +
		"data: [1,2\n\n" +
		`data: {"choices":[{"index":0,"delta":{"content":"two"},"finish_reason":"stop"}]}` + "\n\n"
	got := assembleChunks(FormatOpenAI, []byte(stream), 11)
	assert.True(t, got.Terminal)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, "one two", got.Messages[0].Blocks[0].Text)
}

func TestAssembleDeeplyNestedFrame(t *testing.T) {
	stream := "data: " + strings.Repeat("[", 1<<19) + "\n\n" + `data: {"choices":[{"delta":{"content":"ok"}}]}` + "\n\n"
	got := assembleChunks(FormatOpenAI, []byte(stream), 1<<20)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, "ok", got.Messages[0].Blocks[0].Text)
}

func TestSanitizeAssembledMessages(t *testing.T) {
	secrets := NewSecrets(testBareKey)
	msgs := []ViewMessage{{Role: roleAssistant, Source: sourceResponse, Blocks: []ViewBlock{
		{Type: blockText, Text: "key " + testBareKey + " here"},
		{Type: blockToolCall, Id: "c", Name: "f", Arguments: `{"k":"` + testBareKey + `"}`},
		{Type: blockText, Text: "see " + base64Text(2000)},
	}}}
	got, trimmed := SanitizeAssembledMessages(msgs, secrets)
	assert.False(t, trimmed)
	assert.Equal(t, "key [REDACTED] here", got[0].Blocks[0].Text)
	assert.Equal(t, `{"k":"[REDACTED]"}`, got[0].Blocks[1].Arguments)
	assert.Equal(t, "see [base64 unknown 1500 bytes omitted]", got[0].Blocks[2].Text)
	assert.Contains(t, msgs[0].Blocks[0].Text, testBareKey, "input is not modified")
}

func TestSanitizeAssembledMessagesStaysValidAndWithinLimit(t *testing.T) {
	secrets := NewSecrets(testBareKey)
	// Text full of characters JSON escapes, a key across the cut, and a tool call after it.
	text := strings.Repeat("\"語\\\n", 100000)
	msgs := []ViewMessage{{Role: roleAssistant, Source: sourceResponse, Blocks: []ViewBlock{
		{Type: blockText, Text: text[:len(text)/2]},
		{Type: blockThinking, Text: prose(maxAssembled-len(text)/2-6) + testBareKey + prose(100)},
		{Type: blockToolCall, Id: "c", Name: "f", Arguments: `{"a":1}`},
	}}}
	got, trimmed := SanitizeAssembledMessages(msgs, secrets)
	assert.True(t, trimmed)

	total := 0
	for _, block := range got[0].Blocks {
		total += len(block.Text) + len(block.Arguments) + len(block.Id) + len(block.Name)
		assert.NotContains(t, block.Text, testBareKey[:6])
	}
	assert.LessOrEqual(t, total, maxAssembled)

	raw, err := common.Marshal(got)
	require.NoError(t, err)
	var back []ViewMessage
	require.NoError(t, common.Unmarshal(raw, &back))
	assert.Equal(t, got, back)
}
