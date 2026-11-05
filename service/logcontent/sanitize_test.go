package logcontent

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"math/rand/v2"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	testKey = "sk-abc12345def67890"
	// testBareKey has no sk- prefix, so the whole value is matched as stored.
	testBareKey = "live9f8e7d6c5b4a3210"
	// proseUnit is not a base64 run, so filler made of it stays text.
	proseUnit = "the quick brown fox "
)

// prose returns exactly n bytes of text that no base64 rule touches.
func prose(n int) string {
	return strings.Repeat(proseUnit, n/len(proseUnit)+1)[:n]
}

// jsonOfSize returns a valid chat body of exactly n bytes.
func jsonOfSize(t *testing.T, n int) []byte {
	t.Helper()
	const open, closing = `{"messages":[{"role":"user","content":"`, `"}]}`
	require.Greater(t, n, len(open)+len(closing))
	body := []byte(open + prose(n-len(open)-len(closing)) + closing)
	require.Len(t, body, n)
	return body
}

// jsonUnicode spells a JSON unicode escape for use inside test bodies.
func jsonUnicode(hex string) string {
	return "\x5c" + "u" + hex
}

var surrogatePair = jsonUnicode("d83d") + jsonUnicode("de00")

func base64Text(n int) string {
	return strings.Repeat("QUJD", n/4+1)[:n]
}

func TestSanitizeReplacesExactSecrets(t *testing.T) {
	slashKey := "abc/def/ghi/jkl/mno"
	secrets := NewSecrets(testBareKey, slashKey)

	cases := map[string]string{
		"json body":    `{"key":"` + testBareKey + `","model":"gpt"}`,
		"escaped form": `{"key":"abc\/def\/ghi\/jkl\/mno"}`,
		"plain slash":  `{"key":"abc/def/ghi/jkl/mno"}`,
		"plain text":   "token " + testBareKey + " here",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			got := SanitizeBody([]byte(body), "", false, secrets)
			assert.NotContains(t, got.Text, testBareKey)
			assert.NotContains(t, got.Text, slashKey)
			assert.NotContains(t, got.Text, `abc\/def`)
			assert.Contains(t, got.Text, "[REDACTED]")
		})
	}

	t.Run("provider error echoing the key", func(t *testing.T) {
		got := SanitizeBody([]byte(`{"error":{"message":"bad key sk-abc12345"}}`), "", false, NewSecrets("sk-abc12345"))
		assert.NotContains(t, got.Text, "abc12345")
		assert.Equal(t, `{"error":{"message":"bad key sk-[REDACTED]"}}`, got.Text)
	})

	t.Run("token without the sk prefix keeps the visible prefix", func(t *testing.T) {
		got := SanitizeBody([]byte(`Bearer sk-abc12345def`), "", false, NewSecrets("abc12345def"))
		assert.Equal(t, `Bearer sk-[REDACTED]`, got.Text)
	})

	t.Run("sk key appearing bare is still replaced", func(t *testing.T) {
		got := SanitizeBody([]byte(`value abc12345def67890`), "", false, NewSecrets(testKey))
		assert.NotContains(t, got.Text, "abc12345def67890")
	})

	t.Run("input is not mutated", func(t *testing.T) {
		raw := []byte(`{"key":"` + testBareKey + `"}`)
		before := bytes.Clone(raw)
		SanitizeBody(raw, "", false, secrets)
		assert.Equal(t, before, raw)
	})
}

