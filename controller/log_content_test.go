package controller

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	logContentProviderSentinel = "provider-side-sentinel-7f3a"
	logContentChannelKey       = "channel-secret-key-0123456789abcdef"
)

// logContentEnv holds an admin, a root, two regular users, each with a legacy
// token, and a channel that serves the captured calls.
type logContentEnv struct {
	db     *gorm.DB
	router *gin.Engine
	admin  *model.User
	root   *model.User
	alice  *model.User
	bob    *model.User
	tokens map[int]string
}

func newLogContentEnv(t *testing.T) *logContentEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	admin, adminToken := setupAccessTokenAudit(t)
	db := model.DB
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.LogContent{}))
	previousCapture, previousRetention := common.LogContentCaptureEnabled, common.LogContentRetentionDays
	common.LogContentCaptureEnabled, common.LogContentRetentionDays = true, 30
	t.Cleanup(func() {
		common.LogContentCaptureEnabled, common.LogContentRetentionDays = previousCapture, previousRetention
	})

	env := &logContentEnv{db: db, admin: admin, tokens: map[int]string{admin.Id: adminToken}}
	newUser := func(name string, role int) *model.User {
		token := name + "-legacy-token"
		user := &model.User{Username: name, Password: "placeholder", Role: role, Status: common.UserStatusEnabled, Group: "default", AccessToken: &token, AuthVersion: 1, AffCode: name}
		require.NoError(t, db.Create(user).Error)
		env.tokens[user.Id] = token
		return user
	}
	env.root = newUser("content-root", common.RoleRootUser)
	env.alice = newUser("content-alice", common.RoleCommonUser)
	env.bob = newUser("content-bob", common.RoleCommonUser)
	require.NoError(t, db.Create(&model.Channel{Id: 7, Name: "primary-channel", Key: logContentChannelKey}).Error)

	router := gin.New()
	router.Use(middleware.RequestId(), middleware.AccessTokenAudit())
	logRoute := router.Group("/api/log")
	logRoute.GET("/content/:request_id", middleware.AdminAuth(), GetLogContent)
	logRoute.GET("/self/content/:request_id", middleware.UserAuth(), GetSelfLogContent)
	logRoute.GET("/content_capture/stats", middleware.AdminAuth(), GetLogContentCaptureStats)
	env.router = router
	return env
}

func (env *logContentEnv) get(path string, user *model.User) *httptest.ResponseRecorder {
	return env.getWithToken(path, env.tokens[user.Id])
}

func (env *logContentEnv) getWithToken(path, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.RemoteAddr = "192.0.2.8:4567"
	response := httptest.NewRecorder()
	env.router.ServeHTTP(response, request)
	return response
}

