package logcontent

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	queueCapacity = 512
	workerCount   = 3

	maxBatchRows  = 20
	maxBatchBytes = 2 << 20 // under MySQL 5.7's 4 MiB max_allowed_packet
	flushInterval = time.Second
	batchTimeout  = 5 * time.Second
	shutdownDrain = 10 * time.Second
	statsLogEvery = time.Minute

	// Free text in meta is capped so the JSON fits a 64 KB TEXT column on
	// MySQL even if every byte is escaped as \u00XX.
	maxErrorMessageBytes = 4 << 10
	maxPathBytes         = 1 << 10
	maxShortFieldBytes   = 32
)

// capturedBody is a body as the tap copied it, before any cleaning.
type capturedBody struct {
	Data     []byte
	Encoding string
	// HeadOnly: Data is only the start of the body.
	HeadOnly bool
	// Truncated: the copy stopped early, for example for lack of budget.
	Truncated bool
	// OriginalBytes, when set, is the size of the whole body; Data may be less.
	OriginalBytes int64
	// EndKept: Data holds the last bytes of the body.
	EndKept bool
}

// captureAttempt is one attempt of a call: 0 is the client level.
type captureAttempt struct {
	Attempt int
	Meta    model.LogContentMeta
	// NotCaptured stores the marker row with no bodies.
	NotCaptured bool
	// NoAttempt: the marker is for a call that failed before any attempt.
	NoAttempt bool
	Request   capturedBody
	Response  capturedBody
	// Assembled is the message rebuilt from a stream; nil when there is none.
	Assembled []ViewMessage
	// AssembledTruncated: the rebuilt message misses part of the stream.
	AssembledTruncated bool
}

// captureRecord holds the raw data of one call and its budget reservation.
type captureRecord struct {
	RequestId string
	UserId    int
	CreatedAt int64
	// Secrets are the call's keys, expanded by NewSecrets in the worker.
	Secrets  []string
	Attempts []captureAttempt
	// reserved is the budget taken for the copies, returned once the record
	// is compressed or dropped.
	reserved int64
}

// StatsSnapshot holds this node's counters. Enqueued and Dropped count calls;
// Written and WriteErrors count rows, one per attempt.
type StatsSnapshot struct {
	Enqueued    int64 `json:"enqueued"`
	Written     int64 `json:"written"`
	Dropped     int64 `json:"dropped"`
	WriteErrors int64 `json:"write_errors"`
	BudgetInUse int64 `json:"budget_in_use"`
}

var (
	statEnqueued    atomic.Int64
	statWritten     atomic.Int64
	statDropped     atomic.Int64
	statWriteErrors atomic.Int64
	lastWriteError  atomic.Pointer[string]

	insertRows = func(ctx context.Context, rows []*model.LogContent) error {
		if model.LOG_DB == nil {
			return errors.New("log database not initialized")
		}
		return model.InsertLogContents(ctx, rows)
	}
	// testBuildHook runs before a record is cleaned; tests use it to panic.
	testBuildHook func(rec *captureRecord)

	currentPipeline atomic.Pointer[pipeline]
)

func init() {
	currentPipeline.Store(newPipeline(true))
}

// Stats returns this node's counters.
func Stats() StatsSnapshot {
	return StatsSnapshot{
		Enqueued:    statEnqueued.Load(),
		Written:     statWritten.Load(),
		Dropped:     statDropped.Load(),
		WriteErrors: statWriteErrors.Load(),
		BudgetInUse: budgetInUse.Load(),
	}
}

// pipeline is queue → workers (clean + gzip) → one batch writer.
type pipeline struct {
	mu     sync.RWMutex
	closed bool
	queue  chan *captureRecord
	rows   chan []*model.LogContent

	autoStart  bool
	flushEvery time.Duration
	startOnce  sync.Once
	writerDone chan struct{}
}

