package openai

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service/logcontent"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Measurements of capture on against capture off, in the same process, with
// the real recorder writing to SQLite. The default run is a smoke version
// that checks the measuring chain works and the budget comes back; the full
// sizes and the thresholds run only with -tags logcontentmeasure:
//
//	go test -tags logcontentmeasure -count=1 -timeout 30m ./relay/channel/openai/ -run 'LogContentMeasure' -v

const mib = 1 << 20

// lcmScale sets the sizes of a run. thresholds is set only in the full run.
type lcmScale struct {
	thresholds bool

	latencyStreams, latencyRounds int
	durationBytes                 int64
	durationRuns                  int

	memStreams        int
	memLong, memShort int64
	memRuns           int
	largeCalls        int
	largeBytes        int
	finishSmall       int
	finishLarge       int
	finishSmallIter   int
	finishLargeIter   int
	settle            time.Duration
	samplePeriod      time.Duration
}

// lcmFull is set by log_content_measure_full_test.go under the
// logcontentmeasure build tag.
var lcmFull bool

func lcmScaleFor() lcmScale {
	if lcmFull {
		return lcmScale{
			thresholds:      true,
			latencyStreams:  50,
			latencyRounds:   20,
			durationBytes:   100 * mib,
			durationRuns:    5,
			memStreams:      50,
			memLong:         300 * mib,
			memShort:        100 * mib,
			memRuns:         3,
			largeCalls:      20,
			largeBytes:      50 * mib,
			finishSmall:     1 * mib,
			finishLarge:     8 * mib,
			finishSmallIter: 200,
			finishLargeIter: 100,
			settle:          5 * time.Second,
			samplePeriod:    5 * time.Millisecond,
		}
	}
	return lcmScale{
		latencyStreams:  4,
		latencyRounds:   2,
		durationBytes:   1 * mib,
		durationRuns:    1,
		memStreams:      4,
		memLong:         8 * mib,
		memShort:        4 * mib,
		memRuns:         1,
		largeCalls:      2,
		largeBytes:      9 * mib,
		finishSmall:     64 << 10,
		finishLarge:     1 * mib,
		finishSmallIter: 3,
		finishLargeIter: 3,
		samplePeriod:    5 * time.Millisecond,
	}
}

const lcmContent = "lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor. "

// lcmFrame is a 4 KiB chat chunk: 1 KiB of text, so the rebuilt message
// reaches its 1 MiB cap within the first 5 MiB of stream, padded by an
// obfuscation field as OpenAI sends.
func lcmFrame() string {
	text := strings.Repeat(lcmContent, 1024/len(lcmContent)+1)[:1024]
	frame := `data: {"id":"chatcmpl-lcm","object":"chat.completion.chunk","created":1767225600,"model":"gpt-test","choices":[{"index":0,"delta":{"content":"` +
		text + `"}}],"obfuscation":"%s"}` + "\n\n"
	pad := 4096 - len(frame) + 2
	return fmt.Sprintf(frame, strings.Repeat("pad ", pad/4+1)[:pad])
}

var lcmStreamEnd = lccSSE(lccChatFinish, lccChatUsage) + "data: [DONE]\n\n"

// lcmGate holds every stream of a run at its midpoint, so the heap can be
// read with nothing allocating.
type lcmGate struct {
	arrived atomic.Int64
	release chan struct{}
}

// lcmStreamProvider streams the role frame, then 1 MiB blocks of frames until
// size, then finish, usage and [DONE], with no wait between writes. With a
// gate set, each stream waits at half its size until the gate opens.
func lcmStreamProvider(size int64, gate *atomic.Pointer[lcmGate]) http.HandlerFunc {
	frame := lcmFrame()
	block := []byte(strings.Repeat(frame, mib/len(frame)))
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte(lccSSE(lccChatRole)))
		flusher.Flush()
		var held *lcmGate
		if gate != nil {
			held = gate.Load()
		}
		for sent := int64(0); sent < size; sent += int64(len(block)) {
			if held != nil && sent >= size/2 {
				held.arrived.Add(1)
				<-held.release
				held = nil
			}
			if _, err := w.Write(block); err != nil {
				return
			}
			flusher.Flush()
		}
		_, _ = w.Write([]byte(lcmStreamEnd))
	}
}

