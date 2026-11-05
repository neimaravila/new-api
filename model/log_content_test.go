package model

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	sqlmysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// useLogContentDB points LOG_DB at db and the log database type at dbType
// for the duration of the test.
func useLogContentDB(t *testing.T, db *gorm.DB, dbType common.DatabaseType) {
	t.Helper()
	previousDB, previousType := LOG_DB, common.LogDatabaseType()
	LOG_DB = db
	common.SetLogDatabaseType(dbType)
	t.Cleanup(func() {
		LOG_DB = previousDB
		common.SetLogDatabaseType(previousType)
	})
}

func newLogContentSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/log_content.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// newLogContentIsolatedDB creates a throwaway database on a loopback server
// and returns a connection to it.
func newLogContentIsolatedDB(t *testing.T, kind, dsn string) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("newapi_logcontent_%d", time.Now().UnixNano())
	var original, isolated gorm.Dialector
	if kind == "mysql" {
		config, err := sqlmysql.ParseDSN(dsn)
		require.NoError(t, err)
		require.Equal(t, "tcp", config.Net)
		host, _, err := net.SplitHostPort(config.Addr)
		require.NoError(t, err)
		require.True(t, net.ParseIP(host).IsLoopback(), "database tests only permit loopback instances")
		original = mysql.Open(dsn)
		config.DBName = name
		isolated = mysql.Open(config.FormatDSN())
	} else {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		require.True(t, net.ParseIP(parsed.Hostname()).IsLoopback(), "database tests only permit loopback instances")
		original = postgres.Open(dsn)
		if kind == "clickhouse" {
			original = clickhouse.Open(dsn)
		}
		parsed.Path = "/" + name
		if kind == "clickhouse" {
			isolated = clickhouse.Open(parsed.String())
		} else {
			isolated = postgres.Open(parsed.String())
		}
	}
	admin, err := gorm.Open(original, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, admin.Exec("CREATE DATABASE "+name).Error)
	adminConn, err := admin.DB()
	require.NoError(t, err)
	require.NoError(t, adminConn.Close())
	db, err := gorm.Open(isolated, &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if conn, err := db.DB(); err == nil {
			_ = conn.Close()
		}
	})
	return db
}

func binaryLogContentBody(size int) []byte {
	body := bytes.Repeat([]byte{0x00, 0xFF, 'a', 0x7F}, size/4)
	return body[:size]
}

// runLogContentStorageSuite exercises the storage operations on the empty
// database selected through useLogContentDB; Task 12 reuses it on every
// supported log database.
func runLogContentStorageSuite(t *testing.T, dbType common.DatabaseType) {
	ctx := context.Background()
	require.NoError(t, MigrateLogContents())
	require.NoError(t, MigrateLogContents(), "migration must be idempotent")

	body := binaryLogContentBody(2 << 20)
	metaJSON, err := NewLogContentMeta(LogContentMeta{
		RelayFormat:   "openai",
		HttpStatus:    502,
		MultiKeyIndex: -1,
		ErrorMessage:  "a\x00b\xff",
		Request:       BodyMeta{OriginalBytes: int64(len(body)), Truncated: true},
	})
	require.NoError(t, err)
	row := &LogContent{
		RequestId: "req-binary", Attempt: 0, UserId: 7, CreatedAt: 1000,
		CaptureStatus: "captured", Meta: metaJSON,
		RequestBody: body, ResponseBody: []byte{0x00, 0xFF}, AssembledBody: nil,
	}
	require.NoError(t, InsertLogContents(ctx, []*LogContent{row}))

	stored, err := GetLogContents(ctx, "req-binary", 7)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.True(t, bytes.Equal(body, stored[0].RequestBody), "binary body must round trip")
	assert.Equal(t, []byte{0x00, 0xFF}, stored[0].ResponseBody)
	assert.Equal(t, metaJSON, stored[0].Meta)
	var meta LogContentMeta
	require.NoError(t, common.UnmarshalJsonStr(stored[0].Meta, &meta))
	assert.Equal(t, "a\x00b", meta.ErrorMessage[:3])
	assert.Equal(t, 502, meta.HttpStatus)
	assert.True(t, meta.Request.Truncated)

	other, err := GetLogContents(ctx, "req-binary", 8)
	require.NoError(t, err)
	assert.Empty(t, other, "another user must not read the content")
	all, err := GetLogContents(ctx, "req-binary", 0)
	require.NoError(t, err)
	assert.Len(t, all, 1, "user 0 means every user")
}

func TestLogContentStorageOnDatabases(t *testing.T) {
	for _, tc := range []struct {
		kind, env string
		typ       common.DatabaseType
	}{
		{"sqlite", "", common.DatabaseTypeSQLite},
		{"mysql", "AUDIT_MYSQL_DSN", common.DatabaseTypeMySQL},
		{"postgres", "AUDIT_POSTGRES_DSN", common.DatabaseTypePostgreSQL},
		{"clickhouse", "AUDIT_CLICKHOUSE_DSN", common.DatabaseTypeClickHouse},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			var db *gorm.DB
			if tc.env == "" {
				db = newLogContentSQLite(t)
			} else {
				dsn := os.Getenv(tc.env)
				if dsn == "" {
					t.Skip(tc.env + " is not configured")
				}
				db = newLogContentIsolatedDB(t, tc.kind, dsn)
			}
			useLogContentDB(t, db, tc.typ)
			runLogContentStorageSuite(t, tc.typ)
		})
	}
}

func newLogContentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newLogContentSQLite(t)
	useLogContentDB(t, db, common.DatabaseTypeSQLite)
	require.NoError(t, MigrateLogContents())
	return db
}

