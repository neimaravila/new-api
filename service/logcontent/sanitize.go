package logcontent

import (
	"bytes"
	"cmp"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/tidwall/gjson"
)

const (
	redactedMarker = "[REDACTED]"

	minSecretBytes   = 8
	minJSONLeafBytes = 16
	// minSecretFragment is the shortest end of buffer treated as a partial key.
	// Below it the match is noise and reveals nothing about the key.
	minSecretFragment = 4

	minBase64Run        = 1024
	minKnownBase64Bytes = 32
	minShortenedBytes   = 1024

	// maxJSONDepth keeps bodies away from gjson's recursive validator: a stack
	// overflow is fatal in Go, and no real chat payload nests this deep.
	maxJSONDepth = 128
)

var (
	errUnknownEncoding = errors.New("unknown content encoding")
	errBodyTooLarge    = errors.New("decoded body over the parse limit")

	mimePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9!#$&^_.+/-]{0,126}$`)
)

// SanitizedBody is a body ready to store, plus how it relates to the original.
type SanitizedBody struct {
	Text string
	Meta model.BodyMeta
}

// Secrets holds every byte form of a call's keys, longest first, so a composite
// value is replaced whole before its parts.
type Secrets []string

// NewSecrets expands keys into the forms that can appear in a body. Values under
// 8 bytes are ignored: "ollama" is a word in prompts, not a key to hide.
func NewSecrets(values ...string) Secrets {
	forms := make(map[string]struct{})
	for _, value := range values {
		value = strings.TrimSpace(value)
		if gjson.Valid(value) && (strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[")) {
			addSecretLeaves(gjson.Parse(value), forms)
		}
		for part := range strings.SplitSeq(value, "|") {
			addSecretForms(strings.TrimSpace(part), forms)
		}
		addSecretForms(value, forms)
	}
	secrets := slices.Collect(maps.Keys(forms))
	slices.SortFunc(secrets, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(b), len(a)), cmp.Compare(a, b))
	})
	return secrets
}

func addSecretLeaves(node gjson.Result, forms map[string]struct{}) {
	node.ForEach(func(_, leaf gjson.Result) bool {
		switch {
		case leaf.IsObject() || leaf.IsArray():
			addSecretLeaves(leaf, forms)
		case leaf.Type == gjson.String && len(leaf.Str) >= minJSONLeafBytes:
			addSecretForms(leaf.Str, forms)
		}
		return true
	})
}

func addSecretForms(value string, forms map[string]struct{}) {
	// Only the part after sk- is secret: replacing it leaves sk-[REDACTED], and
	// the bare value is caught as well.
	if rest, ok := strings.CutPrefix(value, "sk-"); ok && len(rest) >= minSecretBytes {
		value = rest
	}
	if len(value) < minSecretBytes {
		return
	}
	escape := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	escaped := escape.Replace(value)
	variants := []string{value, escaped, escape.Replace(escaped)}
	if encoded, err := common.Marshal(value); err == nil {
		variants = append(variants, string(encoded[1:len(encoded)-1]))
	}
	for _, variant := range variants {
		forms[variant] = struct{}{}
		forms[strings.ReplaceAll(variant, "/", `\/`)] = struct{}{}
	}
}

// redact returns body without any whole key and without a partial key at its
// end. The end needs its own rule: the buffer can stop anywhere (margin of the
// tap, exhausted budget, dropped connection), and when nothing is cut afterwards
// the partial key would be stored. The input is never modified.
func (s Secrets) redact(body []byte) []byte {
	for _, secret := range s {
		needle := []byte(secret)
		if bytes.Contains(body, needle) {
			body = bytes.ReplaceAll(body, needle, []byte(redactedMarker))
		}
	}
	fragment := 0
	for _, secret := range s {
		for n := min(len(secret)-1, len(body)); n >= minSecretFragment && n > fragment; n-- {
			if string(body[len(body)-n:]) == secret[:n] {
				fragment = n
				break
			}
		}
	}
	if fragment == 0 {
		return body
	}
	kept := len(body) - fragment
	return append(body[:kept:kept], redactedMarker...)
}

