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
import { toCsvLine } from '@/features/usage-logs/lib/export'

import type { ReportRankingRow, ReportRankingSortKey, ReportSortDirection } from '../types'

const LATENCY_KEYS = new Set<ReportRankingSortKey>(['avg_latency_ms', 'p50_latency_ms', 'p95_latency_ms'])

export function sortRankingRows(
  rows: ReportRankingRow[],
  key: ReportRankingSortKey,
  direction: ReportSortDirection
): ReportRankingRow[] {
  const sign = direction === 'asc' ? 1 : -1
  // Zero latency means "no sample" and must not rank as the fastest value.
  const noSample = LATENCY_KEYS.has(key)
  return [...rows].sort((a, b) => {
    if (noSample && (a[key] <= 0 || b[key] <= 0)) {
      return Number(a[key] <= 0) - Number(b[key] <= 0)
    }
    return sign * (a[key] - b[key])
  })
}

/** CSV text of the displayed table; escaping lives in the usage-logs helper. */
export function rankingRowsToCsv(rows: (string | number)[][], headers: string[]): string {
  return [headers, ...rows].map((line) => toCsvLine(line.map(String))).join('')
}
