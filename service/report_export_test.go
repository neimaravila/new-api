package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderReportCSVContainsAllSections(t *testing.T) {
	setupDashboardServiceTestDB(t)
	seedReportFixture(t)

	summary, err := GetReportSummary(99, "admin", common.RoleAdminUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour)
	require.NoError(t, err)

	csv := RenderReportCSV(summary)
	require.NotEmpty(t, csv)

	// All report sections are present.
	for _, header := range []string{
		"new-api report",
		"section,key,value",
		"trend,bucket,quota",
		"models,model_name,quota",
		"errors,model_name,failures",
		"model_performance,name,avg_latency_ms",
		"token_anatomy,model_name,prompt_tokens",
		"channels,channel_id,channel_name",
		"stream_comparison,stream_avg_latency_ms",
	} {
		assert.True(t, strings.Contains(csv, header), "missing CSV section header: %s", header)
	}

	// Seeded data appears in the right sections.
	assert.Contains(t, csv, "models,gpt-4o,")
	assert.Contains(t, csv, "token_anatomy,claude,")
}

func TestRenderReportCSVQuotesCommas(t *testing.T) {
	s := dto.ReportSummary{
		Models: []dto.ReportModelRow{
			{ModelName: "gpt-4o, mini", Quota: 10},
		},
		TokenAnatomy: []dto.ReportTokenAnatomyRow{
			{ModelName: "model \"quoted\"", Total: 5},
		},
	}
	csv := RenderReportCSV(s)
	assert.Contains(t, csv, `"gpt-4o, mini"`)
	assert.Contains(t, csv, `"model ""quoted"""`)
}

func TestRenderReportCSVOmitsAdminSectionsForUser(t *testing.T) {
	setupDashboardServiceTestDB(t)
	seedReportFixture(t)

	summary, err := GetReportSummary(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour)
	require.NoError(t, err)
	csv := RenderReportCSV(summary)

	// Regular user payload has no channels, so the channels section header is absent.
	assert.NotContains(t, csv, "channels,channel_id,channel_name")
}