// SanitizeBody turns a captured body into what may be stored, in the fixed order:
// decode, redact the whole buffer, and only then shorten. raw may run past
// maxStoredBody by the margin of the tap, or by any amount: nothing is cut first.
func SanitizeBody(raw []byte, encoding string, headOnly bool, secrets Secrets) SanitizedBody {
	decoded, err := decodeContent(raw, encoding)
	if err != nil {
		return SanitizedBody{Meta: model.BodyMeta{
			OriginalBytes: int64(len(raw)),
			Truncated:     headOnly,
			HeadOnly:      headOnly,
			Undecodable:   true,
		}}
	}
	meta := model.BodyMeta{OriginalBytes: int64(len(decoded))}

	// Invalid bytes go before redaction, so removing them cannot rejoin a key.
	body := decoded
	if !utf8.Valid(body) {
		body = bytes.ToValidUTF8(body, nil)
	}
	body = secrets.redact(body)

	if headOnly || len(body) > maxParsedBody {
		return sanitizeHead(body, meta)
	}
	if !isParsableJSON(body) {
		stripped, media := stripBase64Runs(body)
		meta.MediaOmitted = media
		meta.Truncated = len(stripped) > maxStoredBody
		return SanitizedBody{Text: string(cutAtRune(stripped, maxStoredBody)), Meta: meta}
	}

	edited, media := replaceJSONBase64(body)
	meta.MediaOmitted = media
	if len(edited) <= maxStoredBody {
		return SanitizedBody{Text: string(edited), Meta: meta}
	}
	shortened, ok := shortenJSONStrings(edited)
	if !ok {
		return sanitizeHead(edited, meta)
	}
	meta.Truncated = true
	return SanitizedBody{Text: string(shortened), Meta: meta}
}

// SanitizeAssembled cleans text rebuilt from a stream: redaction, base64 runs and
// the 1 MiB limit, with no JSON handling.
func SanitizeAssembled(text string, secrets Secrets) string {
	body := secrets.redact([]byte(strings.ToValidUTF8(text, "")))
	stripped, _ := stripBase64Runs(body)
	return string(cutAtRune(stripped, maxAssembled))
}

func sanitizeHead(body []byte, meta model.BodyMeta) SanitizedBody {
	stripped, media := stripBase64Runs(body)
	meta.MediaOmitted += media
	meta.Truncated = true
	meta.HeadOnly = true
	return SanitizedBody{Text: string(cutAtRune(stripped, maxStoredBody)), Meta: meta}
}

func cutAtRune(body []byte, limit int) []byte {
	if len(body) <= limit {
		return body
	}
	end := limit
	for end > 0 && !utf8.RuneStart(body[end]) {
		end--
	}
	return body[:end]
}

// decodeContent returns the decoded body, or an error when it is unknown, damaged
// or over maxParsedBody. raw itself is returned when there is nothing to decode.
func decodeContent(raw []byte, encoding string) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var reader io.Reader
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "identity":
		return raw, nil
	case "gzip", "x-gzip":
		gzipReader, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		reader = gzipReader
	case "deflate":
		// HTTP deflate is zlib, but some servers send the bare stream.
		zlibReader, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			reader = flate.NewReader(bytes.NewReader(raw))
		} else {
			reader = zlibReader
		}
	case "br":
		reader = brotli.NewReader(bytes.NewReader(raw))
	case "zstd":
		zstdReader, err := zstd.NewReader(bytes.NewReader(raw),
			zstd.WithDecoderConcurrency(1),
			zstd.WithDecoderLowmem(true),
			zstd.WithDecoderMaxWindow(maxParsedBody))
		if err != nil {
			return nil, err
		}
		defer zstdReader.Close()
		reader = zstdReader
	default:
		return nil, errUnknownEncoding
	}
	decoded, err := io.ReadAll(io.LimitReader(reader, maxParsedBody+1))
	if err != nil {
		return nil, err
	}
	if len(decoded) > maxParsedBody {
		return nil, errBodyTooLarge
	}
	return decoded, nil
}

// isParsableJSON reports a valid object or array shallow enough to validate.
func isParsableJSON(body []byte) bool {
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return false
	}
	depth := 0
	inString := false
	for i := 0; i < len(trimmed); i++ {
		switch c := trimmed[i]; {
		case inString && c == '\\':
			i++
		case c == '"':
			inString = !inString
		case inString:
		case c == '{' || c == '[':
			depth++
			if depth > maxJSONDepth {
				return false
			}
		case c == '}' || c == ']':
			depth--
		}
	}
	return gjson.ValidBytes(body)
}

func isBase64Byte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
		c == '+' || c == '/' || c == '=' || c == '-' || c == '_'
}

