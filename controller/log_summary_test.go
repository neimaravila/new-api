package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func callLogSummary(t *testing.T, handler gin.HandlerFunc, userId int, role int, query string) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/summary?"+query, nil)
	ctx.Set("id", userId)
	ctx.Set("role", role)
	handler(ctx)
	var body map[string]any
	require.NoError(t, common.UnmarshalJsonStr(recorder.Body.String(), &body))
	return body
}

func TestLogSummarySelfRouteIgnoresUsernameAndChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := newAuditTestDatabase(t, "sqlite", "")
	useLogExportDatabases(t, db, db, common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Log{}))
	now := time.Now().Unix()
	for _, l := range []model.Log{
		{UserId: 1, Username: "alice", CreatedAt: now - 60, Type: model.LogTypeConsume, ChannelId: 9},
		{UserId: 1, Username: "alice", CreatedAt: now - 50, Type: model.LogTypeError, ChannelId: 9},
		{UserId: 2, Username: "bob", CreatedAt: now - 40, Type: model.LogTypeConsume, ChannelId: 8},
	} {
		require.NoError(t, db.Create(&l).Error)
	}

	self := callLogSummary(t, GetUserLogSummary, 1, common.RoleCommonUser, "username=bob&channel=8")
	require.Equal(t, true, self["success"])
	data := self["data"].(map[string]any)
	assert.EqualValues(t, 2, data["requests"])
	assert.Contains(t, data, "error_log_enabled")

	admin := callLogSummary(t, GetAllLogSummary, 3, common.RoleAdminUser, "username=bob")
	assert.EqualValues(t, 1, admin["data"].(map[string]any)["requests"])

	tooLong := callLogSummary(t, GetUserLogSummary, 1, common.RoleCommonUser, "start_timestamp=1")
	assert.Equal(t, false, tooLong["success"])

	negative := callLogSummary(t, GetUserLogSummary, 1, common.RoleCommonUser, "start_timestamp=-9223372036854775808")
	assert.Equal(t, false, negative["success"])
	negative = callLogSummary(t, GetAllLogSummary, 3, common.RoleAdminUser, "end_timestamp=-1")
	assert.Equal(t, false, negative["success"])
}
