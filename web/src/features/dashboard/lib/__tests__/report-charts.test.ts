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

import { ERROR_DONUT_MAX_SLICES, buildErrorShareSpec, buildModelCostSpec, buildTrendSpec } from '../report-charts'
import type { ReportErrorRow, ReportModelRow, ReportTrendPoint } from '../../types'

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

const model = (name: string, quota: number): ReportModelRow => ({
  model_name: name,
  quota,
  requests: 1,
  tokens: 1,
  failures: 0,
  error_rate: 0,
  avg_latency_ms: 100,
})

const errorRow = (name: string, failures: number): ReportErrorRow => ({
  model_name: name,
  failures,
  quota: 0,
  share: 0,
})

describe('buildModelCostSpec', () => {
  it('returns null for no models', () => {
    expect(buildModelCostSpec([])).toBeNull()
  })

  it('ranks models by cost descending so the widest bar is on top', () => {
    const spec = buildModelCostSpec([model('cheap', 10), model('expensive', 90)])
    const data = spec?.data as [{ values: { name: string; quota: number }[] }]
    expect(data[0].values.map((row) => row.name)).toEqual(['expensive', 'cheap'])
  })
})

describe('buildErrorShareSpec', () => {
  it('returns null when every model has zero failures', () => {
    expect(buildErrorShareSpec([errorRow('gpt-4o', 0)], 'Other')).toBeNull()
  })

  it('collapses the tail past the slice cap into a single Other slice that keeps the total', () => {
    const rows = Array.from({ length: ERROR_DONUT_MAX_SLICES + 3 }, (_, i) => errorRow(`m${i}`, 10 - i))
    const spec = buildErrorShareSpec(rows, 'Other')

    const data = spec?.data as [{ values: { type: string; value: number }[] }]
    const values = data[0].values
    expect(values).toHaveLength(ERROR_DONUT_MAX_SLICES + 1)
    expect(values.at(-1)).toEqual({ type: 'Other', value: 4 + 3 + 2 })
    expect(values.reduce((sum, item) => sum + item.value, 0)).toBe(
      rows.reduce((sum, row) => sum + row.failures, 0)
    )
  })

  it('keeps every model when the count is at the cap', () => {
    const rows = Array.from({ length: ERROR_DONUT_MAX_SLICES }, (_, i) => errorRow(`m${i}`, 5))
    const spec = buildErrorShareSpec(rows, 'Other')
    const data = spec?.data as [{ values: unknown[] }]
    expect(data[0].values).toHaveLength(ERROR_DONUT_MAX_SLICES)
  })
})
