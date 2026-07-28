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
	ID          string                   `json:"id"`
	Severity    DashboardInsightSeverity `json:"severity"`
	Title       string                   `json:"title"`
	Description string                   `json:"description"`
	MetricLabel string                   `json:"metric_label,omitempty"`
	MetricValue string                   `json:"metric_value,omitempty"`
	Action      *DashboardInsightAction  `json:"action,omitempty"`
	EntityType  string                   `json:"entity_type,omitempty"`
	EntityID    string                   `json:"entity_id,omitempty"`
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
