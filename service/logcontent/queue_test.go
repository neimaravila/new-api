package logcontent

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newTestPipeline swaps in a fresh queue for the test. With paused set, no
// worker runs until Shutdown. The cleanup drains it and restores LOG_DB only
// after the writer has stopped, so no goroutine outlives the test.
func newTestPipeline(t *testing.T, paused bool) *pipeline {
	t.Helper()
	previousDB, previousType := model.LOG_DB, common.LogDatabaseType()
	previousInsert := insertRows
	p := resetPipelineForTest(paused)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		Shutdown(ctx)
		select {
		case <-p.writerDone:
		case <-time.After(5 * time.Second):
			t.Error("writer did not stop")
		}
		insertRows = previousInsert
		testBuildHook = nil
		model.LOG_DB = previousDB
		common.SetLogDatabaseType(previousType)
		release(budgetInUse.Load())
	})
	return p
}

func withBudgetLimit(t *testing.T, limit int64) {
	t.Helper()
	previous := loadBudgetLimit()
	budgetLimit.Store(limit)
	t.Cleanup(func() { budgetLimit.Store(previous) })
}

func sqliteLogDB(t *testing.T, migrate bool) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/log.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	model.LOG_DB = db
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	if migrate {
		require.NoError(t, model.MigrateLogContents())
	}
}

// countingInsert records every insert call and succeeds.
type countingInsert struct {
	mu      sync.Mutex
	batches []int
	calls   chan int
}

func newCountingInsert() *countingInsert {
	return &countingInsert{calls: make(chan int, 1024)}
}

func (c *countingInsert) insert(_ context.Context, rows []*model.LogContent) error {
	c.mu.Lock()
	c.batches = append(c.batches, len(rows))
	c.mu.Unlock()
	c.calls <- len(rows)
	return nil
}

func smallRecord(id string) *captureRecord {
	return &captureRecord{
		RequestId: id,
		UserId:    1,
		Attempts: []captureAttempt{{
			Attempt:  0,
			Meta:     model.LogContentMeta{RelayFormat: "openai", HttpStatus: 200, MultiKeyIndex: -1},
			Request:  capturedBody{Data: []byte(`{"model":"m"}`)},
			Response: capturedBody{Data: []byte(`{"ok":true}`)},
		}},
	}
}

func waitBatch(t *testing.T, calls chan int, within time.Duration) int {
	t.Helper()
	select {
	case n := <-calls:
		return n
	case <-time.After(within):
		t.Fatalf("no batch written within %v", within)
		return 0
	}
}

func shutdownNow(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	Shutdown(ctx)
}

func TestEnqueueNeverBlocksWhenFull(t *testing.T) {
	newTestPipeline(t, true)
	insertRows = newCountingInsert().insert
	withBudgetLimit(t, 1<<30)

	start := time.Now()
	accepted := 0
	for i := range queueCapacity + 1 {
		rec := smallRecord(fmt.Sprintf("r%d", i))
		require.True(t, tryReserve(10))
		rec.reserved = 10
		if enqueue(rec) {
			accepted++
		}
	}
	// Generous under -race: the point is that nothing waits for a worker.
	assert.Less(t, time.Since(start), time.Second)
	stats := Stats()
	assert.Equal(t, queueCapacity, accepted)
	assert.EqualValues(t, queueCapacity, stats.Enqueued)
	assert.EqualValues(t, 1, stats.Dropped)
	assert.EqualValues(t, queueCapacity*10, stats.BudgetInUse, "the dropped record's reservation is returned")
}

func TestWriteErrorIsCountedAndDropped(t *testing.T) {
	t.Run("table missing", func(t *testing.T) {
		newTestPipeline(t, false)
		sqliteLogDB(t, false)
		for i := range 3 {
			require.True(t, enqueue(smallRecord(fmt.Sprintf("r%d", i))))
		}
		shutdownNow(t)
		stats := Stats()
		assert.EqualValues(t, 3, stats.WriteErrors)
		assert.EqualValues(t, 0, stats.Written)
	})
	t.Run("no log database", func(t *testing.T) {
		newTestPipeline(t, false)
		model.LOG_DB = nil
		require.True(t, enqueue(smallRecord("r")))
		shutdownNow(t)
		assert.EqualValues(t, 1, Stats().WriteErrors)
	})
	t.Run("insert panics", func(t *testing.T) {
		newTestPipeline(t, false)
		insertRows = func(context.Context, []*model.LogContent) error { panic("driver bug") }
		require.True(t, enqueue(smallRecord("a")))
		shutdownNow(t)
		assert.EqualValues(t, 1, Stats().WriteErrors)
	})
}

