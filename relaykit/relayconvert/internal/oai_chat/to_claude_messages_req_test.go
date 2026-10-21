package oaichat

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countWebSearchServerTool conta quantas server-tools Anthropic web_search_20250305
// existem no slice de tools (que entram como *dto.ClaudeWebSearchTool).
func countWebSearchServerTool(tools []any) int {
	count := 0
	for _, tool := range tools {
		if ws, ok := tool.(*dto.ClaudeWebSearchTool); ok && ws != nil && ws.Type == "web_search_20250305" {
			count++
		}
	}
	return count
}

func TestOpenAIChatRequestToClaudeMessages_WebSearchToolCallBecomesServerTool(t *testing.T) {
	// Uma tool-call chamada "web_search" (ex.: o que o Grok emite) deve virar a
	// server-tool nativa da Anthropic, não uma tool comum — senão ninguém executa.
	req := dto.GeneralOpenAIRequest{
		MaxTokens: kitutil.GetPointer[uint](1024),
		Tools: []dto.ToolCallRequest{
			{Type: "function", Function: dto.FunctionRequest{Name: "web_search"}},
			{Type: "function", Function: dto.FunctionRequest{Name: "get_weather",
				Parameters: map[string]any{"type": "object", "properties": map[string]any{}, "required": []any{}}}},
		},
	}

	out, err := OpenAIChatRequestToClaudeMessages(context.Background(), &convmeta.Values{}, req)
	assert.NoError(t, err)
	if assert.NotNil(t, out) {
		tools := out.GetTools()
		// exatamente 1 server-tool web_search
		assert.Equal(t, 1, countWebSearchServerTool(tools), "web_search tool deve virar 1 server-tool")
		// a tool comum (get_weather) segue presente como *dto.Tool
		foundWeather := false
		for _, tool := range tools {
			if tt, ok := tool.(*dto.Tool); ok && tt != nil && tt.Name == "get_weather" {
				foundWeather = true
			}
		}
		assert.True(t, foundWeather, "tool comum get_weather deve ser preservada")
	}
}

func TestOpenAIChatRequestToClaudeMessages_WebSearchOptionsStillWorks(t *testing.T) {
	// Regression: o caminho web_search_options (formato OpenAI) continua gerando
	// a server-tool, igual ao comportamento anterior à extração do helper.
	req := dto.GeneralOpenAIRequest{
		MaxTokens:        kitutil.GetPointer[uint](1024),
		WebSearchOptions: &dto.WebSearchOptions{SearchContextSize: "high"},
	}

	out, err := OpenAIChatRequestToClaudeMessages(context.Background(), &convmeta.Values{}, req)
	assert.NoError(t, err)
	if assert.NotNil(t, out) {
		tools := out.GetTools()
		assert.Equal(t, 1, countWebSearchServerTool(tools), "web_search_options deve gerar 1 server-tool")
	}
}

func TestOpenAIChatRequestToClaudeMessages_WebSearchToolAndOptionsDedup(t *testing.T) {
	// Idempotência: tool "web_search" + web_search_options juntos não devem
	// emitir a server-tool duas vezes.
	req := dto.GeneralOpenAIRequest{
		MaxTokens: kitutil.GetPointer[uint](1024),
		Tools: []dto.ToolCallRequest{
			{Type: "function", Function: dto.FunctionRequest{Name: "web_search"}},
		},
		WebSearchOptions: &dto.WebSearchOptions{SearchContextSize: "low"},
	}

	out, err := OpenAIChatRequestToClaudeMessages(context.Background(), &convmeta.Values{}, req)
	assert.NoError(t, err)
	if assert.NotNil(t, out) {
		assert.Equal(t, 1, countWebSearchServerTool(out.GetTools()),
			"tool + options juntos -> apenas 1 server-tool (sem duplicação)")
	}
}

