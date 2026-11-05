package logcontent

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubWriter accepts at most accept bytes per write and reports err for the
// rest, like a connection that fails mid-write.
type stubWriter struct {
	gin.ResponseWriter
	accept   int
	err      error
	flushErr error
	deadline time.Time
}

func (w *stubWriter) Write(p []byte) (int, error) {
	if w.accept >= 0 && len(p) > w.accept {
		n, _ := w.ResponseWriter.Write(p[:w.accept])
		return n, w.err
	}
	return w.ResponseWriter.Write(p)
}

func (w *stubWriter) WriteString(s string) (int, error) {
	if w.accept >= 0 && len(s) > w.accept {
		n, _ := w.ResponseWriter.WriteString(s[:w.accept])
		return n, w.err
	}
	return w.ResponseWriter.WriteString(s)
}

func (w *stubWriter) FlushError() error { return w.flushErr }

func (w *stubWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}

func newCaptureContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions?key=secret", strings.NewReader(`{"model":"m"}`))
	c.Set(common.RequestIdKey, "req-"+t.Name())
	return c, recorder
}

func withCaptureEnabled(t *testing.T, enabled bool) {
	t.Helper()
	previous := common.LogContentCaptureEnabled
	common.LogContentCaptureEnabled = enabled
	t.Cleanup(func() { common.LogContentCaptureEnabled = previous })
}

func eligibleAttempt() AttemptInfo {
	return AttemptInfo{Eligible: true, ChannelID: 3, MultiKeyIndex: -1, ChannelKey: "sk-channel-secret-0123456789", Format: FormatOpenAI}
}

func upstreamRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://upstream.example/v1/chat/completions?api-key=hidden", strings.NewReader(body))
	require.NoError(t, err)
	return req
}

func takeRecord(t *testing.T, p *pipeline) *captureRecord {
	t.Helper()
	select {
	case rec := <-p.queue:
		return rec
	case <-time.After(5 * time.Second):
		t.Fatal("no record was enqueued")
		return nil
	}
}

func TestClientWriterPassesThroughExactly(t *testing.T) {
	withCaptureEnabled(t, true)
	c, recorder := newCaptureContext(t)
	failure := errors.New("short write")
	stub := &stubWriter{ResponseWriter: c.Writer, accept: 4, err: failure}
	c.Writer = stub
	capture := Begin(c, types.RelayFormatOpenAI)
	require.NotNil(t, capture)
	defer capture.Finish()

	n, err := c.Writer.Write([]byte("abcdefgh"))
	assert.Equal(t, 4, n)
	assert.Same(t, failure, err)
	n, err = c.Writer.WriteString("ijklmnop")
	assert.Equal(t, 4, n)
	assert.Same(t, failure, err)
	stub.accept = -1
	n, err = c.Writer.Write([]byte("full"))
	assert.Equal(t, 4, n)
	assert.NoError(t, err)
	n, err = c.Writer.WriteString("text")
	assert.Equal(t, 4, n)
	assert.NoError(t, err)

	assert.Equal(t, "abcdijklfulltext", recorder.Body.String())
	assert.Equal(t, "abcdijklfulltext", string(capture.ClientCopy()))
}

func TestClientWriterUnwrapReachesFlushError(t *testing.T) {
	withCaptureEnabled(t, true)
	c, _ := newCaptureContext(t)
	failure := errors.New("flush failed")
	c.Writer = &stubWriter{ResponseWriter: c.Writer, accept: -1, flushErr: failure}
	capture := Begin(c, types.RelayFormatOpenAI)
	require.NotNil(t, capture)
	defer capture.Finish()

	// The same walk as relay/helper.FlushWriter.
	var writer http.ResponseWriter = c.Writer
	var got error
	for {
		if flusher, ok := writer.(interface{ FlushError() error }); ok {
			got = flusher.FlushError()
			break
		}
		unwrapper, ok := writer.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			break
		}
		writer = unwrapper.Unwrap()
	}
	assert.Same(t, failure, got)
}

func TestClientWriterPreservesWriteDeadline(t *testing.T) {
	withCaptureEnabled(t, true)
	c, _ := newCaptureContext(t)
	stub := &stubWriter{ResponseWriter: c.Writer, accept: -1}
	c.Writer = stub
	capture := Begin(c, types.RelayFormatOpenAI)
	require.NotNil(t, capture)
	defer capture.Finish()

	deadline := time.Now().Add(time.Minute)
	require.NoError(t, http.NewResponseController(c.Writer).SetWriteDeadline(deadline))
	assert.True(t, stub.deadline.Equal(deadline))
}

