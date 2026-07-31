package elevenlabs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

// outputFormatMap maps OpenAI-style response_format values to ElevenLabs
// output_format query values. Values that already look like an ElevenLabs
// output format (containing "_") are passed through unchanged.
var outputFormatMap = map[string]string{
	"mp3":  "mp3_44100_128",
	"pcm":  "pcm_24000",
	"opus": "opus_48000_128",
	"ulaw": "ulaw_8000",
}

func mapOutputFormat(responseFormat string) string {
	if mapped, ok := outputFormatMap[responseFormat]; ok {
		return mapped
	}
	if strings.Contains(responseFormat, "_") {
		return responseFormat
	}
	return ""
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode != constant.RelayModeAudioSpeech {
		return nil, errors.New("unsupported audio relay mode")
	}

	modelID := request.Model
	if modelID == "" {
		modelID = info.UpstreamModelName
	}

	elevenLabsRequest := ElevenLabsTTSRequest{
		Text:    request.Input,
		ModelID: modelID,
	}
	if request.Speed != nil {
		elevenLabsRequest.VoiceSettings = &ElevenLabsVoiceSettings{
			Speed: request.Speed,
		}
	}

	// metadata carries provider-specific tuning (voice_settings, language_code,
	// seed). It must not rewrite the text the request was billed on, nor the
	// model the request was routed and priced by.
	if len(request.Metadata) > 0 {
		if err := common.Unmarshal(request.Metadata, &elevenLabsRequest); err != nil {
			return nil, fmt.Errorf("error unmarshalling metadata to elevenlabs request: %w", err)
		}
		elevenLabsRequest.Text = request.Input
		elevenLabsRequest.ModelID = modelID
	}

	jsonData, err := common.Marshal(elevenLabsRequest)
	if err != nil {
		return nil, fmt.Errorf("error marshalling elevenlabs request: %w", err)
	}

	return bytes.NewReader(jsonData), nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseUrl := info.ChannelBaseUrl
	if baseUrl == "" {
		baseUrl = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeElevenLabs]
	}
	switch info.RelayMode {
	case constant.RelayModeAudioSpeech:
		voiceID := ""
		outputFormat := ""
		if audioReq, ok := info.Request.(*dto.AudioRequest); ok {
			voiceID = audioReq.Voice
			outputFormat = mapOutputFormat(audioReq.ResponseFormat)
		}
		if voiceID == "" {
			return "", errors.New("voice is required for elevenlabs text-to-speech")
		}
		requestURL := fmt.Sprintf("%s/v1/text-to-speech/%s", baseUrl, url.PathEscape(voiceID))
		if outputFormat != "" {
			requestURL += "?output_format=" + url.QueryEscape(outputFormat)
		}
		return requestURL, nil
	default:
		return "", fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("xi-api-key", info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == constant.RelayModeAudioSpeech {
		return handleTTSResponse(c, resp, info)
	}
	return nil, types.NewError(
		fmt.Errorf("unsupported relay mode: %d", info.RelayMode),
		types.ErrorCodeInvalidRequest,
	)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
