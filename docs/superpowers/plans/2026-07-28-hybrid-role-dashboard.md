# Hybrid Role Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a role-aware dashboard overview with backend summary/insight APIs, deterministic smart insights, and admin/user-specific frontend panels.

**Architecture:** Add dashboard-specific backend DTOs and services that aggregate existing quota, log, channel, token, and user data. Keep the existing `/dashboard` frontend route and section registry, then split the current overview into focused role-aware React components that consume the new APIs through TanStack Query.

**Tech Stack:** Go 1.22+, Gin, GORM v2, React 19, TypeScript, TanStack Query, TanStack Router, Tailwind CSS, i18next, Bun.

## Global Constraints

- Do not modify, remove, rename, or replace protected project/organization identifiers, including `new-api`, `QuantumNous`, package paths, branding, copyright headers, or metadata.
- All Go JSON marshal/unmarshal operations must use wrappers in `common/json.go`; do not call `encoding/json` marshal/unmarshal directly in business code.
- All database code must work with SQLite, MySQL >= 5.7.8, and PostgreSQL >= 9.6; prefer GORM query methods over raw SQL.
- Do not use database-specific SQL unless every supported database has a valid fallback.
- Backend tests must use `github.com/stretchr/testify/require` for setup/fatal assertions and `github.com/stretchr/testify/assert` for non-fatal assertions.
- Frontend package manager and script runner is Bun.
- All new user-facing frontend text must use `useTranslation()` and `t('English key')`.
- New frontend translation keys must be added to all supported locale files through the project i18n workflow.
- Keep the current `/dashboard` route and existing dashboard sections: overview, models, flow, and admin-only users.
- Do not add LLM-generated insights in this phase; insights must be deterministic rules.
- Do not expose raw API keys, channel keys, provider credentials, admin-only log metadata, or other secrets in dashboard responses.

---

## File Structure

### Backend files

- Create `dto/dashboard.go`
  - Public JSON DTOs for dashboard summary, metrics, insights, channel health, top entities, and recent activity.
- Create `service/dashboard.go`
  - Main summary and insights service entry points.
  - Role-aware orchestration and default time windows.
- Create `service/dashboard_insights.go`
  - Deterministic insight rules and thresholds.
- Create `service/dashboard_test.go`
  - Summary builder and scoping tests.
- Create `service/dashboard_insights_test.go`
  - Insight rule tests.
- Create `controller/dashboard.go`
  - Gin handlers for `/api/dashboard/summary` and `/api/dashboard/insights`.
- Modify `router/api-router.go`
  - Register authenticated dashboard routes.
- Modify `model/log.go`
  - Add small aggregate helpers only if the service cannot express the needed query cleanly with existing helpers.
- Modify `model/usedata.go`
  - Add grouped/top quota helpers only if needed.

### Frontend files

- Modify `web/src/features/dashboard/types.ts`
  - Add dashboard summary, metric, insight, channel health, top entity, and recent activity types.
- Modify `web/src/features/dashboard/api.ts`
  - Add `getDashboardSummary()` and `getDashboardInsights()`.
- Create `web/src/features/dashboard/components/overview/dashboard-hero.tsx`
  - Role-aware hero.
- Create `web/src/features/dashboard/components/overview/smart-insights-panel.tsx`
  - Insight list, severity states, action links, empty/error/loading states.
- Create `web/src/features/dashboard/components/overview/dashboard-metric-grid.tsx`
  - Reusable metric cards for admin and user.
- Create `web/src/features/dashboard/components/overview/admin-operations-panel.tsx`
  - Channel health, top entities, recent failures.
- Create `web/src/features/dashboard/components/overview/user-developer-panel.tsx`
  - API key readiness, request example, recent activity, model usage.
- Modify `web/src/features/dashboard/components/overview/overview-dashboard.tsx`
  - Fetch new APIs and compose the new role-aware overview above existing supporting panels.
- Add tests under `web/src/features/dashboard/components/overview/__tests__/`
  - `smart-insights-panel.test.tsx`
  - `overview-role-layout.test.tsx`
- Modify locale files under `web/src/i18n/locales/*.json`
  - Add translations for new UI text.

---

### Task 1: Backend Dashboard DTOs and Pure Insight Rules

**Files:**
- Create: `dto/dashboard.go`
- Create: `service/dashboard_insights.go`
- Create: `service/dashboard_insights_test.go`

**Interfaces:**
- Produces: `dto.DashboardInsight`, `dto.DashboardInsightSeverity`, `dto.DashboardInsightAction`, `dto.DashboardMetric`, `dto.DashboardSummary`, `service.BuildDashboardInsights(input service.DashboardInsightInput) []dto.DashboardInsight`.
- Consumes: no prior task output.

- [ ] **Step 1: Create backend DTOs**

Create `dto/dashboard.go` with the project copyright header and these types:

```go
package dto

type DashboardRole string

const (
	DashboardRoleAdmin DashboardRole = "admin"
	DashboardRoleUser  DashboardRole = "user"
)

type DashboardMetricTone string

const (
	DashboardMetricToneInfo        DashboardMetricTone = "info"
	DashboardMetricToneSuccess     DashboardMetricTone = "success"
	DashboardMetricToneWarning     DashboardMetricTone = "warning"
	DashboardMetricToneDestructive DashboardMetricTone = "destructive"
)

type DashboardInsightSeverity string

const (
	DashboardInsightInfo     DashboardInsightSeverity = "info"
	DashboardInsightWarning  DashboardInsightSeverity = "warning"
	DashboardInsightCritical DashboardInsightSeverity = "critical"
)

type DashboardInsightAction struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

type DashboardInsight struct {
	ID          string                    `json:"id"`
	Severity    DashboardInsightSeverity `json:"severity"`
	Title       string                    `json:"title"`
	Description string                    `json:"description"`
	MetricLabel string                    `json:"metric_label,omitempty"`
	MetricValue string                    `json:"metric_value,omitempty"`
	Action      *DashboardInsightAction  `json:"action,omitempty"`
	EntityType  string                    `json:"entity_type,omitempty"`
	EntityID    string                    `json:"entity_id,omitempty"`
}

type DashboardMetric struct {
	Key         string              `json:"key"`
	Title       string              `json:"title"`
	Value       string              `json:"value"`
	Description string              `json:"description,omitempty"`
	Tone        DashboardMetricTone `json:"tone"`
	Trend       []int               `json:"trend,omitempty"`
}

type DashboardChannelHealth struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Status       int    `json:"status"`
	ResponseTime int    `json:"response_time"`
	UsedQuota    int64  `json:"used_quota"`
	Reason       string `json:"reason,omitempty"`
}

type DashboardTopEntity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Quota       int    `json:"quota"`
	Requests    int    `json:"requests"`
	Tokens      int    `json:"tokens"`
}

type DashboardRecentActivity struct {
	ID          int    `json:"id"`
	CreatedAt   int64  `json:"created_at"`
	Type        int    `json:"type"`
	Content     string `json:"content"`
	ModelName   string `json:"model_name,omitempty"`
	TokenName   string `json:"token_name,omitempty"`
	ChannelID   int    `json:"channel_id,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
	Quota       int    `json:"quota,omitempty"`
	UseTime     int    `json:"use_time,omitempty"`
}

type DashboardHero struct {
	Eyebrow     string `json:"eyebrow"`
	Title       string `json:"title"`
	Description string `json:"description"`
	StatusLabel string `json:"status_label"`
	StatusTone  string `json:"status_tone"`
}

type DashboardSummary struct {
	Role           DashboardRole             `json:"role"`
	GeneratedAt    int64                     `json:"generated_at"`
	PeriodStart    int64                     `json:"period_start"`
	PeriodEnd      int64                     `json:"period_end"`
	Hero           DashboardHero             `json:"hero"`
	Metrics        []DashboardMetric         `json:"metrics"`
	Channels       []DashboardChannelHealth  `json:"channels,omitempty"`
	TopUsers       []DashboardTopEntity      `json:"top_users,omitempty"`
	TopModels      []DashboardTopEntity      `json:"top_models,omitempty"`
	RecentActivity []DashboardRecentActivity `json:"recent_activity,omitempty"`
}
```

- [ ] **Step 2: Create pure insight input and thresholds**

Create `service/dashboard_insights.go` with these constants and input type:

```go
package service

import (
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/dto"
)