// lcmClient is the client side of a call: it keeps no body, only when the
// first data arrived and how many bytes came, also added to progress when set.
type lcmClient struct {
	header   http.Header
	status   int
	start    time.Time
	first    atomic.Int64
	bytes    atomic.Int64
	progress *atomic.Int64
}

func newLcmClient(start time.Time, progress *atomic.Int64) *lcmClient {
	return &lcmClient{header: http.Header{}, status: http.StatusOK, start: start, progress: progress}
}

func (w *lcmClient) Header() http.Header    { return w.header }
func (w *lcmClient) WriteHeader(status int) { w.status = status }
func (w *lcmClient) Flush()                 {}
func (w *lcmClient) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *lcmClient) Write(p []byte) (int, error) {
	if len(p) > 0 && w.first.Load() == 0 {
		w.first.CompareAndSwap(0, int64(time.Since(w.start)))
	}
	w.bytes.Add(int64(len(p)))
	if w.progress != nil {
		w.progress.Add(int64(len(p)))
	}
	return len(p), nil
}

type lcmCall struct {
	baseURL string
	stream  bool
	request []byte
	// progress counts the bytes all clients of a run received.
	progress *atomic.Int64
	// gate, when set, is the midpoint gate of the provider's streams.
	gate *atomic.Pointer[lcmGate]
	// holdAfterResponse holds every call of a run once its response has
	// reached the client, before Finish, so all copies are in flight at once.
	holdAfterResponse bool
	held              *lcmGate
}

type lcmResult struct {
	ttfb, total, finish time.Duration
	bytes               int64
}

var lcmRequestSeq atomic.Int64

// lcmServe runs one call as controller.Relay does: capture hook, client body
// storage, DoApiRequest, DoResponse, the log row, then Finish, whose own
// duration is timed apart. The caller sets the capture switch.
func lcmServe(call lcmCall, capture bool) (lcmResult, error) {
	start := time.Now()
	client := newLcmClient(start, call.progress)
	c, _ := gin.CreateTestContext(client)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(call.request))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.RequestIdKey, fmt.Sprintf("lcm-%d", lcmRequestSeq.Add(1)))
	storage, err := common.CreateBodyStorage(call.request)
	if err != nil {
		return lcmResult{}, err
	}
	defer storage.Close()
	c.Set(common.KeyBodyStorage, storage)

	logCapture := logcontent.Begin(c, types.RelayFormatOpenAI)
	if capture != (logCapture != nil) {
		return lcmResult{}, fmt.Errorf("capture %v but Begin returned %v", capture, logCapture)
	}
	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		IsStream:        call.stream,
		RequestURLPath:  "/v1/chat/completions",
		OriginModelName: "gpt-test",
		StartTime:       start,
		StreamStatus:    relaycommon.NewStreamStatus(),
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			ApiType:           constant.APITypeOpenAI,
			ChannelBaseUrl:    call.baseURL,
			ApiKey:            "sk-lcm-measure-0123456789abcdef",
			UpstreamModelName: "gpt-test",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)
	resp, err := channel.DoApiRequest(adaptor, c, info, bytes.NewReader(call.request))
	if err != nil {
		logCapture.Finish()
		return lcmResult{}, err
	}
	_, apiErr := adaptor.DoResponse(c, resp, info)
	if call.held != nil {
		call.held.arrived.Add(1)
		<-call.held.release
	}
	logCapture.OnLogRow(&model.Log{UserId: 1, Type: model.LogTypeConsume})
	finishStart := time.Now()
	logCapture.Finish()
	result := lcmResult{
		ttfb:   time.Duration(client.first.Load()),
		total:  time.Since(start),
		finish: time.Since(finishStart),
		bytes:  client.bytes.Load(),
	}
	if apiErr != nil {
		return result, apiErr
	}
	if client.status != http.StatusOK {
		return result, fmt.Errorf("client status %d", client.status)
	}
	return result, nil
}

// lcmConcurrent starts n calls together and returns their results.
func lcmConcurrent(n int, call lcmCall, capture bool) ([]lcmResult, error) {
	results := make([]lcmResult, n)
	errs := make([]error, n)
	common.LogContentCaptureEnabled = capture
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			<-start
			results[i], errs[i] = lcmServe(call, capture)
		})
	}
	close(start)
	wg.Wait()
	return results, errors.Join(errs...)
}

