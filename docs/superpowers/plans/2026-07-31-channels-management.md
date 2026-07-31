# Channels Management Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Channels page answer "is anything broken?" at a glance and stop `Tag` and `Groups` from reading as the same concept, without touching the channel editor.

**Architecture:** A new server-side health summary (four counts, one filter parameter) feeds a strip above the existing list. The list itself keeps its current data table and card view; the card is reorganized from column order into three labelled subject rows. Two of the three page-level switches are deleted outright, and the third becomes an explicit grouping control.

**Tech Stack:** Go 1.22 + Gin + GORM v2 (backend), React 19 + TanStack Table/Query/Router + Tailwind + Base UI (frontend), `go test` and `bun test` for tests, i18next for copy.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-07-31-channels-management-design.md`. Read it before Task 1.
- All JSON marshal/unmarshal in Go business code goes through `common.Marshal` / `common.Unmarshal` / `common.UnmarshalJsonStr` / `common.DecodeJson`. Never call `encoding/json` directly.
- All database code must work on SQLite, MySQL >= 5.7.8 and PostgreSQL >= 9.6 simultaneously. Prefer GORM methods over raw SQL. No new raw SQL is needed in this plan.
- Go tests use `github.com/stretchr/testify/require` for setup and fatal assertions, `assert` for value checks.
- Frontend user-facing text uses `useTranslation()` and `t('English source string')`. Locale files are flat JSON at `web/src/i18n/locales/{lang}.json`; languages are en, zh, zh-TW, fr, ru, ja, vi.
- Frontend package manager is `bun`. Run frontend commands from `web/`.
- Frontend tests live in a `__tests__/` directory inside the module they test, never beside production files.
- Every new frontend file needs the project copyright header. Run `bun run copyright` to add it.
- Protected identifiers (`new-api`, `QuantumNous`) must never be renamed, removed, or replaced.
- Slow threshold is **1000 ms**, matching `RESPONSE_TIME_THRESHOLDS.GOOD` in `web/src/features/channels/constants.ts:329-334`.
- No changes to the channel editor drawer, its dialogs, the `Ability` model, or channel selection logic.

---

## File Structure

**Backend — create**

| File | Responsibility |
|-|-|
| `model/channel_health.go` | Health bucket predicates: the counts and the list filter, sharing one definition |
| `model/channel_health_test.go` | Table tests for the predicates and the count-equals-filter invariant |
| `controller/channel_health_test.go` | Table test for `parseHealthFilter` normalization |

**Backend — modify**

| File | Change |
|-|-|
| `controller/channel.go` | `parseHealthFilter`, `health` param on list + search, `health` object on `GetChannelOps`, extra param on `buildChannelListQuery` |

**Frontend — create**

| File | Responsibility |
|-|-|
| `web/src/features/channels/lib/channel-health.ts` | Pure: turn the ops response's health object into the strip's render state |
| `web/src/features/channels/lib/channel-status-info.ts` | Pure: parse `other_info` into `{ statusReason, statusTime }`, never throwing |
| `web/src/features/channels/lib/__tests__/channel-health.test.ts` | Strip state: hidden, calm, alert |
| `web/src/features/channels/lib/__tests__/channel-status-info.test.ts` | Parser fallbacks |
| `web/src/features/channels/components/channel-health-strip.tsx` | Renders the strip and dispatches filter clicks |

**Frontend — modify**

| File | Change |
|-|-|
| `web/src/features/channels/types.ts` | `ChannelOpsResponse.data.health`, `health` on list/search params |
| `web/src/features/channels/api.ts` | Pass `health` through |
| `web/src/features/channels/index.tsx` | Render the strip; share the `channel-ops` query |
| `web/src/features/channels/components/channels-table.tsx` | `health` column filter, drop Status dropdown, drop `batchMode`/`idSort` wiring, group-by control |
| `web/src/features/channels/components/channels-columns.tsx` | Unconditional `select`, status cell uses the shared parser |
| `web/src/features/channels/components/channel-card.tsx` | Three labelled subject rows, Serves row added |
| `web/src/features/channels/components/channels-provider.tsx` | Drop `batchMode` and `idSort`; `enableTagMode` stops persisting |
| `web/src/features/channels/components/channels-primary-buttons.tsx` | Remove two switches, group the actions menu |
| `web/src/features/channels/components/data-table-row-actions.tsx` | Group the row menu by consequence |
| `web/src/features/channels/constants.ts` | Field description copy |
| `web/src/i18n/locales/*.json` | New and renamed keys |

---

### Task 1: Health bucket predicates

The counts and the filter must come from one definition, or a tile's number will disagree with the list it filters to. This task builds both in one file and tests that they agree.

**Files:**
- Create: `model/channel_health.go`
- Create: `model/channel_health_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `const SlowResponseTimeMs = 1000`
  - `type ChannelHealthCounts struct { Active int64; Disabled int64; Slow int64; Untested int64; SlowThresholdMs int64 }` with JSON tags `active`, `disabled`, `slow`, `untested`, `slow_threshold_ms`
  - `func GetChannelHealthCounts() (ChannelHealthCounts, error)`
  - `func ApplyChannelHealthFilter(query *gorm.DB, health string) *gorm.DB`
  - `func ChannelMatchesHealth(channel *Channel, health string) bool`

The last one exists because the keyword branch of `SearchChannels` filters in Go after its query
returns, following the pattern already there for status and type. Two forms of one rule is a
liability, so they live side by side in this file and a test pins them to the same answer — the SQL
form cannot change without the in-memory form failing.

- [x] **Step 1: Write the failing test**

Create `model/channel_health_test.go`. Package `model` already has a `TestMain` in `model/task_cas_test.go` that opens an in-memory SQLite DB and auto-migrates `&Channel{}`, so this file must not declare another one.

```go
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
```

- [x] **Step 2: Run the test to verify it fails**

Run: `go test ./model/ -run 'TestGetChannelHealthCounts|TestApplyChannelHealthFilter' -v`
Expected: FAIL to build — `undefined: SlowResponseTimeMs`, `undefined: GetChannelHealthCounts`, `undefined: ApplyChannelHealthFilter`.

If the seed fails on a NOT NULL or JSON column instead, fill that field in the fixture and rerun — the fixture must insert cleanly before the assertions mean anything.

- [x] **Step 3: Write the implementation**

Create `model/channel_health.go`. Copy the AGPL header from the top of `model/channel.go` verbatim.

```go
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
```

- [x] **Step 4: Run the test to verify it passes**

Run: `go test ./model/ -run 'TestGetChannelHealthCounts|TestApplyChannelHealthFilter' -v`
Expected: PASS, all four subtests of the unknown-value case included.

- [x] **Step 5: Verify the whole package still builds and passes**

Run: `go build ./... && go test ./model/`
Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add model/channel_health.go model/channel_health_test.go
git commit -m "feat(channels): add health bucket counts and list filter"
```

---

### Task 2: Expose health over HTTP

`GetChannelOps` currently returns a single constant and is already queried by the page for the Max Retries badge, so it is the natural home for the counts — no new round trip. The `health` parameter then threads through both list paths.

**Files:**
- Modify: `controller/channel.go`
- Create: `controller/channel_health_test.go`

**Interfaces:**
- Consumes: `model.GetChannelHealthCounts`, `model.ApplyChannelHealthFilter` from Task 1.
- Produces:
  - `func parseHealthFilter(healthParam string) string` — returns `"slow"`, `"untested"`, or `""`
  - `GET /api/channel/ops` response data gains `health` (the `ChannelHealthCounts` shape from Task 1)
  - `GET /api/channel` and `GET /api/channel/search` accept `health=slow|untested`

- [x] **Step 1: Write the failing test**

Create `controller/channel_health_test.go`. This tests the pure normalizer only; the predicates themselves are covered against a real database in Task 1.

```go
package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHealthFilter(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "slow", input: "slow", want: "slow"},
		{name: "slow uppercase", input: "SLOW", want: "slow"},
		{name: "untested", input: "untested", want: "untested"},
		{name: "empty", input: "", want: ""},
		{name: "unknown", input: "broken", want: ""},
		{name: "not trimmed", input: " slow", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseHealthFilter(tc.input))
		})
	}
}
```

- [x] **Step 2: Run the test to verify it fails**

Run: `go test ./controller/ -run TestParseHealthFilter -v`
Expected: FAIL to build — `undefined: parseHealthFilter`.

- [x] **Step 3: Add the normalizer and extend the query builder**

In `controller/channel.go`, add `parseHealthFilter` directly below the existing `parseStatusFilter` (currently at line 56):

```go
func parseHealthFilter(healthParam string) string {
	switch strings.ToLower(healthParam) {
	case "slow":
		return "slow"
	case "untested":
		return "untested"
	default:
		return ""
	}
}
```

Then give `buildChannelListQuery` (currently line 84) a fourth parameter and apply the filter:

```go
func buildChannelListQuery(group string, statusFilter int, typeFilter int, healthFilter string) *gorm.DB {
	query := model.DB.Model(&model.Channel{})
	query = model.ApplyChannelGroupFilter(query, group)
	query = applyChannelStatusFilter(query, statusFilter)
	query = model.ApplyChannelHealthFilter(query, healthFilter)
	if typeFilter >= 0 {
		query = query.Where("type = ?", typeFilter)
	}
	return query
}
```

- [x] **Step 4: Update all eight call sites**

`buildChannelListQuery` is called at lines 122, 128, 139, 150, 156, 172 and 300. Every call inside `GetAllChannels` (122, 128, 139, 150, 156, 172) takes the request's `healthFilter`; the call at line 300 sits in the tag branch of `SearchChannels`, which passes `-1, -1` because it filters in Go afterwards — pass the request's `healthFilter` there too so the tag branch narrows in SQL.

In `GetAllChannels`, read the parameter next to the existing `statusParam` (line 107):

```go
	statusFilter := parseStatusFilter(statusParam)
	healthFilter := parseHealthFilter(c.Query("health"))
```

The type-counts query at line 172 keeps `-1` for the type filter — it deliberately counts across all types — but must receive `healthFilter`, so the type dropdown's numbers describe the same rows the health filter selected:

```go
	countQuery := buildChannelListQuery(groupFilter, statusFilter, -1, healthFilter)
```

- [x] **Step 5: Filter the keyword branch of SearchChannels**

`SearchChannels` runs `model.SearchChannels` and then filters status and type in Go (lines 325-360). The health filter follows that existing shape. Add `healthFilter := parseHealthFilter(c.Query("health"))` beside the existing `statusFilter` (line 283), pass it to the `buildChannelListQuery` call in the tag branch, and add this loop directly after the status filtering block:

```go
	if healthFilter != "" {
		filtered := make([]*model.Channel, 0, len(channelData))
		for _, ch := range channelData {
			if !model.ChannelMatchesHealth(ch, healthFilter) {
				continue
			}
			filtered = append(filtered, ch)
		}
		channelData = filtered
	}
```

Do not re-derive the slow or untested predicate here. `model.ChannelMatchesHealth` is the in-memory
form of the same rule Task 1 defined, and `TestHealthFilterFormsAgree` keeps it identical to the SQL
form. Inlining the comparison would put a third copy of the rule in the tree.

- [x] **Step 6: Return the counts from GetChannelOps**

Replace the body of `GetChannelOps` (line 94):

```go
func GetChannelOps(c *gin.Context) {
	payload := gin.H{"retry_times": common.RetryTimes}
	// A failed health summary must not block the retry badge, and must not
	// report four zeros — the client treats a missing `health` key as "unknown"
	// and hides the strip rather than claiming everything is fine.
	if counts, err := model.GetChannelHealthCounts(); err != nil {
		common.SysError("failed to count channel health: " + err.Error())
	} else {
		payload["health"] = counts
	}
	common.ApiSuccess(c, payload)
}
```

- [x] **Step 7: Run the tests to verify they pass**

Run: `go test ./controller/ -run TestParseHealthFilter -v && go build ./... && go test ./model/ ./controller/`
Expected: PASS.

- [x] **Step 8: Verify relaykit still builds independently**

Run: `cd relaykit && GOWORK=off go build ./...`
Expected: success. (Nothing here should touch it; this confirms it.)

- [x] **Step 9: Commit**

```bash
git add controller/channel.go controller/channel_health_test.go
git commit -m "feat(channels): serve health counts and the health list filter"
```

---

### Task 3: Strip render state

The strip has three states and one rule that matters: a failed health request must not render zeros, because four zeros read as "everything is fine". Keeping that decision in a pure function makes it testable without React.

**Files:**
- Create: `web/src/features/channels/lib/channel-health.ts`
- Create: `web/src/features/channels/lib/__tests__/channel-health.test.ts`
- Modify: `web/src/features/channels/types.ts`
- Modify: `web/src/features/channels/lib/index.ts` (re-export)

**Interfaces:**
- Consumes: the `health` object shape from Task 2.
- Produces:
  - `type ChannelHealth = { active: number; disabled: number; slow: number; untested: number; slow_threshold_ms: number }`
  - `type HealthStripState = { kind: 'hidden' } | { kind: 'calm'; total: number } | { kind: 'alert'; tiles: HealthTile[] }`
  - `type HealthTile = { id: 'active' | 'disabled' | 'slow' | 'untested'; count: number }`
  - `function resolveHealthStripState(health: ChannelHealth | undefined): HealthStripState`

- [x] **Step 1: Add the response types**

In `web/src/features/channels/types.ts`, extend `ChannelOpsResponse` (line 176) and both param interfaces (lines 268 and 280):

```ts
export interface ChannelHealth {
  active: number
  disabled: number
  slow: number
  untested: number
  slow_threshold_ms: number
}

export interface ChannelOpsResponse {
  success: boolean
  message?: string
  data?: {
    retry_times: number
    // Absent when the server could not compute the summary. Treated as
    // "unknown", never as zero.
    health?: ChannelHealth
  }
}
```

Add `health?: 'slow' | 'untested'` to both `GetChannelsParams` and `SearchChannelsParams`.

- [x] **Step 2: Write the failing test**

Create `web/src/features/channels/lib/__tests__/channel-health.test.ts`, following the `node:test` style the neighbouring tests in that directory already use.

```ts
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { resolveHealthStripState } from '../channel-health'

const healthy = {
  active: 18,
  disabled: 0,
  slow: 0,
  untested: 0,
  slow_threshold_ms: 1000,
}

describe('resolveHealthStripState', () => {
  test('hides the strip when the summary is missing', () => {
    assert.deepEqual(resolveHealthStripState(undefined), { kind: 'hidden' })
  })

  test('collapses to a single line when nothing is wrong', () => {
    assert.deepEqual(resolveHealthStripState(healthy), {
      kind: 'calm',
      total: 18,
    })
  })

  test('shows every tile when any bucket is non-empty', () => {
    const state = resolveHealthStripState({ ...healthy, active: 12, slow: 3 })
    assert.equal(state.kind, 'alert')
    assert.deepEqual(state.kind === 'alert' ? state.tiles : [], [
      { id: 'active', count: 12 },
      { id: 'disabled', count: 0 },
      { id: 'slow', count: 3 },
      { id: 'untested', count: 0 },
    ])
  })

  test('a lone untested channel is enough to expand the strip', () => {
    const state = resolveHealthStripState({ ...healthy, untested: 1 })
    assert.equal(state.kind, 'alert')
  })

  test('an instance with no channels at all stays collapsed', () => {
    const state = resolveHealthStripState({ ...healthy, active: 0 })
    assert.deepEqual(state, { kind: 'calm', total: 0 })
  })
})
```

- [x] **Step 3: Run the test to verify it fails**

Run: `cd web && bun test src/features/channels/lib/__tests__/channel-health.test.ts`
Expected: FAIL — cannot resolve `../channel-health`.

- [x] **Step 4: Write the implementation**

Create `web/src/features/channels/lib/channel-health.ts` with the project copyright header.

```ts
import type { ChannelHealth } from '../types'

export type HealthTileId = 'active' | 'disabled' | 'slow' | 'untested'

export type HealthTile = {
  id: HealthTileId
  count: number
}

export type HealthStripState =
  | { kind: 'hidden' }
  | { kind: 'calm'; total: number }
  | { kind: 'alert'; tiles: HealthTile[] }

/**
 * Decide what the health strip renders.
 *
 * A missing summary means the request failed or the backend predates this
 * feature. It renders nothing at all: four zeros would read as "everything is
 * fine", which is the one claim a failed health check must never make.
 *
 * With nothing disabled, slow, or untested there is no problem to point at, so
 * the four tiles collapse to one line. The strip only takes up space when it
 * has something to say.
 */