func gzipText(t *testing.T, text string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := writer.Write([]byte(text))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func (env *logContentEnv) addLogRow(t *testing.T, requestID string, userID int, createdAt int64, requestPath string) {
	t.Helper()
	other := ""
	if requestPath != "" {
		other = `{"request_path":"` + requestPath + `"}`
	}
	row := model.Log{UserId: userID, CreatedAt: createdAt, Type: model.LogTypeConsume, RequestId: requestID, Other: other}
	require.NoError(t, env.db.Create(&row).Error)
}

// addContent stores a captured chat call: the client exchange carries the
// user text, and one provider attempt carries the provider sentinel.
func (env *logContentEnv) addContent(t *testing.T, requestID string, userID int, userText string) {
	t.Helper()
	clientMeta, err := model.NewLogContentMeta(model.LogContentMeta{RelayFormat: "openai", HttpStatus: 200, MultiKeyIndex: -1})
	require.NoError(t, err)
	attemptMeta, err := model.NewLogContentMeta(model.LogContentMeta{
		RelayFormat: "openai", HttpStatus: 200, ChannelId: 7, MultiKeyIndex: 2, Method: "POST", Path: "/v1/chat/completions",
	})
	require.NoError(t, err)
	now := time.Now().Unix()
	rows := []*model.LogContent{
		{
			RequestId: requestID, Attempt: 0, UserId: userID, CreatedAt: now, CaptureStatus: model.LogContentStatusCaptured, Meta: clientMeta,
			RequestBody:  gzipText(t, `{"model":"m","messages":[{"role":"user","content":"`+userText+`"}]}`),
			ResponseBody: gzipText(t, `{"choices":[{"message":{"role":"assistant","content":"answer text"}}]}`),
		},
		{
			RequestId: requestID, Attempt: 1, UserId: userID, CreatedAt: now, CaptureStatus: model.LogContentStatusCaptured, Meta: attemptMeta,
			RequestBody:  gzipText(t, `{"model":"upstream","note":"`+logContentProviderSentinel+`"}`),
			ResponseBody: gzipText(t, `{"id":"`+logContentProviderSentinel+`"}`),
		},
	}
	require.NoError(t, model.InsertLogContents(context.Background(), rows))
}

func (env *logContentEnv) addCall(t *testing.T, requestID string, userID int, userText string) {
	t.Helper()
	env.addLogRow(t, requestID, userID, time.Now().Unix()-60, "/v1/chat/completions")
	env.addContent(t, requestID, userID, userText)
}

func decodeContentResponse(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, common.UnmarshalJsonStr(response.Body.String(), &body))
	return body
}

func contentData(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	body := decodeContentResponse(t, response)
	require.Equal(t, true, body["success"], response.Body.String())
	return body["data"].(map[string]any)
}

func TestSelfRouteNotFoundParity(t *testing.T) {
	env := newLogContentEnv(t)
	env.addCall(t, "bob-call", env.bob.Id, "bob private text")

	reference := env.get("/api/log/self/content/missing-call", env.alice)
	assert.Contains(t, reference.Body.String(), "Log content not found")
	assert.Equal(t, false, decodeContentResponse(t, reference)["success"])
	for _, requestID := range []string{"bob-call", strings.Repeat("a", 65)} {
		response := env.get("/api/log/self/content/"+requestID, env.alice)
		assert.Equal(t, reference.Code, response.Code, requestID)
		assert.Equal(t, reference.Body.String(), response.Body.String(), requestID)
	}
	// The admin route answers not found the same way.
	admin := env.get("/api/log/content/"+strings.Repeat("a", 65), env.admin)
	assert.Equal(t, reference.Body.String(), admin.Body.String())
}

func TestSelfRouteFiltersContentByUser(t *testing.T) {
	env := newLogContentEnv(t)
	// The same request_id has a log row for both users, and content only for bob.
	env.addLogRow(t, "shared-call", env.alice.Id, time.Now().Unix()-60, "/v1/chat/completions")
	env.addCall(t, "shared-call", env.bob.Id, "bob private text")

	response := env.get("/api/log/self/content/shared-call", env.alice)
	data := contentData(t, response)
	assert.Equal(t, false, data["available"])
	assert.NotContains(t, response.Body.String(), "bob private text")

	bob := env.get("/api/log/self/content/shared-call", env.bob)
	assert.Contains(t, bob.Body.String(), "bob private text")

	// Content stored under another user is not found when the caller has no row.
	env.addCall(t, "bob-only", env.bob.Id, "bob only text")
	reference := env.get("/api/log/self/content/missing-call", env.alice)
	other := env.get("/api/log/self/content/bob-only", env.alice)
	assert.Equal(t, reference.Code, other.Code)
	assert.Equal(t, reference.Body.String(), other.Body.String())
}

func TestSelfRouteOmitsProviderData(t *testing.T) {
	env := newLogContentEnv(t)
	env.addCall(t, "root-call", env.root.Id, "root own text")

	response := env.get("/api/log/self/content/root-call", env.root)
	data := contentData(t, response)
	assert.Equal(t, true, data["available"])
	assert.NotContains(t, data, "attempts")
	text := response.Body.String()
	for _, forbidden := range []string{`"attempts"`, `"channel`, logContentProviderSentinel, logContentChannelKey, "primary-channel"} {
		assert.NotContains(t, text, forbidden)
	}
}

