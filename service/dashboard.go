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
	RecentQuota    int
	PreviousQuota  int
	RecentRequests int
	RecentTokens   int
	RecentFailures int
	TopUsers       []dto.DashboardTopEntity
	TopModels      []dto.DashboardTopEntity
	Channels       []dto.DashboardChannelHealth
	RecentActivity []dto.DashboardRecentActivity
	ActiveKeyCount int
	UnusedKeyCount int
	HighestLatency dto.DashboardChannelHealth
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
		ActiveKeyCount:        agg.ActiveKeyCount,
		UnusedKeyCount:        agg.UnusedKeyCount,
		RecentFailureCount:    agg.RecentFailures,
		RecentRequestCount:    agg.RecentRequests,
		HighestLatencyChannel: agg.HighestLatency,
		TotalQuota:            agg.RecentQuota,
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
		var quotaRows []*model.QuotaData
		err := model.DB.Table("quota_data").
			Select("user_id, username, model_name, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used").
			Where("created_at >= ? and created_at <= ?", start, end).
			Group("user_id, username, model_name, created_at").
			Find(&quotaRows).Error
		if err != nil {
			return nil, err
		}
		model.FillQuotaDataDisplayNames(quotaRows)
		return quotaRows, nil
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
	isAdmin := role >= common.RoleAdminUser
	if !isAdmin {
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
		item := dto.DashboardRecentActivity{
			ID:        log.Id,
			CreatedAt: log.CreatedAt,
			Type:      log.Type,
			Content:   log.Content,
			ModelName: log.ModelName,
			TokenName: log.TokenName,
			Quota:     log.Quota,
			UseTime:   log.UseTime,
		}
		if isAdmin {
			item.ChannelID = log.ChannelId
			item.ChannelName = log.ChannelName
		}
		items = append(items, item)
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
