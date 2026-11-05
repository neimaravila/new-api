package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const retentionTestRunner = "runner-retention"

func setupLogContentRetention(t *testing.T) {
	t.Helper()
	truncate(t)
	require.NoError(t, model.MigrateLogContents())
	previousDays := common.LogContentRetentionDays
	previousCapture := common.LogContentCaptureEnabled
	previousType := common.LogDatabaseType()
	t.Cleanup(func() {
		model.LOG_DB.Exec("DELETE FROM log_contents")
		common.LogContentRetentionDays = previousDays
		common.LogContentCaptureEnabled = previousCapture
		common.SetLogDatabaseType(previousType)
	})
	common.LogContentRetentionDays = 30
}

func seedRetentionContents(t *testing.T, createdAt int64, n int, prefix string) {
	t.Helper()
	rows := make([]*model.LogContent, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, &model.LogContent{
			RequestId:     prefix + "-" + time.Duration(i).String(),
			CreatedAt:     createdAt,
			CaptureStatus: model.LogContentStatusCaptured,
		})
	}
	require.NoError(t, model.LOG_DB.CreateInBatches(rows, 50).Error)
}

func countRetentionContents(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.LOG_DB.Model(&model.LogContent{}).Count(&count).Error)
	return count
}

// claimRetentionTask creates and claims a task of the retention type.
func claimRetentionTask(t *testing.T) *model.SystemTask {
	t.Helper()
	created, err := model.CreateSystemTask(SystemTaskTypeLogContentRetention, nil, nil)
	require.NoError(t, err)
	claimed, ok, err := model.ClaimSystemTask(created.ID, SystemTaskTypeLogContentRetention, retentionTestRunner, common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	return claimed
}

func finishedRetentionTask(t *testing.T, task *model.SystemTask) *model.SystemTask {
	t.Helper()
	got, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	return got
}

func TestLogContentRetentionDeletesOnlyExpired(t *testing.T) {
	setupLogContentRetention(t)
	now := common.GetTimestamp()
	cutoff := logContentRetentionCutoff(now, 30)
	seedRetentionContents(t, cutoff-1, 1, "before")
	seedRetentionContents(t, cutoff, 1, "at")
	seedRetentionContents(t, cutoff+1000, 1, "after")

	task := claimRetentionTask(t)
	logContentRetentionHandler{now: func() int64 { return now }}.Run(context.Background(), task, retentionTestRunner)

	var remaining []string
	require.NoError(t, model.LOG_DB.Model(&model.LogContent{}).Order("created_at").Pluck("request_id", &remaining).Error)
	assert.Equal(t, []string{"at-0s", "after-0s"}, remaining, "the row exactly at the cutoff survives")
	assert.Equal(t, model.SystemTaskStatusSucceeded, finishedRetentionTask(t, task).Status)
}

func TestLogContentRetentionUsesConfiguredDays(t *testing.T) {
	now := int64(10_000_000)
	assert.Equal(t, now-604800, logContentRetentionCutoff(now, 7))
	assert.Equal(t, now-30*86400, logContentRetentionCutoff(now, 0))
	assert.Equal(t, now-30*86400, logContentRetentionCutoff(now, -1))

	setupLogContentRetention(t)
	common.LogContentRetentionDays = 7
	nowTs := common.GetTimestamp()
	seedRetentionContents(t, nowTs-8*86400, 1, "old")
	seedRetentionContents(t, nowTs-6*86400, 1, "recent")
	task := claimRetentionTask(t)
	logContentRetentionHandler{}.Run(context.Background(), task, retentionTestRunner)
	assert.Equal(t, int64(1), countRetentionContents(t))
}

func TestLogContentRetentionPausesBetweenBatches(t *testing.T) {
	setupLogContentRetention(t)
	seedRetentionContents(t, 1, 450, "old")

	pauses := 0
	h := logContentRetentionHandler{pause: func(context.Context) error { pauses++; return nil }}
	task := claimRetentionTask(t)
	h.Run(context.Background(), task, retentionTestRunner)

	assert.Equal(t, 2, pauses)
	assert.Equal(t, int64(0), countRetentionContents(t))
	assert.Equal(t, model.SystemTaskStatusSucceeded, finishedRetentionTask(t, task).Status)
}

func TestLogContentRetentionHonorsCancel(t *testing.T) {
	setupLogContentRetention(t)
	seedRetentionContents(t, 1, 450, "old")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := logContentRetentionHandler{pause: func(context.Context) error {
		cancel()
		return nil
	}}
	task := claimRetentionTask(t)
	h.Run(ctx, task, retentionTestRunner)

	assert.Equal(t, int64(250), countRetentionContents(t))
	got := finishedRetentionTask(t, task)
	assert.Equal(t, model.SystemTaskStatusFailed, got.Status)
}

func TestLogContentRetentionRunsWithCaptureOff(t *testing.T) {
	setupLogContentRetention(t)
	common.LogContentCaptureEnabled = false
	seedRetentionContents(t, 1, 3, "old")

	h := logContentRetentionHandler{}
	assert.True(t, h.Enabled())
	task := claimRetentionTask(t)
	h.Run(context.Background(), task, retentionTestRunner)
	assert.Equal(t, int64(0), countRetentionContents(t))
}

func TestLogContentRetentionIsScheduled(t *testing.T) {
	systemTaskHandlersMu.RLock()
	registered, ok := systemTaskHandlers[SystemTaskTypeLogContentRetention]
	systemTaskHandlersMu.RUnlock()
	require.True(t, ok)
	scheduled, ok := registered.(ScheduledSystemTaskHandler)
	require.True(t, ok)
	assert.Equal(t, 24*time.Hour, scheduled.Interval())
}

func TestLogContentRetentionClickHouseUsesTTL(t *testing.T) {
	setupLogContentRetention(t)
	common.LogContentRetentionDays = 12
	seedRetentionContents(t, 1, 3, "old")
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)

	var got []int
	h := logContentRetentionHandler{syncTTL: func(days int) error { got = append(got, days); return nil }}
	task := claimRetentionTask(t)
	h.Run(context.Background(), task, retentionTestRunner)

	assert.Equal(t, []int{12}, got)
	assert.Equal(t, int64(3), countRetentionContents(t))
	assert.Equal(t, model.SystemTaskStatusSucceeded, finishedRetentionTask(t, task).Status)

	// A failing TTL change fails the task.
	failing := logContentRetentionHandler{syncTTL: func(int) error { return errors.New("boom") }}
	task = claimRetentionTask(t)
	failing.Run(context.Background(), task, retentionTestRunner)
	assert.Equal(t, model.SystemTaskStatusFailed, finishedRetentionTask(t, task).Status)
}

func TestLogContentRetentionLeavesLogsAlone(t *testing.T) {
	setupLogContentRetention(t)
	seedRetentionContents(t, 1, 2, "old")
	for i := 0; i < 3; i++ {
		require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, CreatedAt: 1, Type: model.LogTypeConsume, Content: "old log"}).Error)
	}
	task := claimRetentionTask(t)
	logContentRetentionHandler{}.Run(context.Background(), task, retentionTestRunner)

	var logs int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Count(&logs).Error)
	assert.Equal(t, int64(3), logs)
	assert.Equal(t, int64(0), countRetentionContents(t))
}
