package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

// legacyElevenLabsChannelType is the channel type ElevenLabs used before
// upstream took 61 for ChannelTypeTaskPlugin.
const legacyElevenLabsChannelType = 61

const elevenLabsChannelTypeMigrationKey = "migration.elevenlabs_channel_type_64"

// MigrateElevenLabsChannelType moves ElevenLabs channels saved with the legacy
// type 61 to ChannelTypeElevenLabs. Type 61 now means TaskPlugin, so this must
// run exactly once, before any task plugin channel can exist: a marker option
// records completion. Only channels that point at the ElevenLabs API (or the
// empty default base URL) are moved, so a task plugin channel is never touched.
func MigrateElevenLabsChannelType(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var marker Option
		err := tx.Where(&Option{Key: elevenLabsChannelTypeMigrationKey}).First(&marker).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		result := tx.Model(&Channel{}).
			Where("type = ?", legacyElevenLabsChannelType).
			Where("base_url IS NULL OR base_url = ? OR base_url LIKE ?", "", "%elevenlabs.io%").
			Update("type", constant.ChannelTypeElevenLabs)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			common.SysLog(common.LogText("migrated %d ElevenLabs channels from type %d to %d", result.RowsAffected, legacyElevenLabsChannelType, constant.ChannelTypeElevenLabs))
		}
		return tx.Create(&Option{Key: elevenLabsChannelTypeMigrationKey, Value: "done"}).Error
	})
}