const (
	dashboardLowRunwayDaysThreshold        = 3
	dashboardHighLatencyMsThreshold        = 5000
	dashboardHighErrorRatePermille         = 50
	dashboardUsageSpikeRatioPermille       = 1500
	dashboardConcentrationSharePermille    = 700
	dashboardMaxInsights                   = 5
)

type DashboardInsightInput struct {
	Role                  dto.DashboardRole
	RemainQuota           int
	RecentQuota           int
	PreviousQuota         int
	ActiveKeyCount         int
	UnusedKeyCount         int
	RecentFailureCount     int
	RecentRequestCount     int
	HighestLatencyChannel  dto.DashboardChannelHealth
	TopUser                dto.DashboardTopEntity
	TopModel               dto.DashboardTopEntity
	TotalQuota             int
}
```

- [ ] **Step 3: Implement deterministic rules**

Add this implementation below the input type:

```go
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
			Title:       "Create your first API key",
			Description: "Create an API key before sending production traffic through the gateway.",
			Action:      &dto.DashboardInsightAction{Label: "Create API Key", Path: "/keys"},
		})
	} else if input.UnusedKeyCount > 0 {
		insights = append(insights, dto.DashboardInsight{
			ID:          "user-unused-api-key",
			Severity:    dto.DashboardInsightInfo,
			Title:       "One API key has no recent traffic",
			Description: "Send a test request or remove unused keys to keep your integration tidy.",
			MetricLabel: "Unused keys",
			MetricValue: fmt.Sprintf("%d", input.UnusedKeyCount),
			Action:      &dto.DashboardInsightAction{Label: "Review API Keys", Path: "/keys"},
		})
	}

	if input.RecentQuota > 0 && input.RemainQuota > 0 {
		runwayDays := input.RemainQuota / input.RecentQuota
		if runwayDays < dashboardLowRunwayDaysThreshold {
			insights = append(insights, dto.DashboardInsight{
				ID:          "user-low-runway",
				Severity:    dto.DashboardInsightWarning,
				Title:       "Balance may run out soon",
				Description: "Your recent usage suggests the current balance may not last three days.",
				MetricLabel: "Runway",
				MetricValue: fmt.Sprintf("%d days", runwayDays),
				Action:      &dto.DashboardInsightAction{Label: "Open Wallet", Path: "/wallet"},
			})
		}
	} else if input.RemainQuota <= 0 {
		insights = append(insights, dto.DashboardInsight{
			ID:          "user-balance-depleted",
			Severity:    dto.DashboardInsightCritical,
			Title:       "Balance depleted",
			Description: "Add credits before sending more paid requests.",
			Action:      &dto.DashboardInsightAction{Label: "Open Wallet", Path: "/wallet"},
		})
	}

	if input.RecentFailureCount > 0 {
		insights = append(insights, dto.DashboardInsight{
			ID:          "user-recent-failures",
			Severity:    dto.DashboardInsightWarning,
			Title:       "Recent requests failed",
			Description: "Review failed requests to fix model, key, or request payload issues.",
			MetricLabel: "Failures",
			MetricValue: fmt.Sprintf("%d", input.RecentFailureCount),
			Action:      &dto.DashboardInsightAction{Label: "View Logs", Path: "/usage-logs"},
		})
	}

	if isUsageSpike(input.RecentQuota, input.PreviousQuota) {
		insights = append(insights, dto.DashboardInsight{
			ID:          "user-usage-spike",
			Severity:    dto.DashboardInsightInfo,
			Title:       "Usage increased versus the previous period",
			Description: "Check model usage and recent logs to confirm the increase is expected.",
			Action:      &dto.DashboardInsightAction{Label: "View Usage", Path: "/usage-logs"},
		})
	}

	if hasConcentration(input.TopModel.Quota, input.TotalQuota) {
		insights = append(insights, dto.DashboardInsight{
			ID:          "user-model-cost-concentration",
			Severity:    dto.DashboardInsightInfo,
			Title:       "One model dominates recent cost",
			Description: "Consider using a cheaper model for test traffic if quality requirements allow it.",
			MetricLabel: "Top model",
			MetricValue: input.TopModel.Name,
			Action:      &dto.DashboardInsightAction{Label: "Compare Pricing", Path: "/pricing"},
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
			ID:          "admin-high-channel-latency",
			Severity:    dto.DashboardInsightWarning,
			Title:       "A channel has high latency",
			Description: "Investigate upstream availability or routing priority for this provider.",
			MetricLabel: "Latency",
			MetricValue: fmt.Sprintf("%d ms", input.HighestLatencyChannel.ResponseTime),
			Action:      &dto.DashboardInsightAction{Label: "Review Channels", Path: "/channels"},
			EntityType:  "channel",
			EntityID:    fmt.Sprintf("%d", input.HighestLatencyChannel.ID),
		})
	}

	if input.RecentRequestCount > 0 && input.RecentFailureCount*1000/input.RecentRequestCount >= dashboardHighErrorRatePermille {
		insights = append(insights, dto.DashboardInsight{
			ID:          "admin-high-error-rate",
			Severity:    dto.DashboardInsightCritical,
			Title:       "Error rate is elevated",
			Description: "Recent failed requests crossed the operational warning threshold.",
			MetricLabel: "Failures",
			MetricValue: fmt.Sprintf("%d", input.RecentFailureCount),
			Action:      &dto.DashboardInsightAction{Label: "View Logs", Path: "/usage-logs"},
		})
	}

	if isUsageSpike(input.RecentQuota, input.PreviousQuota) {
		insights = append(insights, dto.DashboardInsight{
			ID:          "admin-usage-spike",
			Severity:    dto.DashboardInsightInfo,
			Title:       "Usage is up versus the previous period",
			Description: "Review top users and models to confirm the increase is expected.",
			Action:      &dto.DashboardInsightAction{Label: "Open Analytics", Path: "/dashboard/models"},
		})
	}

	if hasConcentration(input.TopUser.Quota, input.TotalQuota) {
		insights = append(insights, dto.DashboardInsight{
			ID:          "admin-user-cost-concentration",
			Severity:    dto.DashboardInsightWarning,
			Title:       "Spend is concentrated in one user",
			Description: "A single user accounts for most recent quota consumption.",
			MetricLabel: "Top user",
			MetricValue: input.TopUser.Name,
			Action:      &dto.DashboardInsightAction{Label: "Review Users", Path: "/users"},
			EntityType:  "user",
			EntityID:    input.TopUser.ID,
		})
	}

	if hasConcentration(input.TopModel.Quota, input.TotalQuota) {
		insights = append(insights, dto.DashboardInsight{
			ID:          "admin-model-cost-concentration",
			Severity:    dto.DashboardInsightInfo,
			Title:       "One model dominates recent spend",
			Description: "Review pricing and routing rules for the most expensive model traffic.",
			MetricLabel: "Top model",
			MetricValue: input.TopModel.Name,
			Action:      &dto.DashboardInsightAction{Label: "View Models", Path: "/dashboard/models"},
			EntityType:  "model",
			EntityID:    input.TopModel.ID,
		})
	}

	return insights
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
```

- [ ] **Step 4: Write insight tests**

Create `service/dashboard_insights_test.go`:

```go
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
```

- [ ] **Step 5: Run tests and verify failure or pass after implementation**

Run:

```bash
go test ./service -run 'TestBuildDashboardInsights' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add dto/dashboard.go service/dashboard_insights.go service/dashboard_insights_test.go
git commit -m "feat: add dashboard insight rules"
```

---

### Task 2: Backend Dashboard Summary Service and API Routes

**Files:**
- Create: `service/dashboard.go`
- Create: `controller/dashboard.go`
- Create: `service/dashboard_test.go`
- Modify: `router/api-router.go`

**Interfaces:**
- Consumes: `dto.DashboardSummary`, `dto.DashboardInsight`, `service.BuildDashboardInsights` from Task 1.
- Produces: `service.GetDashboardSummary(userID int, username string, role int, remainQuota int) (dto.DashboardSummary, error)`, `service.GetDashboardInsights(userID int, username string, role int, remainQuota int) ([]dto.DashboardInsight, error)`, `controller.GetDashboardSummary`, `controller.GetDashboardInsights`.

- [ ] **Step 1: Write service tests for user scoping and safe defaults**

Create `service/dashboard_test.go`:

```go
package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDashboardServiceTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.QuotaData{}, &model.Log{}, &model.Channel{}, &model.Token{}, &model.User{}))
	model.DB = db
	model.LOG_DB = db
	common.LogConsumeEnabled = true
}