type erroringBody struct {
	reads    int
	readErr  error
	closeErr error
}

func (b *erroringBody) Read(p []byte) (int, error) {
	b.reads++
	if b.reads == 1 {
		return copy(p, "abc"), nil
	}
	return copy(p, "de"), b.readErr
}

func (b *erroringBody) Close() error { return b.closeErr }

func TestBodyTapPassesThroughReadErrors(t *testing.T) {
	withCaptureEnabled(t, true)
	c, _ := newCaptureContext(t)
	capture := Begin(c, types.RelayFormatOpenAI)
	require.NotNil(t, capture)
	defer capture.Finish()

	attempt := BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{"a":1}`))
	require.NotNil(t, attempt)
	body := &erroringBody{readErr: errors.New("connection reset"), closeErr: errors.New("close failed")}
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: body}
	out := attempt.ObserveResponse(resp, nil)
	require.Same(t, resp, out)

	buf := make([]byte, 16)
	n, err := out.Body.Read(buf)
	assert.Equal(t, 3, n)
	assert.NoError(t, err)
	n, err = out.Body.Read(buf)
	assert.Equal(t, 2, n)
	assert.Same(t, body.readErr, err)
	assert.Same(t, body.closeErr, out.Body.Close())

	assert.Equal(t, "abcde", string(attempt.response.peek()))
}

func TestObserveResponseLeavesNoBodyAndErrorsAlone(t *testing.T) {
	withCaptureEnabled(t, true)
	c, _ := newCaptureContext(t)
	capture := Begin(c, types.RelayFormatOpenAI)
	defer capture.Finish()

	attempt := BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{}`))
	resp := &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}
	assert.Equal(t, http.NoBody, attempt.ObserveResponse(resp, nil).Body)

	failed := BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{}`))
	assert.Nil(t, failed.ObserveResponse(nil, errors.New("dial failed")))
}

func sseFrame(i int) string {
	return fmt.Sprintf("data: {\"id\":\"frame-%08d\",\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n", i)
}

func TestTapSSEKeepsHeadAndTailOnly(t *testing.T) {
	before := budgetInUse.Load()
	buf := newTapBuffer(true, nil)
	total := 0
	var chunk strings.Builder
	frames := 0
	for total < 100<<20 {
		chunk.Reset()
		for chunk.Len() < 32<<10 {
			chunk.WriteString(sseFrame(frames))
			frames++
		}
		buf.write([]byte(chunk.String()))
		total += chunk.Len()
	}
	buf.write([]byte("data: [DONE]\n\n"))
	total += len("data: [DONE]\n\n")

	retained := cap(buf.data) + cap(buf.tail)
	assert.LessOrEqual(t, retained, sseHeadBytes+sseTailBytes)
	assert.LessOrEqual(t, budgetInUse.Load()-before, int64(sseHeadBytes+sseTailBytes))

	body := buf.snapshot("")
	defer release(buf.takeReserved())
	text := string(body.Data)
	assert.True(t, strings.HasPrefix(text, sseFrame(0)))
	assert.True(t, strings.HasSuffix(text, "data: [DONE]\n\n"))
	assert.True(t, body.Truncated)
	assert.False(t, body.HeadOnly)
	assert.Equal(t, int64(total), body.OriginalBytes)
	assert.LessOrEqual(t, len(body.Data), sseHeadBytes+sseTailBytes)
	line := regexp.MustCompile(`^(data: \{"id":"frame-\d{8}","choices":\[\{"delta":\{"content":"x"\}\}\]\}|data: \[DONE\]|)$`)
	for l := range strings.SplitSeq(text, "\n") {
		require.Regexp(t, line, l, "every line is whole")
	}
}

func TestTapNonSSEDropsToHeadOver8MiB(t *testing.T) {
	before := budgetInUse.Load()
	buf := newTapBuffer(false, nil)
	piece := bytes.Repeat([]byte("a"), 64<<10)
	for range (8 << 20) / len(piece) {
		buf.write(piece)
	}
	assert.Len(t, buf.data, 8<<20)
	assert.False(t, buf.headOnly)
	assert.Equal(t, int64(8<<20), budgetInUse.Load()-before)

	buf.write([]byte("b"))
	assert.True(t, buf.headOnly)
	assert.Len(t, buf.data, maxStoredBody+tapMarginBytes)
	assert.Equal(t, int64(maxStoredBody+tapMarginBytes), budgetInUse.Load()-before, "the rest of the budget is returned")

	body := buf.snapshot("")
	release(buf.takeReserved())
	assert.True(t, body.HeadOnly)
	assert.Equal(t, int64(8<<20+1), body.OriginalBytes)
	assert.Equal(t, before, budgetInUse.Load())
}

func TestTapHeadOnlyStopsWithoutBudget(t *testing.T) {
	withBudgetLimit(t, budgetInUse.Load()+64<<10)
	buf := newTapBuffer(false, nil)
	buf.write(bytes.Repeat([]byte("a"), 32<<10))
	buf.write(bytes.Repeat([]byte("b"), 128<<10))
	body := buf.snapshot("")
	release(buf.takeReserved())
	assert.True(t, body.HeadOnly)
	assert.True(t, body.Truncated)
	assert.Equal(t, int64(160<<10), body.OriginalBytes)
}

func TestTapRedactsKeyStraddlingTheCut(t *testing.T) {
	key := "sk-straddling-secret-key-0123456789abcdef"
	buf := newTapBuffer(false, nil)
	head := bytes.Repeat([]byte("a"), maxStoredBody-len(key)/2)
	buf.write(head)
	buf.write([]byte(key))
	filler := bytes.Repeat([]byte("z"), 1<<20)
	for range 9 {
		buf.write(filler)
	}
	body := buf.snapshot("")
	defer release(buf.takeReserved())
	require.True(t, body.HeadOnly)

	stored, meta := storeBody(body, NewSecrets(key))
	reader, err := gzip.NewReader(bytes.NewReader(stored))
	require.NoError(t, err)
	text, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.True(t, meta.HeadOnly)
	assert.NotContains(t, string(text), key[:len(key)/2])
	assert.NotContains(t, string(text), "sk-straddling")
}

func TestLateWriteAfterFinishIsIgnored(t *testing.T) {
	withCaptureEnabled(t, true)
	newTestPipeline(t, true)
	c, _ := newCaptureContext(t)
	capture := Begin(c, types.RelayFormatOpenAI)
	attempt := BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{}`))
	pr, pw := io.Pipe()
	resp := attempt.ObserveResponse(&http.Response{StatusCode: 200, Header: http.Header{}, Body: pr}, nil)
	capture.OnLogRow(&model.Log{UserId: 1, Type: model.LogTypeConsume})

	var wg sync.WaitGroup
	stop := make(chan struct{})
	writer := c.Writer
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				n, err := writer.Write([]byte("chunk"))
				assert.Equal(t, 5, n)
				assert.NoError(t, err)
			}
		}
	})
	wg.Go(func() {
		buf := make([]byte, 64)
		for {
			if _, err := resp.Body.Read(buf); err != nil {
				return
			}
		}
	})
	wg.Go(func() {
		for {
			select {
			case <-stop:
				_ = pw.Close()
				return
			default:
				_, _ = pw.Write([]byte("upstream"))
			}
		}
	})
	time.Sleep(10 * time.Millisecond)
	capture.Finish()
	clientLen := len(capture.ClientCopy())
	time.Sleep(10 * time.Millisecond)
	close(stop)
	wg.Wait()
	assert.Equal(t, clientLen, len(capture.ClientCopy()), "writes after Finish are not copied")
	assert.Nil(t, BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{}`)), "no attempt starts after Finish")
}

func TestFinishDoesNotSwallowRelayPanic(t *testing.T) {
	withCaptureEnabled(t, true)
	c, _ := newCaptureContext(t)
	recovered := func() (r any) {
		defer func() { r = recover() }()
		func() {
			defer Begin(c, types.RelayFormatOpenAI).Finish()
			panic("relay boom")
		}()
		return nil
	}()
	assert.Equal(t, "relay boom", recovered)
}

func TestCaptureInternalPanicDoesNotAffectCall(t *testing.T) {
	withCaptureEnabled(t, true)
	p := newTestPipeline(t, true)
	c, recorder := newCaptureContext(t)
	capture := Begin(c, types.RelayFormatOpenAI)
	require.NotNil(t, capture)
	testCopyHook = func() { panic("copy boom") }
	t.Cleanup(func() { testCopyHook = nil })

	n, err := c.Writer.Write([]byte("hello"))
	assert.Equal(t, 5, n)
	assert.NoError(t, err)
	n, err = c.Writer.WriteString(" world")
	assert.Equal(t, 6, n)
	assert.NoError(t, err)
	assert.Equal(t, "hello world", recorder.Body.String())

	capture.OnLogRow(&model.Log{UserId: 1, Type: model.LogTypeConsume})
	assert.NotPanics(t, capture.Finish)
	assert.Empty(t, p.queue, "a disabled capture stores nothing")
}

func TestBeginDisabledInstallsNothing(t *testing.T) {
	withCaptureEnabled(t, false)
	c, _ := newCaptureContext(t)
	writer := c.Writer
	capture := Begin(c, types.RelayFormatOpenAI)
	assert.Nil(t, capture)
	assert.Same(t, writer, c.Writer)
	assert.NotPanics(t, capture.Finish)
	assert.NotPanics(t, func() { capture.OnLogRow(&model.Log{}) })

	attempt := BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{}`))
	assert.Nil(t, attempt)
	resp := &http.Response{Body: io.NopCloser(strings.NewReader("x"))}
	assert.Same(t, resp, attempt.ObserveResponse(resp, nil))
}

