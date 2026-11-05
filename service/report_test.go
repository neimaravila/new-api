package service

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// recordingLogger captures the SQL GORM actually issues. Report queries run against SQLite in
// tests, and SQLite coerces comparisons that PostgreSQL rejects outright, so asserting on the
// emitted statement is the only way to guard cross-database predicate typing here.
type recordingLogger struct {
	logger.Interface
	statements *[]string
}

func (r recordingLogger) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	*r.statements = append(*r.statements, sql)
}

// recordLogSQL routes model.LOG_DB through a recording logger for the duration of the test.
func recordLogSQL(t *testing.T) *[]string {
	t.Helper()
	statements := make([]string, 0, 8)
	previous := model.LOG_DB
	model.LOG_DB = previous.Session(&gorm.Session{
		NewDB:  true,
		Logger: recordingLogger{Interface: logger.Default, statements: &statements},
	})
	t.Cleanup(func() { model.LOG_DB = previous })
	return &statements
}

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
		{UserId: 1, Username: "alice", CreatedAt: twoHoursAgo, Type: model.LogTypeConsume, ModelName: "gpt-4o", ChannelId: 10, Quota: 100, PromptTokens: 50, CompletionTokens: 30, CacheTokens: 20, UseTime: 1, IsStream: true},
		{UserId: 1, Username: "alice", CreatedAt: twoHoursAgo, Type: model.LogTypeConsume, ModelName: "gpt-4o", ChannelId: 10, Quota: 200, PromptTokens: 60, CompletionTokens: 40, CacheTokens: 0, UseTime: 2, IsStream: true},
		// alice: one error on gpt-4o
		{UserId: 1, Username: "alice", CreatedAt: twoHoursAgo, Type: model.LogTypeError, ModelName: "gpt-4o", ChannelId: 10, Quota: 0},
		// alice: claude consume, non-stream, slower
		{UserId: 1, Username: "alice", CreatedAt: threeHoursAgo, Type: model.LogTypeConsume, ModelName: "claude", ChannelId: 11, Quota: 500, PromptTokens: 100, CompletionTokens: 200, CacheTokens: 50, UseTime: 3, IsStream: false},
		// bob: gpt-4o consume (must NOT appear for alice), plus errors
		{UserId: 2, Username: "bob", CreatedAt: twoHoursAgo, Type: model.LogTypeConsume, ModelName: "gpt-4o", ChannelId: 10, Quota: 9999, PromptTokens: 1, CompletionTokens: 1, CacheTokens: 1, UseTime: 1, IsStream: false},
		{UserId: 2, Username: "bob", CreatedAt: twoHoursAgo, Type: model.LogTypeError, ModelName: "claude", ChannelId: 11, Quota: 0},
		// older than 24h bucket to verify day granularity grouping
		{UserId: 1, Username: "alice", CreatedAt: dayAgo, Type: model.LogTypeConsume, ModelName: "gpt-4o", ChannelId: 10, Quota: 50, PromptTokens: 10, CompletionTokens: 5, CacheTokens: 0, UseTime: 1, IsStream: true},
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

	summary, err := GetReportSummary(99, "admin", common.RoleAdminUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour, dto.ReportFilter{})
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

// channel_id is an integer column: PostgreSQL fails the whole admin report with SQLSTATE 22P02
// when the channel performance aggregation filters it with `channel_id <> ''`.
func TestReportChannelPerformanceFiltersChannelIDNumerically(t *testing.T) {
	setupDashboardServiceTestDB(t)
	start, end := seedReportFixture(t)
	statements := recordLogSQL(t)

	rows, err := computeReportPerformance(reportScope(99, "admin", common.RoleAdminUser, dto.ReportFilter{}), start, end, reportPerfByChannel)
	require.NoError(t, err)
	require.NotEmpty(t, rows)

	aggregate := ""
	for _, sql := range *statements {
		if strings.Contains(sql, "GROUP BY") && strings.Contains(sql, "channel_id") {
			aggregate = sql
			break
		}
	}
	require.NotEmpty(t, aggregate, "channel aggregation was not recorded: %v", *statements)
	assert.NotContains(t, aggregate, "channel_id <> ''")
	assert.Contains(t, aggregate, "channel_id <> 0")
}

