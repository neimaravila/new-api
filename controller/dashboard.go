package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetDashboardSummary(c *gin.Context) {
	userID := c.GetInt("id")
	remainQuota, err := model.GetUserQuota(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary, err := service.GetDashboardSummary(
		userID,
		c.GetString("username"),
		c.GetInt("role"),
		remainQuota,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func GetDashboardInsights(c *gin.Context) {
	userID := c.GetInt("id")
	remainQuota, err := model.GetUserQuota(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	insights, err := service.GetDashboardInsights(
		userID,
		c.GetString("username"),
		c.GetInt("role"),
		remainQuota,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, insights)
}