export function resolveHealthStripState(
  health: ChannelHealth | undefined
): HealthStripState {
  if (!health) {
    return { kind: 'hidden' }
  }
  if (health.disabled === 0 && health.slow === 0 && health.untested === 0) {
    return { kind: 'calm', total: health.active }
  }
  return {
    kind: 'alert',
    tiles: [
      { id: 'active', count: health.active },
      { id: 'disabled', count: health.disabled },
      { id: 'slow', count: health.slow },
      { id: 'untested', count: health.untested },
    ],
  }
}
```

Re-export it from `web/src/features/channels/lib/index.ts` alongside the existing exports.

- [x] **Step 5: Run the test to verify it passes**

Run: `cd web && bun test src/features/channels/lib/__tests__/channel-health.test.ts && bun run typecheck`
Expected: PASS, clean typecheck.

- [x] **Step 6: Commit**

```bash
git add web/src/features/channels/lib/channel-health.ts web/src/features/channels/lib/__tests__/channel-health.test.ts web/src/features/channels/lib/index.ts web/src/features/channels/types.ts
git commit -m "feat(channels): add health strip render state"
```

---

### Task 4: Health strip component and page wiring

**Files:**
- Create: `web/src/features/channels/components/channel-health-strip.tsx`
- Modify: `web/src/features/channels/index.tsx`
- Modify: `web/src/features/channels/components/channels-table.tsx`
- Modify: `web/src/features/channels/api.ts`

**Interfaces:**
- Consumes: `resolveHealthStripState`, `ChannelHealth`, `HealthTileId` from Task 3; the `health` request parameter from Task 2.
- Produces: a `health` column filter registered in `useTableUrlState`, with `searchKey: 'health'` and `type: 'string'`.

- [x] **Step 1: Build the strip component**

Create `channel-health-strip.tsx` with the copyright header. It takes the resolved state and a click handler; it holds no query of its own, so the page owns the data and the component stays renderable in isolation.

```tsx
type ActiveHealthTiles = {
  status: 'active' | 'disabled' | null
  health: 'slow' | 'untested' | null
}

