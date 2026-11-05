package logcontent

import (
	"strings"

	"github.com/tidwall/gjson"
)

// contentBlocks reads message content that is a string or a list of parts. The
// part shapes of OpenAI, Responses and Claude overlap, so one reader serves all.
func contentBlocks(content gjson.Result) []ViewBlock {
	switch {
	case content.Type == gjson.String:
		return textOrMedia(content.Str)
	case content.IsArray():
		var blocks []ViewBlock
		for _, part := range content.Array() {
			if part.Type == gjson.String {
				blocks = append(blocks, textOrMedia(part.Str)...)
			} else if part.IsObject() {
				blocks = append(blocks, partBlocks(part)...)
			}
		}
		return blocks
	}
	return nil
}

func partBlocks(part gjson.Result) []ViewBlock {
	switch part.Get("type").String() {
	case "text", "input_text", "output_text":
		return textOrMedia(part.Get("text").String())
	case "refusal":
		return textOrMedia(part.Get("refusal").String())
	case "image_url", "input_image":
		url := part.Get("image_url")
		if url.IsObject() {
			url = url.Get("url")
		}
		return textOrMedia(url.String())
	case "input_audio":
		return mediaBlock("audio/"+part.Get("input_audio.format").String(), part.Get("input_audio.data").String())
	case "file", "input_file":
		file := part
		if part.Get("file").IsObject() {
			file = part.Get("file")
		}
		if blocks := mediaBlock("", file.Get("file_data").String()); blocks != nil {
			return blocks
		}
		return textOrMedia(file.Get("filename").String())
	case "image", "document":
		return claudeSourceBlocks(part.Get("source"))
	case "thinking":
		return thinkingBlock(part.Get("thinking").String())
	case "tool_use":
		return []ViewBlock{{Type: blockToolCall, Id: part.Get("id").String(), Name: part.Get("name").String(), Arguments: rawString(part.Get("input"))}}
	case "tool_result":
		return []ViewBlock{{
			Type:    blockToolResult,
			Id:      part.Get("tool_use_id").String(),
			Text:    blocksText(contentBlocks(part.Get("content"))),
			IsError: part.Get("is_error").Bool(),
		}}
	}
	return textOrMedia(part.Get("text").String())
}

func thinkingBlock(text string) []ViewBlock {
	if text == "" {
		return nil
	}
	return []ViewBlock{{Type: blockThinking, Text: text}}
}

// blocksText flattens blocks into the text a tool result carries.
func blocksText(blocks []ViewBlock) string {
	lines := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == blockMedia {
			lines = append(lines, "["+block.MediaType+"]")
		} else if block.Text != "" {
			lines = append(lines, block.Text)
		}
	}
	return strings.Join(lines, "\n")
}

// openAIMessage reads one chat message, request or response.
func openAIMessage(b *viewBuilder, message gjson.Result) {
	if !message.IsObject() {
		return
	}
	b.split()
	role := normalizeRole(message.Get("role").String())
	if role == roleTool {
		b.add(roleTool, ViewBlock{
			Type: blockToolResult,
			Id:   message.Get("tool_call_id").String(),
			Name: message.Get("name").String(),
			Text: blocksText(contentBlocks(message.Get("content"))),
		})
		return
	}
	b.add(role, thinkingBlock(message.Get("reasoning_content").String())...)
	b.add(role, contentBlocks(message.Get("content"))...)
	b.add(role, textOrMedia(message.Get("refusal").String())...)
	for _, call := range message.Get("tool_calls").Array() {
		b.add(role, ViewBlock{
			Type:      blockToolCall,
			Id:        call.Get("id").String(),
			Name:      call.Get("function.name").String(),
			Arguments: rawString(call.Get("function.arguments")),
		})
	}
	if legacy := message.Get("function_call"); legacy.IsObject() {
		b.add(role, ViewBlock{Type: blockToolCall, Name: legacy.Get("name").String(), Arguments: rawString(legacy.Get("arguments"))})
	}
}

func openAIRequest(b *viewBuilder, root gjson.Result) []ViewTool {
	for _, message := range root.Get("messages").Array() {
		openAIMessage(b, message)
	}
	// The legacy completions API sends a prompt instead of messages.
	prompt := root.Get("prompt")
	if prompt.IsArray() {
		for _, item := range prompt.Array() {
			b.split()
			b.add(roleUser, textOrMedia(item.String())...)
		}
	} else if prompt.Type == gjson.String {
		b.add(roleUser, textOrMedia(prompt.Str)...)
	}

	var tools []ViewTool
	for _, tool := range root.Get("tools").Array() {
		fn := tool.Get("function")
		if !fn.IsObject() {
			fn = tool
		}
		name := fn.Get("name").String()
		if name == "" {
			name = tool.Get("type").String()
		}
		if view, ok := newViewTool(name, fn.Get("description").String(), fn.Get("parameters")); ok {
			tools = append(tools, view)
		}
	}
	for _, fn := range root.Get("functions").Array() {
		if view, ok := newViewTool(fn.Get("name").String(), fn.Get("description").String(), fn.Get("parameters")); ok {
			tools = append(tools, view)
		}
	}
	return tools
}

func openAIResponse(b *viewBuilder, root gjson.Result) {
	for _, choice := range root.Get("choices").Array() {
		if message := choice.Get("message"); message.IsObject() {
			openAIMessage(b, message)
			continue
		}
		b.split()
		b.add(roleAssistant, textOrMedia(choice.Get("text").String())...)
	}
}
