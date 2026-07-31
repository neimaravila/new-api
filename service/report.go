package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

// ReportLimits caps how many breakdown rows are returned per category.
const (
	reportModelLimit            = 50
	reportErrorLimit            = 20
	reportPerformanceLimit      = 20
	reportTokenAnatomyLimit     = 30
	reportChannelLimit          = 30
	reportLatencySampleLimit    = 5000
	reportThroughputMinUseTimeMs = 1
)

// ResolveReportWindow converts a selectable range into a [start,end) unix window.
func ResolveReportWindow(r dto.ReportTimeRange, now time.Time) (int64, int64) {
	switch r {
	case dto.ReportRangeToday:
		y, m, d := now.Date()
		start := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
		return start.Unix(), now.Unix()
	case dto.ReportRangeYesterday:
		y, m, d := now.Date()
		start := time.Date(y, m, d, 0, 0, 0, 0, now.Location()).AddDate(0, 0, -1)
		return start.Unix(), start.Add(24 * time.Hour).Unix()
	case dto.ReportRangeLast7Days:
		return now.AddDate(0, 0, -7).Unix(), now.Unix()
	case dto.ReportRangeThisMonth:
		y, m, _ := now.Date()
		start := time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
		return start.Unix(), now.Unix()
	case dto.ReportRangeLast30Days:
		return now.AddDate(0, 0, -30).Unix(), now.Unix()
	default:
		return now.AddDate(0, 0, -7).Unix(), now.Unix()
	}
}

// DefaultReportGranularity picks a sensible bucket size for a window.
func DefaultReportGranularity(start int64, end int64) dto.ReportGranularity {
	if end-start <= int64(36*60*60) {
		return dto.ReportGranularityHour
	}
	return dto.ReportGranularityDay
}

type reportRoleScope struct {
	isAdmin  bool
	userID   int
	username string
}

func reportScope(userID int, username string, role int) reportRoleScope {
	return reportRoleScope{
		isAdmin:  role >= common.RoleAdminUser,
		userID:   userID,
		username: username,
	}
}

func (s reportRoleScope) apply(tx *gorm.DB) *gorm.DB {
	if s.isAdmin {
		return tx
	}
	if s.userID > 0 {
		return tx.Where("user_id = ?", s.userID)
	}
	return tx.Where("username = ?", s.username)
}

// GetReportSummary computes the full role-scoped report payload.
func GetReportSummary(userID int, username string, role int, r dto.ReportTimeRange, granularity dto.ReportGranularity) (dto.ReportSummary, error) {
	now := time.Now()
	start, end := ResolveReportWindow(r, now)
	if granularity == "" {
		granularity = DefaultReportGranularity(start, end)
	}
	scope := reportScope(userID, username, role)

	trend, err := computeReportTrend(scope, start, end, granularity)
	if err != nil {
		return dto.ReportSummary{}, err
	}
	models, err := computeReportModels(scope, start, end)
	if err != nil {
		return dto.ReportSummary{}, err
	}
	errors, err := computeReportErrors(scope, start, end)
	if err != nil {
		return dto.ReportSummary{}, err
	}
	modelPerf, err := computeReportPerformance(scope, start, end, reportPerfByModel)
	if err != nil {
		return dto.ReportSummary{}, err
	}
	tokenAnatomy, err := computeReportTokenAnatomy(scope, start, end)
	if err != nil {
		return dto.ReportSummary{}, err
	}
	streamPerf, err := computeReportStreamComparison(scope, start, end)
	if err != nil {
		return dto.ReportSummary{}, err
	}

	summary := dto.ReportSummary{
		Role:              dto.DashboardRoleUser,
		Range:             r,
		Granularity:       granularity,
		PeriodStart:       start,
		PeriodEnd:         end,
		Trend:             trend,
		Models:            models,
		Errors:            errors,
		ModelPerformance:  modelPerf,
		TokenAnatomy:      tokenAnatomy,
		StreamPerformance: streamPerf,
	}
	if scope.isAdmin {
		summary.Role = dto.DashboardRoleAdmin
		channels, err := computeReportChannels(scope, start, end)
		if err != nil {
			return dto.ReportSummary{}, err
		}
		summary.Channels = channels
		channelPerf, err := computeReportPerformance(scope, start, end, reportPerfByChannel)
		if err != nil {
			return dto.ReportSummary{}, err
		}
		summary.ChannelPerformance = channelPerf
	}
	return summary, nil
}