func TestNewSecretsCompositeAndShortKeys(t *testing.T) {
	t.Run("composite parts", func(t *testing.T) {
		secrets := NewSecrets("ak12345678|sk12345678|us-east-1")
		got := SanitizeBody([]byte("a ak12345678 b sk12345678 c us-east-1 d ak12345678|sk12345678|us-east-1"), "", false, secrets)
		assert.NotContains(t, got.Text, "ak12345678")
		assert.NotContains(t, got.Text, "sk12345678")
		assert.NotContains(t, got.Text, "us-east-1")
	})

	t.Run("service account leaves", func(t *testing.T) {
		privateKey := "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQ/+abc=\n-----END PRIVATE KEY-----\n"
		account := `{"type":"service_account","private_key":"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQ/+abc=\n-----END PRIVATE KEY-----\n","nested":{"client_email":"bot@project.iam.example.com"}}`
		secrets := NewSecrets(account)

		escaped := `-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQ/+abc=\n-----END PRIVATE KEY-----\n`
		for name, body := range map[string]string{
			"decoded":        "key: " + privateKey,
			"json escaped":   `{"error":"` + escaped + `"}`,
			"slash escaped":  `{"error":"` + strings.ReplaceAll(escaped, "/", `\/`) + `"}`,
			"whole value":    "config " + account,
			"nested in json": `{"message":"{\"key\":\"-----BEGIN PRIVATE KEY-----\\nMIIEvQIBADANBgkqhkiG9w0BAQ/+abc=\\n-----END PRIVATE KEY-----\\n\"}"}`,
		} {
			t.Run(name, func(t *testing.T) {
				got := SanitizeBody([]byte(body), "", false, secrets)
				assert.NotContains(t, got.Text, "MIIEvQIBADANBgkqhkiG9w0BAQ")
			})
		}
		got := SanitizeBody([]byte(`mail bot@project.iam.example.com`), "", false, secrets)
		assert.NotContains(t, got.Text, "bot@project")
		// A leaf under 16 characters is not a secret: "service_account" has 15.
		got = SanitizeBody([]byte(`kind service_account`), "", false, secrets)
		assert.Equal(t, `kind service_account`, got.Text)
	})

	t.Run("short keys do not corrupt text", func(t *testing.T) {
		const text = "use ollama locally"
		assert.Equal(t, text, SanitizeBody([]byte(text), "", false, NewSecrets("ollama")).Text)
		assert.Empty(t, NewSecrets("ollama"))
	})

	t.Run("empty value changes nothing", func(t *testing.T) {
		const text = `{"a": "b"}`
		assert.Empty(t, NewSecrets("", "   "))
		assert.Equal(t, text, SanitizeBody([]byte(text), "", false, NewSecrets("")).Text)
	})

	t.Run("longest variant first", func(t *testing.T) {
		secrets := NewSecrets("ak12345678|sk12345678")
		got := SanitizeBody([]byte("x ak12345678|sk12345678 y"), "", false, secrets)
		assert.Equal(t, "x [REDACTED] y", got.Text)
	})
}

func TestSanitizeSecretNotSplitByCut(t *testing.T) {
	for _, offset := range []int{-len(testBareKey), -len(testBareKey) + 1, -8, -7, -1, 0, 1} {
		t.Run(fmt.Sprintf("offset %d", offset), func(t *testing.T) {
			position := maxStoredBody + offset
			body := prose(position) + testBareKey + prose(9<<20-position-len(testBareKey))
			require.Len(t, body, 9<<20)

			got := SanitizeBody([]byte(body), "", false, NewSecrets(testBareKey))

			assert.NotContains(t, got.Text, testBareKey)
			assert.NotContains(t, got.Text, testBareKey[:8])
			assert.LessOrEqual(t, len(got.Text), maxStoredBody)
			assert.True(t, got.Meta.HeadOnly)
			assert.True(t, got.Meta.Truncated)
		})
	}
}

// The tap in Task 7 hands over 1 MiB plus a margin of up to 64 KiB with
// head_only set: the worker redacts the whole buffer, then cuts.
func TestSanitizeHeadOnlyAcceptsTapMargin(t *testing.T) {
	assert.Equal(t, 64<<10, tapMarginBytes)

	for _, offset := range []int{-len(testBareKey) + 1, -8, -1, 0} {
		t.Run(fmt.Sprintf("key straddling the cut at offset %d", offset), func(t *testing.T) {
			position := maxStoredBody + offset
			raw := prose(position) + testBareKey
			raw += prose(maxStoredBody + tapMarginBytes - len(raw))
			require.Len(t, raw, maxStoredBody+tapMarginBytes)

			got := SanitizeBody([]byte(raw), "", true, NewSecrets(testBareKey))

			assert.NotContains(t, got.Text, testBareKey[:8])
			assert.LessOrEqual(t, len(got.Text), maxStoredBody)
			assert.True(t, got.Meta.HeadOnly)
			assert.True(t, got.Meta.Truncated)
		})
	}

	t.Run("escaped key straddling the cut", func(t *testing.T) {
		key := "abc/def/ghi/jkl/mno"
		escaped := `abc\/def\/ghi\/jkl\/mno`
		raw := prose(maxStoredBody-6) + escaped + prose(tapMarginBytes)
		got := SanitizeBody([]byte(raw), "", true, NewSecrets(key))
		assert.NotContains(t, got.Text, `abc\/d`)
	})
}

