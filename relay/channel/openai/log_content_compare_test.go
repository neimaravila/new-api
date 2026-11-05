package openai

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/logcontent"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixed provider frames: ids and timestamps never change between runs, so the
// two modes can be compared byte for byte.
const (
	lccChatRole    = `{"id":"chatcmpl-lcc","object":"chat.completion.chunk","created":1767225600,"model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}`
	lccChatText    = `{"id":"chatcmpl-lcc","object":"chat.completion.chunk","created":1767225600,"model":"gpt-test","choices":[{"index":0,"delta":{"content":"hello world"}}]}`
	lccChatFinish  = `{"id":"chatcmpl-lcc","object":"chat.completion.chunk","created":1767225600,"model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`
	lccChatUsage   = `{"id":"chatcmpl-lcc","object":"chat.completion.chunk","created":1767225600,"model":"gpt-test","choices":[],"usage":{"prompt_tokens":9,"completion_tokens":2,"total_tokens":11}}`
	lccNonStandard = `{"provider_event":"metrics","latency_ms":12}`
	lccChatFull    = `{"id":"chatcmpl-lcc","object":"chat.completion","created":1767225600,"model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":"hello world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":2,"total_tokens":11}}`
	lccRateLimit   = `{"error":{"message":"Rate limit reached for gpt-test","type":"requests","param":null,"code":"rate_limit_exceeded"}}`
	lccChatRequest = `{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	lccRespRequest = `{"model":"gpt-test","stream":true,"input":"hi"}`
)

const lccResponsesStream = "event: response.created\n" +
	`data: {"type":"response.created","sequence_number":0,"response":{"id":"resp_lcc","object":"response","created_at":1767225600,"status":"in_progress","model":"gpt-test","output":[]}}` + "\n\n" +
	"event: response.output_text.delta\n" +
	`data: {"type":"response.output_text.delta","sequence_number":1,"item_id":"msg_lcc","output_index":0,"content_index":0,"delta":"hello world"}` + "\n\n" +
	"event: response.completed\n" +
	`data: {"type":"response.completed","sequence_number":2,"response":{"id":"resp_lcc","object":"response","created_at":1767225600,"status":"completed","model":"gpt-test","output":[{"id":"msg_lcc","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello world","annotations":[]}]}],"usage":{"input_tokens":9,"output_tokens":2,"total_tokens":11}}}` + "\n\n"

type lccCase struct {
	name         string
	clientPath   string
	upstreamPath string
	format       types.RelayFormat
	mode         int
	nonStream    bool
	includeUsage bool
	override     map[string]any
	request      string
	provider     http.HandlerFunc
	// wantStatus and want pin what the case delivers, so a broken fixture
	// cannot pass by producing the same nothing twice.
	wantStatus int
	want       string
}

type lccResult struct {
	status         int
	header         http.Header
	body           string
	clientCopied   bool
	providerTapped bool
}

func lccSSE(frames ...string) string {
	var b strings.Builder
	for _, frame := range frames {
		b.WriteString("data: " + frame + "\n\n")
	}
	return b.String()
}

// lccWriteParts writes each part in its own flushed chunk, so the relay reads
// them separately.
func lccWriteParts(w http.ResponseWriter, contentType string, parts ...string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	flusher := w.(http.Flusher)
	for _, part := range parts {
		_, _ = io.WriteString(w, part)
		flusher.Flush()
		time.Sleep(5 * time.Millisecond)
	}
}

func lccStreamProvider(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		lccWriteParts(w, "text/event-stream", body)
	}
}

// lccServe runs one call the way the relay does: capture hook, provider
// request through DoApiRequest, client write through DoResponse, or the
// controller's error write when the provider answers with an error.
func lccServe(t *testing.T, tc lccCase, capture bool, ping time.Duration) lccResult {
	t.Helper()
	common.LogContentCaptureEnabled = capture

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A fixed Date keeps the passed-through provider headers equal across runs.
		w.Header().Set("Date", "Thu, 01 Jan 2026 00:00:00 GMT")
		tc.provider(w, r)
	}))
	defer server.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, tc.clientPath, strings.NewReader(tc.request))
	c.Request.Header.Set("Content-Type", "application/json")

	logCapture := logcontent.Begin(c, tc.format)
	require.Equal(t, capture, logCapture != nil)
	defer logCapture.Finish()

	if ping > 0 {
		stop := helper.StartStreamKeepAlive(c, ping)
		defer stop()
	}

	info := &relaycommon.RelayInfo{
		RelayFormat:        tc.format,
		RelayMode:          tc.mode,
		IsStream:           !tc.nonStream,
		ShouldIncludeUsage: tc.includeUsage,
		RequestURLPath:     tc.upstreamPath,
		OriginModelName:    "gpt-test",
		StartTime:          time.Unix(1767225600, 0),
		StreamStatus:       relaycommon.NewStreamStatus(),
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			ApiType:           constant.APITypeOpenAI,
			ChannelBaseUrl:    server.URL,
			ApiKey:            "sk-lcc-compare-0123456789abcdef",
			UpstreamModelName: "gpt-test",
			HeadersOverride:   tc.override,
		},
	}
	if tc.format == types.RelayFormatClaude {
		info.ClaudeConvertInfo = &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone}
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	resp, err := channel.DoApiRequest(adaptor, c, info, strings.NewReader(tc.request))
	require.NoError(t, err)
	result := lccResult{providerTapped: strings.Contains(fmt.Sprintf("%T", resp.Body), "logcontent.")}

	var apiErr *types.NewAPIError
	if resp.StatusCode != http.StatusOK {
		apiErr = service.RelayErrorHandler(c.Request.Context(), resp, false)
	} else {
		_, apiErr = adaptor.DoResponse(c, resp, info)
	}
	if apiErr != nil {
		if c.Writer.Written() && strings.HasPrefix(c.Writer.Header().Get("Content-Type"), "text/event-stream") {
			_ = helper.ObjectData(c, gin.H{"error": apiErr.ToOpenAIError()})
		} else {
			c.Header("Content-Type", "application/json; charset=utf-8")
			c.JSON(apiErr.StatusCode, gin.H{"error": apiErr.ToOpenAIError()})
		}
	}

	result.clientCopied = len(logCapture.ClientCopy()) > 0
	result.status = recorder.Code
	result.header = recorder.Header().Clone()
	result.header.Del("X-Oneapi-Request-Id")
	result.body = recorder.Body.String()
	return result
}

func lccKeepSettings(t *testing.T) {
	previousCapture := common.LogContentCaptureEnabled
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		common.LogContentCaptureEnabled = previousCapture
		constant.StreamingTimeout = previousTimeout
	})
}

func TestLogContentCaptureDoesNotChangeClientBytes(t *testing.T) {
	lccKeepSettings(t)

	multibyte := lccSSE(lccChatRole,
		`{"id":"chatcmpl-lcc","object":"chat.completion.chunk","created":1767225600,"model":"gpt-test","choices":[{"index":0,"delta":{"content":"olá 世界 🙂"}}]}`,
		lccChatFinish) + "data: [DONE]\n\n"
	cutAt := strings.Index(multibyte, "世") + 1
	emojiAt := strings.Index(multibyte, "🙂") + 2

	cases := []lccCase{
		{
			name:     "openai stream",
			want:     "\"hello world\"}}]}\n\ndata: {",
			provider: lccStreamProvider(lccSSE(lccChatRole, lccChatText, lccChatFinish) + "data: [DONE]\n\n"),
		},
		{
			name:         "openai stream with usage-only last frame",
			want:         "data: " + lccChatUsage + "\n\ndata: [DONE]",
			includeUsage: true,
			provider:     lccStreamProvider(lccSSE(lccChatRole, lccChatText, lccChatFinish, lccChatUsage) + "data: [DONE]\n\n"),
		},
		{
			name:         "openai stream with non-standard frame after usage",
			want:         "data: " + lccNonStandard + "\n\ndata: [DONE]",
			includeUsage: true,
			provider:     lccStreamProvider(lccSSE(lccChatRole, lccChatText, lccChatFinish, lccChatUsage, lccNonStandard) + "data: [DONE]\n\n"),
		},
		{
			name:       "openai stream converted to claude",
			want:       "event: message_stop",
			clientPath: "/v1/messages",
			format:     types.RelayFormatClaude,
			provider:   lccStreamProvider(lccSSE(lccChatRole, lccChatText, lccChatFinish, lccChatUsage) + "data: [DONE]\n\n"),
		},
		{
			name:         "responses stream",
			want:         "event: response.completed",
			clientPath:   "/v1/responses",
			upstreamPath: "/v1/responses",
			format:       types.RelayFormatOpenAIResponses,
			mode:         relayconstant.RelayModeResponses,
			request:      lccRespRequest,
			provider:     lccStreamProvider(lccResponsesStream),
		},
		{
			name:      "non stream",
			want:      lccChatFull,
			nonStream: true,
			request:   `{"model":"gpt-test","messages":[{"role":"user","content":"hi"}]}`,
			provider: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, lccChatFull)
			},
		},
		{
			name: "non stream gzip by header override",
			// The relay does not decode a body the override asked compressed;
			// the client sees that error, and it must be the same in both modes.
			wantStatus: http.StatusInternalServerError,
			want:       "bad_response_body",
			nonStream:  true,
			override:   map[string]any{"Accept-Encoding": "gzip"},
			request:    `{"model":"gpt-test","messages":[{"role":"user","content":"hi"}]}`,
			provider: func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
					http.Error(w, "override not applied", http.StatusBadRequest)
					return
				}
				var compressed bytes.Buffer
				zw := gzip.NewWriter(&compressed)
				_, _ = io.WriteString(zw, lccChatFull)
				_ = zw.Close()
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Encoding", "gzip")
				_, _ = w.Write(compressed.Bytes())
			},
		},
		{
			name:       "error 429",
			wantStatus: http.StatusTooManyRequests,
			want:       "rate_limit_exceeded",
			provider: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, lccRateLimit)
			},
		},
		{
			name: "provider cuts the stream",
			want: "\"hello world\"}}]}",
			provider: func(w http.ResponseWriter, _ *http.Request) {
				lccWriteParts(w, "text/event-stream", lccSSE(lccChatRole, lccChatText))
				conn, _, err := http.NewResponseController(w).Hijack()
				if err == nil {
					_ = conn.Close()
				}
			},
		},
		{
			name: "multibyte characters split between reads",
			want: "olá 世界 🙂",
			provider: func(w http.ResponseWriter, _ *http.Request) {
				lccWriteParts(w, "text/event-stream", multibyte[:cutAt], multibyte[cutAt:emojiAt], multibyte[emojiAt:])
			},
		},
	}

	for _, tc := range cases {
		if tc.clientPath == "" {
			tc.clientPath = "/v1/chat/completions"
		}
		if tc.upstreamPath == "" {
			tc.upstreamPath = "/v1/chat/completions"
		}
		if tc.format == "" {
			tc.format = types.RelayFormatOpenAI
		}
		if tc.mode == 0 {
			tc.mode = relayconstant.RelayModeChatCompletions
		}
		if tc.wantStatus == 0 {
			tc.wantStatus = http.StatusOK
		}
		if tc.request == "" {
			tc.request = lccChatRequest
		}
		t.Run(tc.name, func(t *testing.T) {
			off := lccServe(t, tc, false, 0)
			on := lccServe(t, tc, true, 0)

			require.Equal(t, tc.wantStatus, off.status)
			require.Contains(t, off.body, tc.want)
			assert.Equal(t, off.status, on.status)
			assert.Equal(t, off.header, on.header)
			assert.Equal(t, off.body, on.body, "client bytes differ with capture on")
			assert.False(t, off.clientCopied || off.providerTapped, "capture off installs nothing")
			assert.True(t, on.clientCopied, "capture on copied the client response")
			assert.True(t, on.providerTapped, "capture on tapped the provider response")
		})
	}
}

func TestLogContentCaptureKeepsPingDelivery(t *testing.T) {
	lccKeepSettings(t)
	tc := lccCase{
		clientPath:   "/v1/chat/completions",
		upstreamPath: "/v1/chat/completions",
		format:       types.RelayFormatOpenAI,
		mode:         relayconstant.RelayModeChatCompletions,
		request:      lccChatRequest,
		provider: func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(150 * time.Millisecond)
			lccWriteParts(w, "text/event-stream", lccSSE(lccChatRole, lccChatText, lccChatFinish)+"data: [DONE]\n\n")
		},
	}
	withoutPings := func(body string) string {
		return strings.ReplaceAll(body, ": PING\n\n", "")
	}

	off := lccServe(t, tc, false, 20*time.Millisecond)
	on := lccServe(t, tc, true, 20*time.Millisecond)

	for name, result := range map[string]lccResult{"off": off, "on": on} {
		assert.True(t, strings.HasPrefix(result.body, ": PING\n\n"), "%s: pings precede the first data", name)
		assert.NotEqual(t, result.body, withoutPings(result.body), "%s: pings were delivered", name)
	}
	assert.Equal(t, off.status, on.status)
	assert.Equal(t, off.header, on.header)
	assert.Equal(t, withoutPings(off.body), withoutPings(on.body))
	assert.True(t, on.clientCopied)
	assert.True(t, on.providerTapped)
}