// lcmUseLogDB points the recorder at a fresh SQLite log database. Each test
// waits for its rows to be written, so the cleanup only waits for the budget.
func lcmUseLogDB(t *testing.T) {
	t.Helper()
	lccKeepSettings(t)
	previousDB, previousType := model.LOG_DB, common.LogDatabaseType()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/log.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.LOG_DB = db
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	require.NoError(t, model.MigrateLogContents())
	t.Cleanup(func() {
		lcmWaitBudget(10 * time.Second)
		model.LOG_DB = previousDB
		common.SetLogDatabaseType(previousType)
		_ = sqlDB.Close()
	})
}

// lcmWaitBudget waits until every copy in flight has been compressed.
func lcmWaitBudget(limit time.Duration) int64 {
	deadline := time.Now().Add(limit)
	for logcontent.Stats().BudgetInUse != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	return logcontent.Stats().BudgetInUse
}

// lcmWaitWritten waits until every one of calls has been enqueued or
// dropped and every enqueued call has its two rows written. Each poll reads
// all counters at once: a call whose fold enqueues it later is still counted.
func lcmWaitWritten(before logcontent.StatsSnapshot, calls int64) logcontent.StatsSnapshot {
	deadline := time.Now().Add(15 * time.Second)
	for {
		now := logcontent.Stats()
		enqueued, dropped := now.Enqueued-before.Enqueued, now.Dropped-before.Dropped
		settled := enqueued+dropped >= calls && now.Written-before.Written >= 2*enqueued
		if settled || time.Now().After(deadline) {
			return now
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// lcmCheckRecorder asserts that each captured call reached the database as
// two rows (client level and provider attempt), with nothing dropped. With
// mayDrop, a full queue may drop calls; the drops are reported, and every
// call is still either enqueued and written or dropped.
func lcmCheckRecorder(t *testing.T, before logcontent.StatsSnapshot, calls int, mayDrop bool) {
	t.Helper()
	after := lcmWaitWritten(before, int64(calls))
	t.Logf("recorder: enqueued %d, written %d, dropped %d, write errors %d, budget in use %d",
		after.Enqueued-before.Enqueued, after.Written-before.Written, after.Dropped-before.Dropped,
		after.WriteErrors-before.WriteErrors, after.BudgetInUse)
	enqueued, dropped := after.Enqueued-before.Enqueued, after.Dropped-before.Dropped
	assert.Equal(t, int64(calls), enqueued+dropped, "every captured call was enqueued or dropped")
	assert.Equal(t, 2*enqueued, after.Written-before.Written, "every enqueued call was written")
	assert.Zero(t, after.WriteErrors-before.WriteErrors)
	if !mayDrop {
		assert.Zero(t, dropped, "no call dropped")
	}
}

func lcmPercentile(values []time.Duration, p float64) time.Duration {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	return sorted[max(index, 0)]
}

func lcmMedian(values []float64) float64 {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func lcmMode(capture bool) string {
	if capture {
		return "on"
	}
	return "off"
}

func lcmMiB(bytes float64) float64 { return bytes / mib }

func TestLogContentMeasureStreamLatency(t *testing.T) {
	scale := lcmScaleFor()
	lcmUseLogDB(t)
	request := []byte(lccChatRequest)

	// Time to first byte, 50 streams at a time, rounds alternating off and on.
	short := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(lccSSE(lccChatRole, lccChatText)))
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte(strings.Repeat(lcmFrame(), 16) + lcmStreamEnd))
	}))
	defer short.Close()
	call := lcmCall{baseURL: short.URL, stream: true, request: request}
	ttfb := map[bool][]time.Duration{}
	before := logcontent.Stats()
	for range scale.latencyRounds {
		for _, capture := range []bool{false, true} {
			results, err := lcmConcurrent(scale.latencyStreams, call, capture)
			require.NoError(t, err)
			for _, result := range results {
				require.Positive(t, result.ttfb, "the client received data")
				ttfb[capture] = append(ttfb[capture], result.ttfb)
			}
		}
	}
	// Rounds follow each other with no pause, faster than the recorder
	// drains: a full queue drops, by design, and the count is reported.
	lcmCheckRecorder(t, before, scale.latencyStreams*scale.latencyRounds, true)
	p50 := map[bool]time.Duration{}
	p99 := map[bool]time.Duration{}
	for _, capture := range []bool{false, true} {
		p50[capture] = lcmPercentile(ttfb[capture], 0.50)
		p99[capture] = lcmPercentile(ttfb[capture], 0.99)
		t.Logf("first byte, capture %s, %d streams x %d rounds: p50 %v, p99 %v, max %v", lcmMode(capture),
			scale.latencyStreams, scale.latencyRounds, p50[capture], p99[capture], lcmPercentile(ttfb[capture], 1))
	}
	t.Logf("first byte, on minus off: p50 %v (limit 1ms), p99 %v (limit 5ms)", p50[true]-p50[false], p99[true]-p99[false])

	// Total duration of one long stream with no wait at the provider.
	long := httptest.NewServer(lcmStreamProvider(scale.durationBytes, nil))
	defer long.Close()
	call = lcmCall{baseURL: long.URL, stream: true, request: request}
	durations := map[bool][]float64{}
	before = logcontent.Stats()
	for range scale.durationRuns {
		for _, capture := range []bool{false, true} {
			common.LogContentCaptureEnabled = capture
			result, err := lcmServe(call, capture)
			require.NoError(t, err)
			require.Greater(t, result.bytes, scale.durationBytes, "the whole stream reached the client")
			durations[capture] = append(durations[capture], float64(result.total))
			lcmWaitBudget(10 * time.Second)
		}
	}
	lcmCheckRecorder(t, before, scale.durationRuns, false)
	for _, capture := range []bool{false, true} {
		runs := make([]string, 0, len(durations[capture]))
		for _, d := range durations[capture] {
			runs = append(runs, time.Duration(d).Round(time.Millisecond).String())
		}
		t.Logf("%d MiB stream, capture %s: median %v, runs %s", scale.durationBytes/mib, lcmMode(capture),
			time.Duration(lcmMedian(durations[capture])).Round(time.Millisecond), strings.Join(runs, " "))
	}
	overhead := lcmMedian(durations[true])/lcmMedian(durations[false]) - 1
	t.Logf("%d MiB stream, on over off: %+.2f%% (limit +5%%)", scale.durationBytes/mib, overhead*100)

	if scale.thresholds {
		assert.LessOrEqual(t, p50[true]-p50[false], time.Millisecond, "first byte p50, on minus off")
		assert.LessOrEqual(t, p99[true]-p99[false], 5*time.Millisecond, "first byte p99, on minus off")
		assert.LessOrEqual(t, overhead, 0.05, "long stream duration, on over off")
	}
}

