package channel

import (
	"net/http"
	"strings"

	appconstant "github.com/QuantumNous/new-api/constant"
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
	if c == nil || info == nil || req == nil {
		return nil
	}
	format := logContentFormat(info)
	attempt := logcontent.AttemptInfo{
		Eligible:      format != "" && !strings.HasPrefix(strings.ToLower(req.Header.Get("Content-Type")), "multipart/"),
		MultiKeyIndex: -1,
		Format:        format,
		StreamEnd: func() logcontent.StreamEnd {
			return logContentStreamEnd(info.StreamStatus)
		},
	}
	if info.ChannelMeta != nil {
		attempt.ChannelID = info.ChannelId
		attempt.ChannelKey = info.ApiKey
		if info.ChannelIsMultiKey {
			attempt.MultiKeyIndex = info.ChannelMultiKeyIndex
		}
		attempt.KnownStreamFormat = speaksCapturedStreamFormat(info.ApiType)
	}
	for name := range common.GetEffectiveHeaderOverride(info) {
		name = strings.TrimSpace(name)
		lower := strings.ToLower(name)
		if name == "" || name == headerPassthroughAllKey ||
			strings.HasPrefix(lower, headerPassthroughRegexPrefix) || strings.HasPrefix(lower, headerPassthroughRegexPrefixV2) {
			continue
		}
		attempt.SecretHeaders = append(attempt.SecretHeaders, name)
	}
	return logcontent.BeginAttempt(c, attempt, req)
}

// logContentStreamEnd classifies how the relay's stream scanner ended. EOF
// is normal for formats without a final sentinel, so it decides nothing.
func logContentStreamEnd(status *common.StreamStatus) logcontent.StreamEnd {
	if status == nil {
		return logcontent.StreamEndUnknown
	}
	switch status.OutcomeSnapshot().EndReason {
	case common.StreamEndReasonDone, common.StreamEndReasonHandlerStop:
		return logcontent.StreamEndComplete
	case common.StreamEndReasonTimeout, common.StreamEndReasonScannerErr, common.StreamEndReasonPanic,
		common.StreamEndReasonPingFail, common.StreamEndReasonClientGone:
		return logcontent.StreamEndCut
	}
	return logcontent.StreamEndUnknown
}

// speaksCapturedStreamFormat reports whether a provider streams in one of the
// formats whose end frames the capture knows: OpenAI, Responses, Claude or
// Gemini. Others may end in frames of their own and are never judged.
func speaksCapturedStreamFormat(apiType int) bool {
	switch apiType {
	case appconstant.APITypeOpenAI, appconstant.APITypeAnthropic, appconstant.APITypeGemini,
		appconstant.APITypeVertexAi, appconstant.APITypeCodex, appconstant.APITypeOpenRouter,
		appconstant.APITypeDeepSeek, appconstant.APITypeXai:
		return true
	}
	return false
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
