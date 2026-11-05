package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func putLogContentRetention(t *testing.T, value string) bool {
	t.Helper()
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"LogContentRetentionDays","value":"`+value+`"}`),
	)
	UpdateOption(context)
	var payload struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	return payload.Success
}

func TestUpdateOptionRejectsInvalidRetention(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	previous := common.LogContentRetentionDays
	previousMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.LogContentRetentionDays = previous
		common.OptionMap = previousMap
	})
	common.LogContentRetentionDays = 30

	for _, value := range []string{"0", "-1", "3651", "abc"} {
		assert.False(t, putLogContentRetention(t, value), value)
		assert.Equal(t, 30, common.LogContentRetentionDays, value)
	}
	assert.True(t, putLogContentRetention(t, "1"))
	assert.Equal(t, 1, common.LogContentRetentionDays)
	assert.True(t, putLogContentRetention(t, "3650"))
	assert.Equal(t, 3650, common.LogContentRetentionDays)
}

func TestUpdateRetentionSyncsClickHouseTTL(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	previous, previousType := common.LogContentRetentionDays, common.LogDatabaseType()
	previousMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.LogContentRetentionDays = previous
		common.OptionMap = previousMap
		common.SetLogDatabaseType(previousType)
	})

	var mu sync.Mutex
	var statements []string
	const name = "test:record_ttl"
	require.NoError(t, model.LOG_DB.Callback().Raw().Before("gorm:raw").Register(name, func(tx *gorm.DB) {
		if sql := tx.Statement.SQL.String(); strings.Contains(sql, "MODIFY TTL") {
			mu.Lock()
			statements = append(statements, sql)
			mu.Unlock()
			tx.Statement.SQL.Reset()
			tx.Statement.SQL.WriteString("SELECT 1")
		}
	}))
	t.Cleanup(func() { _ = model.LOG_DB.Callback().Raw().Remove(name) })
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)

	require.True(t, putLogContentRetention(t, "45"))
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(statements) == 1
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, "ALTER TABLE log_contents MODIFY TTL toDateTime(created_at) + INTERVAL 45 DAY DELETE", statements[0])

	// A rejected value changes nothing.
	assert.False(t, putLogContentRetention(t, "0"))
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, statements, 1)
}