// The margin only protects when the cut really happens. Removing base64 can
// shrink the buffer under 1 MiB, and an exhausted budget can end the copy
// anywhere, so the end of the buffer can hold a partial key with no cut after it.
func TestSanitizeNoFragmentAtEndOfBuffer(t *testing.T) {
	fragment := testBareKey[:12]

	t.Run("base64 shrinks the margin below the cut", func(t *testing.T) {
		raw := base64Text(200<<10) + " " + prose(maxStoredBody+tapMarginBytes-(200<<10)-1-len(fragment)) + fragment
		require.Len(t, raw, maxStoredBody+tapMarginBytes)

		got := SanitizeBody([]byte(raw), "", true, NewSecrets(testBareKey))

		require.Less(t, len(got.Text), maxStoredBody, "the cut must not be what removes the fragment")
		assert.NotContains(t, got.Text, testBareKey[:8])
	})

	t.Run("budget ended the copy before the cut", func(t *testing.T) {
		raw := prose(1000) + fragment
		got := SanitizeBody([]byte(raw), "", true, NewSecrets(testBareKey))
		assert.NotContains(t, got.Text, testBareKey[:8])
	})

	t.Run("escaped form", func(t *testing.T) {
		raw := prose(1000) + `abc\/def\/gh`
		got := SanitizeBody([]byte(raw), "", true, NewSecrets("abc/def/ghi/jkl/mno"))
		assert.NotContains(t, got.Text, `abc\/def`)
	})

	t.Run("upstream cut mid body without head only", func(t *testing.T) {
		raw := `{"error":"bad key ` + fragment
		got := SanitizeBody([]byte(raw), "", false, NewSecrets(testBareKey))
		assert.NotContains(t, got.Text, testBareKey[:8])
	})

	t.Run("text that only ends like one or two key chars stays", func(t *testing.T) {
		raw := prose(100) + "li"
		got := SanitizeBody([]byte(raw), "", false, NewSecrets(testBareKey))
		assert.Equal(t, raw, got.Text)
	})
}

func TestSanitizeBase64Markers(t *testing.T) {
	data := base64Text(4000)
	cases := []struct {
		name   string
		body   string
		marker string
	}{
		{"data url", `{"image_url":{"url":"data:image/png;base64,` + data + `"}}`, "[base64 image/png 3000 bytes omitted]"},
		{"data url with parameter", `{"a":"data:image/webp;charset=utf-8;base64,` + data + `"}`, "[base64 image/webp 3000 bytes omitted]"},
		{"claude source", `{"source":{"type":"base64","media_type":"image/jpeg","data":"` + data + `"}}`, "[base64 image/jpeg 3000 bytes omitted]"},
		{"gemini inlineData", `{"inlineData":{"mimeType":"audio/wav","data":"` + data + `"}}`, "[base64 audio/wav 3000 bytes omitted]"},
		{"gemini inline_data", `{"inline_data":{"mime_type":"image/gif","data":"` + data + `"}}`, "[base64 image/gif 3000 bytes omitted]"},
		{"input_audio", `{"input_audio":{"format":"mp3","data":"` + data + `"}}`, "[base64 audio/mp3 3000 bytes omitted]"},
		{"audio output", `{"audio":{"format":"wav","data":"` + data + `"}}`, "[base64 audio/wav 3000 bytes omitted]"},
		{"b64_json", `{"data":[{"b64_json":"` + data + `"}]}`, "[base64 unknown 3000 bytes omitted]"},
		{"image_generation_call", `{"type":"image_generation_call","result":"` + data + `"}`, "[base64 unknown 3000 bytes omitted]"},
		{"loose string", `{"x":"` + base64Text(2000) + `"}`, "[base64 unknown 1500 bytes omitted]"},
		{"known field at the minimum", `{"b64_json":"` + base64Text(32) + `"}`, "[base64 unknown 24 bytes omitted]"},
		{"hostile mime is not trusted", `{"source":{"type":"base64","media_type":"a b\"c","data":"` + data + `"}}`, "[base64 unknown 3000 bytes omitted]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeBody([]byte(tc.body), "", false, nil)
			assert.Contains(t, got.Text, tc.marker)
			assert.NotContains(t, got.Text, "QUJDQUJD")
			assert.Equal(t, 1, got.Meta.MediaOmitted)
			assert.True(t, gjson.Valid(got.Text), got.Text)
			assert.False(t, got.Meta.Truncated)
		})
	}

	t.Run("text stays", func(t *testing.T) {
		for name, body := range map[string]string{
			"text with spaces":      `{"x":"` + prose(2000) + `"}`,
			"url in a known field":  `{"image_url":{"url":"https://example.com/images/a-long-picture-name.png"}}`,
			"short known field":     `{"b64_json":"` + base64Text(31) + `"}`,
			"short loose string":    `{"x":"` + base64Text(1023) + `"}`,
			"source that is a url":  `{"source":{"type":"url","data":"` + base64Text(64) + `"}}`,
			"data unknown to rules": `{"payload":{"data":"` + base64Text(64) + `"}}`,
		} {
			t.Run(name, func(t *testing.T) {
				got := SanitizeBody([]byte(body), "", false, nil)
				assert.Equal(t, body, got.Text)
				assert.Zero(t, got.Meta.MediaOmitted)
			})
		}
	})

	t.Run("counts every marker", func(t *testing.T) {
		body := `{"a":"data:image/png;base64,` + data + `","b":[{"b64_json":"` + data + `"},{"c":"` + base64Text(2000) + `"}]}`
		assert.Equal(t, 3, SanitizeBody([]byte(body), "", false, nil).Meta.MediaOmitted)
	})

	t.Run("unicode escape before a run survives the byte scan", func(t *testing.T) {
		body := "data: {\"delta\":\"line" + jsonUnicode("00e9") + base64Text(2000) + "\"}\n\n"
		got := SanitizeBody([]byte(body), "", false, nil)
		assert.Equal(t, 1, got.Meta.MediaOmitted)
		frame := strings.TrimSpace(strings.TrimPrefix(got.Text, "data: "))
		assert.True(t, gjson.Valid(frame), frame)
		assert.Contains(t, frame, jsonUnicode("00e9"))
	})

	t.Run("sse uses the byte scan", func(t *testing.T) {
		body := "data: {\"delta\":\"line\\n" + base64Text(2000) + "\"}\n\n"
		got := SanitizeBody([]byte(body), "", false, nil)
		assert.Equal(t, 1, got.Meta.MediaOmitted)
		assert.NotContains(t, got.Text, "QUJDQUJD")
		frame := strings.TrimSpace(strings.TrimPrefix(got.Text, "data: "))
		assert.True(t, gjson.Valid(frame), "the escape before the run must survive: %s", frame)
	})
}

