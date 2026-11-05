package logcontent

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

const captureContextKey = "log_content_capture"

// Capture follows one call from controller.Relay. Every method accepts a nil
// receiver, recovers its own panics and never changes what the call does: a
// failure only loses the content.
type Capture struct {
	c         *gin.Context
	startedAt time.Time
	writer    *clientWriter
	response  *tapBuffer
	disabled  atomic.Bool
	// skip: nothing more of the client response is copied.
	skip atomic.Bool

	// mu guards everything below and the start of the stream fold.
	mu           sync.Mutex
	finished     bool
	sseChecked   bool
	sse          bool
	format       string
	eligible     bool
	ineligible   bool
	attempts     []*Attempt
	current      *Attempt
	logged       bool
	userID       int
	errorMessage string
	writeFailed  bool
	fold         *streamFold
	foldTried    bool
	foldReserved int64
}

// AttemptInfo describes one call to a provider, as relay/channel sees it.
type AttemptInfo struct {
	Eligible      bool
	ChannelID     int
	MultiKeyIndex int
	ChannelKey    string
	Format        string
	// SecretHeaders names headers set by the channel's header override; their
	// values sent upstream are redacted from every stored body.
	SecretHeaders []string
	// StreamEnd tells, once the call is over, how the relay ended reading the
	// provider stream. Nil means unknown.
	StreamEnd func() StreamEnd
	// KnownStreamFormat: the provider streams OpenAI, Responses, Claude or
	// Gemini frames, so a stream without their end frame was cut.
	KnownStreamFormat bool
}

// StreamEnd is how the relay stopped reading a provider stream.
type StreamEnd int

const (
	// StreamEndUnknown: the stream ended at EOF, or nothing was recorded.
	StreamEndUnknown StreamEnd = iota
	// StreamEndComplete: the provider sent its end frame, or the handler stopped.
	StreamEndComplete
	// StreamEndCut: timeout, read error, panic, failed ping or client gone.
	StreamEndCut
)

// defaultSecretHeaders carry provider credentials in the common APIs.
var defaultSecretHeaders = []string{"Authorization", "X-Api-Key", "Api-Key", "X-Goog-Api-Key"}

// secretQueryWords mark URL query names whose values are credentials.
var secretQueryWords = []string{"key", "token", "secret", "auth", "sig", "password", "credential"}

// publicOverrideHeaders are override headers known to carry no credential;
// every other override value is treated as one. Names are lower case.
var publicOverrideHeaders = []string{
	"user-agent", "content-type", "accept", "accept-encoding", "accept-language",
	"anthropic-version", "anthropic-beta", "openai-beta", "http-referer", "referer",
	"x-title", "origin", "cache-control", "connection",
}

// Attempt follows one provider call of a Capture.
type Attempt struct {
	capture   *Capture
	index     int
	info      AttemptInfo
	method    string
	path      string
	startedAt time.Time

	request            capturedBody
	requestReserved    int64
	requestUnavailable bool
	// secrets are credential values sent upstream in headers or the query.
	secrets []string

	mu           sync.Mutex
	status       int
	encoding     string
	response     *tapBuffer
	endedAt      time.Time
	cut          bool
	logged       bool
	errorMessage string
}

// Begin starts the capture of a call. With capture off, for realtime calls or
// on any failure it installs nothing and returns nil.
func Begin(c *gin.Context, relayFormat types.RelayFormat) (capture *Capture) {
	defer func() {
		if recover() != nil {
			capture = nil
		}
	}()
	if c == nil || c.Writer == nil || c.Request == nil || !common.LogContentCaptureEnabled ||
		relayFormat == types.RelayFormatOpenAIRealtime {
		return nil
	}
	capture = &Capture{c: c, startedAt: time.Now()}
	capture.writer = &clientWriter{ResponseWriter: c.Writer, capture: capture}
	c.Set(captureContextKey, capture)
	model.SetLogRowObserver(c, capture)
	c.Writer = capture.writer
	return capture
}

// ClientCopy returns what has been copied of the client response so far.
func (cp *Capture) ClientCopy() []byte {
	if cp == nil {
		return nil
	}
	cp.mu.Lock()
	response := cp.response
	cp.mu.Unlock()
	return response.peek()
}