// The channel performance aggregation groups by channel_id, so its rows arrive labelled with the
// raw id and must be resolved to the same names the channel cost/health table shows.
func TestGetReportSummaryNamesChannelPerformanceRows(t *testing.T) {
	setupDashboardServiceTestDB(t)
	seedReportFixture(t)
	// The fixture logs reference channels 10 and 11; only 10 still exists.
	require.NoError(t, model.DB.Create(&model.Channel{Id: 10, Name: "primary-openai"}).Error)

	summary, err := GetReportSummary(99, "admin", common.RoleAdminUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour, dto.ReportFilter{})
	require.NoError(t, err)

	perfNames := make([]string, 0, len(summary.ChannelPerformance))
	for _, row := range summary.ChannelPerformance {
		perfNames = append(perfNames, row.Name)
	}
	assert.ElementsMatch(t, []string{"primary-openai", "#11"}, perfNames)

	channelNames := make([]string, 0, len(summary.Channels))
	for _, row := range summary.Channels {
		channelNames = append(channelNames, row.ChannelName)
	}
	assert.ElementsMatch(t, []string{"primary-openai", "#11"}, channelNames)
}

func TestGetReportSummaryRegularUserScoped(t *testing.T) {
	setupDashboardServiceTestDB(t)
	seedReportFixture(t)

	summary, err := GetReportSummary(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour, dto.ReportFilter{})
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

	summary, err := GetReportSummary(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour, dto.ReportFilter{})
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
	summary, err := GetReportSummary(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dto.ReportGranularityDay, dto.ReportFilter{})
	require.NoError(t, err)
	assert.Empty(t, summary.Models)
	assert.Empty(t, summary.Errors)
	assert.Empty(t, summary.Trend.Points)
	assert.Equal(t, 0, summary.Trend.Totals.Requests)
	assert.Empty(t, summary.CostByModel.Models)
	assert.Empty(t, summary.CostByModel.Points)
	assert.Empty(t, summary.LatencySeries)
}

func seedReportTypeMixFixture(t *testing.T) (int64, int64) {
	t.Helper()
	now := time.Now().Unix()
	at := now - 2*3600
	logs := []model.Log{
		{UserId: 1, Username: "alice", CreatedAt: at, Type: model.LogTypeConsume, ModelName: "gpt-4o", ChannelId: 10, Quota: 100, PromptTokens: 50, CompletionTokens: 30, UseTime: 2},
		{UserId: 1, Username: "alice", CreatedAt: at, Type: model.LogTypeError, ModelName: "gpt-4o", ChannelId: 10},
		{UserId: 1, Username: "alice", CreatedAt: at, Type: model.LogTypeTopup, ModelName: "gpt-4o", ChannelId: 10, Quota: 5000},
		{UserId: 1, Username: "alice", CreatedAt: at, Type: model.LogTypeRefund, ModelName: "gpt-4o", ChannelId: 10, Quota: 700},
		{UserId: 1, Username: "alice", CreatedAt: at, Type: model.LogTypeManage, ModelName: "gpt-4o", ChannelId: 10},
	}
	for i := range logs {
		require.NoError(t, model.LOG_DB.Create(&logs[i]).Error)
	}
	return now - 24*3600, now
}

func TestReportTotalsCountOnlyConsumeAndError(t *testing.T) {
	setupDashboardServiceTestDB(t)
	start, end := seedReportTypeMixFixture(t)
	admin := reportScope(99, "admin", common.RoleAdminUser, dto.ReportFilter{})

	trend, err := computeReportTrend(admin, start, end, dto.ReportGranularityHour)
	require.NoError(t, err)
	assert.Equal(t, 2, trend.Totals.Requests)
	assert.Equal(t, 1, trend.Totals.Failures)
	assert.Equal(t, 1, trend.Totals.Consumers)
	assert.Equal(t, 100, trend.Totals.Quota)
	assert.Equal(t, 80, trend.Totals.Tokens)
	assert.Equal(t, 2000, trend.Totals.AvgLatencyMs)

	models, err := computeReportModels(admin, start, end)
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, 2, models[0].Requests)
	assert.Equal(t, 100, models[0].Quota)
	assert.Equal(t, 2000, models[0].AvgLatencyMs)

	channels, err := computeReportChannels(admin, start, end)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.Equal(t, 2, channels[0].Requests)
	assert.Equal(t, 100, channels[0].Quota)
	assert.Equal(t, 2000, channels[0].AvgLatencyMs)

	perf, err := computeReportPerformance(admin, start, end, reportPerfByModel)
	require.NoError(t, err)
	require.Len(t, perf, 1)
	assert.Equal(t, 2, perf[0].Requests)

	stream, err := computeReportStreamComparison(admin, start, end)
	require.NoError(t, err)
	assert.Equal(t, 1, stream.NonStreamRequests)
}