func TestSanitizePreservesJSONOutsideEdits(t *testing.T) {
	image := "data:image/png;base64," + base64Text(64)
	body := `{"z":1.50,"a":12345678901234567890,"img":"` + image + `"}`
	got := SanitizeBody([]byte(body), "", false, nil)
	assert.Equal(t, `{"z":1.50,"a":12345678901234567890,"img":"[base64 image/png 48 bytes omitted]"}`, got.Text)

	t.Run("offsets hold with surrounding whitespace", func(t *testing.T) {
		pretty := "\n\t  {\n  \"z\" : 1.50,\n  \"list\": [ 1, \"keep\", { \"img\" :\t\"" + image + "\" } ],\n  \"é\": \"ç\"\n}\n "
		got := SanitizeBody([]byte(pretty), "", false, nil)
		want := strings.ReplaceAll(pretty, image, "[base64 image/png 48 bytes omitted]")
		assert.Equal(t, want, got.Text)
	})
}

func TestSanitizeShortensLongMessagesKeepingStartAndEnd(t *testing.T) {
	long := "STARTMARK " + prose(3<<20) + " ENDMARK"
	short := "a short neighbour message"
	body := `{"messages":[{"role":"system","content":"` + short + `"},{"role":"user","content":"` + long + `"}]}`

	got := SanitizeBody([]byte(body), "", false, nil)

	assert.LessOrEqual(t, len(got.Text), maxStoredBody)
	assert.True(t, gjson.Valid(got.Text))
	assert.Contains(t, got.Text, "STARTMARK")
	assert.Contains(t, got.Text, "ENDMARK")
	assert.Contains(t, got.Text, "…[")
	assert.Contains(t, got.Text, " chars omitted]…")
	assert.Equal(t, short, gjson.Get(got.Text, "messages.0.content").String())
	assert.True(t, got.Meta.Truncated)
	assert.False(t, got.Meta.HeadOnly)
	// The search keeps as much as fits, not just the minimum.
	assert.Greater(t, len(got.Text), maxStoredBody-256)

	t.Run("cuts keep runes and escapes whole", func(t *testing.T) {
		for _, unit := range []string{`é`, `\n`, `\"`, `\\`, jsonUnicode("00e9"), `日本語`, "\U0001F600", `a\\\n`, surrogatePair} {
			t.Run(unit, func(t *testing.T) {
				text := strings.Repeat(unit, (3<<20)/len(unit))
				body := `{"a":"` + text + `","b":"` + text + `x"}`
				got := SanitizeBody([]byte(body), "", false, nil)
				require.LessOrEqual(t, len(got.Text), maxStoredBody)
				assert.True(t, gjson.Valid(got.Text))
				assert.True(t, utf8.ValidString(got.Text))
				assert.Contains(t, gjson.Get(got.Text, "a").String(), " chars omitted]")
				assert.False(t, got.Meta.HeadOnly)
			})
		}
	})

	t.Run("redacts before shortening", func(t *testing.T) {
		text := prose((1 << 19) - 8)
		body := `{"a":"` + prose(1<<20) + testBareKey + text + `"}`
		got := SanitizeBody([]byte(body), "", false, NewSecrets(testBareKey))
		assert.NotContains(t, got.Text, testBareKey[:8])
	})
}