func TestGetDashboardSummaryScopesRegularUserData(t *testing.T) {
	setupDashboardServiceTestDB(t)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.QuotaData{UserID: 1, Username: "alice", ModelName: "gpt-4o", CreatedAt: now - 3600, Count: 2, Quota: 30, TokenUsed: 90}).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{UserID: 2, Username: "bob", ModelName: "claude", CreatedAt: now - 3600, Count: 5, Quota: 500, TokenUsed: 1000}).Error)

	summary, err := GetDashboardSummary(1, "alice", common.RoleCommonUser, 100)

	require.NoError(t, err)
	assert.Equal(t, dto.DashboardRoleUser, summary.Role)
	assert.NotEmpty(t, summary.Metrics)
	assert.Empty(t, summary.TopUsers)
	assert.NotContains(t, dashboardMetricValues(summary.Metrics), "500")
}

func TestGetDashboardSummaryAdminIncludesOperationalData(t *testing.T) {
	setupDashboardServiceTestDB(t)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.QuotaData{UserID: 1, Username: "alice", ModelName: "gpt-4o", CreatedAt: now - 3600, Count: 3, Quota: 100, TokenUsed: 200}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Name: "slow", Status: common.ChannelStatusEnabled, ResponseTime: 6000, UsedQuota: 100}).Error)

	summary, err := GetDashboardSummary(99, "admin", common.RoleAdminUser, 0)

	require.NoError(t, err)
	assert.Equal(t, dto.DashboardRoleAdmin, summary.Role)
	assert.NotEmpty(t, summary.Channels)
	assert.NotEmpty(t, summary.TopUsers)
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
```

- [ ] **Step 2: Run tests to verify they fail before service exists**

Run:

```bash
go test ./service -run 'TestGetDashboard' -count=1
```

Expected: FAIL with undefined `GetDashboardSummary` and `GetDashboardInsights`.

- [ ] **Step 3: Implement dashboard service orchestration**

Create `service/dashboard.go`:

```go
package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const (
	dashboardRecentWindowSeconds = 24 * 60 * 60
	dashboardTopLimit            = 5
	dashboardRecentLogLimit      = 6
)

type dashboardAggregates struct {
	RecentQuota       int
	PreviousQuota     int
	RecentRequests    int
	RecentTokens      int
	RecentFailures    int
	TopUsers          []dto.DashboardTopEntity
	TopModels         []dto.DashboardTopEntity
	Channels          []dto.DashboardChannelHealth
	RecentActivity    []dto.DashboardRecentActivity
	ActiveKeyCount    int
	UnusedKeyCount    int
	HighestLatency    dto.DashboardChannelHealth
}

func GetDashboardSummary(userID int, username string, role int, remainQuota int) (dto.DashboardSummary, error) {
	periodEnd := time.Now().Unix()
	periodStart := periodEnd - dashboardRecentWindowSeconds
	agg, err := loadDashboardAggregates(userID, username, role, periodStart, periodEnd)
	if err != nil {
		return dto.DashboardSummary{}, err
	}

	dashboardRole := dashboardRoleFromRole(role)
	summary := dto.DashboardSummary{
		Role:           dashboardRole,
		GeneratedAt:    periodEnd,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		Hero:           buildDashboardHero(dashboardRole, remainQuota, agg),
		Metrics:        buildDashboardMetrics(dashboardRole, remainQuota, agg),
		TopModels:      agg.TopModels,
		RecentActivity: agg.RecentActivity,
	}
	if dashboardRole == dto.DashboardRoleAdmin {
		summary.Channels = agg.Channels
		summary.TopUsers = agg.TopUsers
	}
	return summary, nil
}

func GetDashboardInsights(userID int, username string, role int, remainQuota int) ([]dto.DashboardInsight, error) {
	periodEnd := time.Now().Unix()
	periodStart := periodEnd - dashboardRecentWindowSeconds
	agg, err := loadDashboardAggregates(userID, username, role, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	input := DashboardInsightInput{
		Role:                  dashboardRoleFromRole(role),
		RemainQuota:           remainQuota,
		RecentQuota:           agg.RecentQuota,
		PreviousQuota:         agg.PreviousQuota,
		ActiveKeyCount:         agg.ActiveKeyCount,
		UnusedKeyCount:         agg.UnusedKeyCount,
		RecentFailureCount:     agg.RecentFailures,
		RecentRequestCount:     agg.RecentRequests,
		HighestLatencyChannel:  agg.HighestLatency,
		TotalQuota:             agg.RecentQuota,
	}
	if len(agg.TopUsers) > 0 {
		input.TopUser = agg.TopUsers[0]
	}
	if len(agg.TopModels) > 0 {
		input.TopModel = agg.TopModels[0]
	}
	return BuildDashboardInsights(input), nil
}

func dashboardRoleFromRole(role int) dto.DashboardRole {
	if role >= common.RoleAdminUser {
		return dto.DashboardRoleAdmin
	}
	return dto.DashboardRoleUser
}
```

- [ ] **Step 4: Add aggregate query helpers inside `service/dashboard.go`**

Append these functions. Keep them service-local unless they become reusable elsewhere:

```go
func loadDashboardAggregates(userID int, username string, role int, start int64, end int64) (dashboardAggregates, error) {
	var agg dashboardAggregates
	quotaRows, err := loadDashboardQuotaRows(userID, username, role, start, end)
	if err != nil {
		return agg, err
	}
	previousRows, err := loadDashboardQuotaRows(userID, username, role, start-dashboardRecentWindowSeconds, start)
	if err != nil {
		return agg, err
	}
	for _, row := range quotaRows {
		agg.RecentQuota += row.Quota
		agg.RecentRequests += row.Count
		agg.RecentTokens += row.TokenUsed
	}
	for _, row := range previousRows {
		agg.PreviousQuota += row.Quota
	}
	agg.TopUsers = buildDashboardTopUsers(quotaRows)
	agg.TopModels = buildDashboardTopModels(quotaRows)

	failures, err := loadDashboardRecentFailures(userID, role, start, end)
	if err != nil {
		return agg, err
	}
	agg.RecentFailures = failures

	activity, err := loadDashboardRecentActivity(userID, role)
	if err != nil {
		return agg, err
	}
	agg.RecentActivity = activity

	if role >= common.RoleAdminUser {
		channels, err := loadDashboardChannels()
		if err != nil {
			return agg, err
		}
		agg.Channels = channels
		if len(channels) > 0 {
			agg.HighestLatency = channels[0]
		}
	} else {
		active, unused, err := loadDashboardTokenCounts(userID)
		if err != nil {
			return agg, err
		}
		agg.ActiveKeyCount = active
		agg.UnusedKeyCount = unused
	}

	return agg, nil
}

func loadDashboardQuotaRows(userID int, username string, role int, start int64, end int64) ([]*model.QuotaData, error) {
	if role >= common.RoleAdminUser {
		return model.GetAllQuotaDates(start, end, "")
	}
	if userID > 0 {
		return model.GetQuotaDataByUserId(userID, start, end)
	}
	return model.GetQuotaDataByUsername(username, start, end)
}

func loadDashboardRecentFailures(userID int, role int, start int64, end int64) (int, error) {
	tx := model.LOG_DB.Model(&model.Log{}).Where("type = ? AND created_at >= ? AND created_at <= ?", model.LogTypeError, start, end)
	if role < common.RoleAdminUser {
		tx = tx.Where("user_id = ?", userID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}
	return int(total), nil
}

func loadDashboardRecentActivity(userID int, role int) ([]dto.DashboardRecentActivity, error) {
	tx := model.LOG_DB.Model(&model.Log{})
	if role < common.RoleAdminUser {
		tx = tx.Where("user_id = ?", userID)
	}
	order := "created_at desc, id desc"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = "created_at desc, request_id desc"
	}
	var logs []*model.Log
	if err := tx.Order(order).Limit(dashboardRecentLogLimit).Find(&logs).Error; err != nil {
		return nil, err
	}
	items := make([]dto.DashboardRecentActivity, 0, len(logs))
	for _, log := range logs {
		items = append(items, dto.DashboardRecentActivity{
			ID:          log.Id,
			CreatedAt:   log.CreatedAt,
			Type:        log.Type,
			Content:     log.Content,
			ModelName:   log.ModelName,
			TokenName:   log.TokenName,
			ChannelID:   log.ChannelId,
			ChannelName: log.ChannelName,
			Quota:       log.Quota,
			UseTime:     log.UseTime,
		})
	}
	return items, nil
}

func loadDashboardChannels() ([]dto.DashboardChannelHealth, error) {
	channels, err := model.GetAllChannels(0, dashboardTopLimit, true, true)
	if err != nil {
		return nil, err
	}
	items := make([]dto.DashboardChannelHealth, 0, len(channels))
	for _, channel := range channels {
		items = append(items, dto.DashboardChannelHealth{
			ID:           channel.Id,
			Name:         channel.Name,
			Status:       channel.Status,
			ResponseTime: channel.ResponseTime,
			UsedQuota:    channel.UsedQuota,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].ResponseTime > items[j].ResponseTime
	})
	if len(items) > dashboardTopLimit {
		items = items[:dashboardTopLimit]
	}
	return items, nil
}

func loadDashboardTokenCounts(userID int) (int, int, error) {
	var tokens []model.Token
	if err := model.DB.Model(&model.Token{}).Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		return 0, 0, err
	}
	active := 0
	unused := 0
	for _, token := range tokens {
		if token.Status == common.TokenStatusEnabled {
			active++
			if token.UsedQuota == 0 {
				unused++
			}
		}
	}
	return active, unused, nil
}
```

- [ ] **Step 5: Add metric and top-entity builders**

Append:

```go
func buildDashboardHero(role dto.DashboardRole, remainQuota int, agg dashboardAggregates) dto.DashboardHero {
	if role == dto.DashboardRoleAdmin {
		return dto.DashboardHero{
			Eyebrow:     "Platform command center",
			Title:       fmt.Sprintf("%d requests · %d quota used", agg.RecentRequests, agg.RecentQuota),
			Description: fmt.Sprintf("%d recent failures · %d channels need review", agg.RecentFailures, len(agg.Channels)),
			StatusLabel: dashboardAdminStatusLabel(agg),
			StatusTone:  dashboardAdminStatusTone(agg),
		}
	}
	return dto.DashboardHero{
		Eyebrow:     "Developer home",
		Title:       fmt.Sprintf("%d quota remaining", remainQuota),
		Description: fmt.Sprintf("%d requests in the last 24 hours · %d active keys", agg.RecentRequests, agg.ActiveKeyCount),
		StatusLabel: dashboardUserStatusLabel(remainQuota, agg),
		StatusTone:  dashboardUserStatusTone(remainQuota, agg),
	}
}