type ChannelHealthStripProps = {
  state: HealthStripState
  slowThresholdMs: number
  activeTiles: ActiveHealthTiles
  onSelect: (tile: HealthTileId) => void
}
```

`status` and `health` are independent filters the server composes with AND, so a single `activeTile:
HealthTileId | null` cannot represent the strip's real state: Active/Disabled are mutually exclusive
with each other (both come from `status`), Slow/Never tested are mutually exclusive with each other
(both come from `health`), but a status tile and a health tile can be lit together. `activeTiles` is
the pair, and `isHealthTileActive(tile.id, activeTiles)` decides whether a given tile is lit.

Requirements for the render:
- `kind: 'hidden'` renders `null`.
- `kind: 'calm'` renders one line: the total and a "Test all channels" affordance reusing the existing `handleTestAllChannels` action.
- `kind: 'alert'` renders four tiles in a `grid grid-cols-2 gap-2 sm:grid-cols-4`. Each tile is a `<button>` — not a `div` with `onClick` — so it is reachable by keyboard, with `aria-pressed={isHealthTileActive(tile.id, activeTiles)}`.
- Tile labels via `t()`: `'Active'`, `'Disabled'`, `'Slow'`, `'Never tested'`. The slow tile's label reads `t('Slower than {{threshold}}', { threshold: formatResponseTime(slowThresholdMs, t) })` so the number comes from the server, never from a second copy of the constant.
- Reuse the existing status colors: success for active, error for disabled, warning for slow, muted for never tested.

- [x] **Step 2: Share the ops query between the badge and the strip**

`web/src/features/channels/index.tsx` already runs a `['channel-ops']` query for the Max Retries badge. Read `channelOpsQuery.data?.data?.health` from that same query and pass `resolveHealthStripState(health)` into the strip — do not add a second query.

Invalidate the ops summary wherever the channel list is already invalidated, so the counts never lag
the rows. `refreshChannels` in `channels-provider.tsx` is not the chokepoint for that — roughly 25
call sites across this file, dialogs, and drawers already invalidate `channelsQueryKeys.lists()`
after a channel changes, and rewriting each of them individually to also invalidate an ops key would
be exactly the kind of change that's easy to miss a call site on. Instead, `channelsQueryKeys.ops()`
nests under `channelsQueryKeys.lists()`:

```ts
export const channelsQueryKeys = {
  all: ['channels'] as const,
  lists: () => [...channelsQueryKeys.all, 'list'] as const,
  ops: () => [...channelsQueryKeys.lists(), 'ops'] as const,
}
```

TanStack Query invalidation matches by key prefix, so every existing `invalidateQueries({ queryKey:
channelsQueryKeys.lists() })` call already covers the ops summary too, with no changes to those ~25
call sites. The same prefix matching also applies to `setQueriesData`/`getQueriesData`, not just
`invalidateQueries` — the one call scoped to `{queryKey: lists()}` that writes into the list cache
directly (`updateChannelTestCache` in `channel-test-dialog.tsx`) must exclude the ops entry with an
`isChannelOpsQueryKey(queryKey)` predicate, since its payload (`{retry_times, health}`) has no
`items` and an updater written for a paginated list response throws against it.

- [x] **Step 3: Register the health column filter**

In `channels-table.tsx`, add to the `columnFilters` array passed to `useTableUrlState` (currently lines 121-139):

```ts
      { columnId: 'health', searchKey: 'health', type: 'string' },