// lcmMemory is one memory run. Raw peak HeapInuse is reported only: under
// GOGC=100 extra live data raises the heap goal with it, and its run-to-run
// noise exceeded the limits. What the capture holds is measured instead as
// the budget in use and as the heap still allocated after a forced GC in the
// middle of the streams.
type lcmMemory struct {
	base, peak, after   uint64
	baseAlloc, midAlloc uint64
	midSampled          bool
	peakBudget          int64
	heldBudget          int64
	heldSampled         bool
	budgetAfter         int64
	elapsed             time.Duration
}

func (m lcmMemory) retained() float64 { return float64(m.midAlloc) - float64(m.baseAlloc) }

func lcmMemStats() runtime.MemStats {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats
}

// lcmMeasureMemory runs n concurrent calls, sampling HeapInuse and the budget
// until they end. For gated streams, once every stream waits at its midpoint
// and the clients received all that was sent, it forces a GC and reads
// HeapAlloc: the memory the calls keep, with nothing allocating meanwhile.
func lcmMeasureMemory(t *testing.T, scale lcmScale, n int, call lcmCall, capture bool, minBytes int64) lcmMemory {
	t.Helper()
	call.progress = &atomic.Int64{}
	var gate *lcmGate
	if call.gate != nil {
		gate = &lcmGate{release: make(chan struct{})}
		call.gate.Store(gate)
		defer call.gate.Store(nil)
	}
	if call.holdAfterResponse {
		gate = &lcmGate{release: make(chan struct{})}
		call.held = gate
	}
	var lastProgress int64
	var steadySince time.Time
	gateOpen := false
	gateDeadline := time.Now().Add(30 * time.Second)
	runtime.GC()
	runtime.GC()
	stats := lcmMemStats()
	run := lcmMemory{base: stats.HeapInuse, baseAlloc: stats.HeapAlloc}
	stop := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		ticker := time.NewTicker(scale.samplePeriod)
		defer ticker.Stop()
		for {
			run.peak = max(run.peak, lcmMemStats().HeapInuse)
			run.peakBudget = max(run.peakBudget, logcontent.Stats().BudgetInUse)
			if call.held != nil && !gateOpen && gate.arrived.Load() == int64(n) {
				// Every call holds its copies: this is their sum.
				run.heldBudget = logcontent.Stats().BudgetInUse
				run.peakBudget = max(run.peakBudget, run.heldBudget)
				run.heldSampled = true
				close(gate.release)
				gateOpen = true
			}
			if call.gate != nil && !run.midSampled && gate.arrived.Load() == int64(n) {
				// All held: wait until the relay has passed on what it read.
				if progress := call.progress.Load(); progress != lastProgress {
					lastProgress, steadySince = progress, time.Now()
				} else if time.Since(steadySince) >= 100*time.Millisecond {
					runtime.GC()
					runtime.GC()
					run.midAlloc = lcmMemStats().HeapAlloc
					run.midSampled = true
					close(gate.release)
					gateOpen = true
				}
			}
			if gate != nil && !gateOpen && time.Now().After(gateDeadline) {
				// Not every stream reached the gate: open it so none hangs.
				close(gate.release)
				gateOpen = true
			}
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
	start := time.Now()
	results, err := lcmConcurrent(n, call, capture)
	run.elapsed = time.Since(start)
	close(stop)
	<-sampled
	require.NoError(t, err)
	for _, result := range results {
		require.GreaterOrEqual(t, result.bytes, minBytes, "the whole body reached the client")
	}
	if call.gate != nil {
		require.True(t, run.midSampled, "the heap was sampled in the middle of the streams")
	}
	if call.held != nil {
		require.True(t, run.heldSampled, "the budget was read with every call held")
	}

	run.budgetAfter = lcmWaitBudget(10 * time.Second)
	time.Sleep(scale.settle)
	runtime.GC()
	run.after = lcmMemStats().HeapInuse
	midpointText := "not sampled"
	if run.midSampled {
		midpointText = fmt.Sprintf("%.1f MiB", lcmMiB(run.retained()))
	}
	t.Logf("capture %s: retained at midpoint %s, peak budget %.2f MiB (%d B), heap in use base %.1f MiB, peak %.1f MiB (+%.1f), after %.1f MiB (base %+.1f), budget after %d, %v",
		lcmMode(capture), midpointText, lcmMiB(float64(run.peakBudget)), run.peakBudget,
		lcmMiB(float64(run.base)), lcmMiB(float64(run.peak)), lcmMiB(float64(run.peak)-float64(run.base)),
		lcmMiB(float64(run.after)), lcmMiB(float64(run.after)-float64(run.base)),
		run.budgetAfter, run.elapsed.Round(time.Millisecond))
	return run
}

// lcmMemoryCost is what alternating off and on runs measured: the median
// retained heap of each mode and the highest budget in use.
type lcmMemoryCost struct {
	retainedOff, retainedOn float64
	peakBudget              int64
}

func (c lcmMemoryCost) retained() float64 { return c.retainedOn - c.retainedOff }

// lcmCompareMemory alternates off and on runs, checking after every run that
// nothing stayed behind.
func lcmCompareMemory(t *testing.T, scale lcmScale, label string, n int, call lcmCall, minBytes int64) lcmMemoryCost {
	t.Helper()
	retained := map[bool][]float64{}
	var cost lcmMemoryCost
	before := logcontent.Stats()
	for i := range scale.memRuns {
		for _, capture := range []bool{false, true} {
			t.Logf("%s run %d", label, i+1)
			run := lcmMeasureMemory(t, scale, n, call, capture, minBytes)
			retained[capture] = append(retained[capture], run.retained())
			cost.peakBudget = max(cost.peakBudget, run.peakBudget)
			assert.Zero(t, run.budgetAfter, "%s: budget returned after the calls", label)
			if scale.thresholds {
				assert.LessOrEqual(t, float64(run.after), float64(run.base)+16*mib,
					"%s, capture %s: heap after 5 s and GC within base + 16 MiB", label, lcmMode(capture))
			}
		}
	}
	lcmCheckRecorder(t, before, n*scale.memRuns, false)
	cost.retainedOff, cost.retainedOn = lcmMedian(retained[false]), lcmMedian(retained[true])
	t.Logf("%s: peak budget %.2f MiB (%d B)", label, lcmMiB(float64(cost.peakBudget)), cost.peakBudget)
	if call.gate != nil {
		t.Logf("%s: median retained at midpoint off %.1f MiB, on %.1f MiB, on minus off %.1f MiB",
			label, lcmMiB(cost.retainedOff), lcmMiB(cost.retainedOn), lcmMiB(cost.retained()))
	}
	return cost
}

// lcmRequestReserve is the budget a stream's request copies take: the copy
// of the request sent to the provider and, in Finish, the copy of the client
// request. A body under 4 KiB grows to the tap's first capacity, 4 KiB
// (tapBuffer.grow), so each takes 4 KiB.
const lcmRequestReserve = 2 * (4 << 10)

// lcmStreamBudget is the designed budget of one stream: 1 MiB provider copy,
// 1 MiB client copy, 512 KiB of fold blocks and 1 MiB rebuilt message, plus
// the request copies.
const lcmStreamBudget = 7*mib/2 + lcmRequestReserve

// Both lengths fill every cap long before their midpoint (the rebuilt
// message within the first 5 MiB), so a difference between them would be
// memory growing with length. A stream too short to fill its caps holds less,
// which says nothing about growth, so no shorter length is compared.
func TestLogContentMeasureLongStreamMemory(t *testing.T) {
	scale := lcmScaleFor()
	lcmUseLogDB(t)
	request := []byte(lccChatRequest)
	require.Less(t, len(lccChatRequest), 4<<10, "the request copies take the first tap capacity")

	var longGate, shortGate atomic.Pointer[lcmGate]
	long := httptest.NewServer(lcmStreamProvider(scale.memLong, &longGate))
	defer long.Close()
	shortServer := httptest.NewServer(lcmStreamProvider(scale.memShort, &shortGate))
	defer shortServer.Close()

	shortLabel := fmt.Sprintf("%d streams of %d MiB", scale.memStreams, scale.memShort/mib)
	shortCost := lcmCompareMemory(t, scale, shortLabel, scale.memStreams,
		lcmCall{baseURL: shortServer.URL, stream: true, request: request, gate: &shortGate}, scale.memShort)
	longLabel := fmt.Sprintf("%d streams of %d MiB", scale.memStreams, scale.memLong/mib)
	longCost := lcmCompareMemory(t, scale, longLabel, scale.memStreams,
		lcmCall{baseURL: long.URL, stream: true, request: request, gate: &longGate}, scale.memLong)

	budgetLimit := int64(scale.memStreams) * lcmStreamBudget
	const retainedLimit = 207 * mib
	spread := math.Abs(longCost.retained()-shortCost.retained()) / math.Max(longCost.retained(), shortCost.retained())
	for _, cost := range []struct {
		label string
		cost  lcmMemoryCost
	}{{shortLabel, shortCost}, {longLabel, longCost}} {
		t.Logf("%s: peak budget %d B (limit %d x (3.5 MiB + %d B) = %d B), retained on minus off %.1f MiB (limit 207 MiB)",
			cost.label, cost.cost.peakBudget, scale.memStreams, lcmRequestReserve, budgetLimit, lcmMiB(cost.cost.retained()))
		if scale.thresholds {
			assert.LessOrEqual(t, cost.cost.peakBudget, budgetLimit, "%s: peak budget in use", cost.label)
			assert.LessOrEqual(t, cost.cost.retained(), float64(retainedLimit), "%s: retained heap, on minus off", cost.label)
		}
	}
	t.Logf("retained on minus off: %s %.1f MiB, %s %.1f MiB, difference %.1f%% (limit 10%%)",
		shortLabel, lcmMiB(shortCost.retained()), longLabel, lcmMiB(longCost.retained()), spread*100)
	if scale.thresholds {
		assert.LessOrEqual(t, spread, 0.10, "retained capture memory must not depend on stream length")
	}
}

// lcmLargeCallBudget is the designed budget of one non-stream call whose
// response is over maxParsedBody (8 MiB): the provider copy knows the size
// from Content-Length and keeps only the head from the start, maxStoredBody +
// tapMarginBytes (1 MiB + 64 KiB); the client copy does not know it, and may
// grow to maxParsedBody before it switches to the head; plus the request
// copies. Without the switch both copies keep 8 MiB, and 20 held calls go
// over the limit.
const lcmLargeCallBudget = (1<<20 + 64<<10) + 8<<20 + lcmRequestReserve

func TestLogContentMeasureLargeResponseMemory(t *testing.T) {
	scale := lcmScaleFor()
	lcmUseLogDB(t)

	content := strings.Repeat(lcmContent, scale.largeBytes/len(lcmContent)+1)[:scale.largeBytes]
	body := []byte(`{"id":"chatcmpl-lcm","object":"chat.completion","created":1767225600,"model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":"` +
		content + `"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":2,"total_tokens":11}}`)
	content = ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		_, _ = w.Write(body)
	}))
	defer server.Close()

	// The relay itself holds each 50 MiB body several times over, so the heap
	// with capture off varies by more than the capture costs: the budget in
	// use, read with every call held after its response, is the measure.
	label := fmt.Sprintf("%d non-stream responses of %d MiB", scale.largeCalls, scale.largeBytes/mib)
	cost := lcmCompareMemory(t, scale, label, scale.largeCalls, lcmCall{
		baseURL:           server.URL,
		request:           []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hi"}]}`),
		holdAfterResponse: true,
	}, int64(scale.largeBytes))
	budgetLimit := int64(scale.largeCalls) * lcmLargeCallBudget
	t.Logf("%s: peak budget %d B (limit %d x (1 MiB + 64 KiB + 8 MiB + %d B) = %d B)",
		label, cost.peakBudget, scale.largeCalls, lcmRequestReserve, budgetLimit)
	if scale.thresholds {
		assert.LessOrEqual(t, cost.peakBudget, budgetLimit, "peak budget in use, 20 responses of 50 MiB")
	}
}