func buildDashboardMetrics(role dto.DashboardRole, remainQuota int, agg dashboardAggregates) []dto.DashboardMetric {
	metrics := []dto.DashboardMetric{
		{Key: "requests", Title: "Requests", Value: fmt.Sprintf("%d", agg.RecentRequests), Description: "Last 24 hours", Tone: dto.DashboardMetricToneInfo},
		{Key: "quota", Title: "Quota used", Value: fmt.Sprintf("%d", agg.RecentQuota), Description: "Last 24 hours", Tone: dto.DashboardMetricToneSuccess},
		{Key: "tokens", Title: "Tokens", Value: fmt.Sprintf("%d", agg.RecentTokens), Description: "Prompt, completion, and cache tokens", Tone: dto.DashboardMetricToneInfo},
		{Key: "failures", Title: "Failures", Value: fmt.Sprintf("%d", agg.RecentFailures), Description: "Recent failed requests", Tone: dto.DashboardMetricToneWarning},
	}
	if role == dto.DashboardRoleUser {
		metrics = append([]dto.DashboardMetric{{Key: "balance", Title: "Balance", Value: fmt.Sprintf("%d", remainQuota), Description: "Current remaining quota", Tone: dto.DashboardMetricToneSuccess}}, metrics...)
	}
	return metrics
}

func buildDashboardTopUsers(rows []*model.QuotaData) []dto.DashboardTopEntity {
	byUser := map[int]dto.DashboardTopEntity{}
	for _, row := range rows {
		entity := byUser[row.UserID]
		entity.ID = fmt.Sprintf("%d", row.UserID)
		entity.Name = row.Username
		entity.DisplayName = row.DisplayName
		entity.Quota += row.Quota
		entity.Requests += row.Count
		entity.Tokens += row.TokenUsed
		byUser[row.UserID] = entity
	}
	return sortDashboardTopEntities(byUser)
}

func buildDashboardTopModels(rows []*model.QuotaData) []dto.DashboardTopEntity {
	byModel := map[string]dto.DashboardTopEntity{}
	for _, row := range rows {
		name := row.ModelName
		if name == "" {
			name = "unknown"
		}
		entity := byModel[name]
		entity.ID = name
		entity.Name = name
		entity.Quota += row.Quota
		entity.Requests += row.Count
		entity.Tokens += row.TokenUsed
		byModel[name] = entity
	}
	return sortDashboardTopEntities(byModel)
}

func sortDashboardTopEntities[T comparable](items map[T]dto.DashboardTopEntity) []dto.DashboardTopEntity {
	result := make([]dto.DashboardTopEntity, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Quota > result[j].Quota
	})
	if len(result) > dashboardTopLimit {
		return result[:dashboardTopLimit]
	}
	return result
}

func dashboardAdminStatusLabel(agg dashboardAggregates) string {
	if agg.RecentFailures > 0 || agg.HighestLatency.ResponseTime >= dashboardHighLatencyMsThreshold {
		return "Needs attention"
	}
	return "Healthy"
}

func dashboardAdminStatusTone(agg dashboardAggregates) string {
	if agg.RecentFailures > 0 || agg.HighestLatency.ResponseTime >= dashboardHighLatencyMsThreshold {
		return "warning"
	}
	return "success"
}

func dashboardUserStatusLabel(remainQuota int, agg dashboardAggregates) string {
	if remainQuota <= 0 {
		return "Balance depleted"
	}
	if agg.ActiveKeyCount == 0 {
		return "Needs API key"
	}
	return "Ready"
}

