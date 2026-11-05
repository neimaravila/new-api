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

import type { ReportRankingRow } from '../types'
import { rankingRowsToCsv, sortRankingRows } from './report-ranking'

const row = (name: string, over: Partial<ReportRankingRow> = {}): ReportRankingRow => ({
  name,
  requests: 1,
  tokens: 0,
  quota: 0,
  failures: 0,
  error_rate: 0,
  avg_latency_ms: 0,
  p50_latency_ms: 0,
  p95_latency_ms: 0,
  ...over,
})

describe('sortRankingRows', () => {
  it('sorts by any metric without mutating the input', () => {
    const rows = [row('a', { tokens: 5 }), row('b', { tokens: 9 }), row('c', { tokens: 1 })]
    expect(sortRankingRows(rows, 'tokens', 'desc').map((r) => r.name)).toEqual(['b', 'a', 'c'])
    expect(sortRankingRows(rows, 'tokens', 'asc').map((r) => r.name)).toEqual(['c', 'a', 'b'])
    expect(rows.map((r) => r.name)).toEqual(['a', 'b', 'c'])
  })
})

describe('sortRankingRows latency without samples', () => {
  const rows = [
    row('none', { avg_latency_ms: 0, p50_latency_ms: 0, p95_latency_ms: 0 }),
    row('fast', { avg_latency_ms: 10, p50_latency_ms: 10, p95_latency_ms: 10 }),
    row('slow', { avg_latency_ms: 90, p50_latency_ms: 90, p95_latency_ms: 90 }),
  ]

  it.each(['avg_latency_ms', 'p50_latency_ms', 'p95_latency_ms'] as const)(
    'puts rows with no sample last in both directions for %s',
    (key) => {
      expect(sortRankingRows(rows, key, 'asc').map((r) => r.name)).toEqual(['fast', 'slow', 'none'])
      expect(sortRankingRows(rows, key, 'desc').map((r) => r.name)).toEqual(['slow', 'fast', 'none'])
    }
  )

  it('still sorts zero as a real value for other metrics', () => {
    const list = [row('z', { tokens: 0 }), row('y', { tokens: 3 })]
    expect(sortRankingRows(list, 'tokens', 'asc').map((r) => r.name)).toEqual(['z', 'y'])
  })
})

describe('rankingRowsToCsv', () => {
  it('escapes quotes, commas and line breaks', () => {
    const csv = rankingRowsToCsv([['a "q", b\nc', 1]], ['Name', 'N'])
    expect(csv).toBe('Name,N\r\n"a ""q"", b\nc",1')
  })

  it('prefixes formula-like cells with an apostrophe', () => {
    const csv = rankingRowsToCsv([['=1+1', '+x'], ['-y', '@z']], ['A', 'B'])
    expect(csv).toBe("A,B\r\n'=1+1,'+x\r\n'-y,'@z")
  })
})