func TestReportPerformanceUsesMilliseconds(t *testing.T) {
	setupDashboardServiceTestDB(t)
	at := time.Now().Unix() - 3600
	for _, useTime := range []int{2, 4} {
		require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: at, Type: model.LogTypeConsume, ModelName: "gpt-4o", ChannelId: 10, Quota: 10, PromptTokens: 10, CompletionTokens: 20, UseTime: useTime}).Error)
	}

	rows, err := computeReportPerformance(reportScope(99, "admin", common.RoleAdminUser, dto.ReportFilter{}), at-3600, at+3600, reportPerfByModel)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 3000, rows[0].AvgLatencyMs)
	assert.Equal(t, 4000, rows[0].P50LatencyMs)
	assert.Equal(t, 4000, rows[0].P95LatencyMs)
	assert.Equal(t, 10, rows[0].Throughput)
}

func TestReportSummaryFiltersByModel(t *testing.T) {
	setupDashboardServiceTestDB(t)
	seedReportFixture(t)

	summary, err := GetReportSummary(99, "admin", common.RoleAdminUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour, dto.ReportFilter{ModelName: "gpt-4o"})
	require.NoError(t, err)

	require.NotEmpty(t, summary.Models)
	for _, row := range summary.Models {
		assert.Equal(t, "gpt-4o", row.ModelName)
	}
	// gpt-4o consume: alice 100+200+50, bob 9999; requests: 4 consume + 1 error.
	assert.Equal(t, 10349, summary.Trend.Totals.Quota)
	assert.Equal(t, 5, summary.Trend.Totals.Requests)
}

func TestReportSummaryChannelFilterIgnoredForRegularUser(t *testing.T) {
	setupDashboardServiceTestDB(t)
	seedReportFixture(t)

	base, err := GetReportSummary(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour, dto.ReportFilter{})
	require.NoError(t, err)
	filtered, err := GetReportSummary(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour, dto.ReportFilter{ChannelID: 11})
	require.NoError(t, err)

	assert.Equal(t, base.Trend.Totals, filtered.Trend.Totals)
	assert.Equal(t, base.Models, filtered.Models)
}

func TestReportSummaryChannelFilterAppliesForAdmin(t *testing.T) {
	setupDashboardServiceTestDB(t)
	seedReportFixture(t)

	summary, err := GetReportSummary(99, "admin", common.RoleAdminUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour, dto.ReportFilter{ChannelID: 11})
	require.NoError(t, err)
	for _, row := range summary.Models {
		assert.Equal(t, "claude", row.ModelName)
	}
	for _, row := range summary.Channels {
		assert.Equal(t, 11, row.ChannelID)
	}
}

func reportAdminScope() reportRoleScope {
	return reportScope(99, "admin", common.RoleAdminUser, dto.ReportFilter{})
}

func TestReportCostByModelGroupsTopFiveAndOthers(t *testing.T) {
	setupDashboardServiceTestDB(t)
	at := time.Now().Unix() - 3600
	for i := 1; i <= 7; i++ {
		require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: at, Type: model.LogTypeConsume, ModelName: "model-" + strconv.Itoa(i), ChannelId: 10, Quota: i * 100, UseTime: 1}).Error)
	}

	summary, err := GetReportSummary(99, "admin", common.RoleAdminUser, dto.ReportRangeToday, dto.ReportGranularityDay, dto.ReportFilter{})
	require.NoError(t, err)
	cost := summary.CostByModel
	assert.Equal(t, []string{"model-7", "model-6", "model-5", "model-4", "model-3", dto.ReportCostOthersKey}, cost.Models)
	sum := 0
	for _, p := range cost.Points {
		for _, v := range p.Values {
			sum += v
		}
	}
	assert.Equal(t, summary.Trend.Totals.Quota, sum)
	assert.Equal(t, 300, cost.Points[0].Values[dto.ReportCostOthersKey])
}