func dashboardUserStatusTone(remainQuota int, agg dashboardAggregates) string {
	if remainQuota <= 0 {
		return "destructive"
	}
	if agg.ActiveKeyCount == 0 {
		return "warning"
	}
	return "success"
}
```

- [ ] **Step 6: Create controller handlers**

Create `controller/dashboard.go`:

```go
package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetDashboardSummary(c *gin.Context) {
	summary, err := service.GetDashboardSummary(
		c.GetInt("id"),
		c.GetString("username"),
		c.GetInt("role"),
		c.GetInt("quota"),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func GetDashboardInsights(c *gin.Context) {
	insights, err := service.GetDashboardInsights(
		c.GetInt("id"),
		c.GetString("username"),
		c.GetInt("role"),
		c.GetInt("quota"),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, insights)
}
```

- [ ] **Step 7: Register routes**

In `router/api-router.go`, near the existing `dataRoute` registration, add:

```go
		dashboardRoute := apiRouter.Group("/dashboard")
		dashboardRoute.Use(middleware.UserAuth())
		{
			dashboardRoute.GET("/summary", controller.GetDashboardSummary)
			dashboardRoute.GET("/insights", controller.GetDashboardInsights)
		}
```

- [ ] **Step 8: Run backend tests**

Run:

```bash
go test ./service -run 'TestGetDashboard|TestBuildDashboardInsights' -count=1
```

Expected: PASS.

- [ ] **Step 9: Run focused backend build**

Run:

```bash
go test ./controller ./router ./service -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add service/dashboard.go service/dashboard_test.go controller/dashboard.go router/api-router.go
git commit -m "feat: add dashboard summary api"
```

---

### Task 3: Frontend Types, API Client, and Insight Panel

**Files:**
- Modify: `web/src/features/dashboard/types.ts`
- Modify: `web/src/features/dashboard/api.ts`
- Create: `web/src/features/dashboard/components/overview/smart-insights-panel.tsx`
- Create: `web/src/features/dashboard/components/overview/__tests__/smart-insights-panel.test.tsx`

**Interfaces:**
- Consumes: backend JSON DTOs from Task 2.
- Produces: `getDashboardSummary()`, `getDashboardInsights()`, `<SmartInsightsPanel />`.

- [ ] **Step 1: Add frontend dashboard types**

Append to `web/src/features/dashboard/types.ts`:

```ts
export type DashboardRole = 'admin' | 'user'

export type DashboardMetricTone =
  | 'info'
  | 'success'
  | 'warning'
  | 'destructive'

export type DashboardInsightSeverity = 'info' | 'warning' | 'critical'

export interface DashboardInsightAction {
  label: string
  path: string
}

export interface DashboardInsight {
  id: string
  severity: DashboardInsightSeverity
  title: string
  description: string
  metric_label?: string
  metric_value?: string
  action?: DashboardInsightAction
  entity_type?: string
  entity_id?: string
}

export interface DashboardMetric {
  key: string
  title: string
  value: string
  description?: string
  tone: DashboardMetricTone
  trend?: number[]
}

export interface DashboardChannelHealth {
  id: number
  name: string
  status: number
  response_time: number
  used_quota: number
  reason?: string
}

export interface DashboardTopEntity {
  id: string
  name: string
  display_name?: string
  quota: number
  requests: number
  tokens: number
}

export interface DashboardRecentActivity {
  id: number
  created_at: number
  type: number
  content: string
  model_name?: string
  token_name?: string
  channel_id?: number
  channel_name?: string
  quota?: number
  use_time?: number
}

export interface DashboardHero {
  eyebrow: string
  title: string
  description: string
  status_label: string
  status_tone: string
}

export interface DashboardSummary {
  role: DashboardRole
  generated_at: number
  period_start: number
  period_end: number
  hero: DashboardHero
  metrics: DashboardMetric[]
  channels?: DashboardChannelHealth[]
  top_users?: DashboardTopEntity[]
  top_models?: DashboardTopEntity[]
  recent_activity?: DashboardRecentActivity[]
}
```

- [ ] **Step 2: Add API client functions**

Modify imports in `web/src/features/dashboard/api.ts` to include the new types, then append:

```ts
import type { DashboardInsight, DashboardSummary } from './types'

export async function getDashboardSummary() {
  const res = await api.get<{
    success: boolean
    data: DashboardSummary
    message?: string
  }>('/api/dashboard/summary')
  return res.data
}

export async function getDashboardInsights() {
  const res = await api.get<{
    success: boolean
    data: DashboardInsight[]
    message?: string
  }>('/api/dashboard/insights')
  return res.data
}
```

If `api.ts` already imports from `./types`, merge this into the existing import rather than adding a duplicate import statement.

- [ ] **Step 3: Write failing insight panel tests**

Create `web/src/features/dashboard/components/overview/__tests__/smart-insights-panel.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { SmartInsightsPanel } from '../smart-insights-panel'
import type { DashboardInsight } from '../../../types'

const insights: DashboardInsight[] = [
  {
    id: 'user-low-runway',
    severity: 'warning',
    title: 'Balance may run out soon',
    description: 'Your recent usage suggests the current balance may not last three days.',
    metric_label: 'Runway',
    metric_value: '2 days',
    action: { label: 'Open Wallet', path: '/wallet' },
  },
]

describe('SmartInsightsPanel', () => {
  it('renders insight title, metric, and action link when data exists', () => {
    render(<SmartInsightsPanel insights={insights} loading={false} />)

    expect(screen.getByText('Balance may run out soon')).toBeInTheDocument()
    expect(screen.getByText('Runway')).toBeInTheDocument()
    expect(screen.getByText('2 days')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open Wallet' })).toHaveAttribute('href', '/wallet')
  })

  it('renders an empty state when there are no insights', () => {
    render(<SmartInsightsPanel insights={[]} loading={false} />)

    expect(screen.getByText('No urgent insights')).toBeInTheDocument()
  })

  it('renders a loading state while insights are loading', () => {
    render(<SmartInsightsPanel insights={[]} loading />)

    expect(screen.getByLabelText('Loading smart insights')).toBeInTheDocument()
  })
})
```

- [ ] **Step 4: Run test to verify failure before component exists**

Run:

```bash
cd web && bun test src/features/dashboard/components/overview/__tests__/smart-insights-panel.test.tsx
```

Expected: FAIL because `smart-insights-panel` does not exist.

- [ ] **Step 5: Implement `SmartInsightsPanel`**

Create `web/src/features/dashboard/components/overview/smart-insights-panel.tsx`:

```tsx
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Link } from '@tanstack/react-router'
import { AlertTriangle, CheckCircle2, Info, Lightbulb } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import type { DashboardInsight, DashboardInsightSeverity } from '../../types'

interface SmartInsightsPanelProps {
  insights: DashboardInsight[]
  loading: boolean
  error?: boolean
}

const severityConfig: Record<
  DashboardInsightSeverity,
  { icon: typeof Info; className: string }
> = {
  info: { icon: Info, className: 'border-info/25 bg-info/8 text-info' },
  warning: {
    icon: AlertTriangle,
    className: 'border-warning/25 bg-warning/8 text-warning',
  },
  critical: {
    icon: AlertTriangle,
    className: 'border-destructive/25 bg-destructive/8 text-destructive',
  },
}

export function SmartInsightsPanel(props: SmartInsightsPanelProps) {
  const { t } = useTranslation()

  if (props.loading) {
    return (
      <section
        className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'
        aria-label={t('Loading smart insights')}
      >
        <div className='mb-4 flex items-center gap-2'>
          <Skeleton className='size-9 rounded-xl' />
          <div className='space-y-2'>
            <Skeleton className='h-4 w-36' />
            <Skeleton className='h-3 w-56' />
          </div>
        </div>
        <div className='grid gap-2'>
          <Skeleton className='h-20 rounded-xl' />
          <Skeleton className='h-20 rounded-xl' />
        </div>
      </section>
    )
  }

  if (props.error) {
    return (
      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
        <PanelHeader />
        <div className='text-muted-foreground rounded-xl border border-dashed p-4 text-sm'>
          {t('Smart insights are temporarily unavailable.')}
        </div>
      </section>
    )
  }

  return (
    <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
      <PanelHeader />
      {props.insights.length === 0 ? (
        <div className='rounded-xl border border-dashed p-4'>
          <div className='flex items-center gap-2 text-sm font-medium'>
            <CheckCircle2 className='text-success size-4' aria-hidden='true' />
            {t('No urgent insights')}
          </div>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('The dashboard did not find anything that needs immediate attention.')}
          </p>
        </div>
      ) : (
        <div className='grid gap-2'>
          {props.insights.map((insight) => (
            <InsightItem key={insight.id} insight={insight} />
          ))}
        </div>
      )}
    </section>
  )
}

function PanelHeader() {
  const { t } = useTranslation()

  return (
    <div className='mb-4 flex items-start gap-3'>
      <span className='bg-primary/10 text-primary flex size-9 shrink-0 items-center justify-center rounded-xl'>
        <Lightbulb className='size-4' aria-hidden='true' />
      </span>
      <div className='min-w-0'>
        <h3 className='text-sm font-semibold sm:text-base'>
          {t('Smart insights')}
        </h3>
        <p className='text-muted-foreground text-xs sm:text-sm'>
          {t('Rule-based recommendations from recent platform activity')}
        </p>
      </div>
    </div>
  )
}

function InsightItem(props: { insight: DashboardInsight }) {
  const { t } = useTranslation()
  const config = severityConfig[props.insight.severity]
  const Icon = config.icon

  return (
    <article className='bg-background/60 rounded-xl border p-3'>
      <div className='flex items-start gap-3'>
        <span
          className={cn(
            'flex size-8 shrink-0 items-center justify-center rounded-lg border',
            config.className
          )}
        >
          <Icon className='size-4' aria-hidden='true' />
        </span>
        <div className='min-w-0 flex-1'>
          <div className='flex flex-wrap items-start justify-between gap-2'>
            <div className='min-w-0'>
              <h4 className='text-sm font-medium'>{t(props.insight.title)}</h4>
              <p className='text-muted-foreground mt-0.5 text-xs leading-relaxed'>
                {t(props.insight.description)}
              </p>
            </div>
            {props.insight.metric_label && props.insight.metric_value && (
              <div className='bg-muted/60 rounded-lg px-2 py-1 text-right'>
                <div className='text-muted-foreground text-[10px] font-medium tracking-wide uppercase'>
                  {t(props.insight.metric_label)}
                </div>
                <div className='text-xs font-semibold tabular-nums'>
                  {props.insight.metric_value}
                </div>
              </div>
            )}
          </div>
          {props.insight.action && (
            <Button
              size='sm'
              variant='link'
              className='mt-2 h-auto px-0 text-xs'
              render={<Link to={props.insight.action.path} />}
            >
              {t(props.insight.action.label)}
            </Button>
          )}
        </div>
      </div>
    </article>
  )
}
```

- [ ] **Step 6: Run insight panel tests**

Run:

```bash
cd web && bun test src/features/dashboard/components/overview/__tests__/smart-insights-panel.test.tsx
```

Expected: PASS. If the test environment does not support `bun test`, use the repository's established frontend test command or run typecheck/build in Task 6.

- [ ] **Step 7: Commit**

```bash
git add web/src/features/dashboard/types.ts web/src/features/dashboard/api.ts web/src/features/dashboard/components/overview/smart-insights-panel.tsx web/src/features/dashboard/components/overview/__tests__/smart-insights-panel.test.tsx
git commit -m "feat: add dashboard insights panel"
```

---

### Task 4: Frontend Role-Aware Overview Components

**Files:**
- Create: `web/src/features/dashboard/components/overview/dashboard-hero.tsx`
- Create: `web/src/features/dashboard/components/overview/dashboard-metric-grid.tsx`
- Create: `web/src/features/dashboard/components/overview/admin-operations-panel.tsx`
- Create: `web/src/features/dashboard/components/overview/user-developer-panel.tsx`
- Create: `web/src/features/dashboard/components/overview/__tests__/overview-role-layout.test.tsx`

**Interfaces:**
- Consumes: `DashboardSummary`, `DashboardMetric`, `DashboardChannelHealth`, `DashboardTopEntity`, `DashboardRecentActivity` from Task 3.
- Produces: reusable role-aware overview components used by Task 5.

- [ ] **Step 1: Create metric grid**

Create `dashboard-metric-grid.tsx`:

```tsx
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Activity, AlertTriangle, Coins, KeyRound, RadioTower, Timer, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatCard } from '../ui/stat-card'
import type { DashboardMetric } from '../../types'

