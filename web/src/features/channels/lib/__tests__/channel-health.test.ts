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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { TFunction } from 'i18next'

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

const noTiles: ActiveHealthTiles = { status: null, health: null }

describe('resolveActiveHealthTiles', () => {
  test('both dimensions are null when neither filter is set', () => {
    assert.deepEqual(resolveActiveHealthTiles([], undefined), noTiles)
  })

  test('reads enabled/disabled status filters as active/disabled', () => {
    assert.deepEqual(resolveActiveHealthTiles(['enabled'], undefined), {
      status: 'active',
      health: null,
    })
    assert.deepEqual(resolveActiveHealthTiles(['disabled'], undefined), {
      status: 'disabled',
      health: null,
    })
  })

  test('reads the health filter as the slow/untested tile', () => {
    assert.deepEqual(resolveActiveHealthTiles([], 'slow'), {
      status: null,
      health: 'slow',
    })
    assert.deepEqual(resolveActiveHealthTiles([], 'untested'), {
      status: null,
      health: 'untested',
    })
  })

  test('a status tile and a health tile are both active together', () => {
    assert.deepEqual(resolveActiveHealthTiles(['disabled'], 'untested'), {
      status: 'disabled',
      health: 'untested',
    })
  })

  test('both dimensions are null for filter values the strip never writes', () => {
    assert.deepEqual(resolveActiveHealthTiles(['all'], undefined), noTiles)
    assert.deepEqual(
      resolveActiveHealthTiles(['enabled', 'disabled'], undefined),
      noTiles
    )
  })
})

describe('isHealthTileActive', () => {
  test('matches the status dimension for active/disabled, not each other', () => {
    const tiles: ActiveHealthTiles = { status: 'disabled', health: null }
    assert.equal(isHealthTileActive('disabled', tiles), true)
    assert.equal(isHealthTileActive('active', tiles), false)
  })

  test('matches the health dimension for slow/untested, not each other', () => {
    const tiles: ActiveHealthTiles = { status: null, health: 'untested' }
    assert.equal(isHealthTileActive('untested', tiles), true)
    assert.equal(isHealthTileActive('slow', tiles), false)
  })

  test('a status tile and a health tile can both read active', () => {
    const tiles: ActiveHealthTiles = { status: 'disabled', health: 'untested' }
    assert.equal(isHealthTileActive('disabled', tiles), true)
    assert.equal(isHealthTileActive('untested', tiles), true)
  })
})

describe('healthTileFilterPatch', () => {
  test('active and disabled write the status filter', () => {
    assert.deepEqual(healthTileFilterPatch('active', noTiles), {
      status: ['enabled'],
    })
    assert.deepEqual(healthTileFilterPatch('disabled', noTiles), {
      status: ['disabled'],
    })
  })

  test('slow and untested write the health filter', () => {
    assert.deepEqual(healthTileFilterPatch('slow', noTiles), {
      health: 'slow',
    })
    assert.deepEqual(healthTileFilterPatch('untested', noTiles), {
      health: 'untested',
    })
  })

  test('a status tile then a health tile leaves both active (composes)', () => {
    const afterDisabled: ActiveHealthTiles = {
      status: 'disabled',
      health: null,
    }
    assert.deepEqual(healthTileFilterPatch('untested', afterDisabled), {
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
    assert.deepEqual(healthTileFilterPatch('active', bothActive), {
      status: ['enabled'],
    })
  })

  test('selecting the other health tile swaps rather than adds', () => {
    const bothActive: ActiveHealthTiles = {
      status: 'disabled',
      health: 'untested',
    }
    assert.deepEqual(healthTileFilterPatch('slow', bothActive), {
      health: 'slow',
    })
  })

  test('selecting an already-lit tile clears only its own dimension', () => {
    const bothActive: ActiveHealthTiles = {
      status: 'disabled',
      health: 'untested',
    }
    assert.deepEqual(healthTileFilterPatch('disabled', bothActive), {
      status: null,
    })
    assert.deepEqual(healthTileFilterPatch('untested', bothActive), {
      health: null,
    })
  })
})

describe('healthTileLabel', () => {
  test('active and disabled read their plain status label', () => {
    assert.equal(healthTileLabel('active', 1000, fakeT), 'Active')
    assert.equal(healthTileLabel('disabled', 1000, fakeT), 'Disabled')
  })

  test('untested reads a fixed label independent of the threshold', () => {
    assert.equal(healthTileLabel('untested', 1000, fakeT), 'Never tested')
  })

  test('slow interpolates the server-provided threshold, same as the tile', () => {
    assert.equal(healthTileLabel('slow', 1500, fakeT), 'Slower than 1.50s')
    assert.equal(healthTileLabel('slow', 500, fakeT), 'Slower than 500ms')
  })
})
