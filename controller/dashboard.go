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
