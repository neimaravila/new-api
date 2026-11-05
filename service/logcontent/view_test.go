package logcontent

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goldenDir = "../../web/src/features/usage-logs/components/log-content/__tests__/fixtures"

var viewFormats = map[string]string{
	"openai":    FormatOpenAI,
	"responses": FormatResponses,
	"claude":    FormatClaude,
	"gemini":    FormatGemini,
}

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(bytes.TrimSpace(data))
}

type rowSpec struct {
	format   string
	request  string
	response string
	meta     func(*model.LogContentMeta)
	assemble []ViewMessage
}

func clientRow(t *testing.T, spec rowSpec) *model.LogContent {
	t.Helper()
	meta := model.LogContentMeta{
		RelayFormat:   spec.format,
		HttpStatus:    200,
		MultiKeyIndex: -1,
		Request:       model.BodyMeta{OriginalBytes: int64(len(spec.request))},
		Response:      model.BodyMeta{OriginalBytes: int64(len(spec.response))},
	}
	if spec.meta != nil {
		spec.meta(&meta)
	}
	metaText, err := model.NewLogContentMeta(meta)
	require.NoError(t, err)
	row := &model.LogContent{
		RequestId:     "req-1",
		Attempt:       0,
		UserId:        1,
		CaptureStatus: "captured",
		Meta:          metaText,
		RequestBody:   compressGzip(t, []byte(spec.request)),
		ResponseBody:  compressGzip(t, []byte(spec.response)),
	}
	if spec.assemble != nil {
		assembled, err := common.Marshal(spec.assemble)
		require.NoError(t, err)
		row.AssembledBody = compressGzip(t, assembled)
	}
	return row
}

func attemptRow(t *testing.T, attempt int, response string) *model.LogContent {
	t.Helper()
	metaText, err := model.NewLogContentMeta(model.LogContentMeta{
		RelayFormat: FormatOpenAI, HttpStatus: 429, ChannelId: 7, MultiKeyIndex: 2,
		Method: "POST", Path: "/v1/chat/completions", ErrorMessage: "rate limited",
	})
	require.NoError(t, err)
	return &model.LogContent{
		RequestId: "req-1", Attempt: attempt, CaptureStatus: "captured", Meta: metaText,
		ResponseBody: compressGzip(t, []byte(response)),
	}
}

func marshalView(t *testing.T, view ContentView) string {
	t.Helper()
	data, err := common.Marshal(view)
	require.NoError(t, err)
	return string(data)
}

func TestBuildViewGolden(t *testing.T) {
	for name, format := range viewFormats {
		t.Run(name, func(t *testing.T) {
			row := clientRow(t, rowSpec{
				format:   format,
				request:  readTestdata(t, name+"_request.json"),
				response: readTestdata(t, name+"_response.json"),
			})
			view := BuildContentView([]*model.LogContent{row}, false, nil)
			path := filepath.Join(goldenDir, "view-"+name+".json")
			if os.Getenv("UPDATE_GOLDEN") != "" {
				var pretty bytes.Buffer
				require.NoError(t, json.Indent(&pretty, []byte(marshalView(t, view)), "", "  "))
				pretty.WriteByte('\n')
				require.NoError(t, os.WriteFile(path, pretty.Bytes(), 0o644))
			}
			golden, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.JSONEq(t, string(golden), marshalView(t, view))
			assert.True(t, view.Available)
			assert.True(t, view.Conversation.ResponseAvailable)
		})
	}
}

