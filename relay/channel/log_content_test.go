package channel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	common2 "github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service/logcontent"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogContentAttemptChecksUpstreamMultipart(t *testing.T) {
	previous := common2.LogContentCaptureEnabled
	common2.LogContentCaptureEnabled = true
	t.Cleanup(func() { common2.LogContentCaptureEnabled = previous })
	info := &common.RelayInfo{RelayMode: constant.RelayModeChatCompletions, RelayFormat: types.RelayFormatOpenAI, ChannelMeta: &common.ChannelMeta{}}

	for _, tc := range []struct {
		contentType string
		followed    bool
	}{
		{"application/json", true},
		{"multipart/form-data; boundary=x", false},
	} {
		gin.SetMode(gin.TestMode)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		// The client body is JSON; only what goes upstream decides.
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
		c.Request.Header.Set("Content-Type", "application/json")
		capture := logcontent.Begin(c, types.RelayFormatOpenAI)
		require.NotNil(t, capture)
		req, err := http.NewRequest(http.MethodPost, "https://upstream.example/v1/chat/completions", strings.NewReader(`{}`))
		require.NoError(t, err)
		req.Header.Set("Content-Type", tc.contentType)
		assert.Equal(t, tc.followed, beginLogContentAttempt(c, info, req) != nil, tc.contentType)
		capture.Finish()
	}
}

func TestLogContentStreamEnd(t *testing.T) {
	assert.Equal(t, logcontent.StreamEndUnknown, logContentStreamEnd(nil))
	for reason, want := range map[common.StreamEndReason]logcontent.StreamEnd{
		common.StreamEndReasonDone:        logcontent.StreamEndComplete,
		common.StreamEndReasonHandlerStop: logcontent.StreamEndComplete,
		common.StreamEndReasonEOF:         logcontent.StreamEndUnknown,
		common.StreamEndReasonNone:        logcontent.StreamEndUnknown,
		common.StreamEndReasonTimeout:     logcontent.StreamEndCut,
		common.StreamEndReasonScannerErr:  logcontent.StreamEndCut,
		common.StreamEndReasonPanic:       logcontent.StreamEndCut,
		common.StreamEndReasonPingFail:    logcontent.StreamEndCut,
		common.StreamEndReasonClientGone:  logcontent.StreamEndCut,
	} {
		status := common.NewStreamStatus()
		status.SetEndReason(reason, nil)
		assert.Equal(t, want, logContentStreamEnd(status), string(reason))
	}
}

func TestSpeaksCapturedStreamFormat(t *testing.T) {
	assert.True(t, speaksCapturedStreamFormat(appconstant.APITypeOpenAI))
	assert.True(t, speaksCapturedStreamFormat(appconstant.APITypeAnthropic))
	assert.True(t, speaksCapturedStreamFormat(appconstant.APITypeGemini))
	for _, apiType := range []int{appconstant.APITypeDify, appconstant.APITypeCoze, appconstant.APITypeCohere, appconstant.APITypeAli} {
		assert.False(t, speaksCapturedStreamFormat(apiType), apiType)
	}
}
