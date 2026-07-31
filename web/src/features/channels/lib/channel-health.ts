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
 * Which tiles the current `status`/`health` column filters correspond to.
 *
 * `status` and `health` are independent filters on the server (the backend
 * composes them with AND), so Active/Disabled and Slow/Never tested are two
 * separate dimensions, not one four-way choice: a status tile and a health
 * tile can be lit together, but Active and Disabled can't both be lit
 * (`status` holds one value), and neither can Slow and Never tested
 * (`health` holds one value).
 */
export type ActiveHealthTiles = {
  status: 'active' | 'disabled' | null
  health: 'slow' | 'untested' | null
}

export function resolveActiveHealthTiles(
  statusFilter: string[],
  healthFilter: string | undefined
): ActiveHealthTiles {
  let status: ActiveHealthTiles['status'] = null
  if (statusFilter.length === 1 && statusFilter[0] === 'enabled') {
    status = 'active'
  } else if (statusFilter.length === 1 && statusFilter[0] === 'disabled') {
    status = 'disabled'
  }
  const health =
    healthFilter === 'slow' || healthFilter === 'untested'
      ? healthFilter
      : null
  return { status, health }
}

/** Whether a single tile is lit, given the resolved pair. */
export function isHealthTileActive(
  tile: HealthTileId,
  activeTiles: ActiveHealthTiles
): boolean {
  if (tile === 'active' || tile === 'disabled') {
    return activeTiles.status === tile
  }
  return activeTiles.health === tile
}

/**
 * What the `status`/`health` column filters should become after clicking a
 * tile. Active/Disabled only ever set `status`; Slow/Never tested only ever
 * set `health` — the other dimension, wherever it currently stands, is left
 * untouched, so clicking Disabled then Never tested narrows to channels
 * that are both. Clicking a tile that is already lit clears only its own
 * dimension (`null`); an absent key means "leave this dimension as is".
 */
export type HealthTileFilterPatch = {
  status?: ['enabled'] | ['disabled'] | null
  health?: 'slow' | 'untested' | null
}

export function healthTileFilterPatch(
  tile: HealthTileId,
  activeTiles: ActiveHealthTiles
): HealthTileFilterPatch {
  const isActive = isHealthTileActive(tile, activeTiles)
  if (tile === 'active') {
    return { status: isActive ? null : ['enabled'] }
  }
  if (tile === 'disabled') {
    return { status: isActive ? null : ['disabled'] }
  }
  // tile is 'slow' or 'untested'
  return { health: isActive ? null : tile }
}
