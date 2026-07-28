package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDashboardInsightsForUserLowBalanceAndFailures(t *testing.T) {
	insights := BuildDashboardInsights(DashboardInsightInput{
		Role:               dto.DashboardRoleUser,
		RemainQuota:        10,
		RecentQuota:        6,
		ActiveKeyCount:     1,
		RecentFailureCount: 2,
		RecentRequestCount: 10,
	})

	require.NotEmpty(t, insights)
	assert.Equal(t, dto.DashboardInsightWarning, insights[0].Severity)
	assert.Contains(t, collectDashboardInsightIDs(insights), "user-low-runway")
	assert.Contains(t, collectDashboardInsightIDs(insights), "user-recent-failures")
}

func TestBuildDashboardInsightsForUserNoApiKey(t *testing.T) {
	insights := BuildDashboardInsights(DashboardInsightInput{
		Role:           dto.DashboardRoleUser,
		RemainQuota:    100,
		RecentQuota:    1,
		ActiveKeyCount: 0,
	})

	require.NotEmpty(t, insights)
	assert.Equal(t, "user-no-api-key", insights[0].ID)
	assert.Equal(t, "/keys", insights[0].Action.Path)
}

func TestBuildDashboardInsightsForAdminHighErrorRateAndLatency(t *testing.T) {
	insights := BuildDashboardInsights(DashboardInsightInput{
		Role:               dto.DashboardRoleAdmin,
		RecentFailureCount: 6,
		RecentRequestCount: 100,
		HighestLatencyChannel: dto.DashboardChannelHealth{
			ID:           7,
			Name:         "slow-provider",
			ResponseTime: 6000,
		},
	})

	ids := collectDashboardInsightIDs(insights)
	assert.Contains(t, ids, "admin-high-error-rate")
	assert.Contains(t, ids, "admin-high-channel-latency")
	assert.Equal(t, dto.DashboardInsightCritical, insights[0].Severity)
}

func TestBuildDashboardInsightsLimitsAndSortsBySeverity(t *testing.T) {
	insights := BuildDashboardInsights(DashboardInsightInput{
		Role:               dto.DashboardRoleAdmin,
		RecentQuota:        200,
		PreviousQuota:      100,
		RecentFailureCount: 10,
		RecentRequestCount: 100,
		HighestLatencyChannel: dto.DashboardChannelHealth{
			ID:           1,
			Name:         "slow",
			ResponseTime: 9000,
		},
		TopUser:    dto.DashboardTopEntity{ID: "1", Name: "alice", Quota: 80},
		TopModel:   dto.DashboardTopEntity{ID: "gpt-4o", Name: "gpt-4o", Quota: 90},
		TotalQuota: 100,
	})

	require.LessOrEqual(t, len(insights), dashboardMaxInsights)
	assert.Equal(t, dto.DashboardInsightCritical, insights[0].Severity)
}

func collectDashboardInsightIDs(insights []dto.DashboardInsight) []string {
	ids := make([]string, 0, len(insights))
	for _, insight := range insights {
		ids = append(ids, insight.ID)
	}
	return ids
}
