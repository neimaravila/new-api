package service

import (
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/dto"
)

const (
	dashboardLowRunwayDaysThreshold     = 3
	dashboardHighLatencyMsThreshold     = 5000
	dashboardHighErrorRatePermille      = 50
	dashboardUsageSpikeRatioPermille    = 1500
	dashboardConcentrationSharePermille = 700
	dashboardMaxInsights                = 5
)

type DashboardInsightInput struct {
	Role                  dto.DashboardRole
	RemainQuota           int
	RecentQuota           int
	PreviousQuota         int
	ActiveKeyCount        int
	UnusedKeyCount        int
	RecentFailureCount    int
	RecentRequestCount    int
	HighestLatencyChannel dto.DashboardChannelHealth
	TopUser               dto.DashboardTopEntity
	TopModel              dto.DashboardTopEntity
	TotalQuota            int
}

func BuildDashboardInsights(input DashboardInsightInput) []dto.DashboardInsight {
	insights := make([]dto.DashboardInsight, 0, dashboardMaxInsights)

	if input.Role == dto.DashboardRoleUser {
		insights = append(insights, buildUserDashboardInsights(input)...)
	} else {
		insights = append(insights, buildAdminDashboardInsights(input)...)
	}

	sort.SliceStable(insights, func(i, j int) bool {
		return insightSeverityRank(insights[i].Severity) > insightSeverityRank(insights[j].Severity)
	})

	if len(insights) > dashboardMaxInsights {
		return insights[:dashboardMaxInsights]
	}
	return insights
}

func buildUserDashboardInsights(input DashboardInsightInput) []dto.DashboardInsight {
	insights := make([]dto.DashboardInsight, 0, dashboardMaxInsights)

	if input.ActiveKeyCount == 0 {
		insights = append(insights, dto.DashboardInsight{
			ID:          "user-no-api-key",
			Severity:    dto.DashboardInsightWarning,
			Title:       dashboardMessage("Create your first API key"),
			Description: dashboardMessage("Create an API key before sending production traffic through the gateway."),
			Action:      &dto.DashboardInsightAction{Label: dashboardMessage("Create API Key"), Path: "/keys"},
		})
	} else if input.UnusedKeyCount > 0 {
		insights = append(insights, dto.DashboardInsight{
			ID:          "user-unused-api-key",
			Severity:    dto.DashboardInsightInfo,
			Title:       dashboardMessage("One API key has no recent traffic"),
			Description: dashboardMessage("Send a test request or remove unused keys to keep your integration tidy."),
			MetricLabel: dashboardMessagePtr("Unused keys"),
			MetricValue: fmt.Sprintf("%d", input.UnusedKeyCount),
			Action:      &dto.DashboardInsightAction{Label: dashboardMessage("Review API Keys"), Path: "/keys"},
		})
	}

	if input.RecentQuota > 0 && input.RemainQuota > 0 {
		runwayDays := input.RemainQuota / input.RecentQuota
		if runwayDays < dashboardLowRunwayDaysThreshold {
			insights = append(insights, dto.DashboardInsight{
				ID:                 "user-low-runway",
				Severity:           dto.DashboardInsightWarning,
				Title:              dashboardMessage("Balance may run out soon"),
				Description:        dashboardMessage("Your recent usage suggests the current balance may not last three days."),
				MetricLabel:        dashboardMessagePtr("Runway"),
				MetricValueMessage: dashboardMessagePtr("{{days}} days", "days", runwayDays),
				Action:             &dto.DashboardInsightAction{Label: dashboardMessage("Open Wallet"), Path: "/wallet"},
			})
		}
	} else if input.RemainQuota <= 0 {
		insights = append(insights, dto.DashboardInsight{
			ID:          "user-balance-depleted",
			Severity:    dto.DashboardInsightCritical,
			Title:       dashboardMessage("Balance depleted"),
			Description: dashboardMessage("Add credits before sending more paid requests."),
			Action:      &dto.DashboardInsightAction{Label: dashboardMessage("Open Wallet"), Path: "/wallet"},
		})
	}

	if input.RecentFailureCount > 0 {
		insights = append(insights, dto.DashboardInsight{
			ID:          "user-recent-failures",
			Severity:    dto.DashboardInsightWarning,
			Title:       dashboardMessage("Recent requests failed"),
			Description: dashboardMessage("Review failed requests to fix model, key, or request payload issues."),
			MetricLabel: dashboardMessagePtr("Failures"),
			MetricValue: fmt.Sprintf("%d", input.RecentFailureCount),
			Action:      &dto.DashboardInsightAction{Label: dashboardMessage("View Logs"), Path: "/usage-logs"},
		})
	}

	if isUsageSpike(input.RecentQuota, input.PreviousQuota) {
		insights = append(insights, dto.DashboardInsight{
			ID:          "user-usage-spike",
			Severity:    dto.DashboardInsightInfo,
			Title:       dashboardMessage("Usage increased versus the previous period"),
			Description: dashboardMessage("Check model usage and recent logs to confirm the increase is expected."),
			Action:      &dto.DashboardInsightAction{Label: dashboardMessage("View Usage"), Path: "/usage-logs"},
		})
	}

	if hasConcentration(input.TopModel.Quota, input.TotalQuota) {
		insights = append(insights, dto.DashboardInsight{
			ID:          "user-model-cost-concentration",
			Severity:    dto.DashboardInsightInfo,
			Title:       dashboardMessage("One model dominates recent cost"),
			Description: dashboardMessage("Consider using a cheaper model for test traffic if quality requirements allow it."),
			MetricLabel: dashboardMessagePtr("Top model"),
			MetricValue: input.TopModel.Name,
			Action:      &dto.DashboardInsightAction{Label: dashboardMessage("Compare Pricing"), Path: "/pricing"},
			EntityType:  "model",
			EntityID:    input.TopModel.ID,
		})
	}

	return insights
}