func (cp *Capture) disable() {
	cp.disabled.Store(true)
}

// copying reports whether client writes may still be copied.
func (cp *Capture) copying() bool {
	return cp != nil && !cp.disabled.Load() && !cp.skip.Load()
}

// observeClient copies bytes the client accepted. It runs after every write,
// from the handler or a ping goroutine, and never panics. The lock orders the
// copy as the writes were ordered and keeps Finish from closing the fold while
// a write still feeds it.
func (cp *Capture) observeClient(p []byte, err error) {
	if cp == nil || cp.disabled.Load() {
		return
	}
	defer func() {
		if recover() != nil {
			cp.disable()
		}
	}()
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.finished {
		return
	}
	if err != nil {
		cp.writeFailed = true
	}
	if cp.ineligible || len(p) == 0 {
		return
	}
	if !cp.sseChecked {
		header := cp.writer.Header()
		cp.sseChecked = true
		cp.sse = isEventStream(header.Get("Content-Type"), header.Get("Content-Encoding"))
		cp.response = newTapBuffer(cp.sse, cp.disable)
	}
	cp.response.write(p)
	if fold := cp.startFoldLocked(); fold != nil {
		fold.feed(p)
	}
}

// startFoldLocked starts rebuilding the client stream once it is known to be
// SSE in a format that has messages. Pings sent before the format was known
// are not needed for it.
func (cp *Capture) startFoldLocked() *streamFold {
	if cp.fold != nil || cp.foldTried || !cp.sse || cp.format == "" {
		return cp.fold
	}
	switch cp.format {
	case FormatOpenAI, FormatResponses, FormatClaude, FormatGemini:
	default:
		cp.foldTried = true
		return nil
	}
	cp.foldTried = true
	cost := int64(foldChunkBytes*foldQueueChunks + maxAssembled)
	if !tryReserve(cost) {
		return nil
	}
	cp.foldReserved = cost
	cp.fold = newStreamFold(cp.format)
	fold := cp.fold
	gopool.Go(fold.run)
	return fold
}

// OnLogRow is told about every log row written for the call. A row makes the
// current attempt storable, and any row makes the client level storable.
func (cp *Capture) OnLogRow(log *model.Log) {
	if cp == nil || log == nil {
		return
	}
	defer func() {
		if recover() != nil {
			cp.disable()
		}
	}()
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.finished {
		return
	}
	cp.logged = true
	cp.userID = log.UserId
	isError := log.Type == model.LogTypeError
	if isError {
		cp.errorMessage = log.Content
	}
	if attempt := cp.current; attempt != nil {
		attempt.mu.Lock()
		attempt.logged = true
		if isError {
			attempt.errorMessage = log.Content
		}
		attempt.mu.Unlock()
	}
}

// BeginAttempt starts following one provider call. It copies the request
// through req.GetBody, never through req.Body, and returns nil when there is
// nothing to follow.
func BeginAttempt(c *gin.Context, info AttemptInfo, req *http.Request) (attempt *Attempt) {
	defer func() {
		if recover() != nil {
			attempt = nil
		}
	}()
	if c == nil || req == nil {
		return nil
	}
	value, ok := c.Get(captureContextKey)
	if !ok {
		return nil
	}
	cp, _ := value.(*Capture)
	if cp == nil || cp.disabled.Load() {
		return nil
	}

	attempt, dropped := cp.addAttempt(info, req)
	dropped.discard()
	if attempt == nil {
		return nil
	}
	attempt.secrets = requestSecrets(req, info.SecretHeaders)
	attempt.copyRequest(req)
	return attempt
}