func TestBeginSkipsRealtime(t *testing.T) {
	withCaptureEnabled(t, true)
	c, _ := newCaptureContext(t)
	writer := c.Writer
	assert.Nil(t, Begin(c, types.RelayFormatOpenAIRealtime))
	assert.Same(t, writer, c.Writer)
}

func TestFinishStoresClientAndLoggedAttempts(t *testing.T) {
	withCaptureEnabled(t, true)
	p := newTestPipeline(t, true)
	c, _ := newCaptureContext(t)
	storage, err := common.CreateBodyStorage([]byte(`{"model":"m","messages":[]}`))
	require.NoError(t, err)
	c.Set(common.KeyBodyStorage, storage)
	c.Set("token_key", "token-secret-abcdefghijkl")
	capture := Begin(c, types.RelayFormatOpenAI)

	first := BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{"try":1}`))
	resp := first.ObserveResponse(&http.Response{StatusCode: 429, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":"busy"}`))}, nil)
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	capture.OnLogRow(&model.Log{UserId: 9, Type: model.LogTypeError, Content: "busy"})

	unlogged := BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{"try":2}`))
	resp = unlogged.ObserveResponse(&http.Response{StatusCode: 500, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil)
	_, _ = io.ReadAll(resp.Body)

	last := BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{"try":3}`))
	resp = last.ObserveResponse(&http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil)
	_, _ = io.ReadAll(resp.Body)
	c.JSON(http.StatusOK, gin.H{"ok": true})
	capture.OnLogRow(&model.Log{UserId: 9, Type: model.LogTypeConsume})
	capture.Finish()

	rec := takeRecord(t, p)
	assert.Equal(t, "req-"+t.Name(), rec.RequestId)
	assert.Equal(t, 9, rec.UserId)
	assert.Contains(t, rec.Secrets, "token-secret-abcdefghijkl")
	assert.Contains(t, rec.Secrets, "sk-channel-secret-0123456789")
	require.Len(t, rec.Attempts, 3)
	client := rec.Attempts[0]
	assert.Equal(t, 0, client.Attempt)
	assert.Equal(t, `{"model":"m","messages":[]}`, string(client.Request.Data))
	assert.Equal(t, `{"ok":true}`, string(client.Response.Data))
	assert.Equal(t, http.StatusOK, client.Meta.HttpStatus)
	assert.Equal(t, FormatOpenAI, client.Meta.RelayFormat)
	assert.Equal(t, "/v1/chat/completions", client.Meta.Path)

	assert.Equal(t, 1, rec.Attempts[1].Attempt)
	assert.Equal(t, 429, rec.Attempts[1].Meta.HttpStatus)
	assert.Equal(t, "busy", rec.Attempts[1].Meta.ErrorMessage)
	assert.Equal(t, `{"try":1}`, string(rec.Attempts[1].Request.Data))
	assert.Equal(t, "/v1/chat/completions", rec.Attempts[1].Meta.Path)
	assert.Equal(t, http.MethodPost, rec.Attempts[1].Meta.Method)
	assert.Equal(t, 3, rec.Attempts[1].Meta.ChannelId)
	assert.Equal(t, 3, rec.Attempts[2].Attempt)
	assert.Equal(t, `{"ok":true}`, string(rec.Attempts[2].Response.Data))
	assert.Positive(t, rec.reserved)
	release(rec.reserved)
}

