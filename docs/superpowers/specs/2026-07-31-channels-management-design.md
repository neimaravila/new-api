# Channels Management Design

## Summary

The Channels page presents four unrelated subjects with equal weight and hides its most useful
information behind controls that only pay off at a scale this instance does not have.

`useChannelsColumns` in `web/src/features/channels/components/channels-columns.tsx` defines 14
columns spanning identity (ID, Name, Type, Tag), routing (Models, Groups, Priority, Weight), health
(Status, Response, Last Tested) and cost (Used / Remaining). Twelve of them render by default:
`channels-table.tsx` hides only `models` and `tag` via `initialColumnVisibility`, so the curated
default stops one column short of each of the two heaviest cells and leaves the four subjects
competing at equal weight. Three
switches in `channels-primary-buttons.tsx:111-148` — Batch Operations, Tag Mode, Sort by ID — occupy
the primary action bar, and two of them persist in `localStorage`
(`channels-provider.tsx:86-91`), so the page can open in a state the operator set weeks earlier.

Two adjacent columns carry synonymous names for opposite concepts: `Groups` controls **who may use**
the channel (access and routing), `Tag` is an operator-facing label used only for batch editing.
Both field descriptions in `constants.ts:370,378` use the word "group".

Restructure the page around the card view that already exists, group each card's fields by subject,
surface a health strip that doubles as the status filter, and fix the vocabulary. Remove the two
switches that encode no feature, and convert the one that does into an explicit grouping control.

## Goals

- Make "is anything broken?" answerable at a glance, without scanning rows.
- Stop `Tag` and `Groups` from reading as the same concept.
- Give Priority and Weight a meaning at the point of display, without inventing a routing position
  the data cannot support.
- Remove hidden modes; the page should look the same on every visit unless the operator changes it.
- Reuse the existing card view and view-mode toggle rather than building a parallel surface.

## Non-goals

- No change to the channel editor drawer (`channel-mutate-drawer.tsx`, ~4900 lines) or its satellite
  dialogs. It was explicitly excluded from scope.
- No routing-queue inspector. Showing the true per-`(group, model)` order needs a new endpoint over
  `abilities`; it is deferred to its own spec.
- No change to the `Ability` routing model, priority semantics, or channel selection logic.
- No new backend beyond the health counts and the `health` list filter described below. Every other
  field this design surfaces is already returned.
- No renaming of API fields, JSON keys, or database columns. `tag` and `group` keep their wire names;
  only user-facing labels change.

## Current Context

**Card view already exists.** `channels-table.tsx:422-426` passes `viewModeStorageKey`,
`renderCard` and `cardGridClassName` to the shared data table, and `channel-card.tsx` renders each
card by reusing the columns' own cell renderers through `flexRender`. Selection, the type badge,
priority/weight spinners, balance refresh and the row actions menu all work in card mode today. The
card is organized by the table's column order, not by subject, and it omits Models entirely.

**Disable reasons are already stored and already parsed.** `model/channel.go:666,690,772` writes
`other_info.status_reason` and `other_info.status_time` whenever a channel's status changes, and
`common/constants.go:246-247` separates `ChannelStatusManuallyDisabled = 2` from
`ChannelStatusAutoDisabled = 3`. The frontend already reads both at `channels-columns.tsx:918-924`,
but only inside a tooltip on the Status cell.

**Routing order is per `(group, model)`, not per channel.** `model/ability.go:19-24` makes `Group`
and `Model` part of the `Ability` primary key, and `getChannelQuery` (`ability.go:93-106`) selects
candidates by the maximum priority within one such pair. A channel serving 12 models across 2 groups
participates in 24 independent queues and may hold a different position in each. No single "queue
position" exists for a channel, so no card may claim one.