func isBase64Text(text string) bool {
	for i := range len(text) {
		if !isBase64Byte(text[i]) {
			return false
		}
	}
	return true
}

func base64Marker(mime string, encodedBytes int) string {
	return fmt.Sprintf("[base64 %s %d bytes omitted]", mime, encodedBytes*3/4)
}

// stripBase64Runs replaces long base64 runs found by scanning bytes, for text
// that cannot be parsed. A copy is made only when a run is found.
func stripBase64Runs(body []byte) ([]byte, int) {
	var out []byte
	last, count := 0, 0
	for i := 0; i < len(body); {
		if !isBase64Byte(body[i]) {
			i++
			continue
		}
		end := i
		for end < len(body) && isBase64Byte(body[end]) {
			end++
		}
		start := i
		// The first letters of a run may close a JSON escape, as in \n or é.
		// Keeping them leaves the frame around it parsable.
		if start > 0 && body[start-1] == '\\' {
			start++
			if body[i] == 'u' {
				start += 4
			}
		}
		if end-start >= minBase64Run {
			out = append(out, body[last:start]...)
			out = append(out, base64Marker("unknown", end-start)...)
			last = end
			count++
		}
		i = end
	}
	if count == 0 {
		return body, 0
	}
	return append(out, body[last:]...), count
}

type jsonEdit struct {
	start, end int
	text       string
}

func applyJSONEdits(body []byte, edits []jsonEdit) []byte {
	if len(edits) == 0 {
		return body
	}
	var out bytes.Buffer
	out.Grow(len(body))
	last := 0
	for _, edit := range edits {
		out.Write(body[last:edit.start])
		out.WriteString(edit.text)
		last = edit.end
	}
	out.Write(body[last:])
	return out.Bytes()
}

// jsonSiblings holds the fields that give meaning to a base64 value next to them.
// They are read once per object: a lookup per value would rescan the object for
// every string, and a body of repeated keys makes that quadratic.
type jsonSiblings struct {
	kind, mediaType, mimeType, mimeTypeSnake, format string
}

func readJSONSiblings(node gjson.Result) jsonSiblings {
	var siblings jsonSiblings
	if !node.IsObject() {
		return siblings
	}
	node.ForEach(func(key, value gjson.Result) bool {
		if value.Type != gjson.String {
			return true
		}
		switch key.Str {
		case "type":
			siblings.kind = value.Str
		case "media_type":
			siblings.mediaType = value.Str
		case "mimeType":
			siblings.mimeType = value.Str
		case "mime_type":
			siblings.mimeTypeSnake = value.Str
		case "format":
			siblings.format = value.Str
		}
		return true
	})
	return siblings
}

// visitJSONStrings calls visit for every string value, in document order, with the
// key of the object or array holding it and the siblings of that value.
func visitJSONStrings(node gjson.Result, nodeKey string, visit func(key, parentKey string, siblings jsonSiblings, value gjson.Result)) {
	siblings := readJSONSiblings(node)
	node.ForEach(func(key, value gjson.Result) bool {
		switch {
		case value.IsObject() || value.IsArray():
			visitJSONStrings(value, key.Str, visit)
		case value.Type == gjson.String:
			visit(key.Str, nodeKey, siblings, value)
		}
		return true
	})
}

// replaceJSONBase64 swaps base64 strings for markers. Everything outside those
// strings stays byte for byte, so key order and number spelling do not change.
func replaceJSONBase64(body []byte) ([]byte, int) {
	var edits []jsonEdit
	visitJSONStrings(gjson.ParseBytes(body), "", func(key, parentKey string, siblings jsonSiblings, value gjson.Result) {
		marker, ok := base64MarkerFor(key, parentKey, siblings, value.Str)
		if ok {
			edits = append(edits, jsonEdit{value.Index, value.Index + len(value.Raw), `"` + marker + `"`})
		}
	})
	return applyJSONEdits(body, edits), len(edits)
}

func base64MarkerFor(key, parentKey string, siblings jsonSiblings, text string) (string, bool) {
	if rest, ok := strings.CutPrefix(text, "data:"); ok {
		header, payload, found := strings.Cut(rest, ",")
		if found && strings.HasSuffix(header, ";base64") && len(payload) >= minKnownBase64Bytes {
			mime, _, _ := strings.Cut(header, ";")
			if !mimePattern.MatchString(mime) {
				mime = neighbourMime(parentKey, siblings)
			}
			return base64Marker(mime, len(payload)), true
		}
	}
	if isKnownBase64Field(key, parentKey, siblings) && len(text) >= minKnownBase64Bytes {
		// Encoders may wrap base64 lines, and a line break does not make it text.
		if isBase64Text(strings.NewReplacer("\r", "", "\n", "").Replace(text)) {
			return base64Marker(neighbourMime(parentKey, siblings), len(text)), true
		}
	}
	if len(text) >= minBase64Run && isBase64Text(text) {
		return base64Marker(neighbourMime(parentKey, siblings), len(text)), true
	}
	return "", false
}