func TestFinishWithoutLogRowStoresNothing(t *testing.T) {
	withCaptureEnabled(t, true)
	p := newTestPipeline(t, true)
	before := budgetInUse.Load()
	c, _ := newCaptureContext(t)
	capture := Begin(c, types.RelayFormatOpenAI)
	attempt := BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{"a":1}`))
	resp := attempt.ObserveResponse(&http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil)
	_, _ = io.ReadAll(resp.Body)
	c.JSON(http.StatusOK, gin.H{})
	capture.Finish()
	assert.Empty(t, p.queue)
	assert.Equal(t, before, budgetInUse.Load(), "every reservation is returned")
}

func TestFinishIneligibleWritesMarker(t *testing.T) {
	withCaptureEnabled(t, true)
	p := newTestPipeline(t, true)
	c, _ := newCaptureContext(t)
	capture := Begin(c, types.RelayFormatOpenAI)
	attempt := BeginAttempt(c, AttemptInfo{ChannelID: 1, MultiKeyIndex: -1}, upstreamRequest(t, `{}`))
	assert.Nil(t, attempt)
	c.JSON(http.StatusOK, gin.H{})
	capture.OnLogRow(&model.Log{UserId: 2, Type: model.LogTypeConsume})
	capture.Finish()

	rec := takeRecord(t, p)
	require.Len(t, rec.Attempts, 1)
	assert.True(t, rec.Attempts[0].NotCaptured)
	assert.Empty(t, rec.Attempts[0].Request.Data)
	assert.Empty(t, rec.Attempts[0].Response.Data)
	release(rec.reserved)
}

func TestFinishAssemblesClientStream(t *testing.T) {
	withCaptureEnabled(t, true)
	p := newTestPipeline(t, true)
	c, _ := newCaptureContext(t)
	capture := Begin(c, types.RelayFormatOpenAI)
	BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{"stream":true}`))
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	for _, frame := range []string{
		`data: {"choices":[{"index":0,"delta":{"role":"assistant","content":"Hel"}}]}` + "\n\n",
		`data: {"choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":"stop"}]}` + "\n\n",
		"data: [DONE]\n\n",
	} {
		_, _ = c.Writer.WriteString(frame)
	}
	capture.OnLogRow(&model.Log{UserId: 2, Type: model.LogTypeConsume})
	capture.Finish()

	rec := takeRecord(t, p)
	client := rec.Attempts[0]
	assert.True(t, client.Meta.IsStream)
	assert.False(t, client.Meta.Incomplete)
	require.Len(t, client.Assembled, 1)
	require.NotEmpty(t, client.Assembled[0].Blocks)
	assert.Equal(t, "Hello", client.Assembled[0].Blocks[0].Text)
	release(rec.reserved)
}

