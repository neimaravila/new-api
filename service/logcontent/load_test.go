//go:build logcontentload

package logcontent

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Run: AUDIT_POSTGRES_DSN=... go test -tags logcontentload -count=1 -timeout 20m \
//   ./service/logcontent/ -run 'LogContentPostgres' -v

const (
	lcLoadPhase         = 60 * time.Second
	lcLoadRate          = 200 // records per second, 2 rows each
	lcLoadExpiredRows   = 200_000
	lcLoadTargetRows    = 100
	lcLoadProbeEvery    = 5 * time.Millisecond
	lcLoadRetentionDays = 30
	// Same batch as service.logContentRetentionBatchSize, but no pause: the
	// production pause only makes the deletion gentler.
	lcLoadRetentionBatch = 200
)

// lcPostgres opens a throwaway database on a loopback PostgreSQL and makes it
// both the main and the log database, as in production.
func lcPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("AUDIT_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("AUDIT_POSTGRES_DSN is not configured")
	}
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	require.True(t, net.ParseIP(parsed.Hostname()).IsLoopback(), "load tests only permit loopback instances")
	name := fmt.Sprintf("newapi_logcontent_load_%d", time.Now().UnixNano())
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, admin.Exec("CREATE DATABASE "+name).Error)
	adminConn, err := admin.DB()
	require.NoError(t, err)
	require.NoError(t, adminConn.Close())
	parsed.Path = "/" + name
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(40)
	t.Cleanup(func() { _ = sqlDB.Close() })

	previousMain := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previousMain })
	model.LOG_DB = db
	common.SetLogDatabaseType(common.DatabaseTypePostgreSQL)
	return db
}

func lcPercentile(samples []time.Duration, p float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	index := int(float64(len(sorted)-1) * p)
	return sorted[index]
}

type lcProbeResult struct {
	insertLogs, selectUsers []time.Duration
	// insertAt and selectAt hold the start time of each sample.
	insertAt, selectAt []time.Time
	errors             int64
}

// within returns the samples that started inside [from, to].
func within(samples []time.Duration, at []time.Time, from, to time.Time) []time.Duration {
	var out []time.Duration
	for i, d := range samples {
		if !at[i].Before(from) && !at[i].After(to) {
			out = append(out, d)
		}
	}
	return out
}

// lcRunProbes measures INSERT into logs and SELECT of users until stop closes.
func lcRunProbes(db *gorm.DB, stop <-chan struct{}) lcProbeResult {
	var result lcProbeResult
	var wg sync.WaitGroup
	var errCount atomic.Int64
	probe := func(out *[]time.Duration, outAt *[]time.Time, run func(i int) error) {
		wg.Go(func() {
			ticker := time.NewTicker(lcLoadProbeEvery)
			defer ticker.Stop()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				case <-ticker.C:
				}
				start := time.Now()
				if err := run(i); err != nil {
					errCount.Add(1)
					continue
				}
				*out = append(*out, time.Since(start))
				*outAt = append(*outAt, start)
			}
		})
	}
	probe(&result.insertLogs, &result.insertAt, func(i int) error {
		return db.Create(&model.Log{
			UserId: 1 + i%1000, CreatedAt: time.Now().Unix(), Type: model.LogTypeConsume,
			Content: "probe", ModelName: "probe-model", RequestId: fmt.Sprintf("probe-%d-%d", time.Now().UnixNano(), i),
		}).Error
	})
	probe(&result.selectUsers, &result.selectAt, func(i int) error {
		var row struct {
			Id    int
			Quota int
		}
		return db.Raw("SELECT id, quota FROM users WHERE id = ?", 1+rand.Intn(1000)).Scan(&row).Error
	})
	wg.Wait()
	result.errors = errCount.Load()
	return result
}

var lcWords = strings.Fields("the quick brown fox jumps over lazy dog model response token stream " +
	"assistant user content system message function call result data value index error retry " +
	"upstream channel provider request latency billing quota group priority weight alpha beta gamma")

func lcText(rng *rand.Rand, size int) []byte {
	var b strings.Builder
	for b.Len() < size {
		b.WriteString(lcWords[rng.Intn(len(lcWords))])
		b.WriteByte(' ')
	}
	return []byte(b.String()[:size])
}