// requestSecrets collects the credential values sent upstream in headers and
// the URL query. They are only used for redaction; headers and URLs are
// never stored. Every override header counts except those known to be
// public, so benign overrides such as User-Agent stay in the bodies.
func requestSecrets(req *http.Request, overrideHeaders []string) []string {
	var secrets []string
	add := func(value string) {
		if value = strings.TrimSpace(value); len(value) >= minSecretBytes {
			secrets = append(secrets, value)
		}
	}
	names := slices.Clone(defaultSecretHeaders)
	for _, name := range overrideHeaders {
		if !isPublicHeader(name) {
			names = append(names, name)
		}
	}
	for _, name := range names {
		for _, value := range req.Header.Values(name) {
			add(value)
			if _, credential, ok := strings.Cut(strings.TrimSpace(value), " "); ok {
				add(credential)
			}
		}
	}
	if req.URL == nil {
		return secrets
	}
	for name, values := range req.URL.Query() {
		if !isSecretName(name, secretQueryWords) {
			continue
		}
		for _, value := range values {
			add(value)
		}
	}
	return secrets
}

func isPublicHeader(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return slices.Contains(publicOverrideHeaders, lower) || strings.HasPrefix(lower, "x-stainless-")
}

func isSecretName(name string, words []string) bool {
	lower := strings.ToLower(name)
	return slices.ContainsFunc(words, func(word string) bool { return strings.Contains(lower, word) })
}

// addAttempt numbers a new attempt and makes it current. An ineligible
// attempt is not followed; before any eligible one it ends the client copy,
// which is returned to be discarded.
func (cp *Capture) addAttempt(info AttemptInfo, req *http.Request) (*Attempt, *tapBuffer) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.finished {
		return nil, nil
	}
	attempt := &Attempt{
		capture:   cp,
		index:     len(cp.attempts) + 1,
		info:      info,
		method:    req.Method,
		startedAt: time.Now(),
	}
	if req.URL != nil {
		attempt.path = req.URL.Path
	}
	cp.attempts = append(cp.attempts, attempt)
	if !info.Eligible {
		cp.current = nil
		if cp.eligible {
			return nil, nil
		}
		cp.ineligible = true
		cp.skip.Store(true)
		dropped := cp.response
		cp.response = nil
		return nil, dropped
	}
	cp.current = attempt
	cp.eligible = true
	if cp.format == "" {
		cp.format = info.Format
	}
	return attempt, nil
}

// copyRequest reads the request through GetBody, an independent reader, so
// the body the transport sends is never touched.
func (a *Attempt) copyRequest(req *http.Request) {
	if req.Body == nil || req.Body == http.NoBody {
		return
	}
	var body io.ReadCloser
	var err error
	if req.GetBody != nil {
		body, err = req.GetBody()
	}
	if req.GetBody == nil || err != nil || body == nil {
		a.mu.Lock()
		a.requestUnavailable = true
		a.mu.Unlock()
		return
	}
	defer body.Close()
	request, reserved := readCopy(body, req.ContentLength, req.Header.Get("Content-Encoding"), a.capture.disable)
	a.mu.Lock()
	a.request, a.requestReserved = request, reserved
	a.mu.Unlock()
}

// readCopy copies a replayable body with the limits of a tap, reading no more
// than the copy keeps.
func readCopy(reader io.Reader, size int64, encoding string, onPanic func()) (capturedBody, int64) {
	buf := newTapBuffer(false, onPanic)
	if size > maxParsedBody {
		buf.startHeadOnly()
	}
	chunk := make([]byte, foldChunkBytes)
	for empty := 0; !buf.full() && empty < 100; {
		n, err := reader.Read(chunk)
		if n == 0 {
			empty++
		}
		buf.write(chunk[:max(0, min(n, len(chunk)))])
		if err != nil {
			break
		}
	}
	body := buf.snapshot(encoding)
	if body.HeadOnly && size > body.OriginalBytes {
		body.OriginalBytes = size
	}
	return body, buf.takeReserved()
}

// ObserveResponse taps the response body of the attempt. It returns resp
// itself, with only Body wrapped; http.NoBody and errors pass untouched.
func (a *Attempt) ObserveResponse(resp *http.Response, err error) (out *http.Response) {
	out = resp
	if a == nil {
		return out
	}
	defer func() { _ = recover() }()
	if err != nil || resp == nil {
		a.markEnded(err)
		return out
	}
	a.mu.Lock()
	a.status = resp.StatusCode
	a.mu.Unlock()
	if resp.Body == nil || resp.Body == http.NoBody {
		a.markEnded(nil)
		return out
	}
	encoding := resp.Header.Get("Content-Encoding")
	buf := newTapBuffer(isEventStream(resp.Header.Get("Content-Type"), encoding), a.capture.disable)
	if !buf.sse && resp.ContentLength > maxParsedBody {
		buf.startHeadOnly()
	}
	a.mu.Lock()
	a.encoding = encoding
	a.response = buf
	a.mu.Unlock()
	resp.Body = &bodyTap{body: resp.Body, buf: buf, attempt: a}
	return out
}

