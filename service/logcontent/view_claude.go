package logcontent

import "github.com/tidwall/gjson"

// claudeSourceBlocks reads the source of an image or document block.
func claudeSourceBlocks(source gjson.Result) []ViewBlock {
	switch source.Get("type").String() {
	case "base64":
		return mediaBlock(source.Get("media_type").String(), source.Get("data").String())
	case "url":
		return textOrMedia(source.Get("url").String())
	}
	// A text document keeps its text in data.
	return textOrMedia(source.Get("data").String())
}

func claudeRequest(b *viewBuilder, root gjson.Result) []ViewTool {
	b.add(roleSystem, contentBlocks(root.Get("system"))...)
	for _, message := range root.Get("messages").Array() {
		if !message.IsObject() {
			continue
		}
		b.split()
		b.add(normalizeRole(message.Get("role").String()), contentBlocks(message.Get("content"))...)
	}

	var tools []ViewTool
	for _, tool := range root.Get("tools").Array() {
		if view, ok := newViewTool(tool.Get("name").String(), tool.Get("description").String(), tool.Get("input_schema")); ok {
			tools = append(tools, view)
		}
	}
	return tools
}
