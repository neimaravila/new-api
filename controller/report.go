package controller

import (
	"net/http"
	"strconv"
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
	role := c.GetInt("role")

	timeRange := dto.ReportTimeRange(c.DefaultQuery("range", string(dto.ReportRangeLast7Days)))
	granularity := dto.ReportGranularity(c.DefaultQuery("granularity", ""))

	summary, err := service.GetReportSummary(userID, username, role, timeRange, granularity)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

// GetReportExport streams the report as a CSV download.
func GetReportExport(c *gin.Context) {
	userID := c.GetInt("id")
	username := c.GetString("username")
	role := c.GetInt("role")

	timeRange := dto.ReportTimeRange(c.DefaultQuery("range", string(dto.ReportRangeLast7Days)))
	granularity := dto.ReportGranularity(c.DefaultQuery("granularity", ""))

	summary, err := service.GetReportSummary(userID, username, role, timeRange, granularity)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	csv := service.RenderReportCSV(summary)
	filename := "report-" + time.Now().UTC().Format("2006-01-02") + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Header("Content-Length", strconv.Itoa(len(csv)))
	c.String(http.StatusOK, csv)
}
