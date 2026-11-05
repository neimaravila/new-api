package logcontent

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

// providerTailBytes is how much of the end of a provider stream is read to
// find its terminal frame.
const providerTailBytes = 64 << 10

// testCopyHook runs inside every copy; tests use it to make the copy panic.
var testCopyHook func()

// tapBuffer is a bounded copy of a body as it passes by. It never blocks, fails
// or changes the stream it copies: without budget it stops copying and records
// why. A non-SSE body grows up to maxParsedBody and then keeps only its start;
// an SSE body keeps a head and a ring of its tail.
type tapBuffer struct {
	mu  sync.Mutex
	sse bool
	// stopped: nothing more is copied, but bytes are still counted.
	stopped bool
	// sealed: the copy was taken or discarded; writes are ignored.
	sealed    bool
	headOnly  bool
	truncated bool
	total     int64
	reserved  int64
	// data is the non-SSE body, or the head of an SSE body.
	data []byte
	// tail is the SSE ring, allocated whole once the head is full.
	tail     []byte
	tailPos  int
	tailFull bool
	// onPanic is told when a copy panics; the buffer stops either way.
	onPanic func()
}

func newTapBuffer(sse bool, onPanic func()) *tapBuffer {
	return &tapBuffer{sse: sse, onPanic: onPanic}
}

// startHeadOnly makes the buffer keep only the start of the body, for bodies
// already known to be larger than what is parsed.
func (b *tapBuffer) startHeadOnly() {
	b.mu.Lock()
	b.headOnly = true
	b.mu.Unlock()
}

// full reports whether a head-only copy has all it will keep, so a reader
// that only feeds the copy can stop.
func (b *tapBuffer) full() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stopped || (b.headOnly && len(b.data) >= maxStoredBody+tapMarginBytes)
}

// write copies p. It is safe to call from any goroutine and never panics.
func (b *tapBuffer) write(p []byte) {
	if b == nil || len(p) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.sealed {
		return
	}
	b.total += int64(len(p))
	if b.stopped {
		return
	}
	panicked := false
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		if testCopyHook != nil {
			testCopyHook()
		}
		if b.sse {
			b.writeSSE(p)
		} else {
			b.writeBody(p)
		}
	}()
	if panicked {
		b.stopped = true
		b.truncated = true
		if b.onPanic != nil {
			b.onPanic()
		}
	}
}

// grow makes room for need bytes in data, reserving budget for the new
// capacity first. It reports false when the budget has no room.
func (b *tapBuffer) grow(need, limit int) bool {
	if need <= cap(b.data) {
		return true
	}
	newCap := min(max(need, 2*cap(b.data), 4<<10), limit)
	if !tryReserve(int64(newCap - cap(b.data))) {
		return false
	}
	b.reserved += int64(newCap - cap(b.data))
	grown := make([]byte, len(b.data), newCap)
	copy(grown, b.data)
	b.data = grown
	return true
}

// stopForBudget ends a copy that could not grow: what was kept is the start.
func (b *tapBuffer) stopForBudget() {
	b.stopped = true
	b.truncated = true
	if !b.sse {
		b.headOnly = true
	}
}

func (b *tapBuffer) writeBody(p []byte) {
	limit := maxParsedBody
	if !b.headOnly && len(b.data)+len(p) > maxParsedBody {
		b.dropToHead()
	}
	if b.headOnly {
		limit = maxStoredBody + tapMarginBytes
	}
	room := limit - len(b.data)
	if room <= 0 {
		return
	}
	p = p[:min(len(p), room)]
	if !b.grow(len(b.data)+len(p), limit) {
		b.stopForBudget()
		return
	}
	b.data = append(b.data, p...)
}

// dropToHead keeps the first maxStoredBody bytes plus the margin, so a key
// crossing the cut is still whole for redaction, and returns the rest of the
// budget.
func (b *tapBuffer) dropToHead() {
	b.headOnly = true
	keep := maxStoredBody + tapMarginBytes
	if cap(b.data) <= keep {
		return
	}
	head := make([]byte, min(len(b.data), keep), keep)
	copy(head, b.data)
	freed := int64(cap(b.data) - keep)
	release(freed)
	b.reserved -= freed
	b.data = head
}