// lcRecord builds a call with a client row and one provider row.
func lcRecord(rng *rand.Rand, id string, requestSize, responseSize int) *captureRecord {
	body := func(size int) capturedBody {
		return capturedBody{Data: []byte(`{"messages":[{"role":"user","content":"` + string(lcText(rng, size)) + `"}]}`)}
	}
	rec := &captureRecord{RequestId: id, UserId: 1 + rng.Intn(1000), CreatedAt: time.Now().Unix()}
	for attempt := 0; attempt <= 1; attempt++ {
		rec.Attempts = append(rec.Attempts, captureAttempt{
			Attempt:  attempt,
			Meta:     model.LogContentMeta{RelayFormat: "openai", HttpStatus: 200, MultiKeyIndex: -1},
			Request:  body(requestSize),
			Response: body(responseSize),
		})
	}
	return rec
}

func lcEnqueue(rec *captureRecord) bool {
	size := int64(0)
	for _, a := range rec.Attempts {
		size += int64(len(a.Request.Data) + len(a.Response.Data))
	}
	if !tryReserve(size) {
		statDropped.Add(1)
		return false
	}
	rec.reserved = size
	return enqueue(rec)
}

func lcSeed(t *testing.T, db *gorm.DB, cutoff int64) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}))
	require.NoError(t, model.MigrateLogContents())
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, password, quota, "group")
		SELECT g, 'load-user-' || g, 'x', 1000000, 'default' FROM generate_series(1, 1000) g`).Error)
	require.NoError(t, db.Exec(`INSERT INTO logs (user_id, created_at, type, content, model_name, quota)
		SELECT 1 + g % 1000, 1700000000 + g, 2, 'seed', 'seed-model', 10 FROM generate_series(1, 100000) g`).Error)
	// 1 KiB request and 1 KiB response each, roughly what a gzip'd small call weighs.
	require.NoError(t, db.Exec(`INSERT INTO log_contents (request_id, attempt, user_id, created_at, capture_status, meta, request_body, response_body)
		SELECT 'old-' || g, 0, 1 + g % 1000, ? - 1 - (g % 86400), 'captured', '{}',
		       decode(repeat('ab', 1024), 'hex'), decode(repeat('cd', 1024), 'hex')
		FROM generate_series(1, ?) g`, cutoff, lcLoadExpiredRows).Error)
	// Rows just after the cutoff: created_at = cutoff + 0..99, none expired.
	require.NoError(t, db.Exec(`INSERT INTO log_contents (request_id, attempt, user_id, created_at, capture_status, meta, request_body)
		SELECT 'target-' || g, 0, 1, ? + g, 'captured', '{}', decode(repeat('ef', 1024), 'hex')
		FROM generate_series(0, ?) g`, cutoff, lcLoadTargetRows-1).Error)
	require.NoError(t, db.Exec("ANALYZE").Error)
}

// lcLockWaits polls pg_stat_activity and returns every distinct query seen
// waiting on a lock.
func lcLockWaits(db *gorm.DB, stop <-chan struct{}) (get func() []string) {
	var mu sync.Mutex
	seen := map[string]bool{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
			var queries []string
			if err := db.Raw(`SELECT query FROM pg_stat_activity
				WHERE wait_event_type = 'Lock' AND datname = current_database() AND pid <> pg_backend_pid()`).
				Scan(&queries).Error; err != nil {
				continue
			}
			mu.Lock()
			for _, q := range queries {
				seen[q] = true
			}
			mu.Unlock()
		}
	}()
	return func() []string {
		<-done
		mu.Lock()
		defer mu.Unlock()
		out := make([]string, 0, len(seen))
		for q := range seen {
			out = append(out, q)
		}
		return out
	}
}

func lcWaitDrained(t *testing.T, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		s := Stats()
		if s.Enqueued+s.Dropped > 0 && s.Written+s.WriteErrors >= 2*s.Enqueued && s.BudgetInUse == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("recorder did not drain: %+v", Stats())
}

func TestLogContentPostgresDoesNotDisturbLogs(t *testing.T) {
	db := lcPostgres(t)
	newTestPipeline(t, false)
	model.LOG_DB = db // newTestPipeline restores the previous DB on cleanup
	common.SetLogDatabaseType(common.DatabaseTypePostgreSQL)
	cutoff := time.Now().Unix() - lcLoadRetentionDays*86400
	lcSeed(t, db, cutoff)

	// Phase 1: base, no capture load.
	stop := make(chan struct{})
	time.AfterFunc(lcLoadPhase, func() { close(stop) })
	base := lcRunProbes(db, stop)

	// Phase 2: 200 records/s and retention deleting 200k expired rows.
	stop = make(chan struct{})
	locks := lcLockWaits(db, stop)
	var deleted atomic.Int64
	retentionDone := make(chan struct{})
	var retentionStart, retentionEnd time.Time
	var retentionErr error
	go func() {
		defer close(retentionDone)
		retentionStart = time.Now()
		for {
			n, err := model.DeleteOldLogContentBatch(context.Background(), cutoff, lcLoadRetentionBatch)
			if err != nil {
				retentionErr = err
				break
			}
			deleted.Add(n)
			if n < lcLoadRetentionBatch {
				break
			}
		}
		retentionEnd = time.Now()
	}()
	producerStop := make(chan struct{})
	var producerWg sync.WaitGroup
	producerWg.Go(func() {
		rng := rand.New(rand.NewSource(1))
		interval := time.Second / lcLoadRate
		next := time.Now()
		for i := 0; ; i++ {
			select {
			case <-producerStop:
				return
			default:
			}
			lcEnqueue(lcRecord(rng, fmt.Sprintf("load-%d", i), 4<<10, 16<<10))
			next = next.Add(interval)
			time.Sleep(time.Until(next))
		}
	})
	started := time.Now()
	time.AfterFunc(lcLoadPhase, func() {
		// The window ends after 60 s and once the retention has finished.
		<-retentionDone
		close(producerStop)
		close(stop)
	})
	loaded := lcRunProbes(db, stop)
	producerWg.Wait()
	window := time.Since(started)
	lcWaitDrained(t, 60*time.Second)
	waits := locks()

	stats := Stats()
	baseIns, baseSel := lcPercentile(base.insertLogs, 0.99), lcPercentile(base.selectUsers, 0.99)
	loadIns, loadSel := lcPercentile(loaded.insertLogs, 0.99), lcPercentile(loaded.selectUsers, 0.99)
	t.Logf("window %.1fs; retention %.1fs deleted %d (batch %d, no pause), error %v", window.Seconds(),
		retentionEnd.Sub(retentionStart).Seconds(), deleted.Load(), lcLoadRetentionBatch, retentionErr)
	t.Logf("samples base insert/select %d/%d, loaded %d/%d; probe errors base %d loaded %d",
		len(base.insertLogs), len(base.selectUsers), len(loaded.insertLogs), len(loaded.selectUsers), base.errors, loaded.errors)
	t.Logf("INSERT logs  p50 base %v load %v | p99 base %v load %v | limit %v | max base %v load %v", lcPercentile(base.insertLogs, 0.5),
		lcPercentile(loaded.insertLogs, 0.5), baseIns, loadIns, 2*baseIns+5*time.Millisecond,
		lcPercentile(base.insertLogs, 1), lcPercentile(loaded.insertLogs, 1))
	t.Logf("SELECT users p50 base %v load %v | p99 base %v load %v | limit %v | max base %v load %v", lcPercentile(base.selectUsers, 0.5),
		lcPercentile(loaded.selectUsers, 0.5), baseSel, loadSel, 2*baseSel+5*time.Millisecond,
		lcPercentile(base.selectUsers, 1), lcPercentile(loaded.selectUsers, 1))
	// p99 inside the retention window only (the 60 s average dilutes it).
	retIns := within(loaded.insertLogs, loaded.insertAt, retentionStart, retentionEnd)
	retSel := within(loaded.selectUsers, loaded.selectAt, retentionStart, retentionEnd)
	t.Logf("during retention only (%.1fs): INSERT logs n=%d p99 %v max %v | SELECT users n=%d p99 %v max %v",
		retentionEnd.Sub(retentionStart).Seconds(), len(retIns), lcPercentile(retIns, 0.99), lcPercentile(retIns, 1),
		len(retSel), lcPercentile(retSel, 0.99), lcPercentile(retSel, 1))
	t.Logf("recorder: enqueued %d dropped %d written %d write_errors %d budget %d", stats.Enqueued, stats.Dropped, stats.Written, stats.WriteErrors, stats.BudgetInUse)
	t.Logf("lock waits seen: %d %q", len(waits), waits)

	require.NoError(t, retentionErr)
	assert.LessOrEqual(t, lcPercentile(retIns, 0.99), 2*baseIns+5*time.Millisecond, "INSERT logs p99 during retention")
	assert.LessOrEqual(t, lcPercentile(retSel, 0.99), 2*baseSel+5*time.Millisecond, "SELECT users p99 during retention")
	assert.LessOrEqual(t, loadIns, 2*baseIns+5*time.Millisecond, "INSERT logs p99 under load")
	assert.LessOrEqual(t, loadSel, 2*baseSel+5*time.Millisecond, "SELECT users p99 under load")
	for _, q := range waits {
		lower := strings.ToLower(q)
		assert.False(t, strings.Contains(lower, " logs") || strings.Contains(lower, " users") || strings.Contains(lower, "\"logs\"") || strings.Contains(lower, "\"users\""),
			"lock wait on logs/users: %s", q)
	}
	assert.Zero(t, stats.WriteErrors)
	assert.Zero(t, base.errors+loaded.errors, "probe errors")

	// Retention removed every expired row and kept the 100 just after the cutoff.
	var expired, targets, total int64
	require.NoError(t, db.Model(&model.LogContent{}).Where("created_at < ?", cutoff).Count(&expired).Error)
	require.NoError(t, db.Model(&model.LogContent{}).Where("request_id LIKE 'target-%'").Count(&targets).Error)
	require.NoError(t, db.Model(&model.LogContent{}).Count(&total).Error)
	t.Logf("after: expired %d, target rows kept %d, total rows %d", expired, targets, total)
	assert.EqualValues(t, 0, expired)
	assert.EqualValues(t, lcLoadTargetRows, targets, "rows at the cutoff must not be deleted")
	assert.EqualValues(t, lcLoadExpiredRows, deleted.Load())
	assert.EqualValues(t, stats.Written+lcLoadTargetRows, total)
}

// TestLogContentPostgresBurst is the Task 11 burst (1000 calls with no pause,
// arriving within about 250 ms) against PostgreSQL. The instant variant offers
// all 1000 at once, harsher than any real arrival.
func TestLogContentPostgresBurst(t *testing.T) {
	for _, tc := range []struct {
		name   string
		spread time.Duration
	}{{"paced_250ms", 250 * time.Millisecond}, {"instant", 0}} {
		t.Run(tc.name, func(t *testing.T) {
			db := lcPostgres(t)
			newTestPipeline(t, false)
			model.LOG_DB = db
			common.SetLogDatabaseType(common.DatabaseTypePostgreSQL)
			require.NoError(t, model.MigrateLogContents())

			const calls = 1000
			rng := rand.New(rand.NewSource(2))
			records := make([]*captureRecord, calls)
			for i := range records {
				records[i] = lcRecord(rng, fmt.Sprintf("burst-%d", i), 4<<10, 64<<10)
			}
			start := time.Now()
			for i, rec := range records {
				if tc.spread > 0 {
					time.Sleep(time.Until(start.Add(tc.spread * time.Duration(i) / calls)))
				}
				lcEnqueue(rec)
			}
			enqueueTook := time.Since(start)
			lcWaitDrained(t, 60*time.Second)
			total := time.Since(start)
			stats := Stats()
			var rows int64
			require.NoError(t, db.Model(&model.LogContent{}).Count(&rows).Error)
			t.Logf("burst %s: %d calls offered in %v, drained in %v; enqueued %d dropped %d (%.1f%%) written %d write_errors %d rows in table %d",
				tc.name, calls, enqueueTook, total, stats.Enqueued, stats.Dropped, 100*float64(stats.Dropped)/calls, stats.Written, stats.WriteErrors, rows)
			assert.EqualValues(t, calls, stats.Enqueued+stats.Dropped)
			assert.Zero(t, stats.WriteErrors)
			assert.EqualValues(t, 2*stats.Enqueued, stats.Written)
			assert.EqualValues(t, stats.Written, rows)
		})
	}
}
