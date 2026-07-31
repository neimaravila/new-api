package elevenlabs

import (
	"fmt"
	"io"
	"net/http"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// ElevenLabsTTSRequest is the upstream body of POST /v1/text-to-speech/{voice_id}.
// Optional scalar fields use pointers with omitempty so that absent client
// values are omitted while explicit zero values are preserved.
type ElevenLabsTTSRequest struct {
	Text          string                   `json:"text"`
	ModelID       string                   `json:"model_id"`
	LanguageCode  string                   `json:"language_code,omitempty"`
	Seed          *int                     `json:"seed,omitempty"`
	VoiceSettings *ElevenLabsVoiceSettings `json:"voice_settings,omitempty"`
}

type ElevenLabsVoiceSettings struct {
	Stability       *float64 `json:"stability,omitempty"`
	SimilarityBoost *float64 `json:"similarity_boost,omitempty"`
	Style           *float64 `json:"style,omitempty"`
	Speed           *float64 `json:"speed,omitempty"`
	UseSpeakerBoost *bool    `json:"use_speaker_boost,omitempty"`
}

// ElevenLabsModel is one entry of the GET /v1/models response array.
type ElevenLabsModel struct {
	ModelID string `json:"model_id"`
	Name    string `json:"name"`
}

func handleTTSResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to read elevenlabs response: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}

	// ElevenLabs returns the audio binary directly (audio/mpeg by default).
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	c.Data(http.StatusOK, contentType, body)

	// TTS billing convention: the estimated prompt tokens are the input text
	// token/character count consumed downstream by service.PostAudioConsumeQuota.
	usage = &dto.Usage{
		PromptTokens:     info.GetEstimatePromptTokens(),
		CompletionTokens: 0,
		TotalTokens:      info.GetEstimatePromptTokens(),
	}

	return usage, nil
}