func TestViewCountsTools(t *testing.T) {
	request := `{"messages":[{"role":"user","content":"hi"}],"tools":[
		{"type":"function","function":{"name":"a"}},
		{"type":"function","function":{"name":"b"}},
		{"type":"function","function":{"name":"c"}}]}`
	response := `{"choices":[{"message":{"role":"assistant","tool_calls":[
		{"id":"1","function":{"name":"a","arguments":"{}"}},
		{"id":"2","function":{"name":"a","arguments":"{}"}}]}}]}`
	view := BuildContentView([]*model.LogContent{clientRow(t, rowSpec{format: FormatOpenAI, request: request, response: response})}, false, nil)

	require.NotNil(t, view.ToolsAvailable)
	require.NotNil(t, view.ToolCalls)
	assert.Equal(t, 3, *view.ToolsAvailable)
	assert.Equal(t, 2, *view.ToolCalls)
	assert.Equal(t, "a", view.Tools[0].Name)
	assert.Equal(t, 2, view.Tools[0].Called)
	assert.Equal(t, 0, view.Tools[1].Called)
}

func TestViewHeadOnlyGivesNullCounts(t *testing.T) {
	request := `{"messages":[{"role":"user","content":"this is only the start`
	view := BuildContentView([]*model.LogContent{clientRow(t, rowSpec{
		format: FormatOpenAI, request: request, response: `{"choices":[]}`,
		meta: func(m *model.LogContentMeta) { m.Request.HeadOnly = true },
	})}, false, nil)

	assert.False(t, view.Conversation.Available)
	assert.Equal(t, request, view.Client.Request.Text)
	text := marshalView(t, view)
	assert.Contains(t, text, `"tools_available":null`)
	assert.NotContains(t, text, `"tools_available":0`)
	assert.Empty(t, view.Conversation.Messages)
}

func TestViewResponseHeadOnlyKeepsRequest(t *testing.T) {
	view := BuildContentView([]*model.LogContent{clientRow(t, rowSpec{
		format:   FormatOpenAI,
		request:  readTestdata(t, "openai_request.json"),
		response: `{"choices":[{"message":{"role":"assistant","content":"cut`,
		meta:     func(m *model.LogContentMeta) { m.Response.HeadOnly = true },
	})}, false, nil)

	assert.False(t, view.Conversation.ResponseAvailable)
	assert.True(t, view.Conversation.Available)
	require.NotEmpty(t, view.Conversation.Messages)
	for _, msg := range view.Conversation.Messages {
		assert.Equal(t, "request", msg.Source)
	}
	assert.Nil(t, view.ToolCalls)
	require.NotNil(t, view.ToolsAvailable)
}

