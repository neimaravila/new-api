package service

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

// ReportLimits caps how many breakdown rows are returned per category.
const (
	reportModelLimit             = 50
	reportErrorLimit             = 20
	reportPerformanceLimit       = 20
	reportTokenAnatomyLimit      = 30
	reportChannelLimit           = 30
	reportLatencySampleLimit     = 5000
	reportCostModelLimit         = 5
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
	isAdmin   bool
	userID    int
	username  string
	modelName string
	channelID int
}

func reportScope(userID int, username string, role int, filter dto.ReportFilter) reportRoleScope {
	isAdmin := role >= common.RoleAdminUser
	channelID := filter.ChannelID
	if !isAdmin {
		// The channel filter is admin-only.
		channelID = 0
	}
	return reportRoleScope{
		isAdmin:   isAdmin,
		userID:    userID,
		username:  username,
		modelName: filter.ModelName,
		channelID: channelID,
	}
}

func (s reportRoleScope) apply(tx *gorm.DB) *gorm.DB {
	if s.modelName != "" {
		tx = tx.Where("model_name = ?", s.modelName)
	}
	if s.channelID > 0 {
		tx = tx.Where("channel_id = ?", s.channelID)
	}
	if s.isAdmin {
		return tx
	}
	if s.userID > 0 {
		return tx.Where("user_id = ?", s.userID)
	}
	return tx.Where("username = ?", s.username)
}

// GetReportSummary computes the full role-scoped report payload.
func GetReportSummary(userID int, username string, role int, r dto.ReportTimeRange, granularity dto.ReportGranularity, filter dto.ReportFilter) (dto.ReportSummary, error) {
	now := time.Now()
	start, end := ResolveReportWindow(r, now)
	if granularity == "" {
		granularity = DefaultReportGranularity(start, end)
	}
	scope := reportScope(userID, username, role, filter)

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

	costByModel, err := computeReportCostByModel(scope, start, end, granularity)
	if err != nil {
		return dto.ReportSummary{}, err
	}
	latencySeries, err := computeReportLatencySeries(scope, start, end, granularity)
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
		CostByModel:       costByModel,
		LatencySeries:     latencySeries,
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
		nameChannelPerformanceRows(channelPerf)
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
	if common.UsingLogDatabase(common.DatabaseTypeMySQL) {
		// MySQL's / is decimal division, so the bucket number needs an explicit floor.
		bucketExpr = fmt.Sprintf("FLOOR((created_at - %d) / %d)", start, intervalSeconds)
	}
	tx := model.LOG_DB.Table("logs").
		Select(bucketExpr+" as bucket, "+
			"COALESCE(sum(case when type = ? then quota else 0 end), 0) as quota, "+
			"count(*) as requests, "+
			"COALESCE(sum(case when type = ? then prompt_tokens + completion_tokens + cache_tokens else 0 end), 0) as tokens, "+
			"sum(case when type = ? then 1 else 0 end) as failures, "+
			"COALESCE(sum(case when type = ? then use_time else 0 end), 0) as latency_sum, "+
			"sum(case when type = ? then 1 else 0 end) as consumers",
			model.LogTypeConsume, model.LogTypeConsume, model.LogTypeError, model.LogTypeConsume, model.LogTypeConsume).
		Where("created_at >= ? and created_at <= ? and type in (?, ?)", start, end, model.LogTypeConsume, model.LogTypeError).
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
			LatencySum: r.LatencySum * 1000,
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
			"COALESCE(sum(case when type = ? then quota else 0 end), 0) as quota, "+
			"count(*) as requests, "+
			"COALESCE(sum(case when type = ? then prompt_tokens + completion_tokens + cache_tokens else 0 end), 0) as tokens, "+
			"sum(case when type = ? then 1 else 0 end) as failures, "+
			"sum(case when type = ? then 1 else 0 end) as consumers, "+
			"COALESCE(sum(case when type = ? then use_time else 0 end), 0) as latency_sum",
			model.LogTypeConsume, model.LogTypeConsume, model.LogTypeError, model.LogTypeConsume, model.LogTypeConsume).
		Where("created_at >= ? and created_at <= ? and type in (?, ?)", start, end, model.LogTypeConsume, model.LogTypeError).
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
			AvgLatencyMs: safeDivInt(r.LatencySum*1000, r.Consumers),
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
// PostgreSQL rejects comparing channel_id to an empty string with SQLSTATE 22P02 instead of coercing like SQLite/MySQL.
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
		"COALESCE(sum(case when type = ? then use_time else 0 end), 0) as latency_sum, " +
		"sum(case when type = ? then 1 else 0 end) as consumers, " +
		"count(*) as requests, " +
		"COALESCE(sum(case when type = ? then prompt_tokens + completion_tokens + cache_tokens else 0 end), 0) as tokens"
	tx := model.LOG_DB.Table("logs").
		Select(aggExpr, model.LogTypeConsume, model.LogTypeConsume, model.LogTypeConsume).
		Where("created_at >= ? and created_at <= ? and type in (?, ?)", start, end, model.LogTypeConsume, model.LogTypeError).
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
		p50, p95, err := reportGroupPercentiles(scope, start, end, dim.column, r.Name, reportLatencySampleLimit)
		if err != nil {
			return nil, err
		}
		throughput := safeDivInt(r.Tokens, r.LatencySum)
		rows = append(rows, dto.ReportPerformanceRow{
			Name:         r.Name,
			AvgLatencyMs: safeDivInt(r.LatencySum*1000, r.Consumers),
			P50LatencyMs: p50,
			P95LatencyMs: p95,
			Requests:     r.Requests,
			Tokens:       r.Tokens,
			Throughput:   throughput,
		})
	}
	return rows, nil
}

