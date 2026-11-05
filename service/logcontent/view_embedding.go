package logcontent

import "github.com/tidwall/gjson"

// embeddingRequest reads the text of an embeddings input or a rerank query into
// one user message. Token lists and numbers carry nothing to read.
func embeddingRequest(b *viewBuilder, format string, root gjson.Result) {
	if format == FormatRerank {
		b.add(roleUser, textOrMedia(root.Get("query").String())...)
		return
	}
	input := root.Get("input")
	if input.Type == gjson.String {
		b.add(roleUser, textOrMedia(input.Str)...)
		return
	}
	for _, item := range input.Array() {
		if item.Type == gjson.String {
			b.add(roleUser, textOrMedia(item.Str)...)
		}
	}
}