func TestInsertLogContentsIgnoresDuplicateKey(t *testing.T) {
	newLogContentTestDB(t)
	ctx := context.Background()
	first := &LogContent{RequestId: "dup", Attempt: 1, UserId: 1, CreatedAt: 10, CaptureStatus: "captured", RequestBody: []byte("first")}
	second := &LogContent{RequestId: "dup", Attempt: 1, UserId: 1, CreatedAt: 11, CaptureStatus: "captured", RequestBody: []byte("second")}
	require.NoError(t, InsertLogContents(ctx, []*LogContent{first}))
	require.NoError(t, InsertLogContents(ctx, []*LogContent{second}))
	rows, err := GetLogContents(ctx, "dup", 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, []byte("first"), rows[0].RequestBody)
	require.NoError(t, InsertLogContents(ctx, nil))
}

func TestGetLogContentsOrdersByAttemptAndLimits(t *testing.T) {
	newLogContentTestDB(t)
	ctx := context.Background()
	var rows []*LogContent
	for attempt := 40; attempt >= 0; attempt-- {
		rows = append(rows, &LogContent{RequestId: "many", Attempt: attempt, UserId: 1, CreatedAt: 10, CaptureStatus: "captured"})
	}
	require.NoError(t, InsertLogContents(ctx, rows))
	got, err := GetLogContents(ctx, "many", 1)
	require.NoError(t, err)
	require.Len(t, got, 32)
	for i, row := range got {
		assert.Equal(t, i, row.Attempt)
	}
}

func TestDeleteOldLogContentBatchRemovesOldestFirst(t *testing.T) {
	newLogContentTestDB(t)
	ctx := context.Background()
	cutoff := int64(1000)
	var rows []*LogContent
	for i, at := range []int64{cutoff + 1, cutoff - 1, cutoff, cutoff - 2} {
		rows = append(rows, &LogContent{RequestId: fmt.Sprintf("r%d", i), UserId: 1, CreatedAt: at, CaptureStatus: "captured"})
	}
	require.NoError(t, InsertLogContents(ctx, rows))
	remaining := func() []int64 {
		var times []int64
		require.NoError(t, LOG_DB.Model(&LogContent{}).Order("created_at").Pluck("created_at", &times).Error)
		return times
	}

	deleted, err := DeleteOldLogContentBatch(ctx, cutoff, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
	assert.Equal(t, []int64{cutoff - 1, cutoff, cutoff + 1}, remaining())

	deleted, err = DeleteOldLogContentBatch(ctx, cutoff, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
	assert.Equal(t, []int64{cutoff, cutoff + 1}, remaining())

	deleted, err = DeleteOldLogContentBatch(ctx, cutoff, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 0, deleted)
	assert.Equal(t, []int64{cutoff, cutoff + 1}, remaining())
}

func TestDeleteOldLogContentBatchIsNoOpOnClickHouse(t *testing.T) {
	previous := common.LogDatabaseType()
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	t.Cleanup(func() { common.SetLogDatabaseType(previous) })
	deleted, err := DeleteOldLogContentBatch(context.Background(), 1000, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 0, deleted)
}

func TestGetLogRowsForContentFiltersUserAndType(t *testing.T) {
	db := newLogContentTestDB(t)
	require.NoError(t, db.AutoMigrate(&Log{}))
	logs := []Log{
		{UserId: 1, Type: LogTypeConsume, RequestId: "shared", CreatedAt: 1},
		{UserId: 1, Type: LogTypeError, RequestId: "shared", CreatedAt: 2},
		{UserId: 2, Type: LogTypeConsume, RequestId: "shared", CreatedAt: 3},
		{UserId: 1, Type: LogTypeTopup, RequestId: "shared", CreatedAt: 4},
		{UserId: 1, Type: LogTypeConsume, RequestId: "elsewhere", CreatedAt: 5},
	}
	require.NoError(t, db.Create(&logs).Error)
	ctx := context.Background()

	own, err := GetLogRowsForContent(ctx, "shared", 1)
	require.NoError(t, err)
	require.Len(t, own, 2)
	for _, row := range own {
		assert.Equal(t, 1, row.UserId)
		assert.Contains(t, []int{LogTypeConsume, LogTypeError}, row.Type)
	}
	every, err := GetLogRowsForContent(ctx, "shared", 0)
	require.NoError(t, err)
	assert.Len(t, every, 3)
	for _, row := range every {
		assert.NotEqual(t, LogTypeTopup, row.Type)
	}
	none, err := GetLogRowsForContent(ctx, "missing", 0)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestLogContentCreateTableClickHouseSQL(t *testing.T) {
	sql := LogContentCreateTableClickHouseSQL(30)
	assert.Contains(t, sql, "ORDER BY (created_at, request_id, attempt)")
	assert.Contains(t, sql, "bloom_filter")
	assert.Contains(t, sql, "INTERVAL 30 DAY")
	assert.Contains(t, LogContentCreateTableClickHouseSQL(0), "INTERVAL 30 DAY", "invalid retention falls back to 30 days")
}

func TestNewLogContentMetaSanitizesText(t *testing.T) {
	meta, err := NewLogContentMeta(LogContentMeta{ErrorMessage: "a\x00b\xff", Path: "/v1/\xfe"})
	require.NoError(t, err)
	assert.Contains(t, meta, `a\u0000b`)
	assert.NotContains(t, meta, "\x00")
	var parsed LogContentMeta
	require.NoError(t, common.UnmarshalJsonStr(meta, &parsed))
	assert.Equal(t, "a\x00b", parsed.ErrorMessage[:3])
}