```

Read it the way the model filter is read, pass it into both `getChannels` and `searchChannels` params, and add it to the query key so a change refetches.

- [x] **Step 4: Replace the Status dropdown with strip clicks**

Remove the Status faceted filter from `toolbarProps` while the strip is showing — but the
`channel-ops` query the strip reads is declared with `retry: false`, so a failed request or an older
backend leaves the strip rendering nothing (`kind: 'hidden'`) with no way to filter by status at all.
The dropdown stays as a fallback for that one state: present only when `healthState.kind ===
'hidden'`, gone the moment the strip has something to show. Clicking a tile sets the corresponding
filter:

- `active` → `status` column filter `['enabled']`
- `disabled` → `status` column filter `['disabled']`
- `slow` → `health` column filter `'slow'`
- `untested` → `health` column filter `'untested'`

Clicking the tile that is already active clears its filter. `activeTiles` is derived from the current filter values, so a URL pasted into a fresh tab highlights the right tile(s).

Keep `CHANNELS_STATUS_FILTER_STORAGE_KEY` and its `deserialize` fallback exactly as they are — that is existing status persistence, unrelated to this change.

- [x] **Step 5: Verify manually**

Run: `cd web && bun run dev`

Confirm, in this order:
1. With every channel healthy, the strip is one line.
2. Disable a channel; the strip expands to four tiles and Disabled reads 1.
3. Click Disabled; the list narrows and the URL gains `status=disabled`.
4. Reload the page; the tile is still highlighted and the list still narrowed.
5. Click Never tested; the URL gains `health=untested` and the list narrows.
6. Re-enable the channel; the strip collapses without a manual refresh.

- [x] **Step 6: Run checks and commit**

Run: `cd web && bun run typecheck && bun run lint && bun test`

```bash
git add web/src/features/channels/components/channel-health-strip.tsx web/src/features/channels/index.tsx web/src/features/channels/components/channels-table.tsx web/src/features/channels/components/channels-provider.tsx web/src/features/channels/api.ts
git commit -m "feat(channels): add the health strip above the channel list"
```

---

### Task 5: Shared other_info parser

`channels-columns.tsx:918-924` parses `other_info` inline inside the status cell, with a bare `try/catch` and an empty block. The card needs the same two fields, so the parse moves into one tested helper instead of being copied.

**Files:**
- Create: `web/src/features/channels/lib/channel-status-info.ts`
- Create: `web/src/features/channels/lib/__tests__/channel-status-info.test.ts`
- Modify: `web/src/features/channels/components/channels-columns.tsx`
- Modify: `web/src/features/channels/lib/index.ts`

**Interfaces:**
- Produces: `function parseChannelStatusInfo(otherInfo: string | undefined): { statusReason: string; statusTime: number | null }`

- [x] **Step 1: Write the failing test**

```ts
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { parseChannelStatusInfo } from '../channel-status-info'

