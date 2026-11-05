package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func putLogContentRetention(t *testing.T, value string) bool {
	t.Helper()
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"LogContentRetentionDays","value":"`+value+`"}`),
	)
	UpdateOption(context)
	var payload struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	return payload.Success
}

func TestUpdateOptionRejectsInvalidRetention(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	previous := common.LogContentRetentionDays
	previousMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.LogContentRetentionDays = previous
		common.OptionMap = previousMap
	})
	common.LogContentRetentionDays = 30

	for _, value := range []string{"0", "-1", "3651", "abc"} {
		assert.False(t, putLogContentRetention(t, value), value)
		assert.Equal(t, 30, common.LogContentRetentionDays, value)
	}
	assert.True(t, putLogContentRetention(t, "1"))
	assert.Equal(t, 1, common.LogContentRetentionDays)
	assert.True(t, putLogContentRetention(t, "3650"))
	assert.Equal(t, 3650, common.LogContentRetentionDays)
}