// markEnded records when the response ended; a read error other than EOF
// means the provider cut the body.
func (a *Attempt) markEnded(err error) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil && err != io.EOF {
		a.cut = true
	}
	if a.endedAt.IsZero() {
		a.endedAt = time.Now()
	}
}

// Finish hands the call to the queue. It reads the client request only when a
// log row was written, and never waits for anything else. Its own panics are
// recovered; a panic of the call passes through untouched.
func (cp *Capture) Finish() {
	if cp == nil {
		return
	}
	cp.mu.Lock()
	if cp.finished {
		cp.mu.Unlock()
		return
	}
	// From here on no write copies or feeds the fold.
	cp.finished = true
	cp.skip.Store(true)
	attempts, fold, response := cp.attempts, cp.fold, cp.response
	held := cp.foldReserved
	cp.foldReserved = 0
	store := cp.logged && !cp.disabled.Load()
	eligible := cp.eligible
	cp.mu.Unlock()

	defer func() {
		if recover() != nil {
			release(held)
			for _, attempt := range attempts {
				attempt.discard()
			}
			fold.finish(nil)
		}
	}()

	now := time.Now()
	clientBody := response.snapshot(cp.writer.Header().Get("Content-Encoding"))
	held += response.takeReserved()
	if !store || !eligible {
		for _, attempt := range attempts {
			attempt.discard()
		}
		release(held)
		held = 0
		fold.finish(nil)
		if store {
			cp.enqueueNotCaptured(now, len(attempts) == 0)
		}
		return
	}

	rec := cp.newRecord(now)
	client := cp.clientAttempt(now, clientBody)
	request, reserved := cp.clientRequest()
	held += reserved
	client.Request = request
	if !requestStored(cp.c) {
		client.Meta.RequestUnavailable = true
	}
	rec.Attempts = append(rec.Attempts, client)
	for _, attempt := range attempts {
		if attempt.info.ChannelKey != "" {
			rec.Secrets = append(rec.Secrets, attempt.info.ChannelKey)
		}
		rec.Secrets = append(rec.Secrets, attempt.secrets...)
		row, reserved, keep := attempt.record(now)
		held += reserved
		if keep {
			rec.Attempts = append(rec.Attempts, row)
		}
	}
	checkTail := judgeServingStream(rec, attempts)
	rec.reserved = held
	held = 0
	if fold == nil {
		enqueue(rec)
		return
	}
	fold.checkServing = checkTail
	fold.finish(rec)
}

// judgeServingStream marks the call incomplete when the relay stopped reading
// the stream of the attempt that served it early. When the stream simply
// ended, it reports whether its tail should be checked for an end frame,
// which only providers speaking a known format allow.
func judgeServingStream(rec *captureRecord, attempts []*Attempt) bool {
	if len(rec.Attempts) < 2 || len(attempts) == 0 {
		return false
	}
	serving := &rec.Attempts[len(rec.Attempts)-1]
	last := attempts[len(attempts)-1]
	if serving.Attempt != last.index || !serving.Meta.IsStream || serving.Meta.HttpStatus >= http.StatusBadRequest {
		return false
	}
	end := StreamEndUnknown
	if last.info.StreamEnd != nil {
		end = last.info.StreamEnd()
	}
	switch end {
	case StreamEndComplete:
		return false
	case StreamEndCut:
		serving.Meta.Incomplete = true
		rec.Attempts[0].Meta.Incomplete = true
		return false
	}
	return last.info.KnownStreamFormat
}

func (cp *Capture) newRecord(now time.Time) *captureRecord {
	rec := &captureRecord{
		RequestId: cp.c.GetString(common.RequestIdKey),
		UserId:    cp.userID,
		CreatedAt: now.Unix(),
		Secrets:   []string{common.GetContextKeyString(cp.c, constant.ContextKeyTokenKey)},
	}
	if key := common.GetContextKeyString(cp.c, constant.ContextKeyChannelKey); key != "" {
		rec.Secrets = append(rec.Secrets, key)
	}
	return rec
}