**The list is filtered and paginated by the server.** `channels-table.tsx` sets `manualPagination`,
`manualSorting` and `manualFiltering`, with a page size of 20 on desktop and 10 on mobile
(`constants.ts:281`). Anything computed from the rows in the React Query cache describes *the current
page*, not the instance. The server's status filter is also coarser than the UI suggests:
`applyChannelStatusFilter` (`controller/channel.go:74-82`) understands only enabled, disabled, and
all — there is no way to ask it for slow or never-tested channels. Both `GetAllChannels` and the
tag-mode branch of `SearchChannels` build their query through `buildChannelListQuery`, while the
keyword branch of `SearchChannels` applies status and type filters in Go after the query returns
(`controller/channel.go:325-360`).

**Selection is gated only on this page.** `channels-columns.tsx:562` wraps the `select` column in
`enableSelection`, driven by `batchMode`. `users-columns.tsx:49`, `api-keys-columns.tsx:84`,
`models-columns.tsx` and `redemptions-columns.tsx` all define `select` unconditionally. Channels is
the only feature in the codebase that hides its checkboxes behind a mode.

**Tag Mode carries a feature.** Unlike the other two switches, `enableTagMode` produces tag aggregate
rows (`isTagAggregateRow`) that are the only entry point to `edit-tag-dialog` and
`tag-batch-edit-dialog`, with their own actions in `data-table-tag-row-actions.tsx`. It cannot be
deleted without removing tag-level editing.

## Vocabulary

| Current label | New label | Why |
|-|-|-|
| `Tag` | Rótulo / Label | It is an operator-facing organizing label, not an access concept |
| `Groups` | Acesso / Access | It is the set of user groups allowed to reach this channel |

`FIELD_DESCRIPTIONS.TAG` changes from `'Group channels by tag for batch operations'` to a phrasing
that states what it does not do: organizing and bulk-editing only, no effect on which channel serves
a request. `FIELD_DESCRIPTIONS.GROUP` drops the word "group" as a verb and names the access role.

Priority and Weight keep their raw values and gain a one-line gloss where they are displayed:
priority is tried before lower priorities, weight splits load among equal priorities. No computed
role badge ("Primary"/"Backup") and no queue position — both would be false whenever the channel's
position differs across its `(group, model)` pairs, and any client-side cross-referencing silently
degrades once the list is paginated.

## Layout

The page keeps `SectionPageLayout` and gains one element above the table:

```
[ health strip ]                      <- new
[ search · Type · Access · Label · view toggle · Create · ⋯ ]
[ cards | table ]                     <- existing view toggle
```

**Health strip.** Four counts covering the whole instance: Active, Disabled, Slow, Never tested.

These counts cannot be computed in the browser. The list is server-paginated, so the loaded rows are
one page of at most 20 (10 on mobile); counting them would silently answer "is anything broken on
this page" while looking like it answered "is anything broken". The counts therefore come from the
server, and the two new buckets become server-side filters:

- `GetChannelOps` (`controller/channel.go:94`) gains a `health` object alongside `retry_times`:
  `active`, `disabled`, `slow`, `untested`, plus `slow_threshold_ms`. Four GORM `Count` queries over
  `channels`, no raw SQL, so all three databases are covered.
- `GetAllChannels` and `SearchChannels` accept a `health` query parameter with the values `slow` and
  `untested`. It is independent of `status` and composes with it.

Bucket definitions, shared by the counts and the filter:

| Bucket | Predicate |
|-|-|
| Active | `status = 1` |
| Disabled | `status != 1` |
| Slow | `test_time > 0 AND response_time > slow_threshold_ms` |
| Never tested | `test_time = 0` |

`slow_threshold_ms` is 1000, matching the `GOOD` tier in `RESPONSE_TIME_THRESHOLDS`
(`constants.ts:329-334`), so a channel the table already badges `FAIR` or worse is the same channel
the strip counts as slow. The server returns the threshold it used and the strip labels itself from
that value, so the two cannot drift apart. A never-tested channel has `test_time = 0` and is excluded
from Slow, matching `getResponseTimeConfig`, which maps it to `UNKNOWN` rather than to a slow tier.