func TestWorkerRecoversFromPanicAndReleasesBudget(t *testing.T) {
	newTestPipeline(t, false)
	counter := newCountingInsert()
	insertRows = counter.insert
	withBudgetLimit(t, 1000)
	testBuildHook = func(rec *captureRecord) {
		if rec.RequestId == "boom" {
			panic("bad record")
		}
	}
	for _, id := range []string{"boom", "ok"} {
		rec := smallRecord(id)
		require.True(t, tryReserve(100))
		rec.reserved = 100
		require.True(t, enqueue(rec))
	}
	shutdownNow(t)
	stats := Stats()
	assert.EqualValues(t, 1, stats.Written, "the other record still goes through")
	assert.EqualValues(t, 1, stats.WriteErrors)
	assert.EqualValues(t, 0, stats.BudgetInUse)
}

func TestBatchFallsBackToSingleInserts(t *testing.T) {
	newTestPipeline(t, false)
	var mu sync.Mutex
	var stored []string
	insertRows = func(_ context.Context, rows []*model.LogContent) error {
		for _, row := range rows {
			if row.RequestId == "bad" {
				return errors.New("type conflict")
			}
		}
		mu.Lock()
		defer mu.Unlock()
		for _, row := range rows {
			stored = append(stored, row.RequestId)
		}
		return nil
	}
	for _, id := range []string{"a", "b", "bad", "c", "d"} {
		require.True(t, enqueue(smallRecord(id)))
	}
	shutdownNow(t)
	stats := Stats()
	assert.EqualValues(t, 4, stats.Written)
	assert.EqualValues(t, 1, stats.WriteErrors)
	assert.ElementsMatch(t, []string{"a", "b", "c", "d"}, stored)
}

// letterText is text that neither compresses much nor looks like base64.
func letterText(n int, seed int64) []byte {
	rng := rand.New(rand.NewSource(seed))
	out := make([]byte, n)
	for i := range out {
		if i%8 == 7 {
			out[i] = ' '
			continue
		}
		out[i] = byte('a' + rng.Intn(26))
	}
	return out
}

func TestBatchFlushesOnCountSizeAndTime(t *testing.T) {
	t.Run("count", func(t *testing.T) {
		newTestPipeline(t, false)
		counter := newCountingInsert()
		insertRows = counter.insert
		setFlushIntervalForTest(t, time.Hour)
		for i := range maxBatchRows {
			require.True(t, enqueue(smallRecord(fmt.Sprintf("r%d", i))))
		}
		assert.Equal(t, maxBatchRows, waitBatch(t, counter.calls, 5*time.Second))
	})
	t.Run("size", func(t *testing.T) {
		newTestPipeline(t, false)
		counter := newCountingInsert()
		insertRows = counter.insert
		setFlushIntervalForTest(t, time.Hour)
		// Each row is about 700 KiB of gzip: the fourth would pass 2 MiB.
		for i := range 4 {
			rec := smallRecord(fmt.Sprintf("r%d", i))
			rec.Attempts[0].Request = capturedBody{Data: letterText(maxStoredBody-1024, int64(i))}
			require.True(t, enqueue(rec))
		}
		assert.Equal(t, 3, waitBatch(t, counter.calls, 10*time.Second))
	})
	t.Run("time", func(t *testing.T) {
		newTestPipeline(t, false)
		counter := newCountingInsert()
		insertRows = counter.insert
		start := time.Now()
		require.True(t, enqueue(smallRecord("one")))
		assert.Equal(t, 1, waitBatch(t, counter.calls, 3*time.Second))
		assert.GreaterOrEqual(t, time.Since(start), flushInterval/2, "a lone row waits for the timer")
	})
}

func TestShutdownFlushesQueue(t *testing.T) {
	newTestPipeline(t, false)
	insertRows = newCountingInsert().insert
	for i := range 10 {
		require.True(t, enqueue(smallRecord(fmt.Sprintf("r%d", i))))
	}
	shutdownNow(t)
	assert.EqualValues(t, 10, Stats().Written)

	rec := smallRecord("late")
	require.True(t, tryReserve(5))
	rec.reserved = 5
	assert.False(t, enqueue(rec))
	assert.EqualValues(t, 0, Stats().BudgetInUse)
}

func TestShutdownRespectsDeadline(t *testing.T) {
	newTestPipeline(t, false)
	unblock := make(chan struct{})
	t.Cleanup(func() { close(unblock) })
	insertRows = func(context.Context, []*model.LogContent) error {
		<-unblock // a driver that ignores its context
		return nil
	}
	require.True(t, enqueue(smallRecord("stuck")))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	Shutdown(ctx)
	assert.Less(t, time.Since(start), time.Second)
}

func TestBudgetExhaustedAndReleased(t *testing.T) {
	newTestPipeline(t, false)
	insertRows = newCountingInsert().insert
	withBudgetLimit(t, 100)

	require.True(t, tryReserve(60))
	assert.False(t, tryReserve(60), "over the limit")
	assert.True(t, tryReserve(0))
	rec := smallRecord("r")
	rec.reserved = 60
	require.True(t, enqueue(rec))
	shutdownNow(t)
	assert.EqualValues(t, 0, Stats().BudgetInUse)
	assert.True(t, tryReserve(100))
	release(100)
}

