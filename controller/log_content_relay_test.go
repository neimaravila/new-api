package controller

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/logcontent"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	captureModel      = "capture-model"
	captureTokenKey   = "captureTokenSecretValue0123456789abcdefghijklmn"
	captureChannelKey = "upstream-channel-secret-0123456789abcdef"
	captureQuerySafe  = "query-secret-value-0123456789"
)

// captureEnv is a gateway running the real middleware and controller.Relay
// against a provider stub, on SQLite.
type captureEnv struct {
	db       *gorm.DB
	gateway  *httptest.Server
	upstream *httptest.Server
	handler  atomic.Pointer[http.HandlerFunc]
	userID   int
	channel  *model.Channel
}

func newCaptureEnv(t *testing.T) *captureEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	service.InitHttpClient()

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	previousMemory, previousRedis, previousBatch := common.MemoryCacheEnabled, common.RedisEnabled, common.BatchUpdateEnabled
	previousConsume, previousExport, previousError := common.LogConsumeEnabled, common.DataExportEnabled, constant.ErrorLogEnabled
	previousRetry, previousCapture := common.RetryTimes, common.LogContentCaptureEnabled
	previousMaster, previousSQLite := common.IsMasterNode, common.SQLitePath
	previousStreamingTimeout := constant.StreamingTimeout
	previousPerfMetrics := perf_metrics_setting.GetSetting().Enabled
	previousRatios := ratio_setting.ModelRatio2JSONString()
	previousCache := ratio_setting.CacheRatio2JSONString()
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMain, previousLog)
		common.IsMasterNode, common.SQLitePath = previousMaster, previousSQLite
		constant.StreamingTimeout = previousStreamingTimeout
		setPerfMetrics(previousPerfMetrics)
		setFlag(&common.MemoryCacheEnabled, previousMemory)
		setFlag(&common.RedisEnabled, previousRedis)
		setFlag(&common.BatchUpdateEnabled, previousBatch)
		setFlag(&common.LogConsumeEnabled, previousConsume)
		setFlag(&common.DataExportEnabled, previousExport)
		setFlag(&constant.ErrorLogEnabled, previousError)
		common.RetryTimes, common.LogContentCaptureEnabled = previousRetry, previousCapture
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousRatios))
		require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(previousCache))
	})

	common.IsMasterNode = true
	constant.StreamingTimeout = 30
	// Perf metrics write to Redis from goroutines that outlive the call.
	setPerfMetrics(false)
	common.SQLitePath = t.TempDir() + "/capture.db?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_txlock=immediate"
	t.Setenv("SQL_DSN", "local")
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitDB())
	require.NoError(t, model.InitLogDB())
	db := model.DB
	sqlDB, err := db.DB()
	require.NoError(t, err)
	setFlag(&common.MemoryCacheEnabled, false)
	setFlag(&common.RedisEnabled, false)
	setFlag(&common.BatchUpdateEnabled, false)
	setFlag(&common.LogConsumeEnabled, true)
	setFlag(&common.DataExportEnabled, false)
	setFlag(&constant.ErrorLogEnabled, false)
	common.RetryTimes, common.LogContentCaptureEnabled = 0, true
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"`+captureModel+`":1}`))
	require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(`{"`+captureModel+`":0.5}`))

	env := &captureEnv{db: db}
	env.upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		(*env.handler.Load())(w, r)
	}))
	user := &model.User{Username: "capture-user", Password: "unused-password", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Quota: 1_000_000}
	require.NoError(t, db.Create(user).Error)
	env.userID = user.Id
	require.NoError(t, db.Create(&model.Token{UserId: user.Id, Key: captureTokenKey, Name: "capture", Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}).Error)
	env.channel = &model.Channel{Type: constant.ChannelTypeOpenAI, Name: "capture-channel", Key: captureChannelKey, BaseURL: &env.upstream.URL, Status: common.ChannelStatusEnabled, Models: captureModel, Group: "default"}
	require.NoError(t, db.Create(env.channel).Error)
	require.NoError(t, env.channel.AddAbilities(db))

	engine := gin.New()
	engine.Use(middleware.RequestId(), middleware.BodyStorageCleanup())
	v1 := engine.Group("/v1", middleware.TokenAuth(), middleware.Distribute())
	v1.POST("/chat/completions", func(c *gin.Context) { Relay(c, types.RelayFormatOpenAI) })
	v1.POST("/moderations", func(c *gin.Context) { Relay(c, types.RelayFormatOpenAI) })
	env.gateway = httptest.NewServer(engine)
	t.Cleanup(func() {
		env.gateway.Close()
		env.upstream.Close()
		_ = sqlDB.Close()
	})
	return env
}

// setFlag writes a global only when it changes, so goroutines of an earlier
// call that still read it do not race with the test setup.
func setFlag(flag *bool, value bool) {
	if *flag != value {
		*flag = value
	}
}

func setPerfMetrics(enabled bool) {
	if perf_metrics_setting.GetSetting().Enabled != enabled {
		config.UpdateConfigFromMap(config.GlobalConfig.Get("perf_metrics_setting"), map[string]string{"enabled": fmt.Sprint(enabled)})
	}
}

func (env *captureEnv) setUpstream(handler http.HandlerFunc) {
	env.handler.Store(&handler)
}

func (env *captureEnv) post(t *testing.T, ctx context.Context, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, env.gateway.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-"+captureTokenKey)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// call posts and reads the whole answer.
func (env *captureEnv) call(t *testing.T, path, body string) (string, string, int) {
	t.Helper()
	resp := env.post(t, context.Background(), path, body)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.Header.Get(common.RequestIdKey), string(data), resp.StatusCode
}

func waitLogContents(t *testing.T, requestID string) []*model.LogContent {
	t.Helper()
	var rows []*model.LogContent
	require.Eventually(t, func() bool {
		rows = nil
		return model.LOG_DB.Where("request_id = ?", requestID).Order("attempt").Find(&rows).Error == nil && len(rows) > 0
	}, 10*time.Second, 20*time.Millisecond, "content rows for %s", requestID)
	return rows
}

// assertNoLogContents checks that the call enqueued nothing and stored nothing.
func assertNoLogContents(t *testing.T, requestID string, enqueuedBefore int64) {
	t.Helper()
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, enqueuedBefore, logcontent.Stats().Enqueued, "nothing is enqueued")
	var count int64
	require.NoError(t, model.LOG_DB.Model(&model.LogContent{}).Where("request_id = ?", requestID).Count(&count).Error)
	assert.Zero(t, count)
}

func gunzipText(t *testing.T, data []byte) string {
	t.Helper()
	if len(data) == 0 {
		return ""
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	text, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(text)
}

func rowMeta(t *testing.T, row *model.LogContent) model.LogContentMeta {
	t.Helper()
	var meta model.LogContentMeta
	require.NoError(t, common.UnmarshalJsonStr(row.Meta, &meta))
	return meta
}

func attemptNumbers(rows []*model.LogContent) []int {
	numbers := make([]int, 0, len(rows))
	for _, row := range rows {
		numbers = append(numbers, row.Attempt)
	}
	return numbers
}

func assertNoSecrets(t *testing.T, rows []*model.LogContent) {
	t.Helper()
	for _, row := range rows {
		for name, text := range map[string]string{
			"meta":      row.Meta,
			"request":   gunzipText(t, row.RequestBody),
			"response":  gunzipText(t, row.ResponseBody),
			"assembled": gunzipText(t, row.AssembledBody),
		} {
			for _, secret := range []string{captureTokenKey, captureChannelKey, captureQuerySafe} {
				assert.NotContains(t, text, secret, "attempt %d %s", row.Attempt, name)
			}
		}
	}
}

func chatBody(stream bool) string {
	return fmt.Sprintf(`{"model":%q,"stream":%t,"messages":[{"role":"user","content":"say hi, my key is sk-%s"}]}`, captureModel, stream, captureTokenKey)
}

// echoCompletion answers with the provider credentials and request it saw, so
// every stored body has secrets to remove.
func echoCompletion(t *testing.T, seenQuery *atomic.Value) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if seenQuery != nil {
			seenQuery.Store(r.URL.RawQuery)
		}
		body, _ := io.ReadAll(r.Body)
		content := "hi; auth " + r.Header.Get("Authorization") + "; request " + string(body)
		encoded, err := common.Marshal(content)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"c1","object":"chat.completion","model":"`+captureModel+`","choices":[{"index":0,"message":{"role":"assistant","content":`+string(encoded)+`},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":5,"total_tokens":25,"prompt_tokens_details":{"cached_tokens":8}}}`)
	}
}