func TestFinishMarksStreamWithoutTerminalIncomplete(t *testing.T) {
	withCaptureEnabled(t, true)
	p := newTestPipeline(t, true)
	c, _ := newCaptureContext(t)
	capture := Begin(c, types.RelayFormatOpenAI)
	BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{"stream":true}`))
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	_, _ = c.Writer.WriteString(`data: {"choices":[{"index":0,"delta":{"content":"Hel"}}]}` + "\n\n")
	capture.OnLogRow(&model.Log{UserId: 2, Type: model.LogTypeConsume})
	capture.Finish()

	rec := takeRecord(t, p)
	assert.True(t, rec.Attempts[0].Meta.Incomplete)
	release(rec.reserved)
}

func TestUpstreamRequestHeadOnlyOverParseLimit(t *testing.T) {
	withCaptureEnabled(t, true)
	p := newTestPipeline(t, true)
	c, _ := newCaptureContext(t)
	capture := Begin(c, types.RelayFormatOpenAI)
	large := strings.Repeat("x", maxParsedBody+10)
	attempt := BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, large))
	require.NotNil(t, attempt)
	capture.OnLogRow(&model.Log{UserId: 2, Type: model.LogTypeConsume})
	capture.Finish()

	rec := takeRecord(t, p)
	upstream := rec.Attempts[1].Request
	assert.True(t, upstream.HeadOnly)
	assert.Equal(t, int64(len(large)), upstream.OriginalBytes)
	assert.LessOrEqual(t, len(upstream.Data), maxStoredBody+tapMarginBytes)
	release(rec.reserved)
}

func TestUpstreamRequestWithoutGetBodyIsUnavailable(t *testing.T) {
	withCaptureEnabled(t, true)
	p := newTestPipeline(t, true)
	c, _ := newCaptureContext(t)
	capture := Begin(c, types.RelayFormatOpenAI)
	req := upstreamRequest(t, `{"a":1}`)
	req.GetBody = nil
	attempt := BeginAttempt(c, eligibleAttempt(), req)
	require.NotNil(t, attempt)
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"a":1}`, string(body), "the request body is left untouched")
	capture.OnLogRow(&model.Log{UserId: 2, Type: model.LogTypeConsume})
	capture.Finish()

	rec := takeRecord(t, p)
	assert.True(t, rec.Attempts[1].Meta.RequestUnavailable)
	assert.Empty(t, rec.Attempts[1].Request.Data)
	release(rec.reserved)
}

