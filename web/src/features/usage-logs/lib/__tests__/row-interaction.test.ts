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
import { describe, expect, test } from 'vitest'

import type { UsageLog } from '../../data/schema'
import { LOG_ROW_SELECTED_CLASS, isLogRowSelected } from '../row-interaction'

const log = (id: number) => ({ id }) as UsageLog

describe('row selection highlight', () => {
  test('marks only the row shown in the panel', () => {
    expect(isLogRowSelected(log(1), log(1))).toBe(true)
    expect(isLogRowSelected(log(2), log(1))).toBe(false)
    expect(isLogRowSelected(log(1), null)).toBe(false)
  })

  test('uses a theme token class', () => {
    expect(LOG_ROW_SELECTED_CLASS).toBe('bg-accent')
  })
})