const empty = { statusReason: '', statusTime: null }

describe('parseChannelStatusInfo', () => {
  test('reads both fields', () => {
    assert.deepEqual(
      parseChannelStatusInfo('{"status_reason":"401 unauthorized","status_time":1700000000}'),
      { statusReason: '401 unauthorized', statusTime: 1700000000 }
    )
  })

  test('returns empty for an empty string', () => {
    assert.deepEqual(parseChannelStatusInfo(''), empty)
  })

  test('returns empty for undefined', () => {
    assert.deepEqual(parseChannelStatusInfo(undefined), empty)
  })

  test('returns empty for malformed JSON instead of throwing', () => {
    assert.deepEqual(parseChannelStatusInfo('{"status_reason":'), empty)
  })

  test('returns empty for JSON that is not an object', () => {
    assert.deepEqual(parseChannelStatusInfo('"just a string"'), empty)
  })

  test('tolerates a missing status_reason', () => {
    assert.deepEqual(parseChannelStatusInfo('{"status_time":1700000000}'), {
      statusReason: '',
      statusTime: 1700000000,
    })
  })

  test('ignores a non-numeric status_time', () => {
    assert.deepEqual(parseChannelStatusInfo('{"status_time":"yesterday"}'), empty)
  })
})
```

- [x] **Step 2: Run the test to verify it fails**

Run: `cd web && bun test src/features/channels/lib/__tests__/channel-status-info.test.ts`
Expected: FAIL — cannot resolve `../channel-status-info`.

- [x] **Step 3: Write the implementation**

```ts
export type ChannelStatusInfo = {
  statusReason: string
  statusTime: number | null
}