// nameChannelPerformanceRows replaces the raw channel_id each row is grouped under with the channel
// name, so the panel reads like the channel cost/health table instead of listing bare ids.
func nameChannelPerformanceRows(rows []dto.ReportPerformanceRow) {
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		id, err := strconv.Atoi(row.Name)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}

	nameByID := reportChannelDisplayNames(ids)
	for i := range rows {
		id, err := strconv.Atoi(rows[i].Name)
		if err != nil {
			continue
		}
		rows[i].Name = nameByID[id]
	}
}

// reportGroupPercentiles fetches a bounded sample of use_time for a group and computes the 50th and
// 95th percentiles in milliseconds. Cross-DB-safe: only reads scalar use_time values; no percentile
// SQL function is used.
func reportGroupPercentiles(scope reportRoleScope, start int64, end int64, groupCol string, groupVal string, sampleLimit int) (int, int, error) {
	tx := model.LOG_DB.Table("logs").
		Select("use_time").
		Where("created_at >= ? and created_at <= ? and "+groupCol+" = ? and type = ? and use_time > 0", start, end, groupVal, model.LogTypeConsume).
		Order("use_time asc").
		Limit(sampleLimit)
	tx = scope.apply(tx)

	var samples []int
	if err := tx.Pluck("use_time", &samples).Error; err != nil {
		return 0, 0, err
	}
	p50, p95 := reportPercentilesMs(samples)
	return p50, p95, nil
}

// reportPercentilesMs returns the p50 and p95 of use_time samples (seconds) in milliseconds.
func reportPercentilesMs(samples []int) (int, int) {
	if len(samples) == 0 {
		return 0, 0
	}
	sort.Ints(samples)
	at := func(q float64) int {
		idx := int(float64(len(samples)) * q)
		if idx >= len(samples) {
			idx = len(samples) - 1
		}
		return samples[idx] * 1000
	}
	return at(0.5), at(0.95)
}

// ---- cost by model ----