func gunzipText(t *testing.T, data []byte) string {
	t.Helper()
	require.GreaterOrEqual(t, len(data), 2)
	require.Equal(t, []byte{0x1f, 0x8b}, data[:2])
	reader, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	text, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(text)
}

func TestStoredBodiesAreGzip(t *testing.T) {
	newTestPipeline(t, false)
	sqliteLogDB(t, true)
	key := "sk-abcdefghijklmnopqrstuvwxyz0123"
	rec := smallRecord("req-gzip")
	rec.Secrets = []string{key}
	rec.Attempts[0].Request = capturedBody{Data: []byte(`{"key":"` + key + `"}`)}
	rec.Attempts[0].Assembled = []ViewMessage{{Role: roleAssistant, Source: sourceResponse, Blocks: []ViewBlock{{Type: blockText, Text: "hi " + key}}}}
	rec.Attempts = append(rec.Attempts, captureAttempt{Attempt: 1, NotCaptured: true})
	require.True(t, enqueue(rec))
	shutdownNow(t)
	require.EqualValues(t, 2, Stats().Written)

	rows, err := model.GetLogContents(context.Background(), "req-gzip", 0)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, `{"key":"sk-[REDACTED]"}`, gunzipText(t, rows[0].RequestBody))
	assert.Equal(t, `{"ok":true}`, gunzipText(t, rows[0].ResponseBody))
	assembled := gunzipText(t, rows[0].AssembledBody)
	assert.NotContains(t, assembled, key)
	var msgs []ViewMessage
	require.NoError(t, common.Unmarshal([]byte(assembled), &msgs))
	assert.Equal(t, model.LogContentStatusCaptured, rows[0].CaptureStatus)

	assert.Equal(t, model.LogContentStatusNotCaptured, rows[1].CaptureStatus)
	assert.Empty(t, rows[1].RequestBody)
	assert.Empty(t, rows[1].ResponseBody)
}

func TestMetaFreeTextIsCappedAndRedacted(t *testing.T) {
	newTestPipeline(t, false)
	sqliteLogDB(t, true)
	key := "sk-abcdefghijklmnopqrstuvwxyz0123"
	rec := smallRecord("req-meta")
	rec.Secrets = []string{key}
	rec.Attempts[0].Meta.ErrorMessage = key + strings.Repeat("\x00", 200000)
	rec.Attempts[0].Meta.Path = "/v1/" + strings.Repeat("\x01", 200000)
	rec.Attempts[0].Meta.Method = strings.Repeat("P", 1000)
	require.True(t, enqueue(rec))
	shutdownNow(t)
	require.EqualValues(t, 1, Stats().Written)

	rows, err := model.GetLogContents(context.Background(), "req-meta", 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Less(t, len(rows[0].Meta), 60000, "fits the 64 KB text column with room")
	assert.NotContains(t, rows[0].Meta, key)
	var meta model.LogContentMeta
	require.NoError(t, common.Unmarshal([]byte(rows[0].Meta), &meta))
	assert.True(t, strings.HasPrefix(meta.ErrorMessage, "sk-[REDACTED]"))
}

func TestBodyFlagsFromTheTap(t *testing.T) {
	newTestPipeline(t, false)
	var got []*model.LogContent
	var mu sync.Mutex
	insertRows = func(_ context.Context, rows []*model.LogContent) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, rows...)
		return nil
	}
	rec := smallRecord("flags")
	rec.Attempts[0].Response = capturedBody{Data: []byte("partial"), HeadOnly: true, OriginalBytes: 50 << 20}
	rec.Attempts[0].Request = capturedBody{Data: []byte("abc"), Truncated: true}
	require.True(t, enqueue(rec))
	shutdownNow(t)
	require.Len(t, got, 1)
	var meta model.LogContentMeta
	require.NoError(t, common.Unmarshal([]byte(got[0].Meta), &meta))
	assert.True(t, meta.Response.HeadOnly)
	assert.EqualValues(t, 50<<20, meta.Response.OriginalBytes)
	assert.True(t, meta.Request.Truncated)
	assert.EqualValues(t, 3, meta.Request.OriginalBytes)
}

// resetPipelineForTest replaces the global queue and zeroes the counters. The
// previous pipeline must already be drained.
func resetPipelineForTest(paused bool) *pipeline {
	p := newPipeline(!paused)
	currentPipeline.Store(p)
	statEnqueued.Store(0)
	statWritten.Store(0)
	statDropped.Store(0)
	statWriteErrors.Store(0)
	lastWriteError.Store(nil)
	return p
}

// setFlushIntervalForTest changes the timer of the current, not yet started,
// pipeline.
func setFlushIntervalForTest(t *testing.T, every time.Duration) {
	t.Helper()
	currentPipeline.Load().flushEvery = every
}
