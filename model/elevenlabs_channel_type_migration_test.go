package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Type 61 was ElevenLabs before upstream assigned it to TaskPlugin. The
// migration must move only ElevenLabs channels and run a single time, so a
// task plugin channel created afterwards keeps type 61.
func TestMigrateElevenLabsChannelType(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}, &Option{}))

	elevenLabsURL := "https://api.elevenlabs.io"
	emptyURL := ""
	pluginURL := "https://plugin.example.com"
	channels := []Channel{
		{Name: "elevenlabs default url", Type: legacyElevenLabsChannelType, BaseURL: &emptyURL},
		{Name: "elevenlabs explicit url", Type: legacyElevenLabsChannelType, BaseURL: &elevenLabsURL},
		{Name: "already task plugin", Type: legacyElevenLabsChannelType, BaseURL: &pluginURL},
		{Name: "openai", Type: constant.ChannelTypeOpenAI, BaseURL: &emptyURL},
	}
	require.NoError(t, db.Create(&channels).Error)

	require.NoError(t, MigrateElevenLabsChannelType(db))

	typeOf := func(name string) int {
		var channel Channel
		require.NoError(t, db.Where("name = ?", name).First(&channel).Error)
		return channel.Type
	}
	assert.Equal(t, constant.ChannelTypeElevenLabs, typeOf("elevenlabs default url"))
	assert.Equal(t, constant.ChannelTypeElevenLabs, typeOf("elevenlabs explicit url"))
	assert.Equal(t, legacyElevenLabsChannelType, typeOf("already task plugin"))
	assert.Equal(t, constant.ChannelTypeOpenAI, typeOf("openai"))

	taskPlugin := Channel{Name: "new task plugin", Type: constant.ChannelTypeTaskPlugin, BaseURL: &emptyURL}
	require.NoError(t, db.Create(&taskPlugin).Error)
	require.NoError(t, MigrateElevenLabsChannelType(db))
	assert.Equal(t, constant.ChannelTypeTaskPlugin, typeOf("new task plugin"))
}