// ---- temporal trend ----

type reportBucketRow struct {
	Bucket     int64
	Quota      int
	Requests   int
	Tokens     int
	Failures   int
	LatencySum int
	Consumers  int
}

func computeReportTrend(scope reportRoleScope, start int64, end int64, granularity dto.ReportGranularity) (dto.ReportTrend, error) {
	intervalSeconds := int64(3600)
	if granularity == dto.ReportGranularityDay {
		intervalSeconds = 86400
	}
	// Align start to the bucket boundary so labels line up across DBs.
	bucketStart := start - (start % intervalSeconds)

	rows, err := queryReportTrendBuckets(scope, bucketStart, end, intervalSeconds)
	if err != nil {
		return dto.ReportTrend{}, err
	}

	points := make([]dto.ReportTrendPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, dto.ReportTrendPoint{
			BucketLabel:       formatReportBucketLabel(row.Bucket, granularity),
			BucketTimestamp:   row.Bucket,
			Quota:             row.Quota,
			Requests:          row.Requests,
			Tokens:            row.Tokens,
			Failures:          row.Failures,
			AvgLatencyMs:      safeDivInt(row.LatencySum, row.Consumers),
			ConsumingRequests: row.Consumers,
		})
	}

	totals := dto.ReportTotals{
		Quota:        sumReportIntField(rows, func(r reportBucketRow) int { return r.Quota }),
		Requests:     sumReportIntField(rows, func(r reportBucketRow) int { return r.Requests }),
		Tokens:       sumReportIntField(rows, func(r reportBucketRow) int { return r.Tokens }),
		Failures:     sumReportIntField(rows, func(r reportBucketRow) int { return r.Failures }),
		Consumers:    sumReportIntField(rows, func(r reportBucketRow) int { return r.Consumers }),
		AvgLatencyMs: safeDivInt(sumReportIntField(rows, func(r reportBucketRow) int { return r.LatencySum }), sumReportIntField(rows, func(r reportBucketRow) int { return r.Consumers })),
	}

	return dto.ReportTrend{
		Granularity: granularity,
		Points:      points,
		Totals:      totals,
	}, nil
}

func queryReportTrendBuckets(scope reportRoleScope, start int64, end int64, intervalSeconds int64) ([]reportBucketRow, error) {
	// Compute the bucket number portably in Go from created_at, then aggregate in the DB.
	// GORM Scan into a struct works across SQLite/MySQL/PostgreSQL; for ClickHouse the
	// same column arithmetic applies (integer division on Int64).
	bucketExpr := fmt.Sprintf("(created_at - %d) / %d", start, intervalSeconds)
	tx := model.LOG_DB.Table("logs").
		Select(bucketExpr+" as bucket, "+
			"COALESCE(sum(quota), 0) as quota, "+
			"count(*) as requests, "+
			"COALESCE(sum(prompt_tokens), 0) + COALESCE(sum(completion_tokens), 0) + COALESCE(sum(cache_tokens), 0) as tokens, "+
			"sum(case when type = ? then 1 else 0 end) as failures, "+
			"COALESCE(sum(use_time), 0) as latency_sum, "+
			"sum(case when type = ? then 1 else 0 end) as consumers",
			model.LogTypeError, model.LogTypeConsume).
		Where("created_at >= ? and created_at <= ?", start, end).
		Group("bucket").
		Order("bucket asc")
	tx = scope.apply(tx)

	var rawRows []struct {
		Bucket     int64 `gorm:"column:bucket"`
		Quota      int   `gorm:"column:quota"`
		Requests   int   `gorm:"column:requests"`
		Tokens     int   `gorm:"column:tokens"`
		Failures   int   `gorm:"column:failures"`
		LatencySum int   `gorm:"column:latency_sum"`
		Consumers  int   `gorm:"column:consumers"`
	}
	if err := tx.Scan(&rawRows).Error; err != nil {
		return nil, err
	}

	rows := make([]reportBucketRow, 0, len(rawRows))
	for _, r := range rawRows {
		rows = append(rows, reportBucketRow{
			Bucket:     start + r.Bucket*intervalSeconds,
			Quota:      r.Quota,
			Requests:   r.Requests,
			Tokens:     r.Tokens,
			Failures:   r.Failures,
			LatencySum: r.LatencySum,
			Consumers:  r.Consumers,
		})
	}
	return rows, nil
}

