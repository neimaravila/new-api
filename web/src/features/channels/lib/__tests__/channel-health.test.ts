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

import {
  healthTileFilterPatch,
  resolveActiveHealthTile,
  resolveHealthStripState,
} from '../channel-health'

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

describe('resolveActiveHealthTile', () => {
  test('is null when neither filter is set', () => {
    assert.equal(resolveActiveHealthTile([], undefined), null)
  })

  test('reads enabled/disabled status filters as active/disabled tiles', () => {
    assert.equal(resolveActiveHealthTile(['enabled'], undefined), 'active')
    assert.equal(resolveActiveHealthTile(['disabled'], undefined), 'disabled')
  })

  test('reads the health filter as the slow/untested tiles', () => {
    assert.equal(resolveActiveHealthTile([], 'slow'), 'slow')
    assert.equal(resolveActiveHealthTile([], 'untested'), 'untested')
  })

  test('is null for filter values the strip never writes', () => {
    assert.equal(resolveActiveHealthTile(['all'], undefined), null)
    assert.equal(resolveActiveHealthTile(['enabled', 'disabled'], undefined), null)
  })

  test('prefers the health filter when a stale URL sets both', () => {
    assert.equal(resolveActiveHealthTile(['disabled'], 'untested'), 'untested')
  })
})

describe('healthTileFilterPatch', () => {
  test('active and disabled write the status filter', () => {
    assert.deepEqual(healthTileFilterPatch('active', null), {
      status: ['enabled'],
    })
    assert.deepEqual(healthTileFilterPatch('disabled', null), {
      status: ['disabled'],
    })
  })

  test('slow and untested write the health filter', () => {
    assert.deepEqual(healthTileFilterPatch('slow', null), { health: 'slow' })
    assert.deepEqual(healthTileFilterPatch('untested', null), {
      health: 'untested',
    })
  })

  test('selecting a different tile replaces the previous choice', () => {
    assert.deepEqual(healthTileFilterPatch('slow', 'disabled'), {
      health: 'slow',
    })
    assert.deepEqual(healthTileFilterPatch('active', 'untested'), {
      status: ['enabled'],
    })
  })

  test('selecting the already-active tile clears it', () => {
    assert.deepEqual(healthTileFilterPatch('disabled', 'disabled'), {})
    assert.deepEqual(healthTileFilterPatch('untested', 'untested'), {})
  })
})
