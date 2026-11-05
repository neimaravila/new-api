package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestLogContentOptionsDefaultsAndUpdate(t *testing.T) {
	previousDB := DB
	previousMap := common.OptionMap
	previousEnabled := common.LogContentCaptureEnabled
	previousDays := common.LogContentRetentionDays
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/option.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&Option{}))
	DB = db
	t.Cleanup(func() {
		DB = previousDB
		common.OptionMap = previousMap
		common.LogContentCaptureEnabled = previousEnabled
		common.LogContentRetentionDays = previousDays
		_ = sqlDB.Close()
	})

	common.LogContentCaptureEnabled = true
	common.LogContentRetentionDays = 30
	InitOptionMap()
	common.OptionMapRWMutex.RLock()
	assert.Equal(t, "true", common.OptionMap["LogContentCaptureEnabled"])
	assert.Equal(t, "30", common.OptionMap["LogContentRetentionDays"])
	common.OptionMapRWMutex.RUnlock()

	require.NoError(t, UpdateOption("LogContentRetentionDays", "7"))
	assert.Equal(t, 7, common.LogContentRetentionDays)
	require.NoError(t, UpdateOption("LogContentCaptureEnabled", "false"))
	assert.False(t, common.LogContentCaptureEnabled)
}