func formatReportBucketLabel(bucketUnix int64, granularity dto.ReportGranularity) string {
	t := time.Unix(bucketUnix, 0).UTC()
	if granularity == dto.ReportGranularityHour {
		return t.Format("01-02 15:04")
	}
	return t.Format("2006-01-02")
}

// ---- model breakdown ----

func computeReportModels(scope reportRoleScope, start int64, end int64) ([]dto.ReportModelRow, error) {
	tx := model.LOG_DB.Table("logs").
		Select("model_name, "+
			"COALESCE(sum(quota), 0) as quota, "+
			"count(*) as requests, "+
			"COALESCE(sum(prompt_tokens), 0) + COALESCE(sum(completion_tokens), 0) + COALESCE(sum(cache_tokens), 0) as tokens, "+
			"sum(case when type = ? then 1 else 0 end) as failures, "+
			"sum(case when type = ? then 1 else 0 end) as consumers, "+
			"COALESCE(sum(use_time), 0) as latency_sum",
			model.LogTypeError, model.LogTypeConsume).
		Where("created_at >= ? and created_at <= ?", start, end).
		Where("model_name <> ''").
		Group("model_name").
		Order("quota desc").
		Limit(reportModelLimit)
	tx = scope.apply(tx)

	var rawRows []struct {
		ModelName  string `gorm:"column:model_name"`
		Quota      int    `gorm:"column:quota"`
		Requests   int    `gorm:"column:requests"`
		Tokens     int    `gorm:"column:tokens"`
		Failures   int    `gorm:"column:failures"`
		Consumers  int    `gorm:"column:consumers"`
		LatencySum int    `gorm:"column:latency_sum"`
	}
	if err := tx.Scan(&rawRows).Error; err != nil {
		return nil, err
	}

	rows := make([]dto.ReportModelRow, 0, len(rawRows))
	for _, r := range rawRows {
		rows = append(rows, dto.ReportModelRow{
			ModelName:    r.ModelName,
			Quota:        r.Quota,
			Requests:     r.Requests,
			Tokens:       r.Tokens,
			Failures:     r.Failures,
			ErrorRate:    safeRate(r.Failures, r.Requests),
			AvgLatencyMs: safeDivInt(r.LatencySum, r.Consumers),
		})
	}
	return rows, nil
}

// ---- errors ----

func computeReportErrors(scope reportRoleScope, start int64, end int64) ([]dto.ReportErrorRow, error) {
	tx := model.LOG_DB.Table("logs").
		Select("model_name, count(*) as failures, COALESCE(sum(quota), 0) as quota").
		Where("created_at >= ? and created_at <= ? and type = ?", start, end, model.LogTypeError).
		Where("model_name <> ''").
		Group("model_name").
		Order("failures desc").
		Limit(reportErrorLimit)
	tx = scope.apply(tx)

	var rawRows []struct {
		ModelName string `gorm:"column:model_name"`
		Failures  int    `gorm:"column:failures"`
		Quota     int    `gorm:"column:quota"`
	}
	if err := tx.Scan(&rawRows).Error; err != nil {
		return nil, err
	}

	totalFailures := 0
	for _, r := range rawRows {
		totalFailures += r.Failures
	}
	rows := make([]dto.ReportErrorRow, 0, len(rawRows))
	for _, r := range rawRows {
		rows = append(rows, dto.ReportErrorRow{
			ModelName: r.ModelName,
			Failures:  r.Failures,
			Quota:     r.Quota,
			Share:     safeRatePercent(r.Failures, totalFailures),
		})
	}
	return rows, nil
}

// ---- performance (avg + p95 + throughput) ----

// reportPerfDimension pairs a grouping column with the predicate that drops rows carrying no
// value for it. The predicate is column-typed on purpose: channel_id is an integer column, and
// PostgreSQL rejects `channel_id <> ''` with SQLSTATE 22P02 instead of coercing like SQLite/MySQL.
type reportPerfDimension struct {
	column   string
	presence string
}

var (
	reportPerfByModel   = reportPerfDimension{column: "model_name", presence: "model_name <> ''"}
	reportPerfByChannel = reportPerfDimension{column: "channel_id", presence: "channel_id <> 0"}
)