func TestSanitizeBodyAtExactLimits(t *testing.T) {
	t.Run("1 MiB exact is not cut", func(t *testing.T) {
		body := jsonOfSize(t, maxStoredBody)
		got := SanitizeBody(body, "", false, nil)
		assert.Equal(t, string(body), got.Text)
		assert.False(t, got.Meta.Truncated)
		assert.Equal(t, int64(maxStoredBody), got.Meta.OriginalBytes)
	})

	t.Run("1 MiB plus one is shortened", func(t *testing.T) {
		got := SanitizeBody(jsonOfSize(t, maxStoredBody+1), "", false, nil)
		assert.LessOrEqual(t, len(got.Text), maxStoredBody)
		assert.True(t, gjson.Valid(got.Text))
		assert.True(t, got.Meta.Truncated)
		assert.False(t, got.Meta.HeadOnly)
	})

	t.Run("8 MiB exact is still JSON", func(t *testing.T) {
		got := SanitizeBody(jsonOfSize(t, maxParsedBody), "", false, nil)
		assert.True(t, gjson.Valid(got.Text))
		assert.False(t, got.Meta.HeadOnly)
		assert.Equal(t, int64(maxParsedBody), got.Meta.OriginalBytes)
	})

	t.Run("8 MiB plus one keeps only the start", func(t *testing.T) {
		got := SanitizeBody(jsonOfSize(t, maxParsedBody+1), "", false, nil)
		assert.True(t, got.Meta.HeadOnly)
		assert.True(t, got.Meta.Truncated)
		assert.Len(t, got.Text, maxStoredBody)
	})

	t.Run("head only keeps 1 MiB without breaking a rune", func(t *testing.T) {
		raw := strings.Repeat("日本語 ", (2<<20)/10)
		got := SanitizeBody([]byte(raw), "", true, nil)
		assert.True(t, got.Meta.HeadOnly)
		assert.True(t, got.Meta.Truncated)
		assert.LessOrEqual(t, len(got.Text), maxStoredBody)
		assert.Greater(t, len(got.Text), maxStoredBody-4)
		assert.True(t, utf8.ValidString(got.Text))
	})

	t.Run("head only below the limit is still flagged", func(t *testing.T) {
		got := SanitizeBody([]byte(`{"a":"b"}`), "", true, nil)
		assert.True(t, got.Meta.HeadOnly)
		assert.True(t, got.Meta.Truncated)
		assert.Equal(t, `{"a":"b"}`, got.Text)
	})

	t.Run("non JSON over the limit is cut", func(t *testing.T) {
		got := SanitizeBody([]byte(prose(2<<20)), "", false, nil)
		assert.Len(t, got.Text, maxStoredBody)
		assert.True(t, got.Meta.Truncated)
		assert.False(t, got.Meta.HeadOnly)
	})
}

func TestSanitizeEmbeddingVectorsFallBackToHead(t *testing.T) {
	var body strings.Builder
	body.WriteString(`{"data":[{"embedding":[`)
	for body.Len() < 2<<20 {
		body.WriteString("0.0123456789,")
	}
	body.WriteString(`0.5]}]}`)

	got := SanitizeBody([]byte(body.String()), "", false, nil)

	assert.True(t, got.Meta.HeadOnly)
	assert.True(t, got.Meta.Truncated)
	assert.LessOrEqual(t, len(got.Text), maxStoredBody)
}

func TestSanitizeHeadOnlyStripsBase64(t *testing.T) {
	raw := `{"a":1,"img":"data:image/png;base64,` + base64Text(1<<20)
	got := SanitizeBody([]byte(raw), "", true, nil)
	assert.True(t, got.Meta.HeadOnly)
	assert.Equal(t, 1, got.Meta.MediaOmitted)
	assert.NotContains(t, got.Text, "QUJDQUJD")
	assert.Contains(t, got.Text, " bytes omitted]")
}

func compressGzip(t *testing.T, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := gzip.NewWriter(&out)
	_, err := writer.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return out.Bytes()
}