func computeReportCostByModel(scope reportRoleScope, start int64, end int64, granularity dto.ReportGranularity) (dto.ReportCostSeries, error) {
	intervalSeconds := reportIntervalSeconds(granularity)
	bucketStart := start - (start % intervalSeconds)
	bucketExpr := reportBucketExpr(bucketStart, intervalSeconds)

	tx := model.LOG_DB.Table("logs").
		Select(bucketExpr+" as bucket, model_name, COALESCE(sum(quota), 0) as quota").
		Where("created_at >= ? and created_at <= ? and type = ?", bucketStart, end, model.LogTypeConsume).
		Where("model_name <> ''").
		Group("bucket, model_name").
		Order("bucket asc")
	tx = scope.apply(tx)

	var rawRows []struct {
		Bucket    int64  `gorm:"column:bucket"`
		ModelName string `gorm:"column:model_name"`
		Quota     int    `gorm:"column:quota"`
	}
	if err := tx.Scan(&rawRows).Error; err != nil {
		return dto.ReportCostSeries{}, err
	}

	totalByModel := make(map[string]int)
	for _, r := range rawRows {
		totalByModel[r.ModelName] += r.Quota
	}
	names := make([]string, 0, len(totalByModel))
	for name := range totalByModel {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if totalByModel[names[i]] != totalByModel[names[j]] {
			return totalByModel[names[i]] > totalByModel[names[j]]
		}
		return names[i] < names[j]
	})
	hasOthers := len(names) > reportCostModelLimit
	if hasOthers {
		names = names[:reportCostModelLimit]
	}
	top := make(map[string]bool, len(names))
	for _, name := range names {
		top[name] = true
	}
	models := append([]string{}, names...)
	if hasOthers {
		models = append(models, dto.ReportCostOthersKey)
	}

	points := make([]dto.ReportCostPoint, 0)
	byBucket := make(map[int64]*dto.ReportCostPoint)
	for _, r := range rawRows {
		ts := bucketStart + r.Bucket*intervalSeconds
		p, ok := byBucket[ts]
		if !ok {
			points = append(points, dto.ReportCostPoint{
				BucketLabel:     formatReportBucketLabel(ts, granularity),
				BucketTimestamp: ts,
				Values:          map[string]int{},
			})
			p = &points[len(points)-1]
			byBucket[ts] = p
		}
		key := r.ModelName
		if !top[key] {
			key = dto.ReportCostOthersKey
		}
		p.Values[key] += r.Quota
	}
	return dto.ReportCostSeries{Models: models, Points: points}, nil
}

// ---- latency percentile series ----

func computeReportLatencySeries(scope reportRoleScope, start int64, end int64, granularity dto.ReportGranularity) ([]dto.ReportLatencyPoint, error) {
	intervalSeconds := reportIntervalSeconds(granularity)
	bucketStart := start - (start % intervalSeconds)

	// One bounded sample, most recent first; percentiles are computed in Go for cross-DB safety.
	tx := model.LOG_DB.Table("logs").
		Select("created_at, use_time").
		Where("created_at >= ? and created_at <= ? and type = ? and use_time > 0", bucketStart, end, model.LogTypeConsume).
		Order("created_at desc").
		Limit(reportLatencySampleLimit)
	tx = scope.apply(tx)

	var rawRows []struct {
		CreatedAt int64 `gorm:"column:created_at"`
		UseTime   int   `gorm:"column:use_time"`
	}
	if err := tx.Scan(&rawRows).Error; err != nil {
		return nil, err
	}

	samplesByBucket := make(map[int64][]int)
	for _, r := range rawRows {
		ts := bucketStart + (r.CreatedAt-bucketStart)/intervalSeconds*intervalSeconds
		samplesByBucket[ts] = append(samplesByBucket[ts], r.UseTime)
	}
	timestamps := make([]int64, 0, len(samplesByBucket))
	for ts := range samplesByBucket {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })

	points := make([]dto.ReportLatencyPoint, 0, len(timestamps))
	for _, ts := range timestamps {
		p50, p95 := reportPercentilesMs(samplesByBucket[ts])
		points = append(points, dto.ReportLatencyPoint{
			BucketLabel:     formatReportBucketLabel(ts, granularity),
			BucketTimestamp: ts,
			P50Ms:           p50,
			P95Ms:           p95,
		})
	}
	return points, nil
}

func reportIntervalSeconds(granularity dto.ReportGranularity) int64 {
	if granularity == dto.ReportGranularityDay {
		return 86400
	}
	return 3600
}