const EMPTY: ChannelStatusInfo = { statusReason: '', statusTime: null }

/**
 * `other_info` is a free-form string column written by several backend paths.
 * It may be empty, malformed, or missing either key, so this never throws and
 * degrades to showing the status alone.
 */
export function parseChannelStatusInfo(
  otherInfo: string | undefined
): ChannelStatusInfo {
  if (!otherInfo) {
    return EMPTY
  }
  let parsed: unknown
  try {
    parsed = JSON.parse(otherInfo)
  } catch {
    return EMPTY
  }
  if (typeof parsed !== 'object' || parsed === null) {
    return EMPTY
  }
  const record = parsed as Record<string, unknown>
  return {
    statusReason:
      typeof record.status_reason === 'string' ? record.status_reason : '',
    statusTime:
      typeof record.status_time === 'number' ? record.status_time : null,
  }
}
```

- [x] **Step 4: Replace the inline parse in the status cell**

In `channels-columns.tsx`, delete the `try/catch` block at lines 918-931 and call the helper:

```ts
            const { statusReason, statusTime } = parseChannelStatusInfo(
              channel.other_info
            )
            const statusTimeLabel = statusTime
              ? formatTimestampToDate(statusTime)
              : ''
```

The tooltip below it keeps its current markup, reading `statusReason` and `statusTimeLabel`.

- [x] **Step 5: Run the tests to verify they pass**

Run: `cd web && bun test src/features/channels/lib/__tests__/channel-status-info.test.ts && bun run typecheck`
Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add web/src/features/channels/lib/channel-status-info.ts web/src/features/channels/lib/__tests__/channel-status-info.test.ts web/src/features/channels/lib/index.ts web/src/features/channels/components/channels-columns.tsx
git commit -m "refactor(channels): extract the other_info status parser"
```

---

### Task 6: Reorganize the card by subject

The card currently mirrors the table's column order, so the four subjects compete inside it exactly as they do in the table. It also omits Models entirely.

**Files:**
- Modify: `web/src/features/channels/components/channel-card.tsx`

**Interfaces:**
- Consumes: `parseChannelStatusInfo` from Task 5.

- [x] **Step 1: Restructure into three labelled rows**

