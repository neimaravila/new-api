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
import type { ReportRankingRow, ReportRankingSortKey, ReportSortDirection } from '../types'

export function sortRankingRows(
  rows: ReportRankingRow[],
  key: ReportRankingSortKey,
  direction: ReportSortDirection
): ReportRankingRow[] {
  const sign = direction === 'asc' ? 1 : -1
  return [...rows].sort((a, b) => sign * (a[key] - b[key]))
}

function escapeCell(value: string | number): string {
  let text = String(value)
  // Spreadsheets run cells starting with these characters as formulas.
  if (/^[=+\-@]/.test(text)) text = `'${text}`
  if (/[",\r\n]/.test(text)) text = `"${text.replaceAll('"', '""')}"`
  return text
}

export function rankingRowsToCsv(rows: (string | number)[][], headers: string[]): string {
  return [headers, ...rows].map((line) => line.map(escapeCell).join(',')).join('\r\n')
}