// reportBucketExpr builds the portable bucket-number expression for created_at.
func reportBucketExpr(start int64, intervalSeconds int64) string {
	if common.UsingLogDatabase(common.DatabaseTypeMySQL) {
		// MySQL's / is decimal division, so the bucket number needs an explicit floor.
		return fmt.Sprintf("FLOOR((created_at - %d) / %d)", start, intervalSeconds)
	}
	return fmt.Sprintf("(created_at - %d) / %d", start, intervalSeconds)
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
		ModelName        string `gorm:"column:model_name"`
		PromptTokens     int    `gorm:"column:prompt_tokens"`
		CompletionTokens int    `gorm:"column:completion_tokens"`
		CacheTokens      int    `gorm:"column:cache_tokens"`
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
		IsStream   bool `gorm:"column:is_stream"`
		LatencySum int  `gorm:"column:latency_sum"`
		Consumers  int  `gorm:"column:consumers"`
		Requests   int  `gorm:"column:requests"`
	}
	if err := tx.Scan(&rawRows).Error; err != nil {
		return nil, err
	}

	out := &dto.ReportStreamComparison{}
	for _, r := range rawRows {
		if r.IsStream {
			out.StreamAvgLatencyMs = safeDivInt(r.LatencySum*1000, r.Consumers)
			out.StreamRequests = r.Requests
		} else {
			out.NonStreamAvgLatencyMs = safeDivInt(r.LatencySum*1000, r.Consumers)
			out.NonStreamRequests = r.Requests
		}
	}
	return out, nil
}

// ---- channel cost/health (admin) ----

func computeReportChannels(scope reportRoleScope, start int64, end int64) ([]dto.ReportChannelRow, error) {
	tx := model.LOG_DB.Table("logs").
		Select("channel_id, "+
			"COALESCE(sum(case when type = ? then quota else 0 end), 0) as quota, "+
			"count(*) as requests, "+
			"sum(case when type = ? then 1 else 0 end) as failures, "+
			"sum(case when type = ? then 1 else 0 end) as consumers, "+
			"COALESCE(sum(case when type = ? then use_time else 0 end), 0) as latency_sum",
			model.LogTypeConsume, model.LogTypeError, model.LogTypeConsume, model.LogTypeConsume).
		Where("created_at >= ? and created_at <= ? and type in (?, ?)", start, end, model.LogTypeConsume, model.LogTypeError).
		Where("channel_id <> 0")
	tx = scope.apply(tx).
		Group("channel_id").
		Order("quota desc").
		Limit(reportChannelLimit)

	var rawRows []struct {
		ChannelID  int `gorm:"column:channel_id"`
		Quota      int `gorm:"column:quota"`
		Requests   int `gorm:"column:requests"`
		Failures   int `gorm:"column:failures"`
		Consumers  int `gorm:"column:consumers"`
		LatencySum int `gorm:"column:latency_sum"`
	}
	if err := tx.Scan(&rawRows).Error; err != nil {
		return nil, err
	}

	ids := make([]int, 0, len(rawRows))
	for _, r := range rawRows {
		ids = append(ids, r.ChannelID)
	}
	nameByID := reportChannelDisplayNames(ids)

	rows := make([]dto.ReportChannelRow, 0, len(rawRows))
	for _, r := range rawRows {
		rows = append(rows, dto.ReportChannelRow{
			ChannelID:    r.ChannelID,
			ChannelName:  nameByID[r.ChannelID],
			Quota:        r.Quota,
			Requests:     r.Requests,
			Failures:     r.Failures,
			ErrorRate:    safeRate(r.Failures, r.Requests),
			AvgLatencyMs: safeDivInt(r.LatencySum*1000, r.Consumers),
		})
	}
	return rows, nil
}

// reportChannelDisplayNames maps channel ids to the label the report shows for them. Channels that
// were deleted since the logs were written keep a "#id" placeholder, and so does every id if the
// lookup fails: a missing name must not cost the caller its aggregation.
func reportChannelDisplayNames(ids []int) map[int]string {
	nameByID := make(map[int]string, len(ids))
	for _, id := range ids {
		nameByID[id] = fmt.Sprintf("#%d", id)
	}
	if len(ids) == 0 {
		return nameByID
	}

	var channels []struct {
		ID   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err := model.DB.Table("channels").Select("id, name").Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return nameByID
	}
	for _, c := range channels {
		if c.Name != "" {
			nameByID[c.ID] = c.Name
		}
	}
	return nameByID
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
