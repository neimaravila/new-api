package elevenlabs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		baseURL   string
		relayMode int
		request   *dto.AudioRequest
		want      string
		wantErr   bool
	}{
		{
			name:      "voice in path with mapped output format",
			baseURL:   "https://api.elevenlabs.io",
			relayMode: relayconstant.RelayModeAudioSpeech,
			request:   &dto.AudioRequest{Voice: "21m00Tcm4TlvDq8ikWAM", ResponseFormat: "mp3"},
			want:      "https://api.elevenlabs.io/v1/text-to-speech/21m00Tcm4TlvDq8ikWAM?output_format=mp3_44100_128",
		},
		{
			name:      "native output format passes through",
			baseURL:   "https://api.elevenlabs.io",
			relayMode: relayconstant.RelayModeAudioSpeech,
			request:   &dto.AudioRequest{Voice: "voice-1", ResponseFormat: "pcm_44100"},
			want:      "https://api.elevenlabs.io/v1/text-to-speech/voice-1?output_format=pcm_44100",
		},
		{
			name:      "unknown output format is dropped so upstream picks its default",
			baseURL:   "https://api.elevenlabs.io",
			relayMode: relayconstant.RelayModeAudioSpeech,
			request:   &dto.AudioRequest{Voice: "voice-1", ResponseFormat: "flac"},
			want:      "https://api.elevenlabs.io/v1/text-to-speech/voice-1",
		},
		{
			name:      "empty channel base url falls back to the registered default",
			relayMode: relayconstant.RelayModeAudioSpeech,
			request:   &dto.AudioRequest{Voice: "voice-1"},
			want:      channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeElevenLabs] + "/v1/text-to-speech/voice-1",
		},
		{
			name:      "voice is escaped into the path",
			baseURL:   "https://api.elevenlabs.io",
			relayMode: relayconstant.RelayModeAudioSpeech,
			request:   &dto.AudioRequest{Voice: "a b/c"},
			want:      "https://api.elevenlabs.io/v1/text-to-speech/a%20b%2Fc",
		},
		{
			name:      "missing voice is rejected",
			baseURL:   "https://api.elevenlabs.io",
			relayMode: relayconstant.RelayModeAudioSpeech,
			request:   &dto.AudioRequest{},
			wantErr:   true,
		},
		{
			name:      "non-speech relay mode is rejected",
			baseURL:   "https://api.elevenlabs.io",
			relayMode: relayconstant.RelayModeChatCompletions,
			request:   &dto.AudioRequest{Voice: "voice-1"},
			wantErr:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			adaptor := &Adaptor{}
			info := &relaycommon.RelayInfo{
				RelayMode:   test.relayMode,
				Request:     test.request,
				ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: test.baseURL},
			}

			got, err := adaptor.GetRequestURL(info)

			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestSetupRequestHeaderUsesXiApiKey(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeAudioSpeech,
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "sk_eleven_secret"},
	}
	header := http.Header{}

	require.NoError(t, adaptor.SetupRequestHeader(c, &header, info))

	assert.Equal(t, "sk_eleven_secret", header.Get("xi-api-key"))
	assert.Empty(t, header.Get("Authorization"), "ElevenLabs must not receive a bearer token")
}

func TestConvertAudioRequest(t *testing.T) {
	t.Parallel()

	speed := 0.9

	tests := []struct {
		name    string
		request dto.AudioRequest
		want    map[string]any
	}{
		{
			name:    "minimal speech request",
			request: dto.AudioRequest{Model: "eleven_multilingual_v2", Input: "olá mundo", Voice: "voice-1"},
			want: map[string]any{
				"text":     "olá mundo",
				"model_id": "eleven_multilingual_v2",
			},
		},
		{
			name: "speed becomes a voice setting",
			request: dto.AudioRequest{
				Model: "eleven_multilingual_v2",
				Input: "olá mundo",
				Voice: "voice-1",
				Speed: &speed,
			},
			want: map[string]any{
				"text":           "olá mundo",
				"model_id":       "eleven_multilingual_v2",
				"voice_settings": map[string]any{"speed": 0.9},
			},
		},
		{
			name: "metadata tunes the voice but cannot rewrite the billed text or routed model",
			request: dto.AudioRequest{
				Model:    "eleven_multilingual_v2",
				Input:    "olá mundo",
				Voice:    "voice-1",
				Metadata: json.RawMessage(`{"text":"a much longer text the caller was never billed for","model_id":"eleven_v3","language_code":"pt","voice_settings":{"stability":0.5,"use_speaker_boost":false}}`),
			},
			want: map[string]any{
				"text":          "olá mundo",
				"model_id":      "eleven_multilingual_v2",
				"language_code": "pt",
				"voice_settings": map[string]any{
					"stability":         0.5,
					"use_speaker_boost": false,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			adaptor := &Adaptor{}
			c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
			info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeAudioSpeech}

			reader, err := adaptor.ConvertAudioRequest(c, info, test.request)
			require.NoError(t, err)

			body, err := io.ReadAll(reader)
			require.NoError(t, err)

			var payload map[string]any
			require.NoError(t, common.Unmarshal(body, &payload))
			assert.Equal(t, test.want, payload)
		})
	}
}

func TestConvertAudioRequestRejectsNonSpeechMode(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeAudioTranscription}

	_, err := adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{Input: "hi", Voice: "voice-1"})

	require.Error(t, err)
}