func newPipeline(autoStart bool) *pipeline {
	return &pipeline{
		queue:      make(chan *captureRecord, queueCapacity),
		rows:       make(chan []*model.LogContent, workerCount),
		autoStart:  autoStart,
		flushEvery: flushInterval,
		writerDone: make(chan struct{}),
	}
}

func (p *pipeline) start() {
	p.startOnce.Do(func() {
		var workers sync.WaitGroup
		for range workerCount {
			workers.Go(func() {
				for rec := range p.queue {
					if rows := buildRowsSafely(rec); len(rows) > 0 {
						p.rows <- rows
					}
				}
			})
		}
		go func() {
			workers.Wait()
			close(p.rows)
		}()
		go p.write()
	})
}

// enqueue hands a record to the workers without ever waiting. A full queue or
// a stopped pipeline drops it and returns its budget.
func enqueue(rec *captureRecord) bool {
	if rec == nil {
		return false
	}
	p := currentPipeline.Load()
	if p.autoStart {
		p.start()
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.closed {
		select {
		case p.queue <- rec:
			statEnqueued.Add(1)
			return true
		default:
		}
	}
	statDropped.Add(1)
	release(rec.reserved)
	rec.reserved = 0
	return false
}

// Shutdown refuses new records and drains the queue for up to 10 s, or until
// ctx ends.
func Shutdown(ctx context.Context) {
	p := currentPipeline.Load()
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.queue)
	}
	p.mu.Unlock()
	p.start()

	ctx, cancel := context.WithTimeout(ctx, shutdownDrain)
	defer cancel()
	select {
	case <-p.writerDone:
	case <-ctx.Done():
		common.SysError(common.LogText("log content: shutdown deadline reached with %d records queued", len(p.queue)))
	}
}

// buildRowsSafely cleans and compresses a record. A panic loses the record,
// counted as a write error, but never the worker or the budget accounting.
func buildRowsSafely(rec *captureRecord) (rows []*model.LogContent) {
	defer func() {
		release(rec.reserved)
		rec.reserved = 0
		if r := recover(); r != nil {
			rows = nil
			statWriteErrors.Add(int64(max(len(rec.Attempts), 1)))
			common.SysError(common.LogText("log content: panic while preparing %s: %v", rec.RequestId, r))
		}
	}()
	if testBuildHook != nil {
		testBuildHook(rec)
	}
	return buildRows(rec)
}

// buildRows runs the cleaning on every body of the record and gzips the result.
func buildRows(rec *captureRecord) []*model.LogContent {
	secrets := NewSecrets(rec.Secrets...)
	createdAt := rec.CreatedAt
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}
	rows := make([]*model.LogContent, 0, len(rec.Attempts))
	for _, attempt := range rec.Attempts {
		row := &model.LogContent{
			RequestId:     rec.RequestId,
			Attempt:       attempt.Attempt,
			UserId:        rec.UserId,
			CreatedAt:     createdAt,
			CaptureStatus: model.LogContentStatusCaptured,
		}
		meta := attempt.Meta
		if attempt.NotCaptured {
			row.CaptureStatus = model.LogContentStatusNotCaptured
			if attempt.NoAttempt {
				row.CaptureStatus = model.LogContentStatusNoAttempt
			}
		} else {
			row.RequestBody, meta.Request = storeBody(attempt.Request, secrets)
			row.ResponseBody, meta.Response = storeBody(attempt.Response, secrets)
			if attempt.Assembled != nil {
				msgs, trimmed := SanitizeAssembledMessages(attempt.Assembled, secrets)
				if raw, err := common.Marshal(msgs); err == nil {
					row.AssembledBody = gzipBytes(raw)
				}
				meta.Response.Truncated = meta.Response.Truncated || trimmed || attempt.AssembledTruncated
			}
		}
		meta.ErrorMessage = capText(meta.ErrorMessage, maxErrorMessageBytes, secrets)
		meta.Path = capText(meta.Path, maxPathBytes, secrets)
		meta.Method = capText(meta.Method, maxShortFieldBytes, secrets)
		meta.RelayFormat = capText(meta.RelayFormat, maxShortFieldBytes, secrets)
		encoded, err := model.NewLogContentMeta(meta)
		if err != nil {
			encoded = "{}"
		}
		row.Meta = encoded
		rows = append(rows, row)
	}
	return rows
}

