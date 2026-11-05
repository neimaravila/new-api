package controller

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetReportSummary returns the role-scoped logs aggregation payload for the reports UI.
func GetReportSummary(c *gin.Context) {
	userID := c.GetInt("id")
	username := c.GetString("username")
	role := dashboardViewerRole(c)

	timeRange := dto.ReportTimeRange(c.DefaultQuery("range", string(dto.ReportRangeLast7Days)))
	granularity := dto.ReportGranularity(c.DefaultQuery("granularity", ""))

	summary, err := service.GetReportSummary(userID, username, role, timeRange, granularity, reportFilterFromQuery(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

// GetReportRanking returns the top entries of one dimension (channel, model, user, token, group).
func GetReportRanking(c *gin.Context) {
	userID := c.GetInt("id")
	username := c.GetString("username")
	role := dashboardViewerRole(c)

	timeRange := dto.ReportTimeRange(c.DefaultQuery("range", string(dto.ReportRangeLast7Days)))
	dimension := dto.ReportDimension(c.Query("dimension"))

	rows, err := service.GetReportRanking(userID, username, role, timeRange, dimension, reportFilterFromQuery(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// GetReportExport streams the report as a CSV download.
func GetReportExport(c *gin.Context) {
	userID := c.GetInt("id")
	username := c.GetString("username")
	role := dashboardViewerRole(c)

	timeRange := dto.ReportTimeRange(c.DefaultQuery("range", string(dto.ReportRangeLast7Days)))
	granularity := dto.ReportGranularity(c.DefaultQuery("granularity", ""))

	summary, err := service.GetReportSummary(userID, username, role, timeRange, granularity, reportFilterFromQuery(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	csv := service.RenderReportCSV(summary)
	filename := reportExportFilename(time.Now(), reportFilterFromQuery(c), role)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Header("Content-Length", strconv.Itoa(len(csv)))
	c.String(http.StatusOK, csv)
}

// dashboardViewerRole is the role the dashboard and report aggregations scope
// to. A scoped access token reaches these routes with usage:read, so it gets
// the site-wide admin view only when it also holds log:read, the scope that
// guards the admin log and data routes.
func dashboardViewerRole(c *gin.Context) int {
	role := c.GetInt("role")
	if role < common.RoleAdminUser || c.GetBool("access_token_legacy") {
		return role
	}
	scopes, scoped := c.Get("access_token_scopes")
	if !scoped {
		return role
	}
	granted, _ := scopes.([]string)
	if service.AccessTokenScopeGranted(granted, "log:read") {
		return role
	}
	return common.RoleCommonUser
}

// reportFilterFromQuery reads the optional model_name and channel filters.
func reportFilterFromQuery(c *gin.Context) dto.ReportFilter {
	channelID, _ := strconv.Atoi(c.Query("channel"))
	if channelID < 0 {
		channelID = 0
	}
	return dto.ReportFilter{ModelName: c.Query("model_name"), ChannelID: channelID}
}

// reportExportFilename names the download after the date and, when filtered,
// the model and channel, restricted to characters safe in a header.
func reportExportFilename(now time.Time, filter dto.ReportFilter, role int) string {
	name := "report-" + now.UTC().Format("2006-01-02")
	if filter.ModelName != "" {
		var b strings.Builder
		for _, r := range filter.ModelName {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-':
				b.WriteRune(r)
			default:
				b.WriteByte('_')
			}
			if b.Len() >= 60 {
				break
			}
		}
		name += "-model-" + b.String()
	}
	if filter.ChannelID > 0 && role >= common.RoleAdminUser {
		name += "-channel-" + strconv.Itoa(filter.ChannelID)
	}
	return name + ".csv"
}