func TestViewErrorBodies(t *testing.T) {
	cases := map[string]struct{ format, body, want string }{
		"openai":    {FormatOpenAI, `{"error":{"message":"bad key","type":"invalid_request_error","code":"x"}}`, "bad key"},
		"responses": {FormatResponses, `{"id":"r","status":"failed","error":{"code":"server_error","message":"boom"}}`, "boom"},
		"claude":    {FormatClaude, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`, "Overloaded"},
		"gemini":    {FormatGemini, `{"error":{"code":400,"message":"API key not valid","status":"INVALID_ARGUMENT"}}`, "API key not valid"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			msgs := ParseResponseView(tc.format, tc.body)
			require.Len(t, msgs, 1)
			require.Len(t, msgs[0].Blocks, 1)
			assert.Equal(t, ViewBlock{Type: "error", Text: tc.want}, msgs[0].Blocks[0])
			assert.Equal(t, "response", msgs[0].Source)
		})
	}

	t.Run("responses success has null error", func(t *testing.T) {
		msgs := ParseResponseView(FormatResponses, readTestdata(t, "responses_response.json"))
		for _, msg := range msgs {
			for _, block := range msg.Blocks {
				assert.NotEqual(t, "error", block.Type)
			}
		}
	})

	t.Run("claude tool_result is_error", func(t *testing.T) {
		msgs, _ := ParseRequestView(FormatClaude, `{"messages":[{"role":"user","content":[
			{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"failed"},
			{"type":"tool_result","tool_use_id":"t2","content":"fine"},
			{"type":"text","text":"next"}]}]}`)
		require.Len(t, msgs, 2)
		assert.Equal(t, "tool", msgs[0].Role)
		require.Len(t, msgs[0].Blocks, 2)
		assert.True(t, msgs[0].Blocks[0].IsError)
		assert.Equal(t, "t1", msgs[0].Blocks[0].Id)
		assert.False(t, msgs[0].Blocks[1].IsError)
		assert.Equal(t, "user", msgs[1].Role)
	})
}

func TestViewToleratesGarbage(t *testing.T) {
	bodies := []string{"", "not json", "[]", `{"messages":"x"}`, `{"messages":[1,null]}`,
		`{"contents":[1,{"parts":"x"}],"systemInstruction":7,"tools":"x"}`,
		`{"input":[1,null,{"type":"message","content":5}],"tools":[1]}`,
		`{"system":[1,null],"messages":[{"content":[1,{"type":"tool_result","content":{}}]}],"tools":[null]}`,
		`{"candidates":"x","choices":[1,{"message":5}],"content":7,"output":"x","error":5}`}
	for format := range map[string]bool{FormatOpenAI: true, FormatResponses: true, FormatClaude: true, FormatGemini: true, FormatEmbedding: true, FormatRerank: true, "other": true} {
		for _, body := range bodies {
			assert.NotPanics(t, func() {
				msgs, _ := ParseRequestView(format, body)
				for _, msg := range msgs {
					assert.NotEmpty(t, msg.Blocks, "format %s body %q", format, body)
				}
				ParseResponseView(format, body)
			}, "format %s body %q", format, body)
		}
	}
	for _, body := range bodies[:5] {
		msgs, tools := ParseRequestView(FormatOpenAI, body)
		assert.Empty(t, msgs, body)
		assert.Empty(t, tools, body)
		assert.Empty(t, ParseResponseView(FormatOpenAI, body), body)
	}
	view := BuildContentView([]*model.LogContent{clientRow(t, rowSpec{format: FormatOpenAI, request: "not json", response: "[]"})}, false, nil)
	assert.NotNil(t, view.Conversation.Messages)
	assert.Contains(t, marshalView(t, view), `"messages":[]`)
}

func TestViewEmbeddingsAndRerank(t *testing.T) {
	msgs, tools := ParseRequestView(FormatEmbedding, `{"input":"hello"}`)
	require.Len(t, msgs, 1)
	assert.Equal(t, []ViewBlock{{Type: "text", Text: "hello"}}, msgs[0].Blocks)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Empty(t, tools)

	msgs, _ = ParseRequestView(FormatEmbedding, `{"input":["a","b",[1,2]]}`)
	require.Len(t, msgs, 1)
	assert.Equal(t, []ViewBlock{{Type: "text", Text: "a"}, {Type: "text", Text: "b"}}, msgs[0].Blocks)

	msgs, _ = ParseRequestView(FormatRerank, `{"query":"best fruit","documents":["x"]}`)
	require.Len(t, msgs, 1)
	assert.Equal(t, []ViewBlock{{Type: "text", Text: "best fruit"}}, msgs[0].Blocks)

	for _, format := range []string{FormatEmbedding, FormatRerank} {
		view := BuildContentView([]*model.LogContent{clientRow(t, rowSpec{
			format: format, request: `{"input":"hello","query":"hello"}`, response: `{"data":[{"embedding":[0.1]}]}`,
		})}, false, nil)
		assert.True(t, view.Conversation.Available)
		assert.False(t, view.Conversation.ResponseAvailable)
		assert.Nil(t, view.ToolCalls)
		require.Len(t, view.Conversation.Messages, 1)
	}
}

func TestViewEmbeddingAndRerankErrors(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
		want   string
	}{
		"error body":        {400, `{"error":{"message":"bad input"}}`, "bad input"},
		"error body at 200": {200, `{"error":{"message":"odd"}}`, "odd"},
		"plain text at 502": {502, `upstream down`, "upstream down"},
	}
	for _, format := range []string{FormatEmbedding, FormatRerank} {
		for name, tc := range cases {
			t.Run(format+" "+name, func(t *testing.T) {
				view := BuildContentView([]*model.LogContent{clientRow(t, rowSpec{
					format: format, request: `{"input":"x","query":"x"}`, response: tc.body,
					meta: func(m *model.LogContentMeta) { m.HttpStatus = tc.status },
				})}, false, nil)
				assert.True(t, view.Conversation.ResponseAvailable)
				require.Len(t, view.Conversation.Messages, 2)
				assert.Equal(t, ViewBlock{Type: "error", Text: tc.want}, view.Conversation.Messages[1].Blocks[0])
				require.NotNil(t, view.ToolCalls)
				assert.Equal(t, 0, *view.ToolCalls)
			})
		}
	}
}

func TestViewUnparsableRequestIsUnavailable(t *testing.T) {
	for _, request := range []string{"not json", "[]", "", `"x"`} {
		view := BuildContentView([]*model.LogContent{clientRow(t, rowSpec{
			format: FormatOpenAI, request: request, response: `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`,
		})}, false, nil)
		assert.False(t, view.Conversation.Available, request)
		assert.Nil(t, view.ToolsAvailable, request)
		assert.Contains(t, marshalView(t, view), `"tools_available":null`)
		assert.Equal(t, request, view.Client.Request.Text)
		// The response is still readable, so its counts are real.
		assert.True(t, view.Conversation.ResponseAvailable, request)
		require.NotNil(t, view.ToolCalls, request)
	}

	view := BuildContentView([]*model.LogContent{clientRow(t, rowSpec{
		format: FormatOpenAI, request: `{"messages":[]}`, response: "[]",
	})}, false, nil)
	assert.True(t, view.Conversation.Available)
	assert.Nil(t, view.ToolCalls)
	assert.False(t, view.Conversation.ResponseAvailable)
}

func TestViewMediaMarkers(t *testing.T) {
	msgs, _ := ParseRequestView(FormatOpenAI, `{"messages":[{"role":"user","content":[
		{"type":"image_url","image_url":{"url":"[base64 image/png 2048 bytes omitted]"}},
		{"type":"image_url","image_url":{"url":"https://example.com/a.png"}},
		{"type":"text","text":"not a marker [base64 image/png 1 bytes omitted]"}]}]}`)
	require.Len(t, msgs, 1)
	assert.Equal(t, []ViewBlock{
		{Type: "media", MediaType: "image/png", SizeBytes: 2048},
		{Type: "text", Text: "https://example.com/a.png"},
		{Type: "text", Text: "not a marker [base64 image/png 1 bytes omitted]"},
	}, msgs[0].Blocks)

	msgs, _ = ParseRequestView(FormatGemini, `{"contents":[{"role":"user","parts":[
		{"inlineData":{"mimeType":"audio/wav","data":"[base64 unknown 99 bytes omitted]"}},
		{"inline_data":{"mime_type":"image/jpeg","data":"AAAA"}}]}]}`)
	require.Len(t, msgs, 1)
	assert.Equal(t, []ViewBlock{
		{Type: "media", MediaType: "audio/wav", SizeBytes: 99},
		{Type: "media", MediaType: "image/jpeg", SizeBytes: 3},
	}, msgs[0].Blocks)
}

func TestViewLegacyAndVariantShapes(t *testing.T) {
	msgs, tools := ParseRequestView(FormatOpenAI, `{"prompt":["one","two"],"functions":[{"name":"f","description":"d","parameters":{"type":"object"}}]}`)
	require.Len(t, msgs, 2)
	assert.Equal(t, "one", msgs[0].Blocks[0].Text)
	require.Len(t, tools, 1)
	assert.Equal(t, "f", tools[0].Name)
	assert.JSONEq(t, `{"type":"object"}`, string(tools[0].Schema))

	msgs, tools = ParseRequestView(FormatResponses, `{"input":"hi","tools":[{"type":"web_search"}]}`)
	require.Len(t, msgs, 1)
	assert.Equal(t, "user", msgs[0].Role)
	require.Len(t, tools, 1)
	assert.Equal(t, "web_search", tools[0].Name)

	msgs, tools = ParseRequestView(FormatGemini, `{"system_instruction":{"parts":[{"text":"sys"}]},"contents":[{"role":"model","parts":[{"text":"hmm","thought":true}]}],
		"tools":[{"function_declarations":[{"name":"g","parametersJsonSchema":{"type":"object"}}]}]}`)
	require.Len(t, msgs, 2)
	assert.Equal(t, "system", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)
	assert.Equal(t, "thinking", msgs[1].Blocks[0].Type)
	require.Len(t, tools, 1)
	assert.JSONEq(t, `{"type":"object"}`, string(tools[0].Schema))
}

func TestViewUsesAssembledStream(t *testing.T) {
	assembled := []ViewMessage{{Role: "assistant", Source: "response", Blocks: []ViewBlock{
		{Type: "tool_call", Id: "1", Name: "a"},
		{Type: "text", Text: "done"},
	}}}
	view := BuildContentView([]*model.LogContent{clientRow(t, rowSpec{
		format: FormatOpenAI, request: `{"messages":[{"role":"user","content":"hi"}],"tools":[{"function":{"name":"a"}}]}`,
		response: "data: {\"choices\":[]}\n\n", assemble: assembled,
		meta: func(m *model.LogContentMeta) { m.IsStream = true },
	})}, false, nil)

	assert.True(t, view.Conversation.ResponseAvailable)
	require.NotNil(t, view.ToolCalls)
	assert.Equal(t, 1, *view.ToolCalls)
	assert.Equal(t, 1, view.Tools[0].Called)
	assert.Len(t, view.Conversation.Messages, 2)
}

func TestViewOmitsAttemptsForSelf(t *testing.T) {
	const sentinel = "SENTINEL-PROVIDER-ONLY"
	rows := []*model.LogContent{
		clientRow(t, rowSpec{format: FormatOpenAI, request: `{"messages":[{"role":"user","content":"hi"}]}`, response: `{"choices":[]}`}),
		attemptRow(t, 1, `{"error":{"message":"`+sentinel+`"}}`),
	}

	self := marshalView(t, BuildContentView(rows, false, map[int]string{7: "primary"}))
	for _, forbidden := range []string{"attempts", "channel", "multi_key", sentinel, "primary"} {
		assert.NotContains(t, self, forbidden)
	}

	admin := BuildContentView(rows, true, map[int]string{7: "primary"})
	require.Len(t, admin.Attempts, 1)
	attempt := admin.Attempts[0]
	assert.Equal(t, 1, attempt.Attempt)
	assert.Equal(t, "primary", attempt.ChannelName)
	assert.Equal(t, 2, attempt.MultiKeyIndex)
	assert.Equal(t, 429, attempt.StatusCode)
	assert.Contains(t, attempt.Response.Text, sentinel)
	assert.Contains(t, marshalView(t, admin), `"attempts"`)
}

func TestViewCorruptGzipIsUndecodable(t *testing.T) {
	row := clientRow(t, rowSpec{format: FormatOpenAI, request: `{"messages":[{"role":"user","content":"hi"}]}`, response: `{}`})
	row.ResponseBody = []byte("this is not gzip")
	view := BuildContentView([]*model.LogContent{row}, false, nil)

	assert.True(t, view.Available)
	assert.True(t, view.Client.Response.Undecodable)
	assert.Empty(t, view.Client.Response.Text)
	assert.False(t, view.Client.Request.Undecodable)
	assert.True(t, view.Conversation.Available)
	assert.False(t, view.Conversation.ResponseAvailable)
}

func TestViewNotCapturedAndEmpty(t *testing.T) {
	row := &model.LogContent{RequestId: "req-1", CaptureStatus: "not_captured"}
	view := BuildContentView([]*model.LogContent{row}, true, nil)
	assert.False(t, view.Available)
	assert.Equal(t, "not_captured", view.Reason)

	assert.False(t, BuildContentView(nil, true, nil).Available)
	assert.Equal(t, "unavailable", BuildContentView([]*model.LogContent{attemptRow(t, 1, "{}")}, true, nil).Reason)
}
