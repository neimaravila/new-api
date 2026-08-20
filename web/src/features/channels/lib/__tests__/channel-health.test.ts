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
import type { TFunction } from 'i18next'
import { describe, expect, test } from 'vitest'

import {
  healthTileFilterPatch,
  healthTileLabel,
  isHealthTileActive,
  resolveActiveHealthTiles,
  resolveHealthStripState,
  type ActiveHealthTiles,
} from '../channel-health'

// A minimal stand-in for react-i18next's `t`: returns the key with any
// `{{token}}` interpolated from `options`, mirroring what the real i18next
// instance does for these keys once translated.
const fakeT = ((key: string, options?: Record<string, unknown>) =>
  key.replaceAll(/{{(\w+)}}/g, (_, token) =>
    String(options?.[token] ?? '')
  )) as TFunction

const healthy = {
  active: 18,
  disabled: 0,
  slow: 0,
  untested: 0,
  slow_threshold_ms: 1000,
}

describe('resolveHealthStripState', () => {
  test('hides the strip when the summary is missing', () => {
    expect(resolveHealthStripState(undefined)).toStrictEqual({ kind: 'hidden' })
  })

  test('collapses to a single line when nothing is wrong', () => {
    expect(resolveHealthStripState(healthy)).toStrictEqual({
      kind: 'calm',
      total: 18,
    })
  })

  test('shows every tile when any bucket is non-empty', () => {
    const state = resolveHealthStripState({ ...healthy, active: 12, slow: 3 })
    expect(state.kind).toBe('alert')
    expect(state.kind === 'alert' ? state.tiles : []).toStrictEqual([
      { id: 'active', count: 12 },
      { id: 'disabled', count: 0 },
      { id: 'slow', count: 3 },
      { id: 'untested', count: 0 },
    ])
  })

  test('a lone untested channel is enough to expand the strip', () => {
    const state = resolveHealthStripState({ ...healthy, untested: 1 })
    expect(state.kind).toBe('alert')
  })

  test('an instance with no channels at all stays collapsed', () => {
    const state = resolveHealthStripState({ ...healthy, active: 0 })
    expect(state).toStrictEqual({ kind: 'calm', total: 0 })
  })
})

const noTiles: ActiveHealthTiles = { status: null, health: null }

describe('resolveActiveHealthTiles', () => {
  test('both dimensions are null when neither filter is set', () => {
    expect(resolveActiveHealthTiles([], undefined)).toStrictEqual(noTiles)
  })

  test('reads enabled/disabled status filters as active/disabled', () => {
    expect(resolveActiveHealthTiles(['enabled'], undefined)).toStrictEqual({
      status: 'active',
      health: null,
    })
    expect(resolveActiveHealthTiles(['disabled'], undefined)).toStrictEqual({
      status: 'disabled',
      health: null,
    })
  })

  test('reads the health filter as the slow/untested tile', () => {
    expect(resolveActiveHealthTiles([], 'slow')).toStrictEqual({
      status: null,
      health: 'slow',
    })
    expect(resolveActiveHealthTiles([], 'untested')).toStrictEqual({
      status: null,
      health: 'untested',
    })
  })

  test('a status tile and a health tile are both active together', () => {
    expect(resolveActiveHealthTiles(['disabled'], 'untested')).toStrictEqual({
      status: 'disabled',
      health: 'untested',
    })
  })

  test('both dimensions are null for filter values the strip never writes', () => {
    expect(resolveActiveHealthTiles(['all'], undefined)).toStrictEqual(noTiles)
    expect(
      resolveActiveHealthTiles(['enabled', 'disabled'], undefined)
    ).toStrictEqual(noTiles)
  })
})

describe('isHealthTileActive', () => {
  test('matches the status dimension for active/disabled, not each other', () => {
    const tiles: ActiveHealthTiles = { status: 'disabled', health: null }
    expect(isHealthTileActive('disabled', tiles)).toBe(true)
    expect(isHealthTileActive('active', tiles)).toBe(false)
  })

  test('matches the health dimension for slow/untested, not each other', () => {
    const tiles: ActiveHealthTiles = { status: null, health: 'untested' }
    expect(isHealthTileActive('untested', tiles)).toBe(true)
    expect(isHealthTileActive('slow', tiles)).toBe(false)
  })

  test('a status tile and a health tile can both read active', () => {
    const tiles: ActiveHealthTiles = { status: 'disabled', health: 'untested' }
    expect(isHealthTileActive('disabled', tiles)).toBe(true)
    expect(isHealthTileActive('untested', tiles)).toBe(true)
  })
})

describe('healthTileFilterPatch', () => {
  test('active and disabled write the status filter', () => {
    expect(healthTileFilterPatch('active', noTiles)).toStrictEqual({
      status: ['enabled'],
    })
    expect(healthTileFilterPatch('disabled', noTiles)).toStrictEqual({
      status: ['disabled'],
    })
  })

  test('slow and untested write the health filter', () => {
    expect(healthTileFilterPatch('slow', noTiles)).toStrictEqual({
      health: 'slow',
    })
    expect(healthTileFilterPatch('untested', noTiles)).toStrictEqual({
      health: 'untested',
    })
  })

  test('a status tile then a health tile leaves both active (composes)', () => {
    const afterDisabled: ActiveHealthTiles = {
      status: 'disabled',
      health: null,
    }
    expect(healthTileFilterPatch('untested', afterDisabled)).toStrictEqual({
      health: 'untested',
    })
    // The patch only touches `health` — `status` is absent, meaning "leave
    // it alone", so applying this patch on top of afterDisabled keeps both
    // status=disabled and health=untested active.
  })

  test('selecting the other status tile swaps rather than adds', () => {
    const bothActive: ActiveHealthTiles = {
      status: 'disabled',
      health: 'untested',
    }
    expect(healthTileFilterPatch('active', bothActive)).toStrictEqual({
      status: ['enabled'],
    })
  })

  test('selecting the other health tile swaps rather than adds', () => {
    const bothActive: ActiveHealthTiles = {
      status: 'disabled',
      health: 'untested',
    }
    expect(healthTileFilterPatch('slow', bothActive)).toStrictEqual({
      health: 'slow',
    })
  })

  test('selecting an already-lit tile clears only its own dimension', () => {
    const bothActive: ActiveHealthTiles = {
      status: 'disabled',
      health: 'untested',
    }
    expect(healthTileFilterPatch('disabled', bothActive)).toStrictEqual({
      status: null,
    })
    expect(healthTileFilterPatch('untested', bothActive)).toStrictEqual({
      health: null,
    })
  })
})

describe('healthTileLabel', () => {
  test('active and disabled read their plain status label', () => {
    expect(healthTileLabel('active', 1000, fakeT)).toBe('Active')
    expect(healthTileLabel('disabled', 1000, fakeT)).toBe('Disabled')
  })

  test('untested reads a fixed label independent of the threshold', () => {
    expect(healthTileLabel('untested', 1000, fakeT)).toBe('Never tested')
  })

  test('slow interpolates the server-provided threshold, same as the tile', () => {
    expect(healthTileLabel('slow', 1500, fakeT)).toBe('Slower than 1.50s')
    expect(healthTileLabel('slow', 500, fakeT)).toBe('Slower than 500ms')
  })
})
