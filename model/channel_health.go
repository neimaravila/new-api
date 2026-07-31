package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// SlowResponseTimeMs mirrors RESPONSE_TIME_THRESHOLDS.GOOD in
// web/src/features/channels/constants.ts. A channel the table badges "Fair" or
// worse is the same channel the health strip counts as slow. The value is
// returned to the client so the strip labels itself from the server's number
// instead of keeping a second copy.
const SlowResponseTimeMs = 1000

// ChannelHealthCounts summarizes every channel in the instance, not one page.
// The buckets deliberately overlap: Active and Disabled partition the table,
// but Slow and Untested are qualifiers a disabled channel can also carry.
type ChannelHealthCounts struct {
	Active          int64 `json:"active"`
	Disabled        int64 `json:"disabled"`
	Slow            int64 `json:"slow"`
	Untested        int64 `json:"untested"`
	SlowThresholdMs int64 `json:"slow_threshold_ms"`
}

func GetChannelHealthCounts() (ChannelHealthCounts, error) {
	counts := ChannelHealthCounts{SlowThresholdMs: SlowResponseTimeMs}

	base := func() *gorm.DB { return DB.Model(&Channel{}) }

	if err := base().Where("status = ?", common.ChannelStatusEnabled).Count(&counts.Active).Error; err != nil {
		return ChannelHealthCounts{}, err
	}
	if err := base().Where("status != ?", common.ChannelStatusEnabled).Count(&counts.Disabled).Error; err != nil {
		return ChannelHealthCounts{}, err
	}
	if err := ApplyChannelHealthFilter(base(), "slow").Count(&counts.Slow).Error; err != nil {
		return ChannelHealthCounts{}, err
	}
	if err := ApplyChannelHealthFilter(base(), "untested").Count(&counts.Untested).Error; err != nil {
		return ChannelHealthCounts{}, err
	}
	return counts, nil
}

// ApplyChannelHealthFilter narrows a channel query to one health bucket.
// Unrecognized values return the query untouched, matching how the existing
// type and status filters treat input they cannot parse.
func ApplyChannelHealthFilter(query *gorm.DB, health string) *gorm.DB {
	switch strings.ToLower(health) {
	case "slow":
		// test_time > 0 excludes never-tested channels, whose response_time is
		// 0 and which belong in the untested bucket instead.
		return query.Where("test_time > ? AND response_time > ?", 0, SlowResponseTimeMs)
	case "untested":
		return query.Where("test_time = ?", 0)
	default:
		return query
	}
}

// ChannelMatchesHealth is the in-memory form of the same rule, for the keyword
// branch of SearchChannels, which filters in Go after its query returns.
// TestHealthFilterFormsAgree pins the two forms to the same answer, so neither
// can be changed alone.
func ChannelMatchesHealth(channel *Channel, health string) bool {
	switch strings.ToLower(health) {
	case "slow":
		return channel.TestTime > 0 && int64(channel.ResponseTime) > SlowResponseTimeMs
	case "untested":
		return channel.TestTime == 0
	default:
		return true
	}
}
