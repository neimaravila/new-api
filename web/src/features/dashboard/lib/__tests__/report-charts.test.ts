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

import { formatNumber, formatQuota } from '@/lib/format'

import {
  ERROR_DONUT_MAX_SLICES,
  buildChannelCostSpec,
  buildErrorShareSpec,
  buildModelCostSpec,
  buildPerformanceSpec,
  buildStreamSpec,
  buildTokenAnatomySpec,
  buildTrendSpec,
  type ReportChartSpec,
} from '../report-charts'
import type {
  ReportChannelRow,
  ReportErrorRow,
  ReportModelRow,
  ReportPerformanceRow,
  ReportTrendPoint,
} from '../../types'

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

const perfRow = (name: string, avg: number, p95: number): ReportPerformanceRow => ({
  name,
  avg_latency_ms: avg,
  p95_latency_ms: p95,
  requests: 5,
  tokens: 100,
  throughput: 20,
})

describe('buildPerformanceSpec', () => {
  it('returns null for no rows', () => {
    expect(buildPerformanceSpec([], 'Avg', 'p95')).toBeNull()
  })

  it('emits avg and p95 as two series in the same millisecond scale', () => {
    const spec = buildPerformanceSpec([perfRow('gpt-4o', 500, 900)], 'Avg', 'p95')
    const data = spec?.data as [{ values: { name: string; series: string; value: number }[] }]
    const values = data[0].values
    expect(values).toEqual([
      { name: 'gpt-4o', series: 'Avg', value: 500 },
      { name: 'gpt-4o', series: 'p95', value: 900 },
    ])
    expect(spec?.legends).toEqual({ visible: true, orient: 'bottom' })
  })
})

describe('buildTokenAnatomySpec', () => {
  it('stacks prompt, completion and cache per model', () => {
    const spec = buildTokenAnatomySpec(
      [{ model_name: 'claude', prompt_tokens: 100, completion_tokens: 200, cache_tokens: 50, total: 350 }],
      { prompt: 'Prompt', completion: 'Completion', cache: 'Cache' }
    )
    expect(spec?.stack).toBe(true)
    const data = spec?.data as [{ values: { series: string; value: number }[] }]
    const values = data[0].values
    expect(values).toEqual([
      { name: 'claude', series: 'Prompt', value: 100 },
      { name: 'claude', series: 'Completion', value: 200 },
      { name: 'claude', series: 'Cache', value: 50 },
    ])
  })

  it('returns null for no rows', () => {
    expect(buildTokenAnatomySpec([], { prompt: 'p', completion: 'c', cache: 'k' })).toBeNull()
  })
})

const STATUS_LABELS = { healthy: 'Healthy', degraded: 'Degraded' }

const channel = (name: string, quota: number, errorRate: number): ReportChannelRow => ({
  channel_id: name.length,
  channel_name: name,
  quota,
  requests: 10,
  failures: 5,
  error_rate: errorRate,
  avg_latency_ms: 100,
})

type ChannelColor = { type: string; domain: string[]; range: string[] }

describe('buildChannelCostSpec', () => {
  it('marks channels above the error threshold so the bar carries status, not a second axis', () => {
    const spec = buildChannelCostSpec([channel('ok', 100, 0), channel('bad', 90, 0.5)], STATUS_LABELS)
    const data = spec?.data as [{ values: { name: string; status: string }[] }]
    expect(data[0].values).toEqual([
      { name: 'ok', quota: 100, status: 'Healthy' },
      { name: 'bad', quota: 90, status: 'Degraded' },
    ])
  })

  it('pins the status hues to an explicit domain so data order cannot swap them', () => {
    const healthyFirst = buildChannelCostSpec(
      [channel('ok', 100, 0), channel('bad', 90, 0.5)],
      STATUS_LABELS
    )
    const degradedFirst = buildChannelCostSpec(
      [channel('bad', 100, 0.5), channel('ok', 90, 0)],
      STATUS_LABELS
    )

    const first = healthyFirst?.color as ChannelColor
    const second = degradedFirst?.color as ChannelColor
    expect(first.type).toBe('ordinal')
    expect(first.domain).toEqual(['Healthy', 'Degraded'])
    expect(second).toEqual(first)
    expect(first.range.length).toBeGreaterThanOrEqual(2)
    expect(first.range[0]).not.toBe(first.range[1])
  })

  it('names both states in a visible legend so status is never hue alone', () => {
    const spec = buildChannelCostSpec([channel('ok', 100, 0)], STATUS_LABELS)
    expect(spec?.legends).toEqual({ visible: true, orient: 'bottom' })
    expect(spec?.seriesField).toBe('status')
  })
})

describe('buildStreamSpec', () => {
  it('returns null when neither mode had requests', () => {
    const spec = buildStreamSpec(
      { stream_avg_latency_ms: 0, stream_requests: 0, non_stream_avg_latency_ms: 0, non_stream_requests: 0 },
      'Stream',
      'Non-stream'
    )
    expect(spec).toBeNull()
  })

  it('pairs stream against non-stream request counts', () => {
    const spec = buildStreamSpec(
      { stream_avg_latency_ms: 400, stream_requests: 7, non_stream_avg_latency_ms: 900, non_stream_requests: 3 },
      'Stream',
      'Non-stream'
    )
    const data = spec?.data as [{ values: { name: string; value: number }[] }]
    const values = data[0].values
    expect(values).toEqual([
      { name: 'Stream', value: 7 },
      { name: 'Non-stream', value: 3 },
    ])
  })
})

describe('chart value formatting', () => {
  interface AxisSpec {
    orient: string
    label?: { formatMethod?: (value: number) => string }
  }
  interface TooltipSpec {
    mark: { content: { value: (datum: Record<string, unknown>) => string }[] }
  }

  const valueAxisFormat = (spec: ReportChartSpec | null, orient: string) => {
    const axes = spec?.axes as AxisSpec[]
    return axes.find((axis) => axis.orient === orient)?.label?.formatMethod
  }

  it('renders trend cost on the value axis as currency, not raw quota units', () => {
    const spec = buildTrendSpec([point({ quota: 500_000 })], 'quota', 'Cost')
    const format = valueAxisFormat(spec, 'left')
    expect(format?.(500_000)).toBe(formatQuota(500_000))
  })

  it('renders trend counts with separators rather than bare integers', () => {
    const spec = buildTrendSpec([point({ requests: 12_345 })], 'requests', 'Requests')
    const format = valueAxisFormat(spec, 'left')
    expect(format?.(12_345)).toBe(formatNumber(12_345))
  })

  it('formats the trend tooltip with the selected metric unit', () => {
    const spec = buildTrendSpec([point({ quota: 500_000 })], 'quota', 'Cost')
    const tooltip = spec?.tooltip as TooltipSpec
    expect(tooltip.mark.content[0].value({ value: 500_000 })).toBe(formatQuota(500_000))
  })

  it('formats model cost tooltips as currency', () => {
    const spec = buildModelCostSpec([model('gpt-4o', 500_000)])
    const tooltip = spec?.tooltip as TooltipSpec
    expect(tooltip.mark.content[0].value({ quota: 500_000 })).toBe(formatQuota(500_000))
  })

  it('labels latency values with their millisecond unit', () => {
    const spec = buildPerformanceSpec([perfRow('gpt-4o', 500, 900)], 'Avg', 'p95')
    const format = valueAxisFormat(spec, 'bottom')
    expect(format?.(900)).toBe(`${formatNumber(900)} ms`)
  })
})