// enqueueNotCaptured stores the marker of a logged call that had no eligible
// attempt: the client level with no bodies. A call that failed before any
// attempt gets its own status, since it may well have been eligible.
func (cp *Capture) enqueueNotCaptured(now time.Time, noAttempt bool) {
	rec := cp.newRecord(now)
	client := cp.clientAttempt(now, capturedBody{})
	client.NotCaptured = true
	client.NoAttempt = noAttempt
	rec.Attempts = []captureAttempt{client}
	enqueue(rec)
}

func (cp *Capture) clientAttempt(now time.Time, body capturedBody) captureAttempt {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	meta := model.LogContentMeta{
		RelayFormat:   cp.format,
		HttpStatus:    cp.writer.Status(),
		IsStream:      cp.sse || common.GetContextKeyBool(cp.c, constant.ContextKeyIsStream),
		MultiKeyIndex: -1,
		Method:        cp.c.Request.Method,
		StartedAtMs:   cp.startedAt.UnixMilli(),
		DurationMs:    now.Sub(cp.startedAt).Milliseconds(),
		Incomplete:    cp.writeFailed || cp.c.Request.Context().Err() != nil,
	}
	if cp.c.Request.URL != nil {
		meta.Path = cp.c.Request.URL.Path
	}
	if meta.HttpStatus >= http.StatusBadRequest {
		meta.ErrorMessage = cp.errorMessage
	}
	return captureAttempt{Attempt: 0, Meta: meta, Response: body}
}

// clientRequest copies the client body from its storage, never through
// GetBodyStorage, which would consume a body that was not stored.
func (cp *Capture) clientRequest() (capturedBody, int64) {
	value, ok := cp.c.Get(common.KeyBodyStorage)
	if !ok {
		return capturedBody{}, 0
	}
	storage, ok := value.(common.BodyStorage)
	if !ok || storage == nil {
		return capturedBody{}, 0
	}
	reader, err := storage.NewReader()
	if err != nil {
		return capturedBody{}, 0
	}
	defer reader.Close()
	return readCopy(reader, storage.Size(), "", cp.disable)
}

func requestStored(c *gin.Context) bool {
	value, ok := c.Get(common.KeyBodyStorage)
	return ok && value != nil
}

// discard returns the budget of an attempt that is not stored.
func (a *Attempt) discard() {
	a.mu.Lock()
	response := a.response
	reserved := a.requestReserved
	a.requestReserved = 0
	a.request = capturedBody{}
	a.mu.Unlock()
	release(reserved)
	response.discard()
}

// record turns an attempt into its row data and hands over its budget; keep is
// false, with the budget returned, when the attempt has no log row.
func (a *Attempt) record(now time.Time) (captureAttempt, int64, bool) {
	a.mu.Lock()
	logged := a.logged
	a.mu.Unlock()
	if !logged {
		a.discard()
		return captureAttempt{}, 0, false
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	body := a.response.snapshot(a.encoding)
	reserved := a.requestReserved + a.response.takeReserved()
	a.requestReserved = 0
	ended := a.endedAt
	if ended.IsZero() {
		ended = now
	}
	row := captureAttempt{
		Attempt: a.index,
		Meta: model.LogContentMeta{
			RelayFormat:        a.info.Format,
			HttpStatus:         a.status,
			IsStream:           a.response != nil && a.response.sse,
			Incomplete:         a.cut,
			ChannelId:          a.info.ChannelID,
			MultiKeyIndex:      a.info.MultiKeyIndex,
			Method:             a.method,
			Path:               a.path,
			StartedAtMs:        a.startedAt.UnixMilli(),
			DurationMs:         ended.Sub(a.startedAt).Milliseconds(),
			ErrorMessage:       a.errorMessage,
			RequestUnavailable: a.requestUnavailable,
		},
		Request:  a.request,
		Response: body,
	}
	a.request = capturedBody{}
	return row, reserved, true
}
