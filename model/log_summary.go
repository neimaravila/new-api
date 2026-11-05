package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

const maxLogSummaryBuckets = 60

var logSummaryBucketSizes = []int64{60, 300, 900, 3600, 21600, 86400}

type LogSummaryBucket struct {
	Timestamp int64 `json:"timestamp"`
	Requests  int   `json:"requests"`
	Failures  int   `json:"failures"`
}

type LogSummary struct {
	Requests          int                `json:"requests"`
	Successes         int                `json:"successes"`
	Failures          int                `json:"failures"`
	Tokens            int                `json:"tokens"`
	Quota             int                `json:"quota"`
	AvgUseTimeSeconds float64            `json:"avg_use_time_seconds"`
	BucketSeconds     int64              `json:"bucket_seconds"`
	Buckets           []LogSummaryBucket `json:"buckets"`
}

// logSummaryBucketSeconds returns the smallest bucket size that keeps the range
// within maxLogSummaryBuckets buckets.
func logSummaryBucketSeconds(rangeSeconds int64) int64 {
	for _, size := range logSummaryBucketSizes {
		if (rangeSeconds+size-1)/size <= maxLogSummaryBuckets {
			return size
		}
	}
	return logSummaryBucketSizes[len(logSummaryBucketSizes)-1]
}

func logSummaryBucketExpr(bucketSize int64) string {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		// ClickHouse's / is float division, so use intDiv.
		return fmt.Sprintf("intDiv(logs.created_at, %d) * %d", bucketSize, bucketSize)
	}
	if common.UsingLogDatabase(common.DatabaseTypeMySQL) {
		return fmt.Sprintf("FLOOR(logs.created_at / %d) * %d", bucketSize, bucketSize)
	}
	return fmt.Sprintf("(logs.created_at / %d) * %d", bucketSize, bucketSize)
}

// GetLogSummary totals the requests (consume plus error logs) matching the
// filter. Tokens, quota and latency come only from consume logs. userID 0
// means every user. A filter without a start covers the last 24 hours.
func GetLogSummary(userID int, filter LogFilter) (LogSummary, error) {
	if filter.StartTimestamp < 0 || filter.EndTimestamp < 0 {
		return LogSummary{}, common.NewMessage("Invalid time range")
	}
	now := common.GetTimestamp()
	if filter.StartTimestamp == 0 {
		filter.StartTimestamp = now - 24*3600
	}
	end := filter.EndTimestamp
	if end == 0 || end > now {
		end = now
	}
	summary := LogSummary{
		BucketSeconds: logSummaryBucketSeconds(max(end-filter.StartTimestamp, 1)),
		Buckets:       []LogSummaryBucket{},
	}
	if filter.Type != LogTypeUnknown && filter.Type != LogTypeConsume && filter.Type != LogTypeError {
		return summary, nil
	}
	tx, err := filter.apply(LOG_DB.Model(&Log{}))
	if err != nil {
		return summary, err
	}
	if userID != 0 {
		tx = tx.Where("logs.user_id = ?", userID)
	}
	bucketExpr := logSummaryBucketExpr(summary.BucketSeconds)
	consume := fmt.Sprintf("logs.type = %d", LogTypeConsume)
	var rows []struct {
		Bucket    int64
		Successes int
		Failures  int
		Tokens    int
		Quota     int
		UseTime   int
	}
	err = tx.Where("logs.type IN ?", []int{LogTypeConsume, LogTypeError}).
		Select(fmt.Sprintf(
			"%s AS bucket, "+
				"SUM(CASE WHEN %s THEN 1 ELSE 0 END) AS successes, "+
				"SUM(CASE WHEN %s THEN 0 ELSE 1 END) AS failures, "+
				"SUM(CASE WHEN %s THEN logs.prompt_tokens + logs.completion_tokens ELSE 0 END) AS tokens, "+
				"SUM(CASE WHEN %s THEN logs.quota ELSE 0 END) AS quota, "+
				"SUM(CASE WHEN %s THEN logs.use_time ELSE 0 END) AS use_time",
			bucketExpr, consume, consume, consume, consume, consume)).
		Group(bucketExpr).
		Order("bucket ASC").
		Scan(&rows).Error
	if err != nil {
		return summary, err
	}
	useTime := 0
	for _, row := range rows {
		summary.Successes += row.Successes
		summary.Failures += row.Failures
		summary.Tokens += row.Tokens
		summary.Quota += row.Quota
		useTime += row.UseTime
		summary.Buckets = append(summary.Buckets, LogSummaryBucket{
			Timestamp: row.Bucket,
			Requests:  row.Successes + row.Failures,
			Failures:  row.Failures,
		})
	}
	summary.Requests = summary.Successes + summary.Failures
	if summary.Successes > 0 {
		summary.AvgUseTimeSeconds = float64(useTime) / float64(summary.Successes)
	}
	return summary, nil
}
