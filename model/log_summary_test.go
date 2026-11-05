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
