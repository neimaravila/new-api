package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedHealthChannels(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	t.Cleanup(func() { DB.Exec("DELETE FROM channels") })

	// `Key` is `not null`, so set it explicitly rather than relying on the zero
	// value satisfying the constraint on every database.
	channels := []*Channel{
		// active, fast
		{Id: 1, Name: "fast", Key: "sk-test", Status: common.ChannelStatusEnabled, TestTime: 1000, ResponseTime: 300},
		// active, exactly at the threshold: NOT slow, the predicate is strictly greater
		{Id: 2, Name: "boundary", Key: "sk-test", Status: common.ChannelStatusEnabled, TestTime: 1000, ResponseTime: SlowResponseTimeMs},
		// active, slow
		{Id: 3, Name: "slow", Key: "sk-test", Status: common.ChannelStatusEnabled, TestTime: 1000, ResponseTime: 2500},
		// active, never tested: untested, and NOT slow despite response_time 0
		{Id: 4, Name: "untested", Key: "sk-test", Status: common.ChannelStatusEnabled, TestTime: 0, ResponseTime: 0},
		// manually disabled and never tested: counted in BOTH disabled and untested
		{Id: 5, Name: "off-untested", Key: "sk-test", Status: common.ChannelStatusManuallyDisabled, TestTime: 0, ResponseTime: 0},
		// auto disabled and slow: counted in BOTH disabled and slow
		{Id: 6, Name: "off-slow", Key: "sk-test", Status: common.ChannelStatusAutoDisabled, TestTime: 1000, ResponseTime: 9000},
	}
	for _, ch := range channels {
		require.NoError(t, DB.Create(ch).Error)
	}
}

func TestGetChannelHealthCounts(t *testing.T) {
	seedHealthChannels(t)

	counts, err := GetChannelHealthCounts()
	require.NoError(t, err)

	assert.Equal(t, int64(4), counts.Active, "ids 1-4 are enabled")
	assert.Equal(t, int64(2), counts.Disabled, "ids 5-6 are disabled, manual and auto alike")
	assert.Equal(t, int64(2), counts.Slow, "ids 3 and 6; the boundary row and the untested rows are excluded")
	assert.Equal(t, int64(2), counts.Untested, "ids 4 and 5")
	assert.Equal(t, int64(SlowResponseTimeMs), counts.SlowThresholdMs)
}

func TestApplyChannelHealthFilterMatchesCounts(t *testing.T) {
	seedHealthChannels(t)

	counts, err := GetChannelHealthCounts()
	require.NoError(t, err)

	cases := []struct {
		health string
		want   int64
	}{
		{health: "slow", want: counts.Slow},
		{health: "untested", want: counts.Untested},
	}
	for _, tc := range cases {
		t.Run(tc.health, func(t *testing.T) {
			var got int64
			require.NoError(t, ApplyChannelHealthFilter(DB.Model(&Channel{}), tc.health).Count(&got).Error)
			assert.Equal(t, tc.want, got, "the tile's number and the list it filters to must agree")
		})
	}
}

func TestHealthFilterFormsAgree(t *testing.T) {
	seedHealthChannels(t)

	var all []*Channel
	require.NoError(t, DB.Order("id").Find(&all).Error)

	// The SQL filter serves the list endpoint; the in-memory predicate serves
	// the keyword branch of SearchChannels, which filters after its query
	// returns. This is the test that stops the two from drifting apart.
	for _, health := range []string{"slow", "untested", "", "unknown"} {
		t.Run(health, func(t *testing.T) {
			var matched []*Channel
			require.NoError(t, ApplyChannelHealthFilter(DB.Model(&Channel{}), health).Order("id").Find(&matched).Error)

			want := make([]int, 0, len(all))
			for _, ch := range all {
				if ChannelMatchesHealth(ch, health) {
					want = append(want, ch.Id)
				}
			}
			got := make([]int, 0, len(matched))
			for _, ch := range matched {
				got = append(got, ch.Id)
			}
			assert.Equal(t, want, got, "the SQL filter and the in-memory predicate must select the same channels")
		})
	}
}

func TestApplyChannelHealthFilterComposesWithStatus(t *testing.T) {
	seedHealthChannels(t)

	// The controller stacks the health filter on top of the status filter. If
	// the health filter ever reset the query instead of narrowing it, this
	// would return the union (ids 4 and 5) instead of the intersection.
	query := DB.Model(&Channel{}).Where("status = ?", common.ChannelStatusEnabled)

	var got int64
	require.NoError(t, ApplyChannelHealthFilter(query, "untested").Count(&got).Error)
	assert.Equal(t, int64(1), got, "only id 4 is both enabled and never tested")
}

func TestApplyChannelHealthFilterIgnoresUnknownValues(t *testing.T) {
	seedHealthChannels(t)

	for _, health := range []string{"", "all", "broken", "SLOW "} {
		t.Run(health, func(t *testing.T) {
			var got int64
			require.NoError(t, ApplyChannelHealthFilter(DB.Model(&Channel{}), health).Count(&got).Error)
			assert.Equal(t, int64(6), got, "an unrecognized value returns the unfiltered list")
		})
	}
}
