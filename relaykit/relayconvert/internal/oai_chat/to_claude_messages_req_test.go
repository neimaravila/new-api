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
		for _, tool := range tools {
			if ws, ok := tool.(*dto.ClaudeWebSearchTool); ok && ws != nil {
				assert.Equal(t, webSearchMaxUsesHigh, ws.MaxUses, "high -> MaxUses high")
			}
		}
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
