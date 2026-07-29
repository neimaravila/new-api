package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedReportFixture inserts a representative mix of logs for users 1 (alice) and 2 (bob).
// Returns the window that covers every seeded row.
func seedReportFixture(t *testing.T) (start int64, end int64) {
	t.Helper()
	now := time.Now().Unix()
	twoHoursAgo := now - 2*3600
	threeHoursAgo := now - 3*3600
	dayAgo := now - 26*3600

	logs := []model.Log{
		// alice: gpt-4o consume, stream, fast
		{UserId: 1, Username: "alice", CreatedAt: twoHoursAgo, Type: model.LogTypeConsume, ModelName: "gpt-4o", ChannelId: 10, Quota: 100, PromptTokens: 50, CompletionTokens: 30, CacheTokens: 20, UseTime: 500, IsStream: true},
		{UserId: 1, Username: "alice", CreatedAt: twoHoursAgo, Type: model.LogTypeConsume, ModelName: "gpt-4o", ChannelId: 10, Quota: 200, PromptTokens: 60, CompletionTokens: 40, CacheTokens: 0, UseTime: 700, IsStream: true},
		// alice: one error on gpt-4o
		{UserId: 1, Username: "alice", CreatedAt: twoHoursAgo, Type: model.LogTypeError, ModelName: "gpt-4o", ChannelId: 10, Quota: 0},
		// alice: claude consume, non-stream, slower
		{UserId: 1, Username: "alice", CreatedAt: threeHoursAgo, Type: model.LogTypeConsume, ModelName: "claude", ChannelId: 11, Quota: 500, PromptTokens: 100, CompletionTokens: 200, CacheTokens: 50, UseTime: 3000, IsStream: false},
		// bob: gpt-4o consume (must NOT appear for alice), plus errors
		{UserId: 2, Username: "bob", CreatedAt: twoHoursAgo, Type: model.LogTypeConsume, ModelName: "gpt-4o", ChannelId: 10, Quota: 9999, PromptTokens: 1, CompletionTokens: 1, CacheTokens: 1, UseTime: 100, IsStream: false},
		{UserId: 2, Username: "bob", CreatedAt: twoHoursAgo, Type: model.LogTypeError, ModelName: "claude", ChannelId: 11, Quota: 0},
		// older than 24h bucket to verify day granularity grouping
		{UserId: 1, Username: "alice", CreatedAt: dayAgo, Type: model.LogTypeConsume, ModelName: "gpt-4o", ChannelId: 10, Quota: 50, PromptTokens: 10, CompletionTokens: 5, CacheTokens: 0, UseTime: 400, IsStream: true},
	}
	for i := range logs {
		require.NoError(t, model.LOG_DB.Create(&logs[i]).Error)
	}
	return dayAgo - 3600, now
}

func TestResolveReportWindowAndGranularity(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	start, end := ResolveReportWindow(dto.ReportRangeToday, now)
	assert.Equal(t, time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC).Unix(), start)
	assert.Equal(t, now.Unix(), end)

	// <=36h window -> hourly
	assert.Equal(t, dto.ReportGranularityHour, DefaultReportGranularity(now.Add(-12*time.Hour).Unix(), now.Unix()))
	// >36h window -> daily
	assert.Equal(t, dto.ReportGranularityDay, DefaultReportGranularity(now.Add(-72*time.Hour).Unix(), now.Unix()))
}