func streamFrames() []string {
	return []string{
		`data: {"id":"s1","object":"chat.completion.chunk","model":"` + captureModel + `","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}` + "\n\n",
		`data: {"id":"s1","object":"chat.completion.chunk","model":"` + captureModel + `","choices":[{"index":0,"delta":{"content":" there"},"finish_reason":"stop"}]}` + "\n\n",
		`data: {"id":"s1","object":"chat.completion.chunk","model":"` + captureModel + `","choices":[],"usage":{"prompt_tokens":20,"completion_tokens":2,"total_tokens":22}}` + "\n\n",
		"data: [DONE]\n\n",
	}
}

func TestCaptureNonStreamStoresCallAndAttempt(t *testing.T) {
	env := newCaptureEnv(t)
	var seenQuery atomic.Value
	env.setUpstream(echoCompletion(t, &seenQuery))

	requestID, body, status := env.call(t, "/v1/chat/completions?secret="+captureQuerySafe, chatBody(false))
	require.Equal(t, http.StatusOK, status, body)
	assert.Contains(t, seenQuery.Load(), captureQuerySafe, "the provider gets the query")

	rows := waitLogContents(t, requestID)
	require.Equal(t, []int{0, 1}, attemptNumbers(rows))
	for _, row := range rows {
		assert.Equal(t, env.userID, row.UserId)
		assert.Equal(t, model.LogContentStatusCaptured, row.CaptureStatus)
	}
	client, upstream := rowMeta(t, rows[0]), rowMeta(t, rows[1])
	assert.Equal(t, logcontent.FormatOpenAI, client.RelayFormat)
	assert.Equal(t, http.StatusOK, client.HttpStatus)
	assert.Equal(t, "/v1/chat/completions", client.Path)
	assert.Equal(t, http.MethodPost, upstream.Method)
	assert.Equal(t, "/v1/chat/completions", upstream.Path)
	assert.Equal(t, http.StatusOK, upstream.HttpStatus)
	assert.Equal(t, env.channel.Id, upstream.ChannelId)
	assert.Equal(t, -1, upstream.MultiKeyIndex)

	assert.Contains(t, gunzipText(t, rows[0].RequestBody), "say hi")
	assert.Contains(t, gunzipText(t, rows[0].ResponseBody), `"chat.completion"`)
	assert.Contains(t, gunzipText(t, rows[1].RequestBody), "say hi")
	assert.Contains(t, gunzipText(t, rows[1].ResponseBody), "hi; auth [REDACTED]")
	assertNoSecrets(t, rows)
}