func TestSelfRouteReturnsOwnContent(t *testing.T) {
	env := newLogContentEnv(t)
	env.addCall(t, "alice-call", env.alice.Id, "alice own text")

	data := contentData(t, env.get("/api/log/self/content/alice-call", env.alice))
	assert.Equal(t, "alice-call", data["request_id"])
	assert.Equal(t, true, data["available"])
	conversation := data["conversation"].(map[string]any)
	assert.Equal(t, true, conversation["available"])
	assert.NotEmpty(t, conversation["messages"])
	client := data["client"].(map[string]any)
	assert.Contains(t, client["request"].(map[string]any)["text"], "alice own text")
	assert.Contains(t, client["response"].(map[string]any)["text"], "answer text")
}

func TestAdminRouteReadsAnyUserIncludingRoot(t *testing.T) {
	env := newLogContentEnv(t)
	env.addCall(t, "root-call", env.root.Id, "root own text")

	response := env.get("/api/log/content/root-call", env.admin)
	data := contentData(t, response)
	assert.Equal(t, true, data["available"])
	attempts := data["attempts"].([]any)
	require.Len(t, attempts, 1)
	attempt := attempts[0].(map[string]any)
	assert.EqualValues(t, 7, attempt["channel_id"])
	assert.Equal(t, "primary-channel", attempt["channel_name"])
	assert.EqualValues(t, 2, attempt["multi_key_index"])
	assert.Contains(t, attempt["response"].(map[string]any)["text"], logContentProviderSentinel)
	assert.NotContains(t, response.Body.String(), logContentChannelKey)
}

func TestAdminRouteRejectsCommonUser(t *testing.T) {
	env := newLogContentEnv(t)
	env.addCall(t, "alice-call", env.alice.Id, "alice own text")
	// Any AdminAuth route gives a regular user the same denial.
	env.router.GET("/api/log/stat", middleware.AdminAuth(), func(c *gin.Context) { common.ApiSuccess(c, nil) })
	reference := env.get("/api/log/stat", env.alice)
	require.NotEqual(t, http.StatusOK, reference.Code)

	response := env.get("/api/log/content/alice-call", env.alice)
	assert.Equal(t, reference.Code, response.Code)
	assert.Equal(t, reference.Body.String(), response.Body.String())
	assert.NotContains(t, response.Body.String(), "alice own text")
}

func logContentAuditRows(t *testing.T, env *logContentEnv) []model.AuditLog {
	t.Helper()
	var rows []model.AuditLog
	require.NoError(t, env.db.Where("action = ?", "log_content.view").Order("id").Find(&rows).Error)
	return rows
}

func TestAdminReadWritesAuditLog(t *testing.T) {
	env := newLogContentEnv(t)
	env.addCall(t, "root-call", env.root.Id, "root conversation text")

	require.Equal(t, http.StatusOK, env.get("/api/log/content/root-call", env.admin).Code)
	rows := logContentAuditRows(t, env)
	require.Len(t, rows, 1)
	audit := rows[0]
	assert.Equal(t, env.admin.Id, audit.UserId)
	assert.Equal(t, common.RoleAdminUser, audit.ActorRole)
	assert.Equal(t, model.AuditCategoryOperation, audit.Category)
	require.NotNil(t, audit.Other.Op)
	params, err := common.Marshal(audit.Other.Op.Params)
	require.NoError(t, err)
	assert.JSONEq(t, fmt.Sprintf(`{"request_id":"root-call","owner_user_id":%d}`, env.root.Id), string(params))
	require.NotNil(t, audit.Other.AdminInfo)
	assert.Equal(t, env.admin.Id, audit.Other.AdminInfo.AdminID)
	stored, err := common.Marshal(audit)
	require.NoError(t, err)
	assert.NotContains(t, string(stored), "root conversation text")
	assert.NotContains(t, string(stored), "answer text")

	// Reading one's own call through the admin route is audited too.
	env.addCall(t, "admin-call", env.admin.Id, "admin text")
	require.Equal(t, http.StatusOK, env.get("/api/log/content/admin-call", env.admin).Code)
	assert.Len(t, logContentAuditRows(t, env), 2)

	// Not found and the self route write no audit row.
	env.get("/api/log/content/missing-call", env.admin)
	env.get("/api/log/self/content/admin-call", env.admin)
	env.get("/api/log/self/content/root-call", env.root)
	assert.Len(t, logContentAuditRows(t, env), 2)
}