Keep the `flexRender` approach — cell renderers stay shared with the table, which is what makes the priority/weight spinners and balance refresh keep working. Change only the arrangement:

| Row | Cells |
|-|-|
| Header | `select` checkbox, status dot, name, type, actions menu |
| Health | `response_time`, `test_time`, `balance`, plus the disable reason |
| Serves | `models` — **new**, rendered via `renderCell('models')` |
| Access | `group` badges, `priority`, `weight` |

Each row gets a fixed-width uppercase label using the existing `labelClass`, so the three subjects read as three subjects.

- [x] **Step 2: Surface the disable reason inline**

For a channel with `status !== CHANNEL_STATUS.ENABLED`, append the parsed reason to the Health row instead of hiding it in a tooltip:

```tsx
const { statusReason } = parseChannelStatusInfo(row.original.other_info)
const isAutoDisabled = row.original.status === CHANNEL_STATUS.AUTO_DISABLED
```

Render, when `statusReason` is non-empty, `{isAutoDisabled ? t('Auto-disabled') : t('Disabled')} · {statusReason}`. When it is empty, render the status alone — the tooltip on the status cell still carries the timestamp.

Confirm the exact `CHANNEL_STATUS` member names in `web/src/features/channels/constants.ts` before using them; the file defines `ENABLED` and `MANUAL_DISABLED`, so check what the auto-disabled member is called rather than assuming.

- [x] **Step 3: Keep the checkbox where it is**

The card header already renders `selectCell`. It stays visible unconditionally — Task 7 removes the mode that currently gates it. Do not add a hover-reveal.

- [x] **Step 4: Verify manually**

Run: `cd web && bun run dev`

Switch to card view and confirm: the three rows are labelled, models appear, an auto-disabled channel shows its reason inline, and the priority/weight spinners still edit in place.

- [x] **Step 5: Run checks and commit**

Run: `cd web && bun run typecheck && bun run lint`

```bash
git add web/src/features/channels/components/channel-card.tsx
git commit -m "feat(channels): group card fields by subject and show models"
```

---

### Task 7: Remove the modes

Two switches encode no feature and are deleted. The third is the only entry point to tag-level editing, so it becomes an explicit control instead.

**Files:**
- Modify: `web/src/features/channels/components/channels-provider.tsx`
- Modify: `web/src/features/channels/components/channels-primary-buttons.tsx`
- Modify: `web/src/features/channels/components/channels-table.tsx`
- Modify: `web/src/features/channels/components/channels-columns.tsx`

- [x] **Step 1: Delete Sort by ID**

Remove `idSort` and `setIdSort` from `ChannelsContextType` and the provider (`channels-provider.tsx:61-62, 89-91`), the switch from `channels-primary-buttons.tsx:138-148` and its mobile `DropdownMenuCheckboxItem` at lines 199-206, and the `channels-id-sort` localStorage read. Column-header sorting already covers this; `id_sort` stays in the request params, driven by the table's own sorting state.

- [x] **Step 2: Delete Batch Operations**

Remove `batchMode` / `setBatchMode` from the provider (lines 63, 92), the switch at `channels-primary-buttons.tsx:111-124` and its mobile item at lines 181-188.

In `channels-columns.tsx`, drop the `options.enableSelection` parameter and the conditional spread at line 562, so the `select` column is defined unconditionally like `users-columns.tsx:49` and `api-keys-columns.tsx:84`. In `channels-table.tsx`, change:

```ts
  const columns = useChannelsColumns()
  // ...
    enableRowSelection: (row: Row<Channel>) => !isTagAggregateRow(row.original),
```

The tag-aggregate guard must stay — aggregate rows are not individually selectable.

- [x] **Step 3: Convert Tag Mode into a grouping control**

`enableTagMode` produces the aggregate rows that `data-table-tag-row-actions.tsx` hangs `edit-tag-dialog` and `tag-batch-edit-dialog` off, so the state stays. Change only how it is presented and persisted:

- Replace the switch with a select in the filter toolbar labelled `t('Group by')`, with options `t('None')` and `t('Label')`.
- Drop the `localStorage.getItem('enable-tag-mode')` initializer in `channels-provider.tsx:86-88`; it starts at `false` every session.

- [x] **Step 4: Verify tag editing still works**

Run: `cd web && bun run dev`

Set Group by to Label, confirm aggregate rows appear, open the row actions on one, and confirm both Edit Tag and the batch edit dialog still open and save. This is the regression this task most risks.

- [x] **Step 5: Run checks and commit**

Run: `cd web && bun run typecheck && bun run lint && bun test`

```bash
git add web/src/features/channels/components/channels-provider.tsx web/src/features/channels/components/channels-primary-buttons.tsx web/src/features/channels/components/channels-table.tsx web/src/features/channels/components/channels-columns.tsx
git commit -m "refactor(channels): drop two page modes and make tag grouping explicit"
```

---

### Task 8: Group the action menus by consequence