func TestSanitizeDecodesAndLimitsEncodings(t *testing.T) {
	plain := []byte(`{"text":"hello ` + testBareKey + `"}`)
	secrets := NewSecrets(testBareKey)

	var zlibBody, flateBody, brotliBody, zstdBody bytes.Buffer
	zlibWriter := zlib.NewWriter(&zlibBody)
	_, err := zlibWriter.Write(plain)
	require.NoError(t, err)
	require.NoError(t, zlibWriter.Close())
	flateWriter, err := flate.NewWriter(&flateBody, flate.DefaultCompression)
	require.NoError(t, err)
	_, err = flateWriter.Write(plain)
	require.NoError(t, err)
	require.NoError(t, flateWriter.Close())
	brotliWriter := brotli.NewWriter(&brotliBody)
	_, err = brotliWriter.Write(plain)
	require.NoError(t, err)
	require.NoError(t, brotliWriter.Close())
	zstdWriter, err := zstd.NewWriter(&zstdBody)
	require.NoError(t, err)
	_, err = zstdWriter.Write(plain)
	require.NoError(t, err)
	require.NoError(t, zstdWriter.Close())

	for encoding, raw := range map[string][]byte{
		"gzip":        compressGzip(t, plain),
		"GZIP":        compressGzip(t, plain),
		"deflate":     zlibBody.Bytes(),
		"deflate-raw": flateBody.Bytes(),
		"br":          brotliBody.Bytes(),
		"zstd":        zstdBody.Bytes(),
		"identity":    plain,
		"":            plain,
	} {
		t.Run(encoding, func(t *testing.T) {
			header := encoding
			if header == "deflate-raw" {
				header = "deflate"
			}
			got := SanitizeBody(raw, header, false, secrets)
			assert.Equal(t, `{"text":"hello [REDACTED]"}`, got.Text)
			assert.False(t, got.Meta.Undecodable)
			assert.Equal(t, int64(len(plain)), got.Meta.OriginalBytes)
		})
	}

	t.Run("gzip bomb stops at 8 MiB", func(t *testing.T) {
		// Deflate tops out near 1000:1, so 1 GiB needs about 1 MiB of gzip
		// whatever the writer does. Concatenated members build it cheaply.
		member := compressGzip(t, make([]byte, 1<<20))
		bomb := bytes.Repeat(member, 1024)

		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		got := SanitizeBody(bomb, "gzip", false, secrets)
		runtime.ReadMemStats(&after)

		assert.True(t, got.Meta.Undecodable)
		assert.Empty(t, got.Text)
		assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(128<<20), "decoding must stop near the limit")
	})

	t.Run("exactly 8 MiB decoded is accepted, one more is not", func(t *testing.T) {
		atLimit := compressGzip(t, jsonOfSize(t, maxParsedBody))
		assert.False(t, SanitizeBody(atLimit, "gzip", false, nil).Meta.Undecodable)
		overLimit := compressGzip(t, jsonOfSize(t, maxParsedBody+1))
		got := SanitizeBody(overLimit, "gzip", false, nil)
		assert.True(t, got.Meta.Undecodable)
		assert.Empty(t, got.Text)
	})

	t.Run("failures store nothing", func(t *testing.T) {
		for name, tc := range map[string]struct {
			raw      []byte
			encoding string
		}{
			"unknown encoding": {plain, "snappy"},
			"garbage gzip":     {[]byte("this is not gzip at all"), "gzip"},
			"garbage deflate":  {[]byte("this is not deflate at all"), "deflate"},
			"garbage br":       {[]byte("this is not brotli at all, really"), "br"},
			"garbage zstd":     {[]byte("this is not zstd at all"), "zstd"},
			"truncated gzip":   {compressGzip(t, plain)[:12], "gzip"},
		} {
			t.Run(name, func(t *testing.T) {
				got := SanitizeBody(tc.raw, tc.encoding, false, secrets)
				assert.True(t, got.Meta.Undecodable)
				assert.Empty(t, got.Text)
			})
		}
	})

	t.Run("empty body with an encoding header is not undecodable", func(t *testing.T) {
		got := SanitizeBody(nil, "gzip", false, nil)
		assert.False(t, got.Meta.Undecodable)
		assert.Empty(t, got.Text)
	})
}

func TestSanitizeInvalidUTF8AndNul(t *testing.T) {
	t.Run("raw bytes", func(t *testing.T) {
		got := SanitizeBody([]byte("\xff\x00"), "", false, nil)
		assert.True(t, utf8.ValidString(got.Text))
		assert.False(t, gjson.Valid(got.Text))
		assert.False(t, got.Meta.Undecodable)
	})

	t.Run("invalid byte cannot hide a key", func(t *testing.T) {
		split := testBareKey[:6] + "\xff" + testBareKey[6:]
		got := SanitizeBody([]byte(split), "", false, NewSecrets(testBareKey))
		assert.NotContains(t, got.Text, testBareKey)
		assert.Contains(t, got.Text, "[REDACTED]")
	})

	t.Run("binary disguised as JSON", func(t *testing.T) {
		got := SanitizeBody([]byte("{\"a\":\"\xff\xfe\x00\"}garbage"), "", false, nil)
		assert.True(t, utf8.ValidString(got.Text))
	})
}