func TestAdminAuditOnlyForAvailableContent(t *testing.T) {
	env := newLogContentEnv(t)
	// A fresh log row with no content yet is pending; an older one is unavailable.
	env.addLogRow(t, "pending-call", env.alice.Id, time.Now().Unix(), "/v1/chat/completions")
	env.addLogRow(t, "lost-call", env.alice.Id, time.Now().Unix()-60, "/v1/chat/completions")
	env.addLogRow(t, "marker-call", env.alice.Id, time.Now().Unix()-60, "/v1/chat/completions")
	marker := &model.LogContent{RequestId: "marker-call", UserId: env.alice.Id, CreatedAt: time.Now().Unix(), CaptureStatus: model.LogContentStatusNotCaptured, Meta: "{}"}
	require.NoError(t, model.InsertLogContents(context.Background(), []*model.LogContent{marker}))

	for requestID, reason := range map[string]string{"pending-call": "pending", "lost-call": "unavailable", "marker-call": "not_captured"} {
		data := contentData(t, env.get("/api/log/content/"+requestID, env.admin))
		assert.Equal(t, false, data["available"], requestID)
		assert.Equal(t, reason, data["reason"], requestID)
	}
	assert.Empty(t, logContentAuditRows(t, env), "reads without content are not audited")

	env.addCall(t, "alice-call", env.alice.Id, "alice text")
	require.Equal(t, http.StatusOK, env.get("/api/log/content/alice-call", env.admin).Code)
	rows := logContentAuditRows(t, env)
	require.Len(t, rows, 1)
	assert.Equal(t, "Viewed the content of request alice-call", rows[0].Content)
	assert.Equal(t, auditContentEN("log_content.view", map[string]any{"request_id": "alice-call"}), rows[0].Content)
	assert.Contains(t, auditContentTemplates, "log_content.view")
}

func TestSelfRouteReadsOnlyTheClientLevel(t *testing.T) {
	env := newLogContentEnv(t)
	env.addLogRow(t, "attempts-only", env.alice.Id, time.Now().Unix()-60, "/v1/chat/completions")
	meta, err := model.NewLogContentMeta(model.LogContentMeta{RelayFormat: "openai", HttpStatus: 200, ChannelId: 7, MultiKeyIndex: -1})
	require.NoError(t, err)
	row := &model.LogContent{
		RequestId: "attempts-only", Attempt: 1, UserId: env.alice.Id, CreatedAt: time.Now().Unix(), CaptureStatus: model.LogContentStatusCaptured, Meta: meta,
		RequestBody: gzipText(t, `{"note":"`+logContentProviderSentinel+`"}`), ResponseBody: gzipText(t, `{"id":"`+logContentProviderSentinel+`"}`),
	}
	require.NoError(t, model.InsertLogContents(context.Background(), []*model.LogContent{row}))

	var queries []string
	const name = "test:record_log_contents"
	require.NoError(t, model.LOG_DB.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "log_contents" {
			queries = append(queries, tx.Statement.SQL.String())
		}
	}))
	t.Cleanup(func() { _ = model.LOG_DB.Callback().Query().Remove(name) })

	response := env.get("/api/log/self/content/attempts-only", env.alice)
	require.Len(t, queries, 1)
	assert.Contains(t, queries[0], "attempt = ", "the self route never reads provider attempts")
	data := contentData(t, response)
	assert.Equal(t, false, data["available"])
	assert.Equal(t, "unavailable", data["reason"], "provider attempts alone are no content for the caller")
	assert.NotContains(t, response.Body.String(), logContentProviderSentinel)
}

