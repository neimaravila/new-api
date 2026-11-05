package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetAllLogSummary totals every user's requests matching the search.
func GetAllLogSummary(c *gin.Context) {
	logSummary(c, true)
}

// GetUserLogSummary totals the caller's own requests matching the search.
func GetUserLogSummary(c *gin.Context) {
	logSummary(c, false)
}

func logSummary(c *gin.Context, all bool) {
	filter := logFilterFromQuery(c, all)
	if filter.StartTimestamp < 0 || filter.EndTimestamp < 0 {
		common.ApiErrorT(c, "Invalid time range")
		return
	}
	if !all && c.GetInt("role") < common.RoleAdminUser && filter.StartTimestamp != 0 {
		if _, err := limitLogTimeRange(filter.StartTimestamp, filter.EndTimestamp); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	userId := 0
	if !all {
		userId = c.GetInt("id")
	}
	summary, err := model.GetLogSummary(userId, filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"requests":             summary.Requests,
		"successes":            summary.Successes,
		"failures":             summary.Failures,
		"tokens":               summary.Tokens,
		"quota":                summary.Quota,
		"avg_use_time_seconds": summary.AvgUseTimeSeconds,
		"bucket_seconds":       summary.BucketSeconds,
		"buckets":              summary.Buckets,
		"error_log_enabled":    constant.ErrorLogEnabled,
	})
}
