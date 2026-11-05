package logcontent

import (
	"strings"

	"github.com/tidwall/gjson"
)

// responsesItems reads Responses input or output items.
func responsesItems(b *viewBuilder, items gjson.Result) {
	for _, item := range items.Array() {
		if !item.IsObject() {
			continue
		}
		kind := item.Get("type").String()
		if kind == "" && item.Get("role").Exists() {
			kind = "message"
		}
		// Items of a response are one assistant turn, so only a request keeps
		// its message items apart.
		if kind == "message" && b.source == sourceRequest {
			b.split()
		}
		switch kind {
		case "message":
			b.add(normalizeRole(item.Get("role").String()), contentBlocks(item.Get("content"))...)
		case "function_call":
			id := item.Get("call_id").String()
			if id == "" {
				id = item.Get("id").String()
			}
			b.add(roleAssistant, ViewBlock{Type: blockToolCall, Id: id, Name: item.Get("name").String(), Arguments: rawString(item.Get("arguments"))})
		case "function_call_output":
			b.add(roleTool, ViewBlock{
				Type: blockToolResult,
				Id:   item.Get("call_id").String(),
				Text: blocksText(contentBlocks(item.Get("output"))),
			})
		case "reasoning":
			var lines []string
			for _, field := range []string{"summary", "content"} {
				for _, part := range item.Get(field).Array() {
					if text := part.Get("text").String(); text != "" {
						lines = append(lines, text)
					}
				}
			}
			b.add(roleAssistant, thinkingBlock(strings.Join(lines, "\n"))...)
		case "image_generation_call":
			b.add(roleAssistant, mediaBlock("image/png", item.Get("result").String())...)
		}
	}
}

func responsesRequest(b *viewBuilder, root gjson.Result) []ViewTool {
	if instructions := root.Get("instructions"); instructions.Type == gjson.String {
		b.add(roleSystem, textOrMedia(instructions.Str)...)
	}
	input := root.Get("input")
	if input.Type == gjson.String {
		b.split()
		b.add(roleUser, textOrMedia(input.Str)...)
	}
	responsesItems(b, input)

	var tools []ViewTool
	for _, tool := range root.Get("tools").Array() {
		name := tool.Get("name").String()
		if name == "" {
			name = tool.Get("type").String()
		}
		if view, ok := newViewTool(name, tool.Get("description").String(), tool.Get("parameters")); ok {
			tools = append(tools, view)
		}
	}
	return tools
}