func TestAdminRouteReadsContentOfTheLogOwner(t *testing.T) {
	env := newLogContentEnv(t)
	// alice's log row comes first; only bob has content under the same id.
	env.addLogRow(t, "shared-call", env.alice.Id, time.Now().Unix()-120, "/v1/chat/completions")
	env.addCall(t, "shared-call", env.bob.Id, "bob private text")

	response := env.get("/api/log/content/shared-call", env.admin)
	data := contentData(t, response)
	assert.Equal(t, false, data["available"])
	assert.NotContains(t, response.Body.String(), "bob private text")
	assert.NotContains(t, response.Body.String(), logContentProviderSentinel)

}

func TestRegularUserLogReadTokenCannotReachAdminRoute(t *testing.T) {
	env := newLogContentEnv(t)
	env.addCall(t, "alice-call", env.alice.Id, "alice own text")
	token, _ := createScopedAccessToken(t, env.alice.Id, 0, "log:read")

	response := env.getWithToken("/api/log/content/alice-call", token)
	assert.NotEqual(t, http.StatusOK, response.Code)
	assert.NotContains(t, response.Body.String(), "alice own text")
	assert.NotContains(t, response.Body.String(), logContentProviderSentinel)
	assert.Empty(t, logContentAuditRows(t, env))
}

func TestContentNeedsLogRow(t *testing.T) {
	env := newLogContentEnv(t)
	env.addContent(t, "orphan-call", env.alice.Id, "orphan text")

	reference := env.get("/api/log/self/content/missing-call", env.alice)
	for _, response := range []*httptest.ResponseRecorder{
		env.get("/api/log/self/content/orphan-call", env.alice),
		env.get("/api/log/content/orphan-call", env.admin),
	} {
		assert.Equal(t, reference.Body.String(), response.Body.String())
		assert.NotContains(t, response.Body.String(), "orphan text")
	}
	assert.Empty(t, logContentAuditRows(t, env))
}