func buildAdminDashboardInsights(input DashboardInsightInput) []dto.DashboardInsight {
	insights := make([]dto.DashboardInsight, 0, dashboardMaxInsights)

	if input.HighestLatencyChannel.ID > 0 && input.HighestLatencyChannel.ResponseTime >= dashboardHighLatencyMsThreshold {
		insights = append(insights, dto.DashboardInsight{
			ID:                 "admin-high-channel-latency",
			Severity:           dto.DashboardInsightWarning,
			Title:              dashboardMessage("A channel has high latency"),
			Description:        dashboardMessage("Investigate upstream availability or routing priority for this provider."),
			MetricLabel:        dashboardMessagePtr("Latency"),
			MetricValueMessage: dashboardMessagePtr("{{latency}} ms", "latency", input.HighestLatencyChannel.ResponseTime),
			Action:             &dto.DashboardInsightAction{Label: dashboardMessage("Review Channels"), Path: "/channels"},
			EntityType:         "channel",
			EntityID:           fmt.Sprintf("%d", input.HighestLatencyChannel.ID),
		})
	}

	if input.RecentRequestCount > 0 && input.RecentFailureCount*1000/input.RecentRequestCount >= dashboardHighErrorRatePermille {
		insights = append(insights, dto.DashboardInsight{
			ID:          "admin-high-error-rate",
			Severity:    dto.DashboardInsightCritical,
			Title:       dashboardMessage("Error rate is elevated"),
			Description: dashboardMessage("Recent failed requests crossed the operational warning threshold."),
			MetricLabel: dashboardMessagePtr("Failures"),
			MetricValue: fmt.Sprintf("%d", input.RecentFailureCount),
			Action:      &dto.DashboardInsightAction{Label: dashboardMessage("View Logs"), Path: "/usage-logs"},
		})
	}

	if isUsageSpike(input.RecentQuota, input.PreviousQuota) {
		insights = append(insights, dto.DashboardInsight{
			ID:          "admin-usage-spike",
			Severity:    dto.DashboardInsightInfo,
			Title:       dashboardMessage("Usage is up versus the previous period"),
			Description: dashboardMessage("Review top users and models to confirm the increase is expected."),
			Action:      &dto.DashboardInsightAction{Label: dashboardMessage("Open Analytics"), Path: "/dashboard/models"},
		})
	}

	if hasConcentration(input.TopUser.Quota, input.TotalQuota) {
		insights = append(insights, dto.DashboardInsight{
			ID:          "admin-user-cost-concentration",
			Severity:    dto.DashboardInsightWarning,
			Title:       dashboardMessage("Spend is concentrated in one user"),
			Description: dashboardMessage("A single user accounts for most recent quota consumption."),
			MetricLabel: dashboardMessagePtr("Top user"),
			MetricValue: input.TopUser.Name,
			Action:      &dto.DashboardInsightAction{Label: dashboardMessage("Review Users"), Path: "/users"},
			EntityType:  "user",
			EntityID:    input.TopUser.ID,
		})
	}

	if hasConcentration(input.TopModel.Quota, input.TotalQuota) {
		insights = append(insights, dto.DashboardInsight{
			ID:          "admin-model-cost-concentration",
			Severity:    dto.DashboardInsightInfo,
			Title:       dashboardMessage("One model dominates recent spend"),
			Description: dashboardMessage("Review pricing and routing rules for the most expensive model traffic."),
			MetricLabel: dashboardMessagePtr("Top model"),
			MetricValue: input.TopModel.Name,
			Action:      &dto.DashboardInsightAction{Label: dashboardMessage("View Models"), Path: "/dashboard/models"},
			EntityType:  "model",
			EntityID:    input.TopModel.ID,
		})
	}

	return insights
}

func dashboardMessagePtr(key string, valuePairs ...any) *dto.DashboardMessage {
	message := dashboardMessage(key, valuePairs...)
	return &message
}

func insightSeverityRank(severity dto.DashboardInsightSeverity) int {
	switch severity {
	case dto.DashboardInsightCritical:
		return 3
	case dto.DashboardInsightWarning:
		return 2
	default:
		return 1
	}
}

func isUsageSpike(recentQuota int, previousQuota int) bool {
	if previousQuota <= 0 || recentQuota <= previousQuota {
		return false
	}
	return recentQuota*1000/previousQuota >= dashboardUsageSpikeRatioPermille
}

func hasConcentration(part int, total int) bool {
	if part <= 0 || total <= 0 {
		return false
	}
	return part*1000/total >= dashboardConcentrationSharePermille
}