func TestCaptureStreamStoresRawAndAssembled(t *testing.T) {
	env := newCaptureEnv(t)
	env.setUpstream(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, frame := range streamFrames() {
			_, _ = io.WriteString(w, frame)
			w.(http.Flusher).Flush()
		}
	})

	requestID, body, status := env.call(t, "/v1/chat/completions", chatBody(true))
	require.Equal(t, http.StatusOK, status, body)
	require.Contains(t, body, "data: [DONE]")

	rows := waitLogContents(t, requestID)
	require.Equal(t, []int{0, 1}, attemptNumbers(rows))
	meta := rowMeta(t, rows[0])
	assert.True(t, meta.IsStream)
	assert.False(t, meta.Incomplete)
	assert.Equal(t, body, gunzipText(t, rows[0].ResponseBody), "the raw client stream is stored")
	assembled := gunzipText(t, rows[0].AssembledBody)
	assert.Contains(t, assembled, `"text":"Hello there"`)
	assert.Contains(t, gunzipText(t, rows[1].ResponseBody), `"content":" there"`)
	assert.True(t, rowMeta(t, rows[1]).IsStream)
	assertNoSecrets(t, rows)
}

func TestCaptureRetryStoresEachAttempt(t *testing.T) {
	for _, tc := range []struct {
		errorLogs bool
		want      []int
	}{
		{true, []int{0, 1, 2}},
		{false, []int{0, 2}},
	} {
		t.Run(fmt.Sprintf("error_logs=%t", tc.errorLogs), func(t *testing.T) {
			env := newCaptureEnv(t)
			constant.ErrorLogEnabled = tc.errorLogs
			common.RetryTimes = 2
			var calls atomic.Int32
			success := echoCompletion(t, nil)
			env.setUpstream(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = io.WriteString(w, `{"error":{"message":"slow down","type":"rate_limit"}}`)
					return
				}
				success(w, r)
			})

			requestID, body, status := env.call(t, "/v1/chat/completions", chatBody(false))
			require.Equal(t, http.StatusOK, status, body)
			require.Equal(t, int32(2), calls.Load())

			rows := waitLogContents(t, requestID)
			require.Equal(t, tc.want, attemptNumbers(rows))
			if tc.errorLogs {
				first := rowMeta(t, rows[1])
				assert.Equal(t, http.StatusTooManyRequests, first.HttpStatus)
				assert.NotEmpty(t, first.ErrorMessage)
				assert.Contains(t, gunzipText(t, rows[1].ResponseBody), "slow down")
			}
			assert.Equal(t, http.StatusOK, rowMeta(t, rows[len(rows)-1]).HttpStatus)
			assertNoSecrets(t, rows)
		})
	}
}

