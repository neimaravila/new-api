/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
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

/**
 * Which tile, if any, the current `status`/`health` column filters
 * correspond to.
 *
 * The strip presents the four buckets as one choice, like a segmented
 * control, even though `status` and `health` are independent filters on the
 * server. In normal use only one of the two is ever set at a time (selecting
 * a tile clears the other), but a hand-edited or stale URL can set both; in
 * that case `health` wins because it is the more specific signal.
 */
export function resolveActiveHealthTile(
  statusFilter: string[],
  healthFilter: string | undefined
): HealthTileId | null {
  if (healthFilter === 'slow' || healthFilter === 'untested') {
    return healthFilter
  }
  if (statusFilter.length === 1 && statusFilter[0] === 'enabled') {
    return 'active'
  }
  if (statusFilter.length === 1 && statusFilter[0] === 'disabled') {
    return 'disabled'
  }
  return null
}

/**
 * What the `status`/`health` column filters should become after clicking a
 * tile. Selecting the already-active tile clears it (toggle off); selecting
 * any other tile replaces whichever filter was driving the strip, since the
 * four tiles present one choice rather than independent checkboxes.
 */
export type HealthTileFilterPatch = {
  status?: ['enabled'] | ['disabled']
  health?: 'slow' | 'untested'
}

export function healthTileFilterPatch(
  tile: HealthTileId,
  activeTile: HealthTileId | null
): HealthTileFilterPatch {
  if (tile === activeTile) {
    return {}
  }
  switch (tile) {
    case 'active':
      return { status: ['enabled'] }
    case 'disabled':
      return { status: ['disabled'] }
    case 'slow':
      return { health: 'slow' }
    case 'untested':
      return { health: 'untested' }
  }
}
