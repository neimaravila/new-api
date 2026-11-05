package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm/clause"
)

const (
	LogContentStatusCaptured    = "captured"
	LogContentStatusNotCaptured = "not_captured"
	// LogContentStatusNoAttempt marks a logged call that failed before any
	// provider attempt; the view reports it as unavailable.
	LogContentStatusNoAttempt = "no_attempt"

	defaultLogContentRetentionDays = 30
	maxLogContentsPerRequest       = 32
)

// LogContent is the captured content of one attempt of one call. Attempt 0 is
// the client level; attempts >= 1 are the calls made to providers. The bodies
// are gzip bytes, kept binary so arbitrary content never fails a batch.
type LogContent struct {
	Id            int64  `json:"id" gorm:"primaryKey"`
	RequestId     string `json:"request_id" gorm:"type:varchar(64);not null;uniqueIndex:idx_log_contents_request_attempt,priority:1"`
	Attempt       int    `json:"attempt" gorm:"not null;uniqueIndex:idx_log_contents_request_attempt,priority:2"`
	UserId        int    `json:"user_id" gorm:"not null"`
	CreatedAt     int64  `json:"created_at" gorm:"type:bigint;not null;autoCreateTime:false;index:idx_log_contents_created_at"`
	CaptureStatus string `json:"capture_status" gorm:"type:varchar(16);not null"`
	Meta          string `json:"meta" gorm:"type:text"`
	RequestBody   []byte `json:"-" gorm:"size:16777215"`
	ResponseBody  []byte `json:"-" gorm:"size:16777215"`
	AssembledBody []byte `json:"-" gorm:"size:16777215"`
}

func (LogContent) TableName() string {
	return "log_contents"
}

// BodyMeta describes how a stored body relates to the original one.
type BodyMeta struct {
	OriginalBytes int64 `json:"original_bytes"`
	Truncated     bool  `json:"truncated"`
	HeadOnly      bool  `json:"head_only"`
	Undecodable   bool  `json:"undecodable"`
	MediaOmitted  int   `json:"media_omitted"`
}

// LogContentMeta is the JSON stored in LogContent.Meta.
type LogContentMeta struct {
	RelayFormat        string   `json:"relay_format"`
	HttpStatus         int      `json:"http_status"`
	IsStream           bool     `json:"is_stream"`
	Incomplete         bool     `json:"incomplete"`
	ChannelId          int      `json:"channel_id"`
	MultiKeyIndex      int      `json:"multi_key_index"`
	Method             string   `json:"method"`
	Path               string   `json:"path"`
	StartedAtMs        int64    `json:"started_at_ms"`
	DurationMs         int64    `json:"duration_ms"`
	ErrorMessage       string   `json:"error_message"`
	RequestUnavailable bool     `json:"request_unavailable"`
	Request            BodyMeta `json:"request"`
	Response           BodyMeta `json:"response"`
}

// NewLogContentMeta encodes meta for the text column: invalid UTF-8 is
// dropped, and NUL leaves as \u0000 through JSON escaping.
func NewLogContentMeta(meta LogContentMeta) (string, error) {
	meta.RelayFormat = strings.ToValidUTF8(meta.RelayFormat, "")
	meta.Method = strings.ToValidUTF8(meta.Method, "")
	meta.Path = strings.ToValidUTF8(meta.Path, "")
	meta.ErrorMessage = strings.ToValidUTF8(meta.ErrorMessage, "")
	data, err := common.Marshal(meta)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// MigrateLogContents creates the log_contents table in LOG_DB. A new
// ClickHouse table expires rows after the configured retention.
func MigrateLogContents() error {
	if !common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return LOG_DB.AutoMigrate(&LogContent{})
	}
	return LOG_DB.Exec(LogContentCreateTableClickHouseSQL(common.LogContentRetentionDays)).Error
}

func logContentClickHouseTTLExpression(days int) string {
	if days < 1 {
		days = defaultLogContentRetentionDays
	}
	return fmt.Sprintf("toDateTime(created_at) + INTERVAL %d DAY DELETE", days)
}

// LogContentCreateTableClickHouseSQL returns the ClickHouse DDL; days < 1
// means the default retention.
func LogContentCreateTableClickHouseSQL(days int) string {
	return `CREATE TABLE IF NOT EXISTS log_contents (
	id Int64 DEFAULT 0,
	request_id String,
	attempt Int32,
	user_id Int32,
	created_at Int64,
	capture_status String,
	meta String,
	request_body String,
	response_body String,
	assembled_body String,
	INDEX idx_request_id request_id TYPE bloom_filter GRANULARITY 1
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(toDateTime(created_at))
ORDER BY (created_at, request_id, attempt)
TTL ` + logContentClickHouseTTLExpression(days)
}

// SyncLogContentClickHouseTTL applies the retention to an existing ClickHouse
// table. It does nothing on other databases.
func SyncLogContentClickHouseTTL(days int) error {
	if !common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return nil
	}
	return LOG_DB.Exec("ALTER TABLE log_contents MODIFY TTL " + logContentClickHouseTTLExpression(days)).Error
}

// InsertLogContents stores rows; a repeated (request_id, attempt) is ignored.
func InsertLogContents(ctx context.Context, rows []*LogContent) error {
	if len(rows) == 0 {
		return nil
	}
	tx := LOG_DB.WithContext(ctx)
	if !common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		tx = tx.Clauses(clause.OnConflict{DoNothing: true})
	}
	return tx.Create(&rows).Error
}

// GetLogContents returns the attempts of one call ordered by attempt.
// userID 0 means every user.
func GetLogContents(ctx context.Context, requestID string, userID int) ([]*LogContent, error) {
	tx := LOG_DB.WithContext(ctx).Where("request_id = ?", requestID)
	if userID != 0 {
		tx = tx.Where("user_id = ?", userID)
	}
	var rows []*LogContent
	if err := tx.Order("attempt").Limit(maxLogContentsPerRequest).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetClientLogContent returns the client level (attempt 0) of one user's
// call, never the provider attempts.
func GetClientLogContent(ctx context.Context, requestID string, userID int) ([]*LogContent, error) {
	var rows []*LogContent
	err := LOG_DB.WithContext(ctx).
		Where("request_id = ? AND attempt = ? AND user_id = ?", requestID, 0, userID).
		Limit(1).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetLogRowsForContent returns the consume and error log rows of a call.
// userID 0 means every user.
func GetLogRowsForContent(ctx context.Context, requestID string, userID int) ([]*Log, error) {
	tx := LOG_DB.WithContext(ctx).
		Where("request_id = ? AND type IN ?", requestID, []int{LogTypeConsume, LogTypeError})
	if userID != 0 {
		tx = tx.Where("user_id = ?", userID)
	}
	var rows []*Log
	if err := tx.Order("created_at, id").Limit(maxLogContentsPerRequest).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteOldLogContentBatch deletes up to limit rows older than cutoff, oldest
// first, and returns how many. ClickHouse expires rows through its TTL.
func DeleteOldLogContentBatch(ctx context.Context, cutoff int64, limit int) (int64, error) {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) || limit < 1 {
		return 0, nil
	}
	var ids []int64
	err := LOG_DB.WithContext(ctx).Model(&LogContent{}).
		Where("created_at < ?", cutoff).Order("created_at").Limit(limit).
		Pluck("id", &ids).Error
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	result := LOG_DB.WithContext(ctx).Where("id IN ?", ids).Delete(&LogContent{})
	return result.RowsAffected, result.Error
}