func TestOpenAIChatRequestToClaudeMessages_NoWebSearchByDefault(t *testing.T) {
	// Garantia: tools comuns sem web_search não injetam server-tool nenhuma.
	req := dto.GeneralOpenAIRequest{
		MaxTokens: kitutil.GetPointer[uint](1024),
		Tools: []dto.ToolCallRequest{
			{Type: "function", Function: dto.FunctionRequest{Name: "calc",
				Parameters: map[string]any{"type": "object", "properties": map[string]any{}, "required": []any{}}}},
		},
	}

	out, err := OpenAIChatRequestToClaudeMessages(context.Background(), &convmeta.Values{}, req)
	assert.NoError(t, err)
	if assert.NotNil(t, out) {
		assert.Equal(t, 0, countWebSearchServerTool(out.GetTools()), "sem web_search -> 0 server-tools")
	}
}

func TestOpenAIChatRequestToClaudeMessagesNormalizesToolInputSchema(t *testing.T) {
	tests := []struct {
		name       string
		parameters any
		wantSchema map[string]any
	}{
		{
			name:       "omitted parameters",
			parameters: nil,
			wantSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			name: "missing type and properties",
			parameters: map[string]any{
				"additionalProperties": false,
			},
			wantSchema: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
		{
			name: "non-string type",
			parameters: map[string]any{
				"type":       123,
				"properties": map[string]any{},
			},
			wantSchema: map[string]any{
				"type":       123,
				"properties": map[string]any{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxTokens := uint(1024)
			got, err := OpenAIChatRequestToClaudeMessages(context.Background(), nil, dto.GeneralOpenAIRequest{
				Model:     "claude-test",
				MaxTokens: &maxTokens,
				Messages: []dto.Message{
					{Role: "user", Content: "Call the tool."},
				},
				Tools: []dto.ToolCallRequest{
					{
						Type: "function",
						Function: dto.FunctionRequest{
							Name:        "get_current_time",
							Description: "Get the current time",
							Parameters:  tt.parameters,
						},
					},
				},
			})

			require.NoError(t, err)
			tools, ok := got.Tools.([]any)
			require.True(t, ok)
			require.Len(t, tools, 1)
			tool, ok := tools[0].(*dto.Tool)
			require.True(t, ok)
			assert.Equal(t, "get_current_time", tool.Name)
			assert.Equal(t, tt.wantSchema, tool.InputSchema)
		})
	}
}

func TestOpenAIChatRequestToClaudeMessagesMergesAdjacentAssistantToolCalls(t *testing.T) {
	toolCall := func(id string) []dto.ToolCallRequest {
		return []dto.ToolCallRequest{{
			ID:       id,
			Type:     "function",
			Function: dto.FunctionRequest{Name: "lookup_" + id, Arguments: `{"q":"` + id + `"}`},
		}}
	}
	assistant := func(content any, toolCalls []dto.ToolCallRequest) dto.Message {
		message := dto.Message{Role: "assistant", Content: content}
		if toolCalls != nil {
			message.SetToolCalls(toolCalls)
		}
		return message
	}

	tests := []struct {
		name         string
		messages     []dto.Message
		wantTexts    []string
		wantToolUses []string
	}{
		{
			name: "both assistants have tool calls",
			messages: []dto.Message{
				{Role: "user", Content: "Look up a and b."},
				assistant("", toolCall("call_a")),
				assistant("", toolCall("call_b")),
				{Role: "tool", ToolCallId: "call_a", Content: "result a"},
				{Role: "tool", ToolCallId: "call_b", Content: "result b"},
			},
			wantTexts:    []string{"..."},
			wantToolUses: []string{"call_a", "call_b"},
		},
		{
			name: "tool calls followed by plain text",
			messages: []dto.Message{
				{Role: "user", Content: "Look up a."},
				assistant(nil, toolCall("call_a")),
				assistant("Checking a.", nil),
				{Role: "tool", ToolCallId: "call_a", Content: "result a"},
			},
			wantTexts:    []string{"Checking a."},
			wantToolUses: []string{"call_a"},
		},
		{
			name: "literal ellipsis with tool calls is preserved",
			messages: []dto.Message{
				{Role: "user", Content: "Look up a and b."},
				assistant("...", toolCall("call_a")),
				assistant("Checking b.", toolCall("call_b")),
				{Role: "tool", ToolCallId: "call_a", Content: "result a"},
				{Role: "tool", ToolCallId: "call_b", Content: "result b"},
			},
			wantTexts:    []string{"... Checking b."},
			wantToolUses: []string{"call_a", "call_b"},
		},
		{
			name: "literal ellipsis replacing a placeholder survives the next merge",
			messages: []dto.Message{
				{Role: "user", Content: "Look up a."},
				assistant(nil, toolCall("call_a")),
				assistant("...", nil),
				assistant("Checking a.", nil),
				{Role: "tool", ToolCallId: "call_a", Content: "result a"},
			},
			wantTexts:    []string{"... Checking a."},
			wantToolUses: []string{"call_a"},
		},
		{
			name: "literal ellipsis before tool calls survives repeated merges",
			messages: []dto.Message{
				{Role: "user", Content: "Look up a."},
				assistant("...", nil),
				assistant("", toolCall("call_a")),
				assistant("Checking a.", nil),
				{Role: "tool", ToolCallId: "call_a", Content: "result a"},
			},
			wantTexts:    []string{"... Checking a."},
			wantToolUses: []string{"call_a"},
		},
		{
			name: "three tool-only assistants keep one placeholder",
			messages: []dto.Message{
				{Role: "user", Content: "Look up a, b and c."},
				assistant(nil, toolCall("call_a")),
				assistant("", toolCall("call_b")),
				assistant("", toolCall("call_c")),
				{Role: "tool", ToolCallId: "call_a", Content: "result a"},
				{Role: "tool", ToolCallId: "call_b", Content: "result b"},
				{Role: "tool", ToolCallId: "call_c", Content: "result c"},
			},
			wantTexts:    []string{"..."},
			wantToolUses: []string{"call_a", "call_b", "call_c"},
		},
		{
			name: "both assistants are plain text",
			messages: []dto.Message{
				{Role: "user", Content: "Hello."},
				assistant("First.", nil),
				assistant("Second.", nil),
			},
			wantTexts: []string{"First. Second."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxTokens := uint(1024)
			got, err := OpenAIChatRequestToClaudeMessages(context.Background(), nil, dto.GeneralOpenAIRequest{
				Model:     "claude-test",
				MaxTokens: &maxTokens,
				Messages:  tt.messages,
			})
			require.NoError(t, err)

			var wantToolResults []dto.ClaudeMediaMessage
			for _, message := range tt.messages {
				if message.Role == "tool" {
					wantToolResults = append(wantToolResults, dto.ClaudeMediaMessage{
						Type:      "tool_result",
						ToolUseId: message.ToolCallId,
						Content:   message.Content,
					})
				}
			}
			if len(wantToolResults) > 0 {
				require.Len(t, got.Messages, 3)
				assert.Equal(t, "user", got.Messages[2].Role)
				assert.Equal(t, wantToolResults, got.Messages[2].Content)
			} else {
				require.Len(t, got.Messages, 2)
			}
			assert.Equal(t, "assistant", got.Messages[1].Role)

			var texts, toolUses []string
			switch content := got.Messages[1].Content.(type) {
			case string:
				texts = append(texts, content)
			case []dto.ClaudeMediaMessage:
				for _, block := range content {
					switch block.Type {
					case "text":
						texts = append(texts, block.GetText())
					case "tool_use":
						toolUses = append(toolUses, block.Id)
						assert.Equal(t, "lookup_"+block.Id, block.Name)
						assert.Equal(t, map[string]any{"q": block.Id}, block.Input)
					}
				}
			default:
				require.Failf(t, "unexpected assistant content type", "%T", content)
			}
			assert.Equal(t, tt.wantTexts, texts)
			assert.Equal(t, tt.wantToolUses, toolUses)
		})
	}
}
