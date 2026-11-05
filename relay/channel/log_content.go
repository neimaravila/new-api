package channel

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service/logcontent"

	"github.com/gin-gonic/gin"
)

// beginLogContentAttempt starts following one provider call for content
// capture. Calls in a format the view cannot read, or with a multipart body,
// are followed only to mark the call as not captured.
func beginLogContentAttempt(c *gin.Context, info *common.RelayInfo, req *http.Request) *logcontent.Attempt {
	if c == nil || info == nil || c.Request == nil {
		return nil
	}
	format := logContentFormat(info)
	attempt := logcontent.AttemptInfo{
		Eligible:      format != "" && !strings.HasPrefix(strings.ToLower(c.Request.Header.Get("Content-Type")), "multipart/"),
		MultiKeyIndex: -1,
		Format:        format,
	}
	if info.ChannelMeta != nil {
		attempt.ChannelID = info.ChannelId
		attempt.ChannelKey = info.ApiKey
		if info.ChannelIsMultiKey {
			attempt.MultiKeyIndex = info.ChannelMultiKeyIndex
		}
	}
	return logcontent.BeginAttempt(c, attempt, req)
}

// logContentFormat is the client format of the call as the content view reads
// it, or "" when the call is not captured. Claude messages carry no relay mode
// of their own, so the client format decides them.
func logContentFormat(info *common.RelayInfo) string {
	switch info.RelayMode {
	case constant.RelayModeEmbeddings:
		if info.RelayFormat == types.RelayFormatGemini {
			return logcontent.FormatGemini
		}
		return logcontent.FormatEmbedding
	case constant.RelayModeRerank:
		return logcontent.FormatRerank
	case constant.RelayModeResponses, constant.RelayModeResponsesCompact:
		return logcontent.FormatResponses
	case constant.RelayModeGemini:
		return logcontent.FormatGemini
	case constant.RelayModeChatCompletions, constant.RelayModeCompletions, constant.RelayModeUnknown:
		switch info.RelayFormat {
		case types.RelayFormatClaude:
			return logcontent.FormatClaude
		case types.RelayFormatGemini:
			return logcontent.FormatGemini
		case types.RelayFormatOpenAI:
			if info.RelayMode != constant.RelayModeUnknown {
				return logcontent.FormatOpenAI
			}
		}
	}
	return ""
}
