package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func seedLogSummary(t *testing.T, logs []Log) int64 {
	t.Helper()
	require.NoError(t, LOG_DB.Where("1 = 1").Delete(&Log{}).Error)
	for i := range logs {
		require.NoError(t, LOG_DB.Create(&logs[i]).Error)
	}
	return common.GetTimestamp()
}

func TestGetLogSummaryCountsConsumeAndError(t *testing.T) {
	now := common.GetTimestamp()
	seedLogSummary(t, []Log{
		{UserId: 1, CreatedAt: now - 100, Type: LogTypeConsume, UseTime: 1, Quota: 10, PromptTokens: 3, CompletionTokens: 2},
		{UserId: 1, CreatedAt: now - 90, Type: LogTypeConsume, UseTime: 3, Quota: 20, PromptTokens: 1, CompletionTokens: 1},
		{UserId: 1, CreatedAt: now - 80, Type: LogTypeError, Quota: 5},
		{UserId: 1, CreatedAt: now - 70, Type: LogTypeTopup, Quota: 1000},
	})
	got, err := GetLogSummary(0, LogFilter{})
	require.NoError(t, err)
	require.Equal(t, 3, got.Requests)
	require.Equal(t, 2, got.Successes)
	require.Equal(t, 1, got.Failures)
	require.Equal(t, 2.0, got.AvgUseTimeSeconds)
	require.Equal(t, 30, got.Quota)
	require.Equal(t, 7, got.Tokens)
}

func TestGetLogSummaryAppliesFilterAndUser(t *testing.T) {
	now := common.GetTimestamp()
	seedLogSummary(t, []Log{
		{UserId: 1, CreatedAt: now - 100, Type: LogTypeConsume, ModelName: "a"},
		{UserId: 1, CreatedAt: now - 100, Type: LogTypeError, ModelName: "a"},
		{UserId: 1, CreatedAt: now - 100, Type: LogTypeConsume, ModelName: "b"},
		{UserId: 2, CreatedAt: now - 100, Type: LogTypeConsume, ModelName: "a"},
	})
	got, err := GetLogSummary(1, LogFilter{ModelName: "a"})
	require.NoError(t, err)
	require.Equal(t, 2, got.Requests)
	require.Equal(t, 1, got.Failures)
	total := 0
	for _, b := range got.Buckets {
		total += b.Requests
	}
	require.Equal(t, 2, total)

	got, err = GetLogSummary(0, LogFilter{ModelName: "a", Type: LogTypeError})
	require.NoError(t, err)
	require.Equal(t, 1, got.Requests)
}

func TestGetLogSummaryNonRequestTypeIsEmpty(t *testing.T) {
	now := common.GetTimestamp()
	seedLogSummary(t, []Log{{UserId: 1, CreatedAt: now - 100, Type: LogTypeManage}})
	got, err := GetLogSummary(0, LogFilter{Type: LogTypeManage})
	require.NoError(t, err)
	require.Zero(t, got.Requests)
	require.Zero(t, got.Quota)
	require.Empty(t, got.Buckets)
}

func TestGetLogSummaryBucketsSumToRequests(t *testing.T) {
	now := common.GetTimestamp()
	var logs []Log
	for i := range 200 {
		logs = append(logs, Log{UserId: 1, CreatedAt: now - int64(i)*400, Type: LogTypeConsume})
	}
	logs = append(logs, Log{UserId: 1, CreatedAt: now - 50, Type: LogTypeError})
	seedLogSummary(t, logs)
	got, err := GetLogSummary(0, LogFilter{StartTimestamp: now - 24*3600, EndTimestamp: now})
	require.NoError(t, err)
	total := 0
	for _, b := range got.Buckets {
		total += b.Requests
	}
	require.Equal(t, got.Requests, total)
	require.Equal(t, 201, got.Requests)
	require.LessOrEqual(t, len(got.Buckets), 60)
	require.Equal(t, int64(3600), got.BucketSeconds)
}

func TestLogSummaryBucketExprByDatabase(t *testing.T) {
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	t.Cleanup(func() { common.SetDatabaseTypes(previousMain, previousLog) })
	for typ, want := range map[common.DatabaseType]string{
		common.DatabaseTypeClickHouse: "intDiv(logs.created_at, 300) * 300",
		common.DatabaseTypeMySQL:      "FLOOR(logs.created_at / 300) * 300",
		common.DatabaseTypeSQLite:     "(logs.created_at / 300) * 300",
	} {
		common.SetLogDatabaseType(typ)
		require.Equal(t, want, logSummaryBucketExpr(300), string(typ))
	}
}

func TestLogSummaryBucketSecondsBoundaries(t *testing.T) {
	day := int64(24 * 3600)
	require.Equal(t, int64(3600), logSummaryBucketSeconds(day))
	week := logSummaryBucketSeconds(7 * day)
	require.Equal(t, int64(21600), week)
	require.LessOrEqual(t, (7*day+week-1)/week+1, int64(61))
	require.Equal(t, int64(86400), logSummaryBucketSeconds(1000*day))
}

func TestGetLogSummaryRejectsNegativeTimestamps(t *testing.T) {
	_, err := GetLogSummary(0, LogFilter{StartTimestamp: -9223372036854775808})
	require.Error(t, err)
	_, err = GetLogSummary(0, LogFilter{EndTimestamp: -1})
	require.Error(t, err)
}

func TestGetLogSummaryTokensIncludeCacheTokens(t *testing.T) {
	now := common.GetTimestamp()
	seedLogSummary(t, []Log{
		{UserId: 1, CreatedAt: now - 100, Type: LogTypeConsume, PromptTokens: 3, CompletionTokens: 2, CacheTokens: 5},
		{UserId: 1, CreatedAt: now - 90, Type: LogTypeError, PromptTokens: 100, CacheTokens: 100},
	})
	got, err := GetLogSummary(0, LogFilter{})
	require.NoError(t, err)
	require.Equal(t, 10, got.Tokens)
}
