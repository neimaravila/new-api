package logcontent

import "github.com/tidwall/gjson"

// geminiParts reads the parts of a Gemini content. Field names come in
// camelCase from the API and snake_case from some clients.
func geminiParts(parts gjson.Result) []ViewBlock {
	var blocks []ViewBlock
	for _, part := range parts.Array() {
		if !part.IsObject() {
			continue
		}
		if call := firstOf(part, "functionCall", "function_call"); call.IsObject() {
			blocks = append(blocks, ViewBlock{Type: blockToolCall, Id: call.Get("id").String(), Name: call.Get("name").String(), Arguments: rawString(call.Get("args"))})
			continue
		}
		if result := firstOf(part, "functionResponse", "function_response"); result.IsObject() {
			blocks = append(blocks, ViewBlock{Type: blockToolResult, Id: result.Get("id").String(), Name: result.Get("name").String(), Text: rawString(result.Get("response"))})
			continue
		}
		if inline := firstOf(part, "inlineData", "inline_data"); inline.IsObject() {
			blocks = append(blocks, mediaBlock(firstOf(inline, "mimeType", "mime_type").String(), inline.Get("data").String())...)
			continue
		}
		if file := firstOf(part, "fileData", "file_data"); file.IsObject() {
			blocks = append(blocks, textOrMedia(firstOf(file, "fileUri", "file_uri").String())...)
			continue
		}
		if part.Get("thought").Bool() {
			blocks = append(blocks, thinkingBlock(part.Get("text").String())...)
			continue
		}
		blocks = append(blocks, textOrMedia(part.Get("text").String())...)
	}
	return blocks
}

func geminiRequest(b *viewBuilder, root gjson.Result) []ViewTool {
	system := firstOf(root, "systemInstruction", "system_instruction")
	if system.Type == gjson.String {
		b.add(roleSystem, textOrMedia(system.Str)...)
	} else {
		b.add(roleSystem, geminiParts(system.Get("parts"))...)
	}
	for _, content := range root.Get("contents").Array() {
		if !content.IsObject() {
			continue
		}
		b.split()
		b.add(normalizeRole(content.Get("role").String()), geminiParts(content.Get("parts"))...)
	}

	var tools []ViewTool
	for _, tool := range root.Get("tools").Array() {
		for _, declaration := range firstOf(tool, "functionDeclarations", "function_declarations").Array() {
			schema := firstOf(declaration, "parameters", "parametersJsonSchema", "parameters_json_schema")
			if view, ok := newViewTool(declaration.Get("name").String(), declaration.Get("description").String(), schema); ok {
				tools = append(tools, view)
			}
		}
	}
	return tools
}

func geminiResponse(b *viewBuilder, root gjson.Result) {
	for _, candidate := range root.Get("candidates").Array() {
		b.split()
		b.add(roleAssistant, geminiParts(candidate.Get("content.parts"))...)
	}
}