func TestLogContentMeasureFinishCost(t *testing.T) {
	scale := lcmScaleFor()
	lcmUseLogDB(t)

	cases := []struct {
		size, iterations int
		limit            time.Duration
	}{
		{scale.finishSmall, scale.finishSmallIter, 2 * time.Millisecond},
		{scale.finishLarge, scale.finishLargeIter, 10 * time.Millisecond},
	}
	for _, tc := range cases {
		// The request and the response are each about size bytes, so Finish
		// reads a client body of that size and seals copies of that size.
		text := strings.Repeat(lcmContent, tc.size/len(lcmContent)+1)
		request := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"` + text[:tc.size-80] + `"}]}`)
		response := []byte(`{"id":"chatcmpl-lcm","object":"chat.completion","created":1767225600,"model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":"` +
			text[:tc.size-260] + `"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":2,"total_tokens":11}}`)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(response)
		}))
		call := lcmCall{baseURL: server.URL, request: request}

		finish := map[bool][]time.Duration{}
		before := logcontent.Stats()
		for range tc.iterations {
			for _, capture := range []bool{false, true} {
				common.LogContentCaptureEnabled = capture
				result, err := lcmServe(call, capture)
				require.NoError(t, err)
				require.GreaterOrEqual(t, result.bytes, int64(len(response)))
				finish[capture] = append(finish[capture], result.finish)
				// Each call starts with the budget free, as a lone call would.
				lcmWaitBudget(10 * time.Second)
			}
		}
		server.Close()
		label := fmt.Sprintf("Finish, bodies of %.2f MiB (request %d B, response %d B)", float64(tc.size)/mib, len(request), len(response))
		lcmCheckRecorder(t, before, tc.iterations, false)
		for _, capture := range []bool{false, true} {
			t.Logf("%s, capture %s, %d calls: p50 %v, p99 %v, max %v", label, lcmMode(capture), tc.iterations,
				lcmPercentile(finish[capture], 0.50), lcmPercentile(finish[capture], 0.99), lcmPercentile(finish[capture], 1))
		}
		t.Logf("%s: p99 on %v (limit %v)", label, lcmPercentile(finish[true], 0.99), tc.limit)
		if scale.thresholds {
			assert.LessOrEqual(t, lcmPercentile(finish[true], 0.99), tc.limit, "%s p99", label)
		}
	}
}