func TestFinishMarksProviderCutIncomplete(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider string
		cut      bool
	}{
		{"cut", `data: {"choices":[{"index":0,"delta":{"content":"Hel"}}]}` + "\n\n", true},
		{"done", `data: {"choices":[{"index":0,"delta":{"content":"Hel"}}]}` + "\n\ndata: [DONE]\n\n", false},
		{"claude", "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withCaptureEnabled(t, true)
			p := newTestPipeline(t, true)
			c, _ := newCaptureContext(t)
			capture := Begin(c, types.RelayFormatOpenAI)
			attempt := BeginAttempt(c, eligibleAttempt(), upstreamRequest(t, `{"stream":true}`))
			header := http.Header{"Content-Type": {"text/event-stream"}}
			resp := attempt.ObserveResponse(&http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(tc.provider))}, nil)
			_, _ = io.ReadAll(resp.Body)
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			// The gateway closes the client stream cleanly either way.
			_, _ = c.Writer.WriteString(`data: {"choices":[{"index":0,"delta":{"content":"Hel"},"finish_reason":"stop"}]}` + "\n\ndata: [DONE]\n\n")
			capture.OnLogRow(&model.Log{UserId: 2, Type: model.LogTypeConsume})
			capture.Finish()

			rec := takeRecord(t, p)
			require.Len(t, rec.Attempts, 2)
			assert.Equal(t, tc.cut, rec.Attempts[0].Meta.Incomplete)
			assert.Equal(t, tc.cut, rec.Attempts[1].Meta.Incomplete)
			release(rec.reserved)
		})
	}
}

func TestBuildRowsMarksTruncatedAssembly(t *testing.T) {
	for _, tc := range []struct {
		name      string
		assembled []ViewMessage
		dropped   bool
		want      bool
	}{
		{"whole", []ViewMessage{{Role: roleAssistant, Blocks: []ViewBlock{{Type: blockText, Text: "hi"}}}}, false, false},
		{"lost block", []ViewMessage{{Role: roleAssistant, Blocks: []ViewBlock{{Type: blockText, Text: "hi"}}}}, true, true},
		{"trimmed", []ViewMessage{{Role: roleAssistant, Blocks: []ViewBlock{{Type: blockText, Text: strings.Repeat("hello world ", maxAssembled/10)}}}}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := buildRows(&captureRecord{RequestId: "r", Attempts: []captureAttempt{{
				Response:           capturedBody{Data: []byte("data: x\n\n")},
				Assembled:          tc.assembled,
				AssembledTruncated: tc.dropped,
			}}})
			require.Len(t, rows, 1)
			var meta model.LogContentMeta
			require.NoError(t, common.UnmarshalJsonStr(rows[0].Meta, &meta))
			assert.Equal(t, tc.want, meta.Response.Truncated)
		})
	}
}