func isKnownBase64Field(key, parentKey string, siblings jsonSiblings) bool {
	switch key {
	case "b64_json":
		return true
	case "url":
		return parentKey == "image_url"
	case "result":
		return siblings.kind == "image_generation_call"
	case "data":
		switch parentKey {
		case "input_audio", "inlineData", "inline_data", "audio":
			return true
		case "source":
			return siblings.kind == "base64"
		}
	}
	return false
}

// neighbourMime reads the media type next to a base64 field. The value comes from
// the body and ends up in a marker, so anything unexpected becomes unknown.
func neighbourMime(parentKey string, siblings jsonSiblings) string {
	mime := cmp.Or(siblings.mediaType, siblings.mimeType, siblings.mimeTypeSnake)
	if mime == "" {
		mime = siblings.format
		if (parentKey == "input_audio" || parentKey == "audio") && mime != "" && !strings.Contains(mime, "/") {
			mime = "audio/" + mime
		}
	}
	if !mimePattern.MatchString(mime) {
		return "unknown"
	}
	return mime
}

// jsonStringSpan is the content of a string value between its quotes, still
// escaped, as stored in the body.
type jsonStringSpan struct {
	start   int
	content string
}

// shortenJSONStrings finds the largest L such that shortening every string longer
// than L fits maxStoredBody. ok is false when even L = minShortenedBytes does not
// fit, as with embedding vectors, where the bytes are not in long strings.
func shortenJSONStrings(body []byte) ([]byte, bool) {
	var spans []jsonStringSpan
	longest := 0
	visitJSONStrings(gjson.ParseBytes(body), "", func(_, _ string, _ jsonSiblings, value gjson.Result) {
		content := value.Raw[1 : len(value.Raw)-1]
		if len(content) > minShortenedBytes {
			spans = append(spans, jsonStringSpan{value.Index + 1, content})
			longest = max(longest, len(content))
		}
	})
	sizeWith := func(limit int) int {
		size := len(body)
		for _, span := range spans {
			if len(span.content) > limit {
				size += min(0, len(shortenedString(span.content, limit))-len(span.content))
			}
		}
		return size
	}
	if sizeWith(minShortenedBytes) > maxStoredBody {
		return nil, false
	}
	low, high := minShortenedBytes, longest
	for low < high {
		middle := low + (high-low+1)/2
		if sizeWith(middle) <= maxStoredBody {
			low = middle
		} else {
			high = middle - 1
		}
	}
	var edits []jsonEdit
	for _, span := range spans {
		if len(span.content) <= low {
			continue
		}
		if text := shortenedString(span.content, low); len(text) < len(span.content) {
			edits = append(edits, jsonEdit{span.start, span.start + len(span.content), text})
		}
	}
	return applyJSONEdits(body, edits), true
}

// shortenedString keeps limit/2 bytes on each side of an escaped JSON string,
// moving each cut back to a place that splits neither a rune nor an escape.
func shortenedString(content string, limit int) string {
	head := safeJSONCut(content, limit/2)
	tail := safeJSONCut(content, len(content)-limit/2)
	omitted := utf8.RuneCountInString(content[head:tail])
	return fmt.Sprintf("%s…[%d chars omitted]…%s", content[:head], omitted, content[tail:])
}

func safeJSONCut(content string, cut int) int {
	for cut > 0 && !utf8.RuneStart(content[cut]) {
		cut--
	}
	// An escape holds no other backslash, so only the nearest one can be open.
	for i := cut - 1; i >= 0 && i >= cut-5; i-- {
		if content[i] != '\\' {
			continue
		}
		run := 0
		for k := i; k >= 0 && content[k] == '\\'; k-- {
			run++
		}
		if run%2 == 0 {
			return cut
		}
		width := 2
		if i+1 < len(content) && content[i+1] == 'u' {
			width = 6
		}
		if i+width > cut {
			return i
		}
		return cut
	}
	return cut
}