**Files:**
- Modify: `web/src/features/channels/components/channels-primary-buttons.tsx`
- Modify: `web/src/features/channels/components/data-table-row-actions.tsx`

- [x] **Step 1: Group the header menu**

Insert `DropdownMenuLabel` headings and `DropdownMenuSeparator` between three groups, in this order:

| Group | Items |
|-|-|
| `t('Check')` | Test All Channels, Update All Balances, Detect All Upstream Updates |
| `t('Apply')` | Apply All Upstream Updates |
| `t('Maintenance')` | Repair Channel Consistency, Delete All Disabled |

Both items in the last group keep their existing confirmation dialogs, and Delete All Disabled keeps `variant='destructive'`.

- [x] **Step 2: Group the row menu**

Same treatment in `data-table-row-actions.tsx`:

| Group | Items |
|-|-|
| `t('Check')` | Test Connection, Query Balance, Fetch Models |
| `t('Manage')` | Upstream Updates, Manage Ollama Models, Manage Keys, Copy Channel |
| `t('Danger')` | Delete |

Provider-conditional items (Manage Ollama Models, Manage Keys) stay conditional and stay inside their group, so a shorter menu reads as a shorter group rather than a different menu.

- [x] **Step 3: Verify manually**

Run: `cd web && bun run dev`

Open both menus on an Ollama channel and on a plain OpenAI channel; confirm the groups hold and no group renders an empty heading when all of its items are hidden.

- [x] **Step 4: Run checks and commit**

Run: `cd web && bun run typecheck && bun run lint`

```bash
git add web/src/features/channels/components/channels-primary-buttons.tsx web/src/features/channels/components/data-table-row-actions.tsx
git commit -m "refactor(channels): group channel actions by consequence"
```

---

### Task 9: Vocabulary and translations

Done last so every new string introduced by Tasks 4 through 8 is translated in one pass.

**Files:**
- Modify: `web/src/features/channels/constants.ts`
- Modify: `web/src/features/channels/components/channels-columns.tsx`
- Modify: `web/src/i18n/locales/*.json`

- [x] **Step 1: Rename the two labels**

`Tag` becomes `Label` and `Groups` becomes `Access` in every user-facing position: the column headers in `channels-columns.tsx` (lines 1018 and 1051), the drawer's field labels, the filter toolbar, and the bulk action copy in `data-table-bulk-actions.tsx`.

Do **not** rename the `tag` or `group` accessor keys, request parameters, JSON fields, or database columns. This is a copy change only.

- [x] **Step 2: Rewrite the field descriptions**

In `constants.ts`, replace the two entries that both currently use the word "group":

```ts
  TAG: 'Organizes channels so you can edit several at once. Does not affect which channel serves a request.',
  GROUP: 'User groups allowed to reach this channel.',
```

- [x] **Step 3: Add the priority and weight gloss**

Add the two descriptions that give the numbers meaning where they are displayed:

```ts
  PRIORITY: 'Tried before channels with a lower priority.',
  WEIGHT: 'Splits traffic between channels of equal priority.',
```

Render both in the card's Access row, not only in the drawer.

- [x] **Step 4: Sync and fill the locales**

Run: `cd web && bun run i18n:sync`

Fill every new key in all seven locales: `en`, `zh`, `zh-TW`, `fr`, `ru`, `ja`, `vi`. New keys from this plan are the four tile labels, the slow-tile threshold string, the calm-strip line, `Group by` / `None` / `Label`, the six menu group headings, `Auto-disabled`, and the four description strings above. Check `docs/translation-glossary.md` for terms with an established translation.

- [x] **Step 5: Verify no key is left untranslated**

Run: `cd web && bun run i18n:sync && git diff --stat web/src/i18n/locales/`
Expected: every locale file touched, no locale left with fewer keys than `en.json`.

- [x] **Step 6: Run the full frontend check**

Run: `cd web && bun run typecheck && bun run lint && bun run format:check && bun run copyright:check && bun test && bun run build`
Expected: all pass.

- [x] **Step 7: Commit**

```bash
git add web/src/features/channels/constants.ts web/src/features/channels/components/channels-columns.tsx web/src/features/channels/components/data-table-bulk-actions.tsx web/src/i18n/locales/
git commit -m "i18n(channels): separate the Label and Access vocabulary"
```

---

## Final Verification

- [ ] Run the backend suite: `go build ./... && go test ./model/ ./controller/`
- [ ] Run the relaykit independence check: `cd relaykit && GOWORK=off go build ./...`
- [ ] Run the frontend suite: `cd web && bun run typecheck && bun run lint && bun test && bun run build`
- [ ] Walk the four tasks the redesign is for: diagnose a broken channel from the strip; create a channel; read one card's Access row; select several cards and act on them in bulk.
- [ ] Confirm no stale `localStorage` key remains in use: `rg 'enable-tag-mode|channels-id-sort' web/src`
