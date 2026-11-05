package controller

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service/logcontent"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	logContentRequestIdMaxLen = 64
	// logContentPendingSeconds is how long after its log row a call's content
	// may still be on its way to the database.
	logContentPendingSeconds = 15

	logContentReasonPending     = "pending"
	logContentReasonExpired     = "expired"
	logContentReasonNotCaptured = "not_captured"
	logContentReasonUnavailable = "unavailable"
)

// GetLogContent returns the captured content of any user's call, with the
// provider attempts, and records the read in the audit log.
func GetLogContent(c *gin.Context) {
	logContent(c, true)
}

// GetSelfLogContent returns the caller's own captured content, without the
// provider attempts, whatever the caller's role.
func GetSelfLogContent(c *gin.Context) {
	logContent(c, false)
}

// GetLogContentCaptureStats returns this node's capture counters.
func GetLogContentCaptureStats(c *gin.Context) {
	common.ApiSuccess(c, logcontent.Stats())
}

// logContent answers both content routes. Every way of not finding a call,
// including a call of another user on the self route, gives the same response.
func logContent(c *gin.Context, admin bool) {
	requestID := c.Param("request_id")
	userID := 0
	if !admin {
		userID = c.GetInt("id")
	}
	if requestID == "" || len(requestID) > logContentRequestIdMaxLen || (!admin && userID <= 0) {
		common.ApiErrorT(c, "Log content not found")
		return
	}
	ctx := c.Request.Context()
	logs, err := model.GetLogRowsForContent(ctx, requestID, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(logs) == 0 {
		common.ApiErrorT(c, "Log content not found")
		return
	}
	// The admin reads the content of the call's owner: a request_id repeated
	// by another user never mixes in. The self route reads only the client
	// level, so provider bodies never leave the backend for it.
	var rows []*model.LogContent
	if admin {
		rows, err = model.GetLogContents(ctx, requestID, logs[0].UserId)
	} else {
		rows, err = model.GetClientLogContent(ctx, requestID, userID)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var channelNames map[int]string
	if admin {
		channelNames, err = logContentChannelNames(rows)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	view := logcontent.BuildContentView(rows, admin, channelNames)
	view.RequestId = requestID
	if len(rows) == 0 {
		view.Reason = logContentReason(logs, time.Now().Unix())
	}

	// Only a read that returns content is audited: pending polls and calls
	// without content show nothing to audit.
	if admin && view.Available {
		recordManageAudit(c, "log_content.view", map[string]any{"request_id": requestID, "owner_user_id": logs[0].UserId})
	}
	common.ApiSuccess(c, view)
}

// logContentChannelNames reads the names of the channels the attempts used
// from the main database, which may differ from the log database.
func logContentChannelNames(rows []*model.LogContent) (map[int]string, error) {
	var ids []int
	for _, row := range rows {
		if row.Attempt == 0 {
			continue
		}
		meta := model.LogContentMeta{}
		if common.UnmarshalJsonStr(row.Meta, &meta) == nil && meta.ChannelId > 0 {
			ids = append(ids, meta.ChannelId)
		}
	}
	names := make(map[int]string)
	if len(ids) == 0 {
		return names, nil
	}
	var channels []struct {
		Id   int
		Name string
	}
	if err := model.DB.Table("channels").Select("id, name").Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return nil, err
	}
	for _, channel := range channels {
		names[channel.Id] = channel.Name
	}
	return names, nil
}

// logContentReason explains why a call that has log rows has no stored
// content, given the current Unix time.
func logContentReason(logs []*model.Log, now int64) string {
	var createdAt int64
	requestPath := ""
	for _, log := range logs {
		createdAt = max(createdAt, log.CreatedAt)
		if path := gjson.Get(log.Other, "request_path").String(); path != "" {
			requestPath = path
		}
	}
	if requestPath != "" && !logContentPathCaptured(requestPath) {
		return logContentReasonNotCaptured
	}
	if common.LogContentCaptureEnabled && now-createdAt < logContentPendingSeconds {
		return logContentReasonPending
	}
	days := common.LogContentRetentionDays
	if days < 1 {
		days = 30
	}
	if createdAt < now-int64(days)*86400 {
		return logContentReasonExpired
	}
	return logContentReasonUnavailable
}

// logContentPathCaptured reports whether calls to a relay path can have
// their content captured: chat, completions, Responses, Claude messages,
// Gemini, embeddings and rerank.
func logContentPathCaptured(path string) bool {
	switch relayconstant.Path2RelayMode(path) {
	case relayconstant.RelayModeChatCompletions, relayconstant.RelayModeCompletions,
		relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact,
		relayconstant.RelayModeEmbeddings, relayconstant.RelayModeRerank, relayconstant.RelayModeGemini:
		return true
	case relayconstant.RelayModeUnknown:
		return strings.HasPrefix(path, "/v1/messages")
	}
	return false
}