func (b *tapBuffer) writeSSE(p []byte) {
	if len(b.data) < sseHeadBytes {
		take := min(len(p), sseHeadBytes-len(b.data))
		if !b.grow(len(b.data)+take, sseHeadBytes) {
			b.stopForBudget()
			return
		}
		b.data = append(b.data, p[:take]...)
		p = p[take:]
	}
	if len(p) == 0 {
		return
	}
	if b.tail == nil {
		if !tryReserve(sseTailBytes) {
			b.stopForBudget()
			return
		}
		b.reserved += sseTailBytes
		b.tail = make([]byte, sseTailBytes)
	}
	if len(p) >= sseTailBytes {
		copy(b.tail, p[len(p)-sseTailBytes:])
		b.tailPos, b.tailFull = 0, true
		return
	}
	for len(p) > 0 {
		n := copy(b.tail[b.tailPos:], p)
		p = p[n:]
		b.tailPos += n
		if b.tailPos == sseTailBytes {
			b.tailPos, b.tailFull = 0, true
		}
	}
}

// middleDropped reports whether an SSE copy lost bytes between head and tail.
func (b *tapBuffer) middleDropped() bool {
	ring := b.tailPos
	if b.tailFull {
		ring = sseTailBytes
	}
	return b.total > int64(len(b.data)+ring)
}

// linear returns the copy in order. For SSE with a dropped middle, the head
// ends and the tail starts at a line boundary.
func (b *tapBuffer) linear() []byte {
	if !b.sse || b.tail == nil {
		return bytes.Clone(b.data)
	}
	var tail []byte
	if b.tailFull {
		tail = append(bytes.Clone(b.tail[b.tailPos:]), b.tail[:b.tailPos]...)
	} else {
		tail = bytes.Clone(b.tail[:b.tailPos])
	}
	head := b.data
	if b.middleDropped() {
		if cut := bytes.LastIndexByte(head, '\n'); cut >= 0 {
			head = head[:cut+1]
		}
		if cut := bytes.IndexByte(tail, '\n'); cut >= 0 {
			tail = tail[cut+1:]
		}
	}
	out := make([]byte, 0, len(head)+len(tail))
	out = append(out, head...)
	return append(out, tail...)
}

// peek returns a copy of what was kept so far, without stopping the buffer.
func (b *tapBuffer) peek() []byte {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.linear()
}