func computeReportPerformance(scope reportRoleScope, start int64, end int64, dim reportPerfDimension) ([]dto.ReportPerformanceRow, error) {
	// Aggregate average + throughput in the DB; compute p95 from a bounded sample per group in Go.
	aggExpr := dim.column + " as name, " +
		"COALESCE(sum(use_time), 0) as latency_sum, " +
		"sum(case when type = ? then 1 else 0 end) as consumers, " +
		"count(*) as requests, " +
		"COALESCE(sum(prompt_tokens), 0) + COALESCE(sum(completion_tokens), 0) + COALESCE(sum(cache_tokens), 0) as tokens"
	tx := model.LOG_DB.Table("logs").
		Select(aggExpr, model.LogTypeConsume).
		Where("created_at >= ? and created_at <= ?", start, end).
		Where(dim.presence).
		Group(dim.column).
		Order("latency_sum desc").
		Limit(reportPerformanceLimit)
	tx = scope.apply(tx)

	var rawRows []struct {
		Name       string `gorm:"column:name"`
		LatencySum int    `gorm:"column:latency_sum"`
		Consumers  int    `gorm:"column:consumers"`
		Requests   int    `gorm:"column:requests"`
		Tokens     int    `gorm:"column:tokens"`
	}
	if err := tx.Scan(&rawRows).Error; err != nil {
		return nil, err
	}

	rows := make([]dto.ReportPerformanceRow, 0, len(rawRows))
	for _, r := range rawRows {
		p95, err := reportGroupP95(scope, start, end, dim.column, r.Name)
		if err != nil {
			return nil, err
		}
		throughput := safeDivInt(r.Tokens, safeDivInt(r.LatencySum, 1000))
		rows = append(rows, dto.ReportPerformanceRow{
			Name:         r.Name,
			AvgLatencyMs: safeDivInt(r.LatencySum, r.Consumers),
			P95LatencyMs: p95,
			Requests:     r.Requests,
			Tokens:       r.Tokens,
			Throughput:   throughput,
		})
	}
	return rows, nil
}

// reportGroupP95 fetches a bounded sample of use_time for a group and computes the 95th percentile.
// Cross-DB-safe: only reads scalar use_time values; no percentile SQL function is used.
func reportGroupP95(scope reportRoleScope, start int64, end int64, groupCol string, groupVal string) (int, error) {
	tx := model.LOG_DB.Table("logs").
		Select("use_time").
		Where("created_at >= ? and created_at <= ? and "+groupCol+" = ? and type = ? and use_time > 0", start, end, groupVal, model.LogTypeConsume).
		Order("use_time asc").
		Limit(reportLatencySampleLimit)
	tx = scope.apply(tx)

	var samples []int
	if err := tx.Pluck("use_time", &samples).Error; err != nil {
		return 0, err
	}
	if len(samples) == 0 {
		return 0, nil
	}
	sort.Ints(samples)
	idx := int(float64(len(samples)) * 0.95)
	if idx >= len(samples) {
		idx = len(samples) - 1
	}
	return samples[idx], nil
}

// ---- token anatomy ----

func computeReportTokenAnatomy(scope reportRoleScope, start int64, end int64) ([]dto.ReportTokenAnatomyRow, error) {
	tx := model.LOG_DB.Table("logs").
		Select("model_name, "+
			"COALESCE(sum(prompt_tokens), 0) as prompt_tokens, "+
			"COALESCE(sum(completion_tokens), 0) as completion_tokens, "+
			"COALESCE(sum(cache_tokens), 0) as cache_tokens").
		Where("created_at >= ? and created_at <= ? and type = ?", start, end, model.LogTypeConsume).
		Where("model_name <> ''").
		Group("model_name").
		Order("prompt_tokens desc").
		Limit(reportTokenAnatomyLimit)
	tx = scope.apply(tx)

	var rawRows []struct {
		ModelName       string `gorm:"column:model_name"`
		PromptTokens    int    `gorm:"column:prompt_tokens"`
		CompletionTokens int   `gorm:"column:completion_tokens"`
		CacheTokens     int    `gorm:"column:cache_tokens"`
	}
	if err := tx.Scan(&rawRows).Error; err != nil {
		return nil, err
	}

	rows := make([]dto.ReportTokenAnatomyRow, 0, len(rawRows))
	for _, r := range rawRows {
		rows = append(rows, dto.ReportTokenAnatomyRow{
			ModelName:        r.ModelName,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
			CacheTokens:      r.CacheTokens,
			Total:            r.PromptTokens + r.CompletionTokens + r.CacheTokens,
		})
	}
	return rows, nil
}

