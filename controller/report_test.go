package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reportResponse struct {
	Success bool             `json:"success"`
	Message string           `json:"message"`
	Data    reportSummaryDTO `json:"data"`
}

type reportSummaryDTO struct {
	Role                string                  `json:"role"`
	Range               string                  `json:"range"`
	Granularity         string                  `json:"granularity"`
	PeriodStart         int64                   `json:"period_start"`
	PeriodEnd           int64                   `json:"period_end"`
	Models              []map[string]any        `json:"models"`
	Channels            []map[string]any        `json:"channels"`
	ChannelPerformance  []map[string]any        `json:"channel_performance"`
	ModelPerformance    []map[string]any        `json:"model_performance"`
	TokenAnatomy        []map[string]any        `json:"token_anatomy"`
	Errors              []map[string]any        `json:"errors"`
	StreamPerformance   *map[string]any         `json:"stream_performance"`
}

func setupReportControllerTestDB(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	model.DB = db
	model.LOG_DB = db
	common.LogConsumeEnabled = true
	now := time.Now().Unix()
	logs := []model.Log{
		{UserId: 1, Username: "alice", CreatedAt: now - 3600, Type: model.LogTypeConsume, ModelName: "gpt-4o", ChannelId: 10, Quota: 100, PromptTokens: 50, CompletionTokens: 30, UseTime: 500, IsStream: true},
		{UserId: 1, Username: "alice", CreatedAt: now - 3600, Type: model.LogTypeError, ModelName: "gpt-4o", ChannelId: 10},
		{UserId: 2, Username: "bob", CreatedAt: now - 3600, Type: model.LogTypeConsume, ModelName: "claude", ChannelId: 11, Quota: 9999, UseTime: 200, IsStream: false},
	}
	for i := range logs {
		require.NoError(t, model.LOG_DB.Create(&logs[i]).Error)
	}
}

func newReportRequest(role int, userID int, username string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", userID)
	ctx.Set("username", username)
	ctx.Set("role", role)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/dashboard/report?range=7d&granularity=hour", nil)
	return ctx, recorder
}

func TestGetReportSummaryHandlerAdminGlobal(t *testing.T) {
	setupReportControllerTestDB(t)
	ctx, recorder := newReportRequest(common.RoleAdminUser, 99, "admin")
	GetReportSummary(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload reportResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)

	assert.Equal(t, "admin", payload.Data.Role)
	// Admin sees both models (gpt-4o + claude) and the channels section.
	assert.GreaterOrEqual(t, len(payload.Data.Models), 2)
	assert.NotEmpty(t, payload.Data.Channels)
	assert.NotEmpty(t, payload.Data.ChannelPerformance)
}

func TestGetReportSummaryHandlerRegularUserScoped(t *testing.T) {
	setupReportControllerTestDB(t)
	ctx, recorder := newReportRequest(common.RoleCommonUser, 1, "alice")
	GetReportSummary(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload reportResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)

	assert.Equal(t, "user", payload.Data.Role)
	// Regular user sees only their own models and never admin-only channel breakdowns.
	assert.Len(t, payload.Data.Models, 1)
	assert.Equal(t, "gpt-4o", payload.Data.Models[0]["model_name"])
	assert.Empty(t, payload.Data.Channels)
	assert.Empty(t, payload.Data.ChannelPerformance)
}

func TestGetReportExportHandlerReturnsCSV(t *testing.T) {
	setupReportControllerTestDB(t)
	ctx, recorder := newReportRequest(common.RoleAdminUser, 99, "admin")
	GetReportExport(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, recorder.Header().Get("Content-Disposition"), "attachment")
	body := recorder.Body.String()
	assert.Contains(t, body, "new-api report")
	assert.Contains(t, body, "models,gpt-4o,")
}