func TestLogRowReasons(t *testing.T) {
	const now = int64(1_800_000_000)
	retention := int64(30 * 24 * 3600)
	previousCapture, previousRetention := common.LogContentCaptureEnabled, common.LogContentRetentionDays
	t.Cleanup(func() {
		common.LogContentCaptureEnabled, common.LogContentRetentionDays = previousCapture, previousRetention
	})
	common.LogContentRetentionDays = 30
	for _, test := range []struct {
		name     string
		age      int64
		path     string
		capture  bool
		expected string
	}{
		{name: "recent row", age: 5, path: "/v1/chat/completions", capture: true, expected: "pending"},
		{name: "row past the wait", age: 16, path: "/v1/chat/completions", capture: true, expected: "unavailable"},
		{name: "older than retention", age: retention + 1, path: "/v1/chat/completions", capture: true, expected: "expired"},
		{name: "exactly at retention", age: retention, path: "/v1/chat/completions", capture: true, expected: "unavailable"},
		{name: "image call", age: 60, path: "/v1/images/generations", capture: true, expected: "not_captured"},
		{name: "recent image call", age: 5, path: "/v1/images/generations", capture: true, expected: "not_captured"},
		{name: "claude messages", age: 60, path: "/v1/messages", capture: true, expected: "unavailable"},
		{name: "no request path", age: 60, capture: true, expected: "unavailable"},
		{name: "capture off", age: 5, path: "/v1/chat/completions", capture: false, expected: "unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			common.LogContentCaptureEnabled = test.capture
			other := ""
			if test.path != "" {
				other = `{"request_path":"` + test.path + `"}`
			}
			logs := []*model.Log{{CreatedAt: now - test.age, Type: model.LogTypeConsume, Other: other}}
			assert.Equal(t, test.expected, logContentReason(logs, now))
		})
	}

	// Stored markers decide the reason through the route.
	env := newLogContentEnv(t)
	for _, marker := range []struct{ status, expected string }{
		{model.LogContentStatusNotCaptured, "not_captured"},
		{model.LogContentStatusNoAttempt, "unavailable"},
	} {
		requestID := "marker-" + marker.status
		env.addLogRow(t, requestID, env.alice.Id, time.Now().Unix()-1, "/v1/chat/completions")
		row := &model.LogContent{RequestId: requestID, UserId: env.alice.Id, CreatedAt: time.Now().Unix(), CaptureStatus: marker.status, Meta: "{}"}
		require.NoError(t, model.InsertLogContents(context.Background(), []*model.LogContent{row}))
		data := contentData(t, env.get("/api/log/self/content/"+requestID, env.alice))
		assert.Equal(t, false, data["available"], marker.status)
		assert.Equal(t, marker.expected, data["reason"], marker.status)
	}

	// A log row without content gets its reason through the route.
	env.addLogRow(t, "fresh-call", env.alice.Id, time.Now().Unix(), "/v1/chat/completions")
	data := contentData(t, env.get("/api/log/self/content/fresh-call", env.alice))
	assert.Equal(t, "fresh-call", data["request_id"])
	assert.Equal(t, false, data["available"])
	assert.Equal(t, "pending", data["reason"])
}

func TestAccessTokenRouteRules(t *testing.T) {
	for key, scope := range map[string]string{
		"GET /api/log/content/:request_id":      "log:read",
		"GET /api/log/self/content/:request_id": "usage:read",
		"GET /api/log/content_capture/stats":    "log:read",
	} {
		rule, ok := middleware.AccessTokenRouteRule(key)
		require.True(t, ok, key)
		assert.Equal(t, "scope", rule.Kind(), key)
		assert.Equal(t, scope, rule.Scope(), key)
	}

	env := newLogContentEnv(t)
	env.addCall(t, "admin-call", env.admin.Id, "admin text")
	usage, _ := createScopedAccessToken(t, env.admin.Id, 0, "usage:read")
	logRead, _ := createScopedAccessToken(t, env.admin.Id, 0, "log:read")

	assert.Equal(t, true, decodeContentResponse(t, env.getWithToken("/api/log/self/content/admin-call", usage))["success"])
	for _, path := range []string{"/api/log/content/admin-call", "/api/log/content_capture/stats"} {
		denied := env.getWithToken(path, usage)
		assert.Equal(t, http.StatusForbidden, denied.Code, path)
		assert.Contains(t, denied.Body.String(), `"code":"ACCESS_TOKEN_SCOPE_DENIED"`, path)
		assert.NotContains(t, denied.Body.String(), "admin text", path)
	}
	assert.Equal(t, true, decodeContentResponse(t, env.getWithToken("/api/log/content/admin-call", logRead))["success"])
	assert.Equal(t, http.StatusForbidden, env.getWithToken("/api/log/self/content/admin-call", logRead).Code)
}

func TestGetLogContentCaptureStatsAdminOnly(t *testing.T) {
	env := newLogContentEnv(t)
	data := contentData(t, env.get("/api/log/content_capture/stats", env.admin))
	for _, key := range []string{"enqueued", "written", "dropped", "write_errors", "budget_in_use"} {
		assert.Contains(t, data, key)
	}
	denied := env.get("/api/log/content_capture/stats", env.alice)
	assert.NotEqual(t, http.StatusOK, denied.Code)
	assert.NotContains(t, denied.Body.String(), "budget_in_use")
}