interface DashboardMetricGridProps {
  metrics: DashboardMetric[]
  loading?: boolean
}

const metricIcons = {
  balance: Coins,
  quota: Coins,
  requests: Activity,
  tokens: Zap,
  failures: AlertTriangle,
  keys: KeyRound,
  latency: Timer,
  channels: RadioTower,
} as const

const metricTones = {
  info: 'accent-1',
  success: 'accent-2',
  warning: 'accent-3',
  destructive: 'accent-3',
} as const

export function DashboardMetricGrid(props: DashboardMetricGridProps) {
  return (
    <div className='grid gap-2 sm:grid-cols-2 xl:grid-cols-4'>
      {props.metrics.map((metric) => (
        <DashboardMetricItem
          key={metric.key}
          metric={metric}
          loading={props.loading}
        />
      ))}
    </div>
  )
}

function DashboardMetricItem(props: {
  metric: DashboardMetric
  loading?: boolean
}) {
  const { t } = useTranslation()
  const Icon = metricIcons[props.metric.key as keyof typeof metricIcons] ?? Activity

  return (
    <div className='bg-card rounded-2xl border p-3 shadow-xs'>
      <StatCard
        title={t(props.metric.title)}
        value={props.metric.value}
        description={props.metric.description ? t(props.metric.description) : undefined}
        icon={Icon}
        tone={metricTones[props.metric.tone] ?? 'accent-1'}
        sparkline={props.metric.trend}
        sparklineVariant='line'
        loading={props.loading}
      />
    </div>
  )
}
```

- [ ] **Step 2: Create dashboard hero**

Create `dashboard-hero.tsx`:

```tsx
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Link } from '@tanstack/react-router'
import { ArrowRight, ShieldCheck, TerminalSquare } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

import type { DashboardSummary } from '../../types'

interface DashboardHeroProps {
  summary: DashboardSummary
}

const statusToneClass = {
  success: 'border-success/30 bg-success/10 text-success',
  warning: 'border-warning/30 bg-warning/10 text-warning',
  destructive: 'border-destructive/30 bg-destructive/10 text-destructive',
  info: 'border-info/30 bg-info/10 text-info',
} as const