// storeBody cleans a captured body and returns its gzip, nil when empty.
func storeBody(body capturedBody, secrets Secrets) ([]byte, model.BodyMeta) {
	if len(body.Data) == 0 && body.OriginalBytes == 0 {
		return nil, model.BodyMeta{Truncated: body.Truncated}
	}
	clean := SanitizeBody(body.Data, body.Encoding, body.HeadOnly, secrets)
	if body.OriginalBytes > 0 {
		clean.Meta.OriginalBytes = body.OriginalBytes
	}
	clean.Meta.Truncated = clean.Meta.Truncated || body.Truncated
	return gzipBytes([]byte(clean.Text)), clean.Meta
}

// capText redacts free text bound for meta and cuts it at a rune boundary.
func capText(text string, limit int, secrets Secrets) string {
	if text == "" {
		return ""
	}
	return string(cutAtRune(secrets.redact([]byte(text)), limit))
}

func gzipBytes(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	var buf bytes.Buffer
	writer, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	_, _ = writer.Write(data)
	_ = writer.Close()
	return buf.Bytes()
}

// write batches rows: 20 rows, 2 MiB or 1 s, whichever comes first.
func (p *pipeline) write() {
	defer close(p.writerDone)
	var pending []*model.LogContent
	pendingBytes := 0
	timer := time.NewTimer(p.flushEvery)
	timer.Stop()
	var flushC <-chan time.Time
	report := time.NewTicker(statsLogEvery)
	defer report.Stop()
	var reportedDrops, reportedErrors int64

	flush := func() {
		timer.Stop()
		flushC = nil
		if len(pending) > 0 {
			writeBatch(pending)
		}
		pending = nil
		pendingBytes = 0
	}
	for {
		select {
		case rows, ok := <-p.rows:
			if !ok {
				flush()
				return
			}
			for _, row := range rows {
				size := len(row.Meta) + len(row.RequestBody) + len(row.ResponseBody) + len(row.AssembledBody)
				if len(pending) > 0 && pendingBytes+size > maxBatchBytes {
					flush()
				}
				if len(pending) == 0 {
					timer.Reset(p.flushEvery)
					flushC = timer.C
				}
				pending = append(pending, row)
				pendingBytes += size
				if len(pending) >= maxBatchRows || pendingBytes >= maxBatchBytes {
					flush()
				}
			}
		case <-flushC:
			flush()
		case <-report.C:
			drops, errs := statDropped.Load(), statWriteErrors.Load()
			if drops == reportedDrops && errs == reportedErrors {
				continue
			}
			reportedDrops, reportedErrors = drops, errs
			lastErr := ""
			if last := lastWriteError.Load(); last != nil {
				lastErr = *last
			}
			common.SysError(common.LogText("log content: dropped %d, write errors %d, last error: %s", drops, errs, lastErr))
		}
	}
}

// writeBatch inserts a batch; on error it retries row by row, and a row that
// still fails is counted and dropped.
func writeBatch(batch []*model.LogContent) {
	if insertBatch(batch) == nil {
		statWritten.Add(int64(len(batch)))
		return
	}
	if len(batch) == 1 {
		statWriteErrors.Add(1)
		return
	}
	for _, row := range batch {
		if insertBatch([]*model.LogContent{row}) == nil {
			statWritten.Add(1)
		} else {
			statWriteErrors.Add(1)
		}
	}
}

// insertBatch runs one insert with its own timeout, turning a driver panic
// into an error.
func insertBatch(rows []*model.LogContent) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
		if err != nil {
			message := err.Error()
			lastWriteError.Store(&message)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), batchTimeout)
	defer cancel()
	return insertRows(ctx, rows)
}
