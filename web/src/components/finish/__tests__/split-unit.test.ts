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
import { describe, expect, it } from 'vitest'

import { splitUnit } from '../split-unit'
import { toneFromStatusVariant } from '../tone'

describe('splitUnit', () => {
  it('splits only a trailing ms, s or percent', () => {
    expect(splitUnit('1.2s')).toEqual({ value: '1.2', unit: 's' })
    expect(splitUnit('350 ms')).toEqual({ value: '350', unit: 'ms' })
    expect(splitUnit('12.5%')).toEqual({ value: '12.5', unit: '%' })
    expect(splitUnit('12,5 %')).toEqual({ value: '12,5', unit: '%' })
    expect(splitUnit('12,5 %')).toEqual({ value: '12,5', unit: '%' })
    expect(splitUnit('1m 3s')).toEqual({ value: '1m 3s', unit: '' })
    expect(splitUnit('$0.5885')).toEqual({ value: '$0.5885', unit: '' })
    expect(splitUnit('-')).toEqual({ value: '-', unit: '' })
    expect(splitUnit('••••')).toEqual({ value: '••••', unit: '' })
  })
})

describe('toneFromStatusVariant', () => {
  it('maps status variants to tones', () => {
    expect(toneFromStatusVariant('green')).toBe('success')
    expect(toneFromStatusVariant('red')).toBe('danger')
    expect(toneFromStatusVariant('amber')).toBe('warning')
    expect(toneFromStatusVariant('blue')).toBe('info')
    expect(toneFromStatusVariant('purple')).toBe('neutral')
    expect(toneFromStatusVariant(undefined)).toBe('neutral')
  })
})
