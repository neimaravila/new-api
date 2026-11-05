package helper

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service/logcontent"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeepAliveOverCaptureWriter(t *testing.T) {
	previous := common.LogContentCaptureEnabled
	common.LogContentCaptureEnabled = true
	t.Cleanup(func() { common.LogContentCaptureEnabled = previous })

	writer := &headerCommitWriter{ResponseRecorder: httptest.NewRecorder(), headerStarted: make(chan struct{})}
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	capture := logcontent.Begin(c, types.RelayFormatClaude)
	require.NotNil(t, capture)
	defer capture.Finish()

	stop := StartStreamKeepAlive(c, time.Millisecond)
	defer stop()
	select {
	case <-writer.headerStarted:
	case <-time.After(time.Second):
		t.Fatal("keepalive did not start")
	}
	_, isKeepAlive := c.Writer.(*streamKeepAliveWriter)
	require.True(t, isKeepAlive, "the keepalive writer stays on top, so the scanner can stop it")

	resp := &http.Response{Header: http.Header{}, Body: io.NopCloser(strings.NewReader("data: chunk\n\ndata: [DONE]\n\n"))}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	var received []string
	StreamScannerHandler(c, resp, info, func(data string, _ *StreamResult) {
		received = append(received, data)
		assert.NoError(t, StringData(c, data))
	})
	stop()

	assert.Equal(t, []string{"chunk"}, received)
	output := writer.Body.String()
	assert.True(t, strings.HasPrefix(output, ": PING\n\n"), output)
	assert.True(t, strings.HasSuffix(output, "data: chunk\n\n"), output)
	assert.Equal(t, output, string(capture.ClientCopy()), "the copy holds pings and data in output order")
}
