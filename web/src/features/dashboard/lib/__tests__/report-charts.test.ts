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

import { buildTrendSpec } from '../report-charts'
import type { ReportTrendPoint } from '../../types'

const point = (over: Partial<ReportTrendPoint>): ReportTrendPoint => ({
  bucket_label: '2026-07-30',
  bucket_timestamp: 1785400000,
  quota: 0,
  requests: 0,
  tokens: 0,
  failures: 0,
  avg_latency_ms: 0,
  consuming_requests: 0,
  ...over,
})

describe('buildTrendSpec', () => {
  it('returns null when there are no points so the panel can show its empty state', () => {
    expect(buildTrendSpec([], 'quota', 'Cost')).toBeNull()
  })

  it('plots the selected metric and leaves the other measures out of the series', () => {
    const spec = buildTrendSpec(
      [point({ bucket_label: 'a', quota: 10, requests: 4 }), point({ bucket_label: 'b', quota: 30, requests: 9 })],
      'requests',
      'Requests'
    )

    const data = spec?.data as [{ values: { time: string; value: number; series: string }[] }]
    expect(data[0].values).toEqual([
      { time: 'a', value: 4, series: 'Requests' },
      { time: 'b', value: 9, series: 'Requests' },
    ])
  })

  it('hides the legend for its single series', () => {
    const spec = buildTrendSpec([point({ quota: 1 })], 'quota', 'Cost')
    expect(spec?.legends).toEqual({ visible: false })
  })
})
