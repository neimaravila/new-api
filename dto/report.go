package dto

// ReportTimeRange is the selectable window for a report.
type ReportTimeRange string

const (
	ReportRangeToday       ReportTimeRange = "today"
	ReportRangeYesterday   ReportTimeRange = "yesterday"
	ReportRangeLast7Days   ReportTimeRange = "7d"
	ReportRangeLast30Days  ReportTimeRange = "30d"
	ReportRangeThisMonth   ReportTimeRange = "month"
)

// ReportGranularity is the bucket size for temporal aggregations.
type ReportGranularity string

const (
	ReportGranularityHour  ReportGranularity = "hour"
	ReportGranularityDay   ReportGranularity = "day"
)

// ReportTrendPoint is a single temporal bucket.
type ReportTrendPoint struct {
	BucketLabel       string `json:"bucket_label"`
	BucketTimestamp   int64  `json:"bucket_timestamp"`
	Quota             int    `json:"quota"`
	Requests          int    `json:"requests"`
	Tokens            int    `json:"tokens"`
	Failures          int    `json:"failures"`
	AvgLatencyMs      int    `json:"avg_latency_ms"`
	ConsumingRequests int    `json:"consuming_requests"`
}

// ReportTrend is the temporal trend payload.
type ReportTrend struct {
	Granularity ReportGranularity   `json:"granularity"`
	Points      []ReportTrendPoint `json:"points"`
	Totals      ReportTotals        `json:"totals"`
}

// ReportTotals holds period totals.
type ReportTotals struct {
	Quota        int `json:"quota"`
	Requests     int `json:"requests"`
	Tokens       int `json:"tokens"`
	Failures     int `json:"failures"`
	Consumers    int `json:"consumers"`
	AvgLatencyMs int `json:"avg_latency_ms"`
}

// ReportModelRow is one model's aggregated metrics.
type ReportModelRow struct {
	ModelName    string  `json:"model_name"`
	Quota        int     `json:"quota"`
	Requests     int     `json:"requests"`
	Tokens       int     `json:"tokens"`
	Failures     int     `json:"failures"`
	ErrorRate    float64 `json:"error_rate"`
	AvgLatencyMs int     `json:"avg_latency_ms"`
}

// ReportModelErrorRow is the top failure reasons / models.
type ReportErrorRow struct {
	ModelName string `json:"model_name"`
	Failures  int    `json:"failures"`
	Quota     int    `json:"quota"`
	Share     int    `json:"share"`
}

// ReportPerformanceRow is avg/p95 latency and throughput per entity.
type ReportPerformanceRow struct {
	Name        string  `json:"name"`
	AvgLatencyMs int    `json:"avg_latency_ms"`
	P95LatencyMs int    `json:"p95_latency_ms"`
	Requests    int     `json:"requests"`
	Tokens      int     `json:"tokens"`
	Throughput  int     `json:"throughput"`
}

// ReportTokenAnatomyRow is prompt/completion/cache split per model.
type ReportTokenAnatomyRow struct {
	ModelName       string `json:"model_name"`
	PromptTokens    int    `json:"prompt_tokens"`
	CompletionTokens int   `json:"completion_tokens"`
	CacheTokens     int    `json:"cache_tokens"`
	Total           int    `json:"total"`
}

// ReportChannelRow is per-channel cost/health (admin only).
type ReportChannelRow struct {
	ChannelID    int     `json:"channel_id"`
	ChannelName  string  `json:"channel_name"`
	Quota        int     `json:"quota"`
	Requests     int     `json:"requests"`
	Failures     int     `json:"failures"`
	ErrorRate    float64 `json:"error_rate"`
	AvgLatencyMs int     `json:"avg_latency_ms"`
}

// ReportSummary is the full aggregation payload consumed by the reports UI.
type ReportSummary struct {
	Role          DashboardRole             `json:"role"`
	Range         ReportTimeRange        `json:"range"`
	Granularity   ReportGranularity      `json:"granularity"`
	PeriodStart   int64                  `json:"period_start"`
	PeriodEnd     int64                  `json:"period_end"`
	Trend         ReportTrend            `json:"trend"`
	Models        []ReportModelRow       `json:"models"`
	Errors        []ReportErrorRow       `json:"errors"`
	ModelPerformance  []ReportPerformanceRow `json:"model_performance"`
	ChannelPerformance []ReportPerformanceRow `json:"channel_performance,omitempty"`
	TokenAnatomy  []ReportTokenAnatomyRow `json:"token_anatomy"`
	Channels      []ReportChannelRow     `json:"channels,omitempty"`
	StreamPerformance *ReportStreamComparison `json:"stream_performance,omitempty"`
}

// ReportStreamComparison compares stream vs non-stream throughput.
type ReportStreamComparison struct {
	StreamAvgLatencyMs   int `json:"stream_avg_latency_ms"`
	StreamRequests       int `json:"stream_requests"`
	NonStreamAvgLatencyMs int `json:"non_stream_avg_latency_ms"`
	NonStreamRequests    int `json:"non_stream_requests"`
}
