package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDashboardServiceTestDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousLogConsumeEnabled := common.LogConsumeEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.QuotaData{}, &model.Log{}, &model.Channel{}, &model.Token{}, &model.User{}))
	model.DB = db
	model.LOG_DB = db
	common.LogConsumeEnabled = true
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.LogConsumeEnabled = previousLogConsumeEnabled
		require.NoError(t, sqlDB.Close())
	})
}

func TestGetDashboardSummaryScopesRegularUserData(t *testing.T) {
	setupDashboardServiceTestDB(t)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.QuotaData{UserID: 1, Username: "alice", ModelName: "gpt-4o", CreatedAt: now - 3600, Count: 2, Quota: 30, TokenUsed: 90}).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{UserID: 2, Username: "bob", ModelName: "claude", CreatedAt: now - 3600, Count: 5, Quota: 500, TokenUsed: 1000}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 1, CreatedAt: now, Type: model.LogTypeConsume, Content: "user request", ChannelId: 7, ChannelName: "admin-channel"}).Error)

	summary, err := GetDashboardSummary(1, "alice", common.RoleCommonUser, 100)

	require.NoError(t, err)
	assert.Equal(t, dto.DashboardRoleUser, summary.Role)
	assert.NotEmpty(t, summary.Metrics)
	assert.Empty(t, summary.TopUsers)
	assert.NotContains(t, dashboardMetricValues(summary.Metrics), "500")
	require.Len(t, summary.RecentActivity, 1)
	assert.Zero(t, summary.RecentActivity[0].ChannelID)
	assert.Empty(t, summary.RecentActivity[0].ChannelName)
}

func TestGetDashboardSummaryAdminIncludesOperationalData(t *testing.T) {
	setupDashboardServiceTestDB(t)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.User{Id: 1, Username: "alice", DisplayName: "Alice Ops", AffCode: "alice-aff"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 2, Username: "bob", DisplayName: "Bob Ops", AffCode: "bob-aff"}).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{UserID: 1, Username: "alice", ModelName: "gpt-4o", CreatedAt: now - 3600, Count: 3, Quota: 100, TokenUsed: 200}).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{UserID: 2, Username: "bob", ModelName: "claude", CreatedAt: now - 3600, Count: 5, Quota: 300, TokenUsed: 600}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Name: "slow", Status: common.ChannelStatusEnabled, ResponseTime: 6000, UsedQuota: 100}).Error)

	summary, err := GetDashboardSummary(99, "admin", common.RoleAdminUser, 0)

	require.NoError(t, err)
	assert.Equal(t, dto.DashboardRoleAdmin, summary.Role)
	assert.NotEmpty(t, summary.Channels)
	require.NotEmpty(t, summary.TopUsers)
	assert.Equal(t, "2", summary.TopUsers[0].ID)
	assert.Equal(t, "bob", summary.TopUsers[0].Name)
	assert.Equal(t, "Bob Ops", summary.TopUsers[0].DisplayName)
	assert.Equal(t, 300, summary.TopUsers[0].Quota)
	assert.Equal(t, 5, summary.TopUsers[0].Requests)
	assert.Equal(t, 600, summary.TopUsers[0].Tokens)
}

func TestGetDashboardInsightsReturnsDeterministicDefaults(t *testing.T) {
	setupDashboardServiceTestDB(t)

	insights, err := GetDashboardInsights(1, "alice", common.RoleCommonUser, 0)

	require.NoError(t, err)
	assert.Contains(t, collectDashboardInsightIDs(insights), "user-balance-depleted")
}

func dashboardMetricValues(metrics []dto.DashboardMetric) []string {
	values := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		values = append(values, metric.Value)
	}
	return values
}