func TestCaptureAllAttemptsFailed(t *testing.T) {
	for _, errorLogs := range []bool{true, false} {
		t.Run(fmt.Sprintf("error_logs=%t", errorLogs), func(t *testing.T) {
			env := newCaptureEnv(t)
			constant.ErrorLogEnabled = errorLogs
			env.setUpstream(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":{"message":"bad prompt","type":"invalid_request_error"}}`)
			})
			enqueued := logcontent.Stats().Enqueued

			requestID, body, status := env.call(t, "/v1/chat/completions", chatBody(false))
			require.Equal(t, http.StatusBadRequest, status, body)
			if !errorLogs {
				assertNoLogContents(t, requestID, enqueued)
				return
			}
			rows := waitLogContents(t, requestID)
			require.Equal(t, []int{0, 1}, attemptNumbers(rows))
			client := rowMeta(t, rows[0])
			assert.Equal(t, http.StatusBadRequest, client.HttpStatus)
			assert.NotEmpty(t, client.ErrorMessage)
			assert.Equal(t, body, gunzipText(t, rows[0].ResponseBody), "the error JSON written by Relay's defer is stored")
			assert.Contains(t, body, "bad prompt")
		})
	}
}

func TestCaptureGatedByConsumeLogOff(t *testing.T) {
	env := newCaptureEnv(t)
	common.LogConsumeEnabled = false
	env.setUpstream(echoCompletion(t, nil))
	enqueued := logcontent.Stats().Enqueued

	requestID, body, status := env.call(t, "/v1/chat/completions", chatBody(false))
	require.Equal(t, http.StatusOK, status, body)
	assertNoLogContents(t, requestID, enqueued)
}