The four counts do not partition the list. Active and Disabled are mutually exclusive because both
come from `status`, but Slow and Never tested are qualifiers that a channel can carry alongside
either — a disabled channel that was never tested appears in both Disabled and Never tested. The
tiles are filters, not a breakdown, so they are not expected to sum to the total and must not be
presented as if they did.

Clicking Active or Disabled writes the existing `status` column filter, unchanged. Clicking Slow or
Never tested writes a new `health` column filter, registered in `useTableUrlState` alongside the
others so it round-trips through the URL the same way. Either click adds a removable chip to the
filter bar. The Status dropdown is removed, since the strip replaces it.

When no channel is disabled, slow, or untested, the strip collapses to a single line stating the
total and the most recent test time, with a "Test all" affordance. The strip must never occupy four
tiles' worth of space to report that nothing is wrong.

**Card structure.** `ChannelCard` is reorganized from column order into three labelled rows:

| Row | Content |
|-|-|
| Saúde / Health | response time · last tested · balance · when disabled, `status_reason` and whether it was auto or manual |
| Atende / Serves | model count plus a truncated sample — new to the card |
| Acesso / Access | group badges · priority and weight with their gloss |

The card keeps its existing `flexRender` approach so cell renderers stay shared with the table. The
checkbox stays where it is today: always visible in the card header, matching every other table in
the project.

## Modes

| Switch | Disposition |
|-|-|
| Sort by ID | Removed. Column header sorting already does this. Drop `idSort` from `ChannelsProvider` and the `channels-id-sort` localStorage key. |
| Batch Operations | Removed. `select` becomes unconditional; the bulk bar appears on selection. Drop `batchMode` and the `enableSelection` parameter. |
| Tag Mode | Becomes a "Group by: none / label" control in the filter bar. Keeps tag aggregate rows and tag-level editing. Stops persisting in `localStorage`. |

Removing `enableSelection` from `useChannelsColumns` also removes the conditional array spread at
`channels-columns.tsx:562`, so the `select` column definition matches the other features'.

## Actions

Both menus are grouped by consequence, with separators and section labels, read-first:

- **Header menu**: read (Test All Channels, Update All Balances, Detect All Upstream Updates) ·
  write (Apply All Upstream Updates) · destructive (Repair Channel Consistency, Delete All Disabled),
  visually separated.
- **Row menu**: read (Test Connection, Query Balance, Fetch Models) · write (Upstream Updates,
  Manage Ollama Models, Manage Keys, Copy Channel) · destructive (Delete).

Provider-conditional entries (Manage Ollama Models, Manage Keys) stay conditional but sit inside
their consequence group, so a shorter menu reads as a shorter group rather than a different menu.

## Data Flow

The health strip reads the `channel-ops` query that `features/channels/index.tsx` already issues for
the Max Retries badge — the same request now carries the `health` object, so the strip costs no
additional round trip. It is invalidated alongside the channel list whenever a channel is created,
edited, enabled, disabled, deleted, or tested, so the counts never lag the rows beneath them.

Filtering by Slow or Never tested sends `health=slow` or `health=untested` on the existing list and
search requests. No other request changes.

`status_reason` and `status_time` come from parsing `channel.other_info`, which
`channels-columns.tsx:918-924` already does — that parse moves into a shared helper in `lib/` so the
card and the status cell share one implementation instead of duplicating the `JSON.parse` and its
failure handling.

## Error Handling

`other_info` is a free-form string column and may be empty, malformed, or missing the keys. The
shared parser returns an empty result on any parse failure and never throws; a card with no
`status_reason` shows the status alone, exactly as today.

The `channel-ops` query is already declared with `retry: false`, so a failure is silent and
non-blocking. The strip follows the Max Retries badge's existing behavior: if `health` is absent from
the response — request failed, or an older backend — the strip renders nothing at all rather than
zeros. Four zeros would read as "everything is fine", which is the one thing a failed health check
must never claim.