// ---- stream vs non-stream ----

func computeReportStreamComparison(scope reportRoleScope, start int64, end int64) (*dto.ReportStreamComparison, error) {
	tx := model.LOG_DB.Table("logs").
		Select("is_stream, "+
			"COALESCE(sum(use_time), 0) as latency_sum, "+
			"sum(case when type = ? then 1 else 0 end) as consumers, "+
			"count(*) as requests",
			model.LogTypeConsume).
		Where("created_at >= ? and created_at <= ? and type = ?", start, end, model.LogTypeConsume).
		Group("is_stream")
	tx = scope.apply(tx)

	var rawRows []struct {
		IsStream  bool `gorm:"column:is_stream"`
		LatencySum int `gorm:"column:latency_sum"`
		Consumers  int `gorm:"column:consumers"`
		Requests   int `gorm:"column:requests"`
	}
	if err := tx.Scan(&rawRows).Error; err != nil {
		return nil, err
	}

	out := &dto.ReportStreamComparison{}
	for _, r := range rawRows {
		if r.IsStream {
			out.StreamAvgLatencyMs = safeDivInt(r.LatencySum, r.Consumers)
			out.StreamRequests = r.Requests
		} else {
			out.NonStreamAvgLatencyMs = safeDivInt(r.LatencySum, r.Consumers)
			out.NonStreamRequests = r.Requests
		}
	}
	return out, nil
}

// ---- channel cost/health (admin) ----

func computeReportChannels(scope reportRoleScope, start int64, end int64) ([]dto.ReportChannelRow, error) {
	tx := model.LOG_DB.Table("logs").
		Select("channel_id, "+
			"COALESCE(sum(quota), 0) as quota, "+
			"count(*) as requests, "+
			"sum(case when type = ? then 1 else 0 end) as failures, "+
			"sum(case when type = ? then 1 else 0 end) as consumers, "+
			"COALESCE(sum(use_time), 0) as latency_sum",
			model.LogTypeError, model.LogTypeConsume).
		Where("created_at >= ? and created_at <= ?", start, end).
		Where("channel_id <> 0").
		Group("channel_id").
		Order("quota desc").
		Limit(reportChannelLimit)

	var rawRows []struct {
		ChannelID  int   `gorm:"column:channel_id"`
		Quota      int   `gorm:"column:quota"`
		Requests   int   `gorm:"column:requests"`
		Failures   int   `gorm:"column:failures"`
		Consumers  int   `gorm:"column:consumers"`
		LatencySum int   `gorm:"column:latency_sum"`
	}
	if err := tx.Scan(&rawRows).Error; err != nil {
		return nil, err
	}

	ids := make([]int, 0, len(rawRows))
	for _, r := range rawRows {
		ids = append(ids, r.ChannelID)
	}
	nameByID := make(map[int]string)
	if len(ids) > 0 {
		var channels []struct {
			ID   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if err := model.DB.Table("channels").Select("id, name").Where("id IN ?", ids).Find(&channels).Error; err == nil {
			for _, c := range channels {
				nameByID[c.ID] = c.Name
			}
		}
	}

	rows := make([]dto.ReportChannelRow, 0, len(rawRows))
	for _, r := range rawRows {
		name := nameByID[r.ChannelID]
		if name == "" {
			name = fmt.Sprintf("#%d", r.ChannelID)
		}
		rows = append(rows, dto.ReportChannelRow{
			ChannelID:    r.ChannelID,
			ChannelName:  name,
			Quota:        r.Quota,
			Requests:     r.Requests,
			Failures:     r.Failures,
			ErrorRate:    safeRate(r.Failures, r.Requests),
			AvgLatencyMs: safeDivInt(r.LatencySum, r.Consumers),
		})
	}
	return rows, nil
}

// ---- helpers ----

func sumReportIntField(rows []reportBucketRow, pick func(reportBucketRow) int) int {
	total := 0
	for _, r := range rows {
		total += pick(r)
	}
	return total
}

func safeDivInt(num int, den int) int {
	if den <= 0 {
		return 0
	}
	return num / den
}

func safeRate(num int, den int) float64 {
	if den <= 0 {
		return 0
	}
	return float64(num) / float64(den)
}

func safeRatePercent(num int, den int) int {
	if den <= 0 {
		return 0
	}
	return int(float64(num) * 100 / float64(den))
}