// gjson validates recursively: a body of nothing but brackets must not reach it,
// because a stack overflow cannot be recovered and would take the process down.
func TestSanitizeDeeplyNestedJSONIsNotParsed(t *testing.T) {
	for name, raw := range map[string]string{
		"open arrays":   strings.Repeat("[", maxParsedBody),
		"open objects":  strings.Repeat(`{"a":`, maxParsedBody/5),
		"closed arrays": strings.Repeat("[", 1<<19) + strings.Repeat("]", 1<<19),
	} {
		t.Run(name, func(t *testing.T) {
			got := SanitizeBody([]byte(raw), "", false, nil)
			assert.LessOrEqual(t, len(got.Text), maxStoredBody)
		})
	}

	t.Run("brackets inside strings do not count", func(t *testing.T) {
		image := "data:image/png;base64," + base64Text(64)
		body := `{"text":"` + strings.Repeat("[", 4096) + `\"","img":"` + image + `"}`
		got := SanitizeBody([]byte(body), "", false, nil)
		assert.Equal(t, 1, got.Meta.MediaOmitted)
		assert.True(t, gjson.Valid(got.Text))
	})
}

func TestSanitizeAssembled(t *testing.T) {
	secrets := NewSecrets(testBareKey)

	assert.Equal(t, "hello [REDACTED] bye", SanitizeAssembled("hello "+testBareKey+" bye", secrets))
	assert.Equal(t, "plain", SanitizeAssembled("plain", nil))

	t.Run("base64 runs become markers", func(t *testing.T) {
		got := SanitizeAssembled("see "+base64Text(2000)+" end", nil)
		assert.Equal(t, "see [base64 unknown 1500 bytes omitted] end", got)
	})

	t.Run("key straddling the limit leaves no fragment", func(t *testing.T) {
		text := prose(maxAssembled-6) + testBareKey + prose(100)
		got := SanitizeAssembled(text, secrets)
		assert.NotContains(t, got, testBareKey[:6])
		assert.LessOrEqual(t, len(got), maxAssembled)
	})

	t.Run("limit keeps runes whole", func(t *testing.T) {
		got := SanitizeAssembled(strings.Repeat("語", maxAssembled), nil)
		assert.LessOrEqual(t, len(got), maxAssembled)
		assert.True(t, utf8.ValidString(got))
	})

	t.Run("invalid UTF-8", func(t *testing.T) {
		assert.Equal(t, "ab", SanitizeAssembled("a\xffb", nil))
	})
}

// A sibling lookup per value would rescan the object for every string, which a
// body of repeated keys turns into hours of work for the two workers.
func TestSanitizeRepeatedKeysStayLinear(t *testing.T) {
	var body strings.Builder
	body.WriteString(`{"type":"image_generation_call"`)
	value := base64Text(32)
	count := 0
	for body.Len() < maxParsedBody-64 {
		body.WriteString(`,"result":"` + value + `"`)
		count++
	}
	body.WriteString(`}`)

	done := make(chan SanitizedBody, 1)
	go func() { done <- SanitizeBody([]byte(body.String()), "", false, nil) }()

	// The quadratic version ran past 100 s; this takes about 1 s, even under -race.
	select {
	case got := <-done:
		assert.Equal(t, count, got.Meta.MediaOmitted)
	case <-time.After(30 * time.Second):
		t.Fatal("sanitizing repeated keys is not linear")
	}
}

func TestSanitizeShortenedStringKeepsEscapesValid(t *testing.T) {
	units := []string{"a", "é", "語", "\U0001F600", `\n`, `\"`, `\\`, jsonUnicode("00e9"), surrogatePair, `\\` + jsonUnicode("d83d")[1:], jsonUnicode("de00")}
	random := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		var content strings.Builder
		for range 8 + random.IntN(40) {
			content.WriteString(units[random.IntN(len(units))])
		}
		text := content.String()
		for limit := 2; limit < len(text); limit++ {
			got := `"` + shortenedString(text, limit) + `"`
			require.True(t, gjson.Valid(got), "limit %d of %q gave %s", limit, text, got)
			require.True(t, utf8.ValidString(got))
		}
	}
}

func TestSanitizeEncodedSecretForms(t *testing.T) {
	const key = "abc def/ghi+jkl/mno"
	secrets := NewSecrets(key)
	for name, form := range map[string]string{
		"query escape":         url.QueryEscape(key),
		"path escape":          url.PathEscape(key),
		"base64":               base64.StdEncoding.EncodeToString([]byte(key)),
		"slash escaped":        strings.ReplaceAll(key, "/", `\/`),
		"slash double escaped": strings.ReplaceAll(key, "/", `\\\/`),
		"slash escaped twice":  strings.ReplaceAll(key, "/", `\\/`),
	} {
		t.Run(name, func(t *testing.T) {
			got := SanitizeBody([]byte("see "+form+" end"), "", false, secrets)
			assert.Equal(t, "see [REDACTED] end", got.Text)
		})
	}

	t.Run("base64 of the whole sk value", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte(testKey))
		got := SanitizeBody([]byte("see "+encoded+" end"), "", false, NewSecrets(testKey))
		assert.Equal(t, "see [REDACTED] end", got.Text)
	})
}