An unrecognized `health` parameter value is ignored by the server and returns the unfiltered list,
matching how `type` and `status` already treat unparseable input.

## i18n

New keys go in `web/src/i18n/locales/en.json` as English source strings, then `bun run i18n:sync`.
Renaming a label changes the key, so `Tag` → `Label` and `Groups` → `Access` add keys and orphan the
old ones; the sync tooling removes the orphans. Every locale (`zh`, `zh-TW`, `fr`, `ru`, `ja`, `vi`)
must be filled, including the priority/weight gloss and the four strip labels.

## Testing

Backend, following the rules in `AGENTS.md` — `testify/require` for setup, `assert` for value
checks, deterministic table tests against a SQLite fixture:

- Health bucket predicates: a fixture of channels covering every combination, asserting each count.
  Must include a disabled, never-tested channel (counted in both Disabled and Never tested), a
  `test_time` of 0 (never tested, *not* slow), and a `response_time` of exactly `slow_threshold_ms`
  (not slow — the predicate is strictly greater).
- The `health` list filter returns the same set its matching count reports, for both `slow` and
  `untested`. This is the invariant that keeps a tile's number and its filtered list agreeing.
- `health` composes with `status`: `health=untested&status=enabled` returns only channels that are
  both, not the union.
- An unrecognized `health` value returns the unfiltered list.

Frontend, following `web/AGENTS.md` — behavior, not layout:

- `other_info` parsing helper: valid JSON, empty string, malformed JSON, and JSON missing
  `status_reason` — asserting no throw and the exact fallback shape.
- The strip renders nothing when `health` is absent from the ops response, and never renders zeros
  in its place.
- Clicking Active or Disabled writes the same `status` column filter value the removed dropdown
  wrote, so existing URL-state round-tripping keeps working.

No snapshot tests of card markup, and no test asserting the column list.

## Risks

- **Tag Mode conversion.** Tag aggregate rows interact with sorting, selection and row actions. If
  the grouping control regresses them, tag-level editing becomes unreachable. Verify
  `edit-tag-dialog` and `tag-batch-edit-dialog` still open from an aggregate row.
- **Dropping persisted keys.** Operators with `enable-tag-mode` or `channels-id-sort` set will see
  behavior change on first load after deploy. This is intended, but it is a visible change.
- **Two definitions of "slow".** The threshold now lives in the backend and is mirrored by the
  frontend's `RESPONSE_TIME_THRESHOLDS`. Returning `slow_threshold_ms` in the response and labelling
  the tile from it keeps the strip honest, but the table's own response badge still reads from the
  frontend constant. If someone changes one without the other, the badge and the strip disagree.
  The test asserting count-equals-filter catches drift between the count and the filter, not between
  the backend and the badge; that one is a review concern.
- **Four extra counts per page load.** `GetChannelOps` goes from returning a constant to running
  four `Count` queries. On a large instance this is four full scans of `channels` on every visit to
  the page. The table has an index on neither `test_time` nor `response_time`. Acceptable at this
  scale; worth revisiting before anyone runs this with thousands of channels. It is also four
  `Count` queries per *mutation*, not just per page load: `channelsQueryKeys.ops()` nests under
  `channelsQueryKeys.lists()` so that the ~25 existing call sites that invalidate the list after a
  channel is created, edited, enabled, disabled, deleted, or tested cover the ops summary too, by
  prefix match, without touching each call site. The trade-off is that every one of those ~25
  invalidations now refetches the four-count summary alongside the rows. Same acceptable-at-this-
  scale caveat as the page-load case; worth revisiting together if it ever is.
- **Card density.** Adding the Serves row makes cards taller. At this instance's scale (under 20
  channels) that is acceptable; at several hundred, the table view remains the answer.