func TestCaptureSkipsRefusedBeforeAttempt(t *testing.T) {
	env := newCaptureEnv(t)
	constant.ErrorLogEnabled = true
	require.NoError(t, env.db.Model(&model.User{}).Where("id = ?", env.userID).Update("quota", 0).Error)
	var calls atomic.Int32
	env.setUpstream(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		echoCompletion(t, nil)(w, r)
	})
	enqueued := logcontent.Stats().Enqueued

	requestID, body, status := env.call(t, "/v1/chat/completions", chatBody(false))
	require.NotEqual(t, http.StatusOK, status, body)
	require.Zero(t, calls.Load())
	assertNoLogContents(t, requestID, enqueued)
}

func TestCaptureIneligibleWritesMarker(t *testing.T) {
	env := newCaptureEnv(t)
	env.setUpstream(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"m1","model":"`+captureModel+`","results":[{"flagged":false}]}`)
	})

	requestID, body, status := env.call(t, "/v1/moderations", `{"model":"`+captureModel+`","input":"hello"}`)
	require.Equal(t, http.StatusOK, status, body)

	rows := waitLogContents(t, requestID)
	require.Len(t, rows, 1)
	assert.Equal(t, 0, rows[0].Attempt)
	assert.Equal(t, model.LogContentStatusNotCaptured, rows[0].CaptureStatus)
	assert.Empty(t, rows[0].RequestBody)
	assert.Empty(t, rows[0].ResponseBody)
	assert.Empty(t, rows[0].AssembledBody)
}

// blockingStream sends the first two frames and waits for release, then
// sends the rest; it closes early without a terminal frame when cut is set.
// The relay holds a frame back, so two are sent for the first to arrive.
func blockingStream(release <-chan struct{}, cut bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		frames := streamFrames()
		frames[1] = strings.Replace(frames[1], `,"finish_reason":"stop"`, "", 1)
		for _, frame := range frames[:2] {
			_, _ = io.WriteString(w, frame)
			w.(http.Flusher).Flush()
		}
		if cut {
			return
		}
		select {
		case <-release:
		case <-time.After(5 * time.Second):
		}
		for _, frame := range frames[2:] {
			_, _ = io.WriteString(w, frame)
			w.(http.Flusher).Flush()
		}
	}
}

func consumeLogRow(t *testing.T, requestID string) model.Log {
	t.Helper()
	var row model.Log
	require.Eventually(t, func() bool {
		return model.LOG_DB.Where("request_id = ? AND type = ?", requestID, model.LogTypeConsume).First(&row).Error == nil
	}, 10*time.Second, 20*time.Millisecond)
	return row
}

func TestCaptureClientDisconnectMarksIncomplete(t *testing.T) {
	disconnect := func(t *testing.T, env *captureEnv) string {
		release := make(chan struct{})
		env.setUpstream(blockingStream(release, false))
		ctx, cancel := context.WithCancel(context.Background())
		resp := env.post(t, ctx, "/v1/chat/completions", chatBody(true))
		requestID := resp.Header.Get(common.RequestIdKey)
		line, err := bufio.NewReader(resp.Body).ReadString('\n')
		require.NoError(t, err)
		require.Contains(t, line, "Hello")
		cancel()
		_ = resp.Body.Close()
		time.Sleep(50 * time.Millisecond)
		close(release)
		return requestID
	}

	// Each run has its own subtest, so its gateway has stopped before the next.
	var baseline, logged model.Log
	t.Run("capture off", func(t *testing.T) {
		env := newCaptureEnv(t)
		common.LogContentCaptureEnabled = false
		baseline = consumeLogRow(t, disconnect(t, env))
	})
	t.Run("capture on", func(t *testing.T) {
		env := newCaptureEnv(t)
		requestID := disconnect(t, env)
		rows := waitLogContents(t, requestID)
		require.Equal(t, 0, rows[0].Attempt)
		assert.True(t, rowMeta(t, rows[0]).Incomplete)
		assert.Contains(t, gunzipText(t, rows[0].ResponseBody), "Hello")
		logged = consumeLogRow(t, requestID)
	})
	assert.Equal(t, baseline.Type, logged.Type)
	assert.Equal(t, baseline.ModelName, logged.ModelName)
	assert.Equal(t, baseline.IsStream, logged.IsStream)
	assert.Equal(t, baseline.PromptTokens, logged.PromptTokens)
	assert.Equal(t, baseline.CompletionTokens, logged.CompletionTokens)
	assert.Equal(t, baseline.Quota, logged.Quota)
	assert.Equal(t, baseline.ChannelId, logged.ChannelId)
	assert.Contains(t, baseline.Other, `"end_reason":"client_gone"`)
	assert.Contains(t, logged.Other, `"end_reason":"client_gone"`)
}

func TestCaptureUpstreamCutMarksIncomplete(t *testing.T) {
	env := newCaptureEnv(t)
	env.setUpstream(blockingStream(nil, true))

	requestID, body, status := env.call(t, "/v1/chat/completions", chatBody(true))
	require.Equal(t, http.StatusOK, status, body)
	rows := waitLogContents(t, requestID)
	require.Equal(t, 0, rows[0].Attempt)
	assert.True(t, rowMeta(t, rows[0]).Incomplete)
	assert.Contains(t, gunzipText(t, rows[0].AssembledBody), "Hello")
}

var requestIDPattern = regexp.MustCompile(`[0-9A-Za-z]{16,}`)

func TestUpstreamGzipBodyDecodedOffPath(t *testing.T) {
	answer := `{"id":"g1","object":"chat.completion","model":"` + captureModel + `","choices":[{"index":0,"message":{"role":"assistant","content":"zipped"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`
	run := func(t *testing.T, capture bool) string {
		env := newCaptureEnv(t)
		common.LogContentCaptureEnabled = capture
		constant.ErrorLogEnabled = true
		override := `{"Accept-Encoding":"gzip"}`
		require.NoError(t, env.db.Model(env.channel).Update("header_override", override).Error)
		env.setUpstream(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "gzip", r.Header.Get("Accept-Encoding"))
			var compressed bytes.Buffer
			writer := gzip.NewWriter(&compressed)
			_, _ = writer.Write([]byte(answer))
			_ = writer.Close()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Encoding", "gzip")
			_, _ = w.Write(compressed.Bytes())
		})
		requestID, body, _ := env.call(t, "/v1/chat/completions", `{"model":"`+captureModel+`","messages":[{"role":"user","content":"zip"}]}`)
		if capture {
			rows := waitLogContents(t, requestID)
			var upstream *model.LogContent
			for _, row := range rows {
				if row.Attempt == 1 {
					upstream = row
				}
			}
			require.NotNil(t, upstream, "attempts: %v", attemptNumbers(rows))
			assert.Equal(t, answer, gunzipText(t, upstream.ResponseBody), "the stored provider body is the decoded text")
			assert.False(t, rowMeta(t, upstream).Response.Undecodable)
		}
		return requestIDPattern.ReplaceAllString(body, "ID")
	}

	var withoutCapture, withCapture string
	t.Run("capture off", func(t *testing.T) { withoutCapture = run(t, false) })
	t.Run("capture on", func(t *testing.T) { withCapture = run(t, true) })
	assert.Equal(t, withoutCapture, withCapture, "the client gets the same bytes")
}

func TestUpstreamRequestWithoutGetBodyIsMarked(t *testing.T) {
	env := newCaptureEnv(t)
	var received atomic.Value
	env.setUpstream(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received.Store(string(body))
		r.Body = io.NopCloser(bytes.NewReader(body))
		echoCompletion(t, nil)(w, r)
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	c.Set(common.RequestIdKey, "no-get-body-"+t.Name())
	capture := logcontent.Begin(c, types.RelayFormatOpenAI)
	require.NotNil(t, capture)

	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeChatCompletions,
		RelayFormat:    types.RelayFormatOpenAI,
		RequestURLPath: "/v1/chat/completions",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeOpenAI,
			ChannelId:      env.channel.Id,
			ChannelBaseUrl: env.upstream.URL,
			ApiType:        constant.APITypeOpenAI,
			ApiKey:         captureChannelKey,
		},
	}
	adaptor := relay.GetAdaptor(info.ApiType)
	require.NotNil(t, adaptor)
	adaptor.Init(info)
	payload := `{"model":"` + captureModel + `","messages":[{"role":"user","content":"plain reader"}]}`
	resp, err := channel.DoApiRequest(adaptor, c, info, struct{ io.Reader }{strings.NewReader(payload)})
	require.NoError(t, err)
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	assert.Equal(t, payload, received.Load(), "the provider gets the whole body")

	capture.OnLogRow(&model.Log{UserId: env.userID, Type: model.LogTypeConsume})
	capture.Finish()
	rows := waitLogContents(t, "no-get-body-"+t.Name())
	require.Equal(t, []int{0, 1}, attemptNumbers(rows))
	meta := rowMeta(t, rows[1])
	assert.True(t, meta.RequestUnavailable)
	assert.Empty(t, rows[1].RequestBody)
	assert.Contains(t, gunzipText(t, rows[1].ResponseBody), "plain reader")
}

func TestBillingUnchangedWithCapture(t *testing.T) {
	run := func(t *testing.T, capture bool) (model.Log, int) {
		env := newCaptureEnv(t)
		common.LogContentCaptureEnabled = capture
		env.setUpstream(echoCompletion(t, nil))
		requestID, body, status := env.call(t, "/v1/chat/completions", chatBody(false))
		require.Equal(t, http.StatusOK, status, body)
		row := consumeLogRow(t, requestID)
		var user model.User
		require.NoError(t, env.db.First(&user, env.userID).Error)
		return row, user.Quota
	}
	var off, on model.Log
	var offQuota, onQuota int
	t.Run("capture off", func(t *testing.T) { off, offQuota = run(t, false) })
	t.Run("capture on", func(t *testing.T) { on, onQuota = run(t, true) })
	require.NotZero(t, off.CacheTokens, "the fixture bills cached tokens")
	assert.Equal(t, off.Quota, on.Quota)
	assert.Equal(t, off.PromptTokens, on.PromptTokens)
	assert.Equal(t, off.CompletionTokens, on.CompletionTokens)
	assert.Equal(t, off.CacheTokens, on.CacheTokens)
	assert.Equal(t, offQuota, onQuota)
}

func TestCaptureRedactsHeaderOverrideSecret(t *testing.T) {
	const overrideSecret = "override-header-secret-0123456789abcdef"
	env := newCaptureEnv(t)
	override := `{"X-Custom-Auth":"` + overrideSecret + `"}`
	require.NoError(t, env.db.Model(env.channel).Update("header_override", override).Error)
	env.setUpstream(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, overrideSecret, r.Header.Get("X-Custom-Auth"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"h1","object":"chat.completion","model":"`+captureModel+`","choices":[{"index":0,"message":{"role":"assistant","content":"saw `+overrideSecret+`"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`)
	})

	requestID, body, status := env.call(t, "/v1/chat/completions", chatBody(false))
	require.Equal(t, http.StatusOK, status, body)
	require.Contains(t, body, overrideSecret, "the client still gets the answer unchanged")
	rows := waitLogContents(t, requestID)
	require.Equal(t, []int{0, 1}, attemptNumbers(rows))
	for _, row := range rows {
		assert.NotContains(t, gunzipText(t, row.ResponseBody), overrideSecret, "attempt %d", row.Attempt)
		assert.NotContains(t, row.Meta, overrideSecret)
	}
	assert.Contains(t, gunzipText(t, rows[1].ResponseBody), "saw [REDACTED]")
}