export function DashboardHero(props: DashboardHeroProps) {
  const { t } = useTranslation()
  const isAdmin = props.summary.role === 'admin'
  const primaryPath = isAdmin ? '/usage-logs' : '/keys'
  const secondaryPath = isAdmin ? '/channels' : '/playground'

  return (
    <section className='relative overflow-hidden rounded-2xl border bg-[radial-gradient(ellipse_80%_120%_at_80%_0%,color-mix(in_oklch,var(--primary)_16%,transparent)_0%,transparent_60%),linear-gradient(135deg,color-mix(in_oklch,var(--card)_96%,var(--primary)_4%),var(--card))] p-4 shadow-xs sm:p-6'>
      <div className='relative grid gap-5 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end'>
        <div className='min-w-0'>
          <div className='text-muted-foreground mb-2 text-xs font-medium tracking-wider uppercase'>
            {t(props.summary.hero.eyebrow)}
          </div>
          <div className='flex flex-wrap items-center gap-2'>
            <h2 className='text-2xl font-semibold tracking-tight sm:text-3xl'>
              {props.summary.hero.title}
            </h2>
            <Badge
              variant='outline'
              className={cn(
                'rounded-full',
                statusToneClass[
                  props.summary.hero.status_tone as keyof typeof statusToneClass
                ] ?? statusToneClass.info
              )}
            >
              {t(props.summary.hero.status_label)}
            </Badge>
          </div>
          <p className='text-muted-foreground mt-2 max-w-2xl text-sm leading-relaxed'>
            {props.summary.hero.description}
          </p>
        </div>

        <div className='flex flex-wrap gap-2'>
          <Button render={<Link to={primaryPath} />}>
            {isAdmin ? t('View Logs') : t('Review API Keys')}
            <ArrowRight data-icon='inline-end' />
          </Button>
          <Button variant='outline' render={<Link to={secondaryPath} />}>
            {isAdmin ? (
              <ShieldCheck data-icon='inline-start' />
            ) : (
              <TerminalSquare data-icon='inline-start' />
            )}
            {isAdmin ? t('Review Channels') : t('Open Playground')}
          </Button>
        </div>
      </div>
    </section>
  )
}
```

- [ ] **Step 3: Create admin operations panel**

Create `admin-operations-panel.tsx`:

```tsx
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Link } from '@tanstack/react-router'
import { Activity, AlertTriangle, ArrowRight, RadioTower, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { formatQuota } from '@/lib/format'

import type { DashboardSummary } from '../../types'

export function AdminOperationsPanel(props: { summary: DashboardSummary }) {
  const { t } = useTranslation()
  const channels = props.summary.channels ?? []
  const topUsers = props.summary.top_users ?? []
  const topModels = props.summary.top_models ?? []
  const recentActivity = props.summary.recent_activity ?? []

  return (
    <div className='grid gap-4 xl:grid-cols-[1.15fr_.85fr]'>
      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
        <PanelTitle icon={RadioTower} title={t('Channel health')} actionPath='/channels' actionLabel={t('Manage channels')} />
        <div className='grid gap-2'>
          {channels.length === 0 ? (
            <EmptyLine text={t('No channel health data yet.')} />
          ) : (
            channels.map((channel) => (
              <div key={channel.id} className='bg-background/60 flex items-center justify-between gap-3 rounded-xl border px-3 py-2'>
                <div className='min-w-0'>
                  <div className='truncate text-sm font-medium'>{channel.name}</div>
                  <div className='text-muted-foreground text-xs'>{channel.response_time} ms</div>
                </div>
                <Badge variant='outline'>{t('Status')} {channel.status}</Badge>
              </div>
            ))
          )}
        </div>
      </section>

      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
        <PanelTitle icon={AlertTriangle} title={t('Recent activity')} actionPath='/usage-logs' actionLabel={t('View all')} />
        <div className='grid gap-2'>
          {recentActivity.length === 0 ? (
            <EmptyLine text={t('No recent activity yet.')} />
          ) : (
            recentActivity.slice(0, 5).map((activity) => (
              <div key={activity.id} className='bg-background/60 rounded-xl border px-3 py-2'>
                <div className='line-clamp-1 text-sm font-medium'>{activity.content || t('Request log')}</div>
                <div className='text-muted-foreground mt-0.5 text-xs'>{activity.model_name || t('Unknown model')} · {formatQuota(activity.quota ?? 0)}</div>
              </div>
            ))
          )}
        </div>
      </section>

      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5 xl:col-span-2'>
        <div className='grid gap-4 md:grid-cols-2'>
          <TopList icon={Users} title={t('Top users')} items={topUsers} emptyText={t('No user usage data yet.')} />
          <TopList icon={Activity} title={t('Top models')} items={topModels} emptyText={t('No model usage data yet.')} />
        </div>
      </section>
    </div>
  )
}

function PanelTitle(props: { icon: typeof Activity; title: string; actionPath: string; actionLabel: string }) {
  const Icon = props.icon
  return (
    <div className='mb-4 flex items-center justify-between gap-3'>
      <div className='flex items-center gap-2'>
        <Icon className='text-muted-foreground size-4' aria-hidden='true' />
        <h3 className='text-sm font-semibold'>{props.title}</h3>
      </div>
      <Button variant='ghost' size='sm' render={<Link to={props.actionPath} />}>
        {props.actionLabel}
        <ArrowRight data-icon='inline-end' />
      </Button>
    </div>
  )
}

function TopList(props: { icon: typeof Activity; title: string; items: NonNullable<DashboardSummary['top_models']>; emptyText: string }) {
  const Icon = props.icon
  return (
    <div>
      <div className='mb-3 flex items-center gap-2'>
        <Icon className='text-muted-foreground size-4' aria-hidden='true' />
        <h4 className='text-sm font-semibold'>{props.title}</h4>
      </div>
      <div className='grid gap-2'>
        {props.items.length === 0 ? (
          <EmptyLine text={props.emptyText} />
        ) : (
          props.items.map((item) => (
            <div key={item.id} className='bg-background/60 flex items-center justify-between gap-3 rounded-xl border px-3 py-2'>
              <div className='min-w-0'>
                <div className='truncate text-sm font-medium'>{item.display_name || item.name}</div>
                <div className='text-muted-foreground text-xs'>{item.requests} requests · {item.tokens} tokens</div>
              </div>
              <div className='font-mono text-xs font-semibold'>{formatQuota(item.quota)}</div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function EmptyLine(props: { text: string }) {
  return <div className='text-muted-foreground rounded-xl border border-dashed p-3 text-sm'>{props.text}</div>
}
```

- [ ] **Step 4: Create user developer panel**

Create `user-developer-panel.tsx`:

```tsx
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Link } from '@tanstack/react-router'
import { ArrowRight, Code2, FileText, KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { formatQuota } from '@/lib/format'

import type { DashboardSummary } from '../../types'

export function UserDeveloperPanel(props: { summary: DashboardSummary }) {
  const { t } = useTranslation()
  const recentActivity = props.summary.recent_activity ?? []
  const topModels = props.summary.top_models ?? []

  return (
    <div className='grid gap-4 xl:grid-cols-[.9fr_1.1fr]'>
      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
        <div className='mb-4 flex items-center gap-2'>
          <Code2 className='text-muted-foreground size-4' aria-hidden='true' />
          <h3 className='text-sm font-semibold'>{t('Ready-to-run request')}</h3>
        </div>
        <div className='bg-foreground/[0.035] rounded-xl p-3 font-mono text-xs'>
          <code className='block truncate'>curl /v1/chat/completions</code>
          <code className='text-muted-foreground block truncate'>-H "Authorization: Bearer sk-..."</code>
          <code className='text-muted-foreground block truncate'>-d {'{'}"model":"gpt-4o-mini"{'}'}</code>
        </div>
        <div className='mt-4 flex flex-wrap gap-2'>
          <Button render={<Link to='/keys' />}>
            <KeyRound data-icon='inline-start' />
            {t('Review API Keys')}
          </Button>
          <Button variant='outline' render={<Link to='/playground' />}>
            {t('Open Playground')}
            <ArrowRight data-icon='inline-end' />
          </Button>
        </div>
      </section>

      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
        <div className='mb-4 flex items-center justify-between gap-3'>
          <div className='flex items-center gap-2'>
            <FileText className='text-muted-foreground size-4' aria-hidden='true' />
            <h3 className='text-sm font-semibold'>{t('Recent requests')}</h3>
          </div>
          <Button variant='ghost' size='sm' render={<Link to='/usage-logs' />}>
            {t('View Logs')}
            <ArrowRight data-icon='inline-end' />
          </Button>
        </div>
        <div className='grid gap-2'>
          {recentActivity.length === 0 ? (
            <div className='text-muted-foreground rounded-xl border border-dashed p-3 text-sm'>
              {t('Send your first request to see activity here.')}
            </div>
          ) : (
            recentActivity.slice(0, 5).map((activity) => (
              <div key={activity.id} className='bg-background/60 flex items-center justify-between gap-3 rounded-xl border px-3 py-2'>
                <div className='min-w-0'>
                  <div className='truncate text-sm font-medium'>{activity.model_name || t('Unknown model')}</div>
                  <div className='text-muted-foreground text-xs'>{activity.content || t('Request log')}</div>
                </div>
                <div className='font-mono text-xs font-semibold'>{formatQuota(activity.quota ?? 0)}</div>
              </div>
            ))
          )}
        </div>
      </section>

      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5 xl:col-span-2'>
        <h3 className='mb-4 text-sm font-semibold'>{t('Model usage')}</h3>
        <div className='grid gap-2 md:grid-cols-2 xl:grid-cols-3'>
          {topModels.length === 0 ? (
            <div className='text-muted-foreground rounded-xl border border-dashed p-3 text-sm'>
              {t('No model usage data yet.')}
            </div>
          ) : (
            topModels.map((model) => (
              <div key={model.id} className='bg-background/60 rounded-xl border px-3 py-2'>
                <div className='truncate text-sm font-medium'>{model.name}</div>
                <div className='text-muted-foreground mt-0.5 text-xs'>{model.requests} requests · {formatQuota(model.quota)}</div>
              </div>
            ))
          )}
        </div>
      </section>
    </div>
  )
}
```

- [ ] **Step 5: Add role layout tests**

Create `overview-role-layout.test.tsx` with tests for the pure panels:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { AdminOperationsPanel } from '../admin-operations-panel'
import { DashboardHero } from '../dashboard-hero'
import { UserDeveloperPanel } from '../user-developer-panel'
import type { DashboardSummary } from '../../../types'

const baseSummary: DashboardSummary = {
  role: 'admin',
  generated_at: 1,
  period_start: 1,
  period_end: 2,
  hero: {
    eyebrow: 'Platform command center',
    title: '10 requests · 20 quota used',
    description: '1 recent failures · 1 channels need review',
    status_label: 'Needs attention',
    status_tone: 'warning',
  },
  metrics: [],
}

describe('dashboard role panels', () => {
  it('renders admin operational links and channel data', () => {
    render(
      <AdminOperationsPanel
        summary={{
          ...baseSummary,
          channels: [{ id: 1, name: 'OpenAI', status: 1, response_time: 800, used_quota: 10 }],
          top_users: [{ id: '1', name: 'alice', quota: 10, requests: 2, tokens: 20 }],
          top_models: [{ id: 'gpt-4o', name: 'gpt-4o', quota: 10, requests: 2, tokens: 20 }],
        }}
      />
    )

    expect(screen.getByText('Channel health')).toBeInTheDocument()
    expect(screen.getByText('OpenAI')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Manage channels/i })).toHaveAttribute('href', '/channels')
  })

  it('renders user developer actions and hides admin channel management', () => {
    render(<UserDeveloperPanel summary={{ ...baseSummary, role: 'user' }} />)

    expect(screen.getByText('Ready-to-run request')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Review API Keys/i })).toHaveAttribute('href', '/keys')
    expect(screen.queryByText('Channel health')).not.toBeInTheDocument()
  })

  it('renders hero action based on role', () => {
    render(<DashboardHero summary={baseSummary} />)

    expect(screen.getByText('Platform command center')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Review Channels/i })).toHaveAttribute('href', '/channels')
  })
})
```

- [ ] **Step 6: Run role panel tests**

Run:

```bash
cd web && bun test src/features/dashboard/components/overview/__tests__/overview-role-layout.test.tsx
```

Expected: PASS or, if `bun test` is not configured, defer to Task 6 typecheck/build.

- [ ] **Step 7: Commit**

```bash
git add web/src/features/dashboard/components/overview/dashboard-hero.tsx web/src/features/dashboard/components/overview/dashboard-metric-grid.tsx web/src/features/dashboard/components/overview/admin-operations-panel.tsx web/src/features/dashboard/components/overview/user-developer-panel.tsx web/src/features/dashboard/components/overview/__tests__/overview-role-layout.test.tsx
git commit -m "feat: add role aware dashboard panels"
```

---

### Task 5: Compose New Overview and Preserve Existing Supporting Panels

**Files:**
- Modify: `web/src/features/dashboard/components/overview/overview-dashboard.tsx`

**Interfaces:**
- Consumes: all frontend components and API functions from Tasks 3 and 4.
- Produces: the visible hybrid role dashboard overview.

- [ ] **Step 1: Import new APIs and components**

Modify `overview-dashboard.tsx` imports:

```tsx
import { getDashboardInsights, getDashboardSummary } from '../../api'
import { AdminOperationsPanel } from './admin-operations-panel'
import { DashboardHero } from './dashboard-hero'
import { DashboardMetricGrid } from './dashboard-metric-grid'
import { SmartInsightsPanel } from './smart-insights-panel'
import { UserDeveloperPanel } from './user-developer-panel'
```

Keep the existing imports for setup guide, API info, uptime, announcements, FAQ, and summary cards until the new overview is stable.

- [ ] **Step 2: Add summary and insight queries inside `OverviewDashboard`**

Inside `OverviewDashboard`, after existing `modelsQuery`, add:

```tsx
  const dashboardSummaryQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'summary'],
    queryFn: async () => {
      const result = await getDashboardSummary()
      if (!result.success) {
        throw new Error(result.message || 'Failed to load dashboard summary')
      }
      return result.data
    },
    staleTime: 60 * 1000,
  })

  const dashboardInsightsQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'insights'],
    queryFn: async () => {
      const result = await getDashboardInsights()
      if (!result.success) {
        throw new Error(result.message || 'Failed to load dashboard insights')
      }
      return result.data
    },
    staleTime: 60 * 1000,
  })
```

- [ ] **Step 3: Build role-aware top section**

Before the existing setup guide JSX, add this render block in the returned `<div className='flex flex-col gap-4'>`:

```tsx
      {dashboardSummaryQuery.data && (
        <CardStaggerContainer className='grid gap-4'>
          <CardStaggerItem>
            <DashboardHero summary={dashboardSummaryQuery.data} />
          </CardStaggerItem>
          <CardStaggerItem>
            <DashboardMetricGrid metrics={dashboardSummaryQuery.data.metrics} />
          </CardStaggerItem>
          <CardStaggerItem>
            <SmartInsightsPanel
              insights={dashboardInsightsQuery.data ?? []}
              loading={dashboardInsightsQuery.isLoading}
              error={dashboardInsightsQuery.isError}
            />
          </CardStaggerItem>
          <CardStaggerItem>
            {dashboardSummaryQuery.data.role === 'admin' ? (
              <AdminOperationsPanel summary={dashboardSummaryQuery.data} />
            ) : (
              <UserDeveloperPanel summary={dashboardSummaryQuery.data} />
            )}
          </CardStaggerItem>
        </CardStaggerContainer>
      )}
```

- [ ] **Step 4: Add fallback when summary fails**

Immediately after the new block, add:

```tsx
      {dashboardSummaryQuery.isError && (
        <div className='bg-card text-muted-foreground rounded-2xl border border-dashed p-4 text-sm shadow-xs'>
          {t('The enhanced dashboard summary is temporarily unavailable. Existing dashboard panels are still available below.')}
        </div>
      )}
```

- [ ] **Step 5: Keep existing panels below the new top section**

Do not remove setup guide, `SummaryCards`, `PerformanceHealthPanel`, `ApiInfoPanel`, `AnnouncementsPanel`, `FAQPanel`, or `UptimePanel` in this task. If the page becomes too long, move `SummaryCards` below the setup guide only after confirming there is no duplicate card conflict.

- [ ] **Step 6: Run frontend typecheck**

Run:

```bash
cd web && bun run typecheck
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add web/src/features/dashboard/components/overview/overview-dashboard.tsx
git commit -m "feat: compose hybrid dashboard overview"
```

---

### Task 6: i18n, Formatting, and Verification

**Files:**
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/vi.json`
- Modify: any generated/static i18n file if `bun run i18n:sync` changes it.

**Interfaces:**
- Consumes: all new frontend text from Tasks 3-5.
- Produces: complete translations and final verification.

- [ ] **Step 1: Run i18n sync**

Run:

```bash
cd web && bun run i18n:sync
```

Expected: New English keys are detected and added or reported for translation.

- [ ] **Step 2: Translate new keys**

Use the existing locale style and add translations for keys such as:

```json
{
  "Smart insights": "Smart insights",
  "Rule-based recommendations from recent platform activity": "Rule-based recommendations from recent platform activity",
  "No urgent insights": "No urgent insights",
  "The dashboard did not find anything that needs immediate attention.": "The dashboard did not find anything that needs immediate attention.",
  "Smart insights are temporarily unavailable.": "Smart insights are temporarily unavailable.",
  "Platform command center": "Platform command center",
  "Developer home": "Developer home",
  "Needs attention": "Needs attention",
  "Healthy": "Healthy",
  "Ready": "Ready",
  "Needs API key": "Needs API key",
  "Channel health": "Channel health",
  "Manage channels": "Manage channels",
  "Recent activity": "Recent activity",
  "Top users": "Top users",
  "Top models": "Top models",
  "Ready-to-run request": "Ready-to-run request",
  "Recent requests": "Recent requests",
  "Model usage": "Model usage",
  "The enhanced dashboard summary is temporarily unavailable. Existing dashboard panels are still available below.": "The enhanced dashboard summary is temporarily unavailable. Existing dashboard panels are still available below."
}
```

Translate these into zh, zh-TW, fr, ru, ja, and vi. Do not leave values identical to English except for accepted technical terms such as API.

- [ ] **Step 3: Run backend verification**

Run:

```bash
go test ./service ./controller ./router -count=1
```

Expected: PASS.

- [ ] **Step 4: Run frontend verification**

Run:

```bash
cd web && bun run typecheck
cd web && bun run build
```

Expected: PASS.

- [ ] **Step 5: Run formatting/lint checks for touched frontend files**

Run:

```bash
cd web && bun run lint
cd web && bun run format:check
```

Expected: PASS. If unrelated pre-existing lint issues appear, record them clearly and run targeted lint/format checks for touched files if the tooling supports it.

- [ ] **Step 6: Manual smoke check**

Start the app using the project’s normal local setup and verify:

- Admin user sees platform command center, smart insights, channel health, top users, top models, and recent activity.
- Regular user sees developer home, smart insights, ready-to-run request, recent requests, and model usage.
- Regular user does not see admin-only top users or channel management cards.
- Summary/insight API responses do not contain raw API keys, channel keys, or provider credentials.
- Existing dashboard sections `/dashboard/models`, `/dashboard/flow`, and `/dashboard/users` still work.

- [ ] **Step 7: Final commit**

```bash
git add web/src/i18n/locales web/src/i18n/static-keys.ts
git commit -m "chore: translate hybrid dashboard copy"
```

If `web/src/i18n/static-keys.ts` is unchanged, omit it from `git add`.

---

## Final Verification Checklist

- [ ] `go test ./service ./controller ./router -count=1` passes.
- [ ] `cd web && bun run typecheck` passes.
- [ ] `cd web && bun run build` passes.
- [ ] `cd web && bun run lint` passes or only reports unrelated pre-existing issues that are documented.
- [ ] `cd web && bun run format:check` passes or only reports unrelated pre-existing issues that are documented.
- [ ] New dashboard endpoints are protected by `middleware.UserAuth()` and scope data by role.
- [ ] Regular users do not receive admin-only dashboard data.
- [ ] Dashboard responses contain no raw API keys or provider credentials.
- [ ] All new UI text uses `t('English key')`.
- [ ] All supported locale files include the new keys.
- [ ] Existing dashboard analytics sections still render.