// snapshot seals the copy and returns the body. The budget stays reserved
// until takeReserved hands it over.
func (b *tapBuffer) snapshot(encoding string) capturedBody {
	if b == nil {
		return capturedBody{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sealed = true
	data := b.data
	truncated := b.truncated
	if b.sse && b.tail != nil {
		data = b.linear()
		truncated = truncated || b.middleDropped()
	}
	body := capturedBody{Data: data, Encoding: encoding, HeadOnly: b.headOnly, Truncated: truncated, EndKept: !b.stopped && !b.headOnly}
	if b.headOnly || truncated {
		body.OriginalBytes = b.total
	}
	b.data, b.tail = nil, nil
	return body
}

// takeReserved hands the buffer's reservation to the caller.
func (b *tapBuffer) takeReserved() int64 {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	reserved := b.reserved
	b.reserved = 0
	return reserved
}

// discard stops the copy and returns its budget.
func (b *tapBuffer) discard() {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.sealed = true
	b.data, b.tail = nil, nil
	reserved := b.reserved
	b.reserved = 0
	b.mu.Unlock()
	release(reserved)
}

func isEventStream(contentType, encoding string) bool {
	return encoding == "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "text/event-stream")
}

// clientWriter copies what the client accepted, after each write. Everything
// else passes straight to the writer underneath.
type clientWriter struct {
	gin.ResponseWriter
	capture *Capture
}

func (w *clientWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.capture.observeClient(p[:max(0, min(n, len(p)))], err)
	return n, err
}

func (w *clientWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	if accepted := max(0, min(n, len(s))); accepted > 0 || err != nil {
		w.capture.observeClient([]byte(s[:accepted]), err)
	}
	return n, err
}

// Unwrap lets http.ResponseController and FlushWriter reach the writers
// underneath; gin's interface does not promote it.
func (w *clientWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// bodyTap copies a provider response as the relay reads it.
type bodyTap struct {
	body    io.ReadCloser
	buf     *tapBuffer
	attempt *Attempt
}

func (t *bodyTap) Read(p []byte) (int, error) {
	n, err := t.body.Read(p)
	if n > 0 && n <= len(p) && !t.attempt.capture.disabled.Load() {
		t.buf.write(p[:n])
	}
	if err != nil {
		t.attempt.markEnded(err)
	}
	return n, err
}

func (t *bodyTap) Close() error {
	err := t.body.Close()
	t.attempt.markEnded(nil)
	return err
}

// streamFold rebuilds the client stream off the call path: the writer hands
// it copies of what was sent through a small channel and never waits.
type streamFold struct {
	chunks  chan []byte
	asm     *StreamAssembler
	dropped atomic.Bool
	once    sync.Once
	// rec is set by finish before chunks is closed; nil means nothing to store.
	rec *captureRecord
}

// finish closes the fold once. Called after the capture stopped feeding it;
// with a record, the fold stores it when the assembly is done.
func (f *streamFold) finish(rec *captureRecord) {
	if f == nil {
		return
	}
	f.once.Do(func() {
		f.rec = rec
		close(f.chunks)
	})
}

func newStreamFold(format string) *streamFold {
	return &streamFold{chunks: make(chan []byte, foldQueueChunks), asm: NewStreamAssembler(format)}
}

// feed sends copies of p in blocks; a full channel drops the block.
func (f *streamFold) feed(p []byte) {
	for len(p) > 0 {
		n := min(len(p), foldChunkBytes)
		select {
		case f.chunks <- bytes.Clone(p[:n]):
		default:
			f.dropped.Store(true)
		}
		p = p[n:]
	}
}

func (f *streamFold) run() {
	ok := true
	for chunk := range f.chunks {
		if ok {
			ok = f.write(chunk)
		}
	}
	rec := f.rec
	if rec == nil {
		return
	}
	if ok {
		f.apply(rec)
	}
	enqueue(rec)
}

func (f *streamFold) write(chunk []byte) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	f.asm.Write(chunk)
	return true
}

// apply puts the rebuilt message on the client attempt; a panic leaves the
// record as it was. The gateway may end a stream cleanly after the provider
// cut it, so the provider stream that served the call is checked too.
func (f *streamFold) apply(rec *captureRecord) {
	defer func() { _ = recover() }()
	client := &rec.Attempts[0]
	result := f.asm.Result()
	assembled := result.Messages
	if assembled == nil {
		assembled = []ViewMessage{}
	}
	client.Assembled = assembled
	client.AssembledTruncated = result.Truncated || f.dropped.Load()
	if !result.Terminal {
		client.Meta.Incomplete = true
	}
	if len(rec.Attempts) < 2 {
		return
	}
	serving := &rec.Attempts[len(rec.Attempts)-1]
	body := serving.Response
	if !serving.Meta.IsStream || serving.Meta.HttpStatus >= http.StatusBadRequest || !body.EndKept || body.Encoding != "" {
		return
	}
	if !endsWithTerminal(body.Data) {
		serving.Meta.Incomplete = true
		client.Meta.Incomplete = true
	}
}

// endsWithTerminal reports whether the end of a provider stream holds a frame
// its format defines as the end, in any of the formats the gateway speaks.
func endsWithTerminal(stream []byte) bool {
	tail := stream
	if len(tail) > providerTailBytes {
		tail = tail[len(tail)-providerTailBytes:]
		if cut := bytes.IndexByte(tail, '\n'); cut >= 0 {
			tail = tail[cut+1:]
		}
	}
	for _, format := range []string{FormatOpenAI, FormatResponses, FormatClaude, FormatGemini} {
		asm := NewStreamAssembler(format)
		asm.Write(tail)
		if asm.Result().Terminal {
			return true
		}
	}
	return false
}
