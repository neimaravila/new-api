package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	// SystemTaskTypeLogContentRetention is the type of the daily task that
	// expires captured call content.
	SystemTaskTypeLogContentRetention = "log_content_retention"

	logContentRetentionInterval = 24 * time.Hour
	// logContentRetentionBatchSize stays small: PostgreSQL caps bind parameters
	// and the batch delete binds one per id.
	logContentRetentionBatchSize = 200
	logContentRetentionPause     = time.Second
	logContentDefaultDays        = 30
)

// LogContentRetentionResult is the result stored when the task succeeds.
type LogContentRetentionResult struct {
	DeletedCount int64 `json:"deleted_count"`
	Cutoff       int64 `json:"cutoff"`
	Days         int   `json:"days"`
}

// logContentRetentionHandler deletes captured content older than the
// configured retention. The zero value uses production behaviour; the fields
// exist so tests can inject the clock, a pause and the ClickHouse TTL call.
type logContentRetentionHandler struct {
	now     func() int64
	pause   func(ctx context.Context) error
	syncTTL func(days int) error
}

func init() {
	RegisterSystemTaskHandler(logContentRetentionHandler{})
}

func (logContentRetentionHandler) Type() string { return SystemTaskTypeLogContentRetention }

// Enabled is always true: expiry must run even when capture is switched off,
// so content captured earlier still goes away.
func (logContentRetentionHandler) Enabled() bool { return true }

func (logContentRetentionHandler) Interval() time.Duration { return logContentRetentionInterval }

func (logContentRetentionHandler) NewPayload() any { return nil }

// logContentRetentionCutoff returns the timestamp before which content expires.
// A non-positive days falls back to the default retention.
func logContentRetentionCutoff(now int64, days int) int64 {
	if days < 1 {
		days = logContentDefaultDays
	}
	return now - int64(days)*86400
}

func (h logContentRetentionHandler) pauseBetweenBatches(ctx context.Context) error {
	if h.pause != nil {
		return h.pause(ctx)
	}
	timer := time.NewTimer(logContentRetentionPause)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (h logContentRetentionHandler) syncClickHouseTTL(days int) error {
	if h.syncTTL != nil {
		return h.syncTTL(days)
	}
	return model.SyncLogContentClickHouseTTL(days)
}

func (h logContentRetentionHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	days := common.LogContentRetentionDays
	if days < 1 {
		days = logContentDefaultDays
	}
	now := common.GetTimestamp()
	if h.now != nil {
		now = h.now()
	}
	cutoff := logContentRetentionCutoff(now, days)
	result := LogContentRetentionResult{Cutoff: cutoff, Days: days}
	report := NewSystemTaskProgressReporter(task, runnerID)

	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		if err := h.syncClickHouseTTL(days); err != nil {
			failSystemTask(task, runnerID, err)
			return
		}
		report(1, 1)
		h.finish(ctx, task, runnerID, result)
		return
	}

	for {
		if err := ctx.Err(); err != nil {
			failSystemTask(task, runnerID, fmt.Errorf("log content retention canceled: %w", err))
			return
		}
		deleted, err := model.DeleteOldLogContentBatch(ctx, cutoff, logContentRetentionBatchSize)
		if err != nil {
			failSystemTask(task, runnerID, err)
			return
		}
		result.DeletedCount += deleted
		if deleted < logContentRetentionBatchSize {
			break
		}
		if err := h.pauseBetweenBatches(ctx); err != nil {
			failSystemTask(task, runnerID, fmt.Errorf("log content retention canceled: %w", err))
			return
		}
	}
	report(1, 1)
	h.finish(ctx, task, runnerID, result)
}

func (logContentRetentionHandler) finish(ctx context.Context, task *model.SystemTask, runnerID string, result LogContentRetentionResult) {
	err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, "")
	if err != nil && !errors.Is(err, context.Canceled) {
		logSystemTaskLockError(ctx, task, err)
	}
}