func TestReportCostByModelSkipsEmptyModelName(t *testing.T) {
	setupDashboardServiceTestDB(t)
	at := time.Now().Unix() - 3600
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: at, Type: model.LogTypeConsume, ModelName: "", Quota: 50, UseTime: 1}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: at, Type: model.LogTypeConsume, ModelName: "gpt-4o", Quota: 70, UseTime: 1}).Error)

	cost, err := computeReportCostByModel(reportAdminScope(), at-3600, at+3600, dto.ReportGranularityHour)
	require.NoError(t, err)
	assert.Equal(t, []string{"gpt-4o"}, cost.Models)
	for _, p := range cost.Points {
		_, has := p.Values[""]
		assert.False(t, has)
	}
}

func TestReportLatencySeriesPercentiles(t *testing.T) {
	setupDashboardServiceTestDB(t)
	at := time.Now().Unix() - 3600
	for _, useTime := range []int{1, 2, 3, 4, 100} {
		require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: at, Type: model.LogTypeConsume, ModelName: "gpt-4o", Quota: 1, UseTime: useTime}).Error)
	}

	series, err := computeReportLatencySeries(reportAdminScope(), at-3600, at+3600, dto.ReportGranularityDay)
	require.NoError(t, err)
	require.Len(t, series, 1)
	assert.Equal(t, 3000, series[0].P50Ms)
	assert.Equal(t, 100000, series[0].P95Ms)
}

func TestReportLatencySeriesOmitsEmptyBuckets(t *testing.T) {
	setupDashboardServiceTestDB(t)
	at := time.Now().Unix() - 3600
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: at, Type: model.LogTypeError, ModelName: "gpt-4o", UseTime: 5}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: at - 2*3600, Type: model.LogTypeConsume, ModelName: "gpt-4o", Quota: 1, UseTime: 2}).Error)

	series, err := computeReportLatencySeries(reportAdminScope(), at-6*3600, at+3600, dto.ReportGranularityHour)
	require.NoError(t, err)
	require.Len(t, series, 1)
	assert.Equal(t, 2000, series[0].P50Ms)
}

func TestReportGroupPercentilesSamplesMostRecent(t *testing.T) {
	setupDashboardServiceTestDB(t)
	now := time.Now().Unix()
	// 6 old fast rows, then 4 recent slow rows; with limit 4 only the slow ones are sampled.
	for i := 0; i < 6; i++ {
		require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: now - 7200 + int64(i), Type: model.LogTypeConsume, ModelName: "gpt-4o", Quota: 1, UseTime: 1}).Error)
	}
	for i := 0; i < 4; i++ {
		require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: now - 600 + int64(i), Type: model.LogTypeConsume, ModelName: "gpt-4o", Quota: 1, UseTime: 50}).Error)
	}

	p50, p95, err := reportGroupPercentiles(reportAdminScope(), now-8000, now+10, "model_name", "gpt-4o", 4)
	require.NoError(t, err)
	assert.Equal(t, 50000, p50)
	assert.Equal(t, 50000, p95)
}

func TestReportLatencySeriesDropsOldestBucketWhenSampleIsFull(t *testing.T) {
	setupDashboardServiceTestDB(t)
	base := time.Now().Unix()
	base -= base % 3600
	// Hour buckets: base-3h (old), base-2h, base-1h; 2 rows each. Limit 5 cuts the oldest bucket short.
	for h := int64(3); h >= 1; h-- {
		for k := int64(0); k < 2; k++ {
			require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: base - h*3600 + 10 + k, Type: model.LogTypeConsume, ModelName: "gpt-4o", Quota: 1, UseTime: int(h)}).Error)
		}
	}

	full, err := computeReportLatencySeriesWithLimit(reportAdminScope(), base-4*3600, base, dto.ReportGranularityHour, 5)
	require.NoError(t, err)
	require.Len(t, full, 2)
	assert.Equal(t, base-2*3600, full[0].BucketTimestamp)
	assert.Equal(t, 2000, full[0].P50Ms)
	assert.Equal(t, base-1*3600, full[1].BucketTimestamp)
	assert.Equal(t, 1000, full[1].P50Ms)

	// A limit above the row count keeps every bucket.
	all, err := computeReportLatencySeriesWithLimit(reportAdminScope(), base-4*3600, base, dto.ReportGranularityHour, 100)
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestReportCostByModelMultiBucketSumsToTotal(t *testing.T) {
	setupDashboardServiceTestDB(t)
	now := time.Now().Unix()
	for i := 0; i < 7; i++ {
		for _, ago := range []int64{2 * 3600, 5 * 3600} {
			require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: now - ago, Type: model.LogTypeConsume, ModelName: "m-" + strconv.Itoa(i), Quota: (i + 1) * 10, UseTime: 1}).Error)
		}
	}

	summary, err := GetReportSummary(99, "admin", common.RoleAdminUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour, dto.ReportFilter{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(summary.CostByModel.Points), 2)
	sum := 0
	for _, p := range summary.CostByModel.Points {
		for _, v := range p.Values {
			sum += v
		}
	}
	assert.Equal(t, summary.Trend.Totals.Quota, sum)
}

