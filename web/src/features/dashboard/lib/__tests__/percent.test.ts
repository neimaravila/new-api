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

import { formatFractionPercent } from '../percent'

describe('formatFractionPercent', () => {
  it('uses the locale decimal separator and at most one decimal', () => {
    expect(formatFractionPercent(0.126, 'en')).toBe('12.6%')
    expect(formatFractionPercent(0.126, 'pt-BR')).toBe('12,6%')
    expect(formatFractionPercent(0.15, 'en')).toBe('15%')
    expect(formatFractionPercent(0, 'en')).toBe('0%')
  })
})