func TestSanitizeDecodedJSONStrings(t *testing.T) {
	const key = "abc/def/ghi/jkl/mno"
	secrets := NewSecrets(key)
	lower := strings.ReplaceAll(key, "/", jsonUnicode("002f"))
	for name, spelling := range map[string]string{
		"lower unicode slash": lower,
		"upper unicode slash": strings.ReplaceAll(key, "/", jsonUnicode("002F")),
		"unicode letters":     jsonUnicode("0061") + jsonUnicode("0062") + jsonUnicode("0063") + "/def/ghi/jkl/mno",
		"mixed":               `abc\/def` + jsonUnicode("002f") + `ghi/jkl\/mn` + jsonUnicode("006f"),
	} {
		t.Run(name, func(t *testing.T) {
			other := `"other":"caf` + jsonUnicode("00e9") + `"`
			body := "{ \"z\" : 1.50, \"a\":12345678901234567890,\n\"m\":\"bad " + spelling + " end\\nline\", " + other + " }"
			got := SanitizeBody([]byte(body), "", false, secrets)

			assert.Equal(t, "bad [REDACTED] end\nline", gjson.Get(got.Text, "m").String())
			want := "{ \"z\" : 1.50, \"a\":12345678901234567890,\n\"m\":\"bad [REDACTED] end\\nline\", " + other + " }"
			assert.Equal(t, want, got.Text, "bytes outside the edited value must not change")
		})
	}

	t.Run("object keys and nested values", func(t *testing.T) {
		body := `{"` + lower + `":1,"list":[{"m":"` + lower + `"}]}`
		got := SanitizeBody([]byte(body), "", false, secrets)
		assert.Equal(t, `{"[REDACTED]":1,"list":[{"m":"[REDACTED]"}]}`, got.Text)
	})

	t.Run("escaped strings without a secret stay as written", func(t *testing.T) {
		body := `{"m":"caf` + jsonUnicode("00e9") + ` \/ line\n"}`
		assert.Equal(t, body, SanitizeBody([]byte(body), "", false, secrets).Text)
	})
}

func TestNewSecretsShortCredentialLeaves(t *testing.T) {
	account := `{"type":"service_account","region":"us-east-1","api_key":"abcd1234","Client_SECRET":"efgh5678","Password":"ijkl9012","authToken":"mnop3456","myCredential":"qrst7890","tiny_key":"abc1234","keys":["uvwx1234"]}`
	secrets := NewSecrets(account)

	for _, value := range []string{"abcd1234", "efgh5678", "ijkl9012", "mnop3456", "qrst7890", "uvwx1234"} {
		t.Run(value, func(t *testing.T) {
			assert.Equal(t, "x [REDACTED] y", SanitizeBody([]byte("x "+value+" y"), "", false, secrets).Text)
		})
	}
	for _, value := range []string{"service_account", "us-east-1", "abc1234"} {
		t.Run("stays "+value, func(t *testing.T) {
			text := "x " + value + " y"
			assert.Equal(t, text, SanitizeBody([]byte(text), "", false, secrets).Text)
		})
	}
}

func TestSanitizeDataURLMustHoldBase64(t *testing.T) {
	hidden := "ignore this: it is plain text that must stay visible in the logs"
	for name, body := range map[string]string{
		"text after the prefix": `{"a":"data:image/png;base64,` + hidden + `"}`,
		"in a known field":      `{"image_url":{"url":"data:image/png;base64,` + hidden + `"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			got := SanitizeBody([]byte(body), "", false, nil)
			assert.Equal(t, body, got.Text)
			assert.Zero(t, got.Meta.MediaOmitted)
		})
	}

	t.Run("wrapped base64 is still media", func(t *testing.T) {
		body := `{"a":"data:image/png;base64,` + base64Text(40) + `\r\n` + base64Text(40) + `\n"}`
		got := SanitizeBody([]byte(body), "", false, nil)
		assert.Equal(t, `{"a":"[base64 image/png 60 bytes omitted]"}`, got.Text)
		assert.Equal(t, 1, got.Meta.MediaOmitted)
	})
}

func TestSanitizeShortenedStringKeepsSurrogatePairs(t *testing.T) {
	upper := jsonUnicode("D83D") + jsonUnicode("DE00")
	for _, text := range []string{strings.Repeat(surrogatePair, 12), strings.Repeat(upper, 12), "ab" + strings.Repeat(surrogatePair, 12) + "c"} {
		for limit := 2; limit < len(text); limit++ {
			got := gjson.Parse(`"` + shortenedString(text, limit) + `"`).String()
			require.NotContains(t, got, "\uFFFD", "limit %d", limit)
		}
	}
}