func TestReportRankingOrdersByRequestsAndLimits(t *testing.T) {
	setupDashboardServiceTestDB(t)
	now := time.Now().Unix()
	for i := 1; i <= 60; i++ {
		for j := 0; j < i; j++ {
			require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: now - 600, Type: model.LogTypeConsume, TokenName: "tok-" + strconv.Itoa(i), ModelName: "gpt-4o", Quota: 1, UseTime: 1}).Error)
		}
	}

	rows, err := GetReportRanking(99, "admin", common.RoleAdminUser, dto.ReportRangeLast7Days, dto.ReportDimensionToken, dto.ReportFilter{})
	require.NoError(t, err)
	require.Len(t, rows, reportRankingLimit)
	assert.Equal(t, "tok-60", rows[0].Name)
	assert.Equal(t, 60, rows[0].Requests)
	for i := 1; i < len(rows); i++ {
		assert.GreaterOrEqual(t, rows[i-1].Requests, rows[i].Requests)
	}
}

func TestReportRankingRejectsAdminDimensionsForUser(t *testing.T) {
	setupDashboardServiceTestDB(t)
	seedReportFixture(t)
	for _, dim := range []dto.ReportDimension{dto.ReportDimensionUser, dto.ReportDimensionChannel} {
		rows, err := GetReportRanking(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dim, dto.ReportFilter{})
		require.Error(t, err, string(dim))
		assert.Empty(t, rows)
	}
}

func TestReportRankingScopesTokensToUser(t *testing.T) {
	setupDashboardServiceTestDB(t)
	now := time.Now().Unix()
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, Username: "alice", CreatedAt: now - 600, Type: model.LogTypeConsume, TokenName: "alice-key", ModelName: "m", Quota: 1, UseTime: 1}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 2, Username: "bob", CreatedAt: now - 600, Type: model.LogTypeConsume, TokenName: "bob-key", ModelName: "m", Quota: 1, UseTime: 1}).Error)

	rows, err := GetReportRanking(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dto.ReportDimensionToken, dto.ReportFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "alice-key", rows[0].Name)
}

func TestReportRankingRejectsUnknownDimension(t *testing.T) {
	setupDashboardServiceTestDB(t)
	statements := recordLogSQL(t)
	_, err := GetReportRanking(99, "admin", common.RoleAdminUser, dto.ReportRangeLast7Days, dto.ReportDimension("foo"), dto.ReportFilter{})
	require.Error(t, err)
	assert.Empty(t, *statements)
}

func TestReportRankingNamesChannels(t *testing.T) {
	setupDashboardServiceTestDB(t)
	seedReportFixture(t)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 10, Name: "primary-openai"}).Error)

	rows, err := GetReportRanking(99, "admin", common.RoleAdminUser, dto.ReportRangeLast7Days, dto.ReportDimensionChannel, dto.ReportFilter{})
	require.NoError(t, err)
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}
	assert.ElementsMatch(t, []string{"primary-openai", "#11"}, names)
	// Percentiles of a channel are computed with a numeric channel_id filter.
	assert.Greater(t, rows[0].P95LatencyMs, 0)
}

func TestReportRankingMetrics(t *testing.T) {
	setupDashboardServiceTestDB(t)
	seedReportFixture(t)
	rows, err := GetReportRanking(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dto.ReportDimensionModel, dto.ReportFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "gpt-4o", rows[0].Name)
	assert.Equal(t, 4, rows[0].Requests)
	assert.Equal(t, 1, rows[0].Failures)
	assert.InDelta(t, 0.25, rows[0].ErrorRate, 1e-9)
	assert.Equal(t, 350, rows[0].Quota)
	assert.Equal(t, 1333, rows[0].AvgLatencyMs)
}
