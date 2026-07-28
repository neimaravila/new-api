package oaichat

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/stretchr/testify/assert"
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