func TestGetReportSummaryAdminScopesGlobal(t *testing.T) {
	setupDashboardServiceTestDB(t)
	start, end := seedReportFixture(t)

	summary, err := GetReportSummary(99, "admin", common.RoleAdminUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour)
	require.NoError(t, err)

	assert.Equal(t, dto.DashboardRoleAdmin, summary.Role)
	// Admin sees both alice and bob models, and channels.
	assert.NotEmpty(t, summary.Models)
	assert.NotEmpty(t, summary.Channels)
	assert.NotEmpty(t, summary.ChannelPerformance)

	// gpt-4o quota across alice + bob = 100+200+50+9999 = 10349
	var gptRow *dto.ReportModelRow
	for i := range summary.Models {
		if summary.Models[i].ModelName == "gpt-4o" {
			gptRow = &summary.Models[i]
		}
	}
	require.NotNil(t, gptRow)
	assert.Equal(t, 10349, gptRow.Quota)
	// admin gpt-4o: 4 consumes + 1 error = 5 rows, 1 failure => 0.2 error rate
	assert.InDelta(t, 0.2, gptRow.ErrorRate, 0.01)

	// trend covers the fixture window
	assert.NotEmpty(t, summary.Trend.Points)
	assert.Greater(t, summary.Trend.Totals.Requests, 0)
	// bucket timestamps fall inside the fixture span
	for _, p := range summary.Trend.Points {
		assert.GreaterOrEqual(t, p.BucketTimestamp, start)
		assert.LessOrEqual(t, p.BucketTimestamp, end+3600)
	}
}

func TestGetReportSummaryRegularUserScoped(t *testing.T) {
	setupDashboardServiceTestDB(t)
	seedReportFixture(t)

	summary, err := GetReportSummary(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour)
	require.NoError(t, err)

	assert.Equal(t, dto.DashboardRoleUser, summary.Role)
	// User never sees admin-only channel breakdown / channel performance.
	assert.Empty(t, summary.Channels)
	assert.Empty(t, summary.ChannelPerformance)

	// gpt-4o quota is alice-only: 100+200+50 = 350 (bob's 9999 excluded)
	var gptRow *dto.ReportModelRow
	for i := range summary.Models {
		if summary.Models[i].ModelName == "gpt-4o" {
			gptRow = &summary.Models[i]
		}
	}
	require.NotNil(t, gptRow)
	assert.Equal(t, 350, gptRow.Quota)

	// claude tokens for alice: 100+200+50 = 350
	var claudeRow *dto.ReportTokenAnatomyRow
	for i := range summary.TokenAnatomy {
		if summary.TokenAnatomy[i].ModelName == "claude" {
			claudeRow = &summary.TokenAnatomy[i]
		}
	}
	require.NotNil(t, claudeRow)
	assert.Equal(t, 100, claudeRow.PromptTokens)
	assert.Equal(t, 200, claudeRow.CompletionTokens)
	assert.Equal(t, 50, claudeRow.CacheTokens)
	assert.Equal(t, 350, claudeRow.Total)
}

func TestGetReportSummaryErrorsAndPerformance(t *testing.T) {
	setupDashboardServiceTestDB(t)
	seedReportFixture(t)

	summary, err := GetReportSummary(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour)
	require.NoError(t, err)

	// alice has 1 error on gpt-4o
	require.NotEmpty(t, summary.Errors)
	assert.Equal(t, "gpt-4o", summary.Errors[0].ModelName)
	assert.Equal(t, 1, summary.Errors[0].Failures)
	assert.Equal(t, 100, summary.Errors[0].Share) // 100% of alice's failures

	// performance: avg latency computed, p95 from samples
	require.NotEmpty(t, summary.ModelPerformance)
	var gptPerf *dto.ReportPerformanceRow
	for i := range summary.ModelPerformance {
		if summary.ModelPerformance[i].Name == "gpt-4o" {
			gptPerf = &summary.ModelPerformance[i]
		}
	}
	require.NotNil(t, gptPerf)
	assert.Greater(t, gptPerf.AvgLatencyMs, 0)
	assert.Greater(t, gptPerf.P95LatencyMs, 0)

	// stream comparison: alice has stream (gpt-4o) and non-stream (claude)
	require.NotNil(t, summary.StreamPerformance)
	assert.Greater(t, summary.StreamPerformance.StreamRequests, 0)
	assert.Greater(t, summary.StreamPerformance.NonStreamRequests, 0)
}

func TestGetReportSummaryEmptyFixtureSafe(t *testing.T) {
	setupDashboardServiceTestDB(t)
	// no logs at all
	summary, err := GetReportSummary(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dto.ReportGranularityDay)
	require.NoError(t, err)
	assert.Empty(t, summary.Models)
	assert.Empty(t, summary.Errors)
	assert.Empty(t, summary.Trend.Points)
	assert.Equal(t, 0, summary.Trend.Totals.Requests)
}
