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
import { formatLatencyMs, formatNumber, formatQuota } from '@/lib/format'

import {
  REPORT_COST_OTHERS_KEY,
  type ReportChannelRow,
  type ReportCostSeries,
  type ReportLatencyPoint,
  type ReportErrorRow,
  type ReportModelRow,
  type ReportPerformanceRow,
  type ReportStreamComparison,
  type ReportTokenAnatomyRow,
  type ReportTrendPoint,
} from '../types'
import { getDashboardChartColors } from './charts'

export type ReportChartSpec = Record<string, unknown>

export type ReportTrendMetric = 'quota' | 'requests' | 'tokens' | 'failures'

const TREND_METRIC_FIELD: Record<ReportTrendMetric, keyof ReportTrendPoint> = {
  quota: 'quota',
  requests: 'requests',
  tokens: 'tokens',
  failures: 'failures',
}

/**
 * Renders one chart value the way the table under that chart renders it.
 *
 * Left alone, VChart prints the raw datum on axis labels and tooltips, which
 * shows quota in internal units rather than currency and token counts without
 * separators. Every builder below routes its value through one of these.
 */
type ChartValueFormat = (value: number) => string

const formatCount: ChartValueFormat = (value) => formatNumber(value)
const formatLatency: ChartValueFormat = (value) => formatLatencyMs(value)

const TREND_METRIC_FORMAT: Record<ReportTrendMetric, ChartValueFormat> = {
  quota: formatQuota,
  requests: formatCount,
  tokens: formatCount,
  failures: formatCount,
}

function valueAxisLabel(format: ChartValueFormat): Record<string, unknown> {
  return { formatMethod: (value: number) => format(Number(value) || 0) }
}

function valueTooltip(
  keyField: string,
  valueField: string,
  format: ChartValueFormat
): Record<string, unknown> {
  const content = [
    {
      key: (datum: Record<string, unknown>) => datum?.[keyField],
      value: (datum: Record<string, unknown>) => format(Number(datum?.[valueField]) || 0),
    },
  ]
  return { mark: { content }, dimension: { content } }
}

export function buildTrendSpec(
  points: ReportTrendPoint[],
  metric: ReportTrendMetric,
  seriesLabel: string
): ReportChartSpec | null {
  if (points.length === 0) return null

  const field = TREND_METRIC_FIELD[metric]
  const format = TREND_METRIC_FORMAT[metric]
  const values = points.map((item) => ({
    time: item.bucket_label,
    value: Number(item[field]) || 0,
    series: seriesLabel,
  }))

  return {
    type: 'area',
    data: [{ id: 'reportTrend', values }],
    xField: 'time',
    yField: 'value',
    seriesField: 'series',
    stack: false,
    legends: { visible: false },
    color: getDashboardChartColors(1).slice(0, 1),
    line: { style: { lineWidth: 2 } },
    point: { visible: false },
    crosshair: { xField: { visible: true, line: { visible: true } } },
    axes: [
      { orient: 'bottom', type: 'band' },
      { orient: 'left', type: 'linear', label: valueAxisLabel(format) },
    ],
    tooltip: valueTooltip('series', 'value', format),
    background: 'transparent',
    animation: true,
  }
}

export const ERROR_DONUT_MAX_SLICES = 6

export function buildModelCostSpec(models: ReportModelRow[]): ReportChartSpec | null {
  if (models.length === 0) return null

  const values = [...models]
    .sort((a, b) => b.quota - a.quota)
    .map((item) => ({ name: item.model_name, quota: item.quota }))

  return {
    type: 'bar',
    data: [{ id: 'reportModelCost', values }],
    xField: 'quota',
    yField: 'name',
    direction: 'horizontal',
    legends: { visible: false },
    color: getDashboardChartColors(1).slice(0, 1),
    bar: { state: { hover: { stroke: '#000', lineWidth: 1 } } },
    axes: [
      { orient: 'left', type: 'band' },
      { orient: 'bottom', type: 'linear', visible: false },
    ],
    tooltip: valueTooltip('name', 'quota', formatQuota),
    background: 'transparent',
    animation: true,
  }
}

export function buildErrorShareSpec(
  errors: ReportErrorRow[],
  otherLabel: string
): ReportChartSpec | null {
  if (errors.length === 0) return null

  const ranked = [...errors].sort((a, b) => b.failures - a.failures)
  const values = ranked
    .slice(0, ERROR_DONUT_MAX_SLICES)
    .map((item) => ({ type: item.model_name, value: item.failures }))
  const tail = ranked.slice(ERROR_DONUT_MAX_SLICES)
  if (tail.length > 0) {
    values.push({
      type: otherLabel,
      value: tail.reduce((sum, item) => sum + item.failures, 0),
    })
  }

  const total = values.reduce((sum, item) => sum + item.value, 0)
  if (total <= 0) return null

  return {
    type: 'pie',
    data: [{ id: 'reportErrorShare', values }],
    outerRadius: 0.8,
    innerRadius: 0.55,
    padAngle: 0.6,
    valueField: 'value',
    categoryField: 'type',
    legends: { visible: true, orient: 'bottom' },
    label: { visible: true },
    color: getDashboardChartColors(values.length),
    pie: { state: { hover: { outerRadius: 0.85, stroke: '#000', lineWidth: 1 } } },
    tooltip: valueTooltip('type', 'value', formatCount),
    background: 'transparent',
    animation: true,
  }
}

export const CHANNEL_ERROR_RATE_THRESHOLD = 0.05

export function buildPerformanceSpec(
  rows: ReportPerformanceRow[],
  avgLabel: string,
  p95Label: string
): ReportChartSpec | null {
  if (rows.length === 0) return null

  // A latency of 0 means no sample, not an instant response: leave it out.
  const values = rows
    .flatMap((item) => [
      { name: item.name, series: avgLabel, value: item.avg_latency_ms },
      { name: item.name, series: p95Label, value: item.p95_latency_ms },
    ])
    .filter((item) => item.value > 0)

  return {
    type: 'bar',
    data: [{ id: 'reportPerformance', values }],
    xField: 'value',
    yField: ['name', 'series'],
    seriesField: 'series',
    direction: 'horizontal',
    legends: { visible: true, orient: 'bottom' },
    color: getDashboardChartColors(2).slice(0, 2),
    axes: [
      { orient: 'left', type: 'band' },
      { orient: 'bottom', type: 'linear', label: valueAxisLabel(formatLatency) },
    ],
    tooltip: valueTooltip('series', 'value', formatLatency),
    background: 'transparent',
    animation: true,
  }
}

export function buildTokenAnatomySpec(
  rows: ReportTokenAnatomyRow[],
  labels: { prompt: string; completion: string; cache: string }
): ReportChartSpec | null {
  if (rows.length === 0) return null

  const values = rows.flatMap((item) => [
    { name: item.model_name, series: labels.prompt, value: item.prompt_tokens },
    { name: item.model_name, series: labels.completion, value: item.completion_tokens },
    { name: item.model_name, series: labels.cache, value: item.cache_tokens },
  ])

  return {
    type: 'bar',
    data: [{ id: 'reportTokenAnatomy', values }],
    xField: 'value',
    yField: 'name',
    seriesField: 'series',
    direction: 'horizontal',
    stack: true,
    legends: { visible: true, orient: 'bottom' },
    color: getDashboardChartColors(3).slice(0, 3),
    bar: { style: { stroke: 'transparent', lineWidth: 2 } },
    axes: [
      { orient: 'left', type: 'band' },
      { orient: 'bottom', type: 'linear', label: valueAxisLabel(formatCount) },
    ],
    tooltip: valueTooltip('series', 'value', formatCount),
    background: 'transparent',
    animation: true,
  }
}

export interface ChannelStatusLabels {
  healthy: string
  degraded: string
}

export function buildChannelCostSpec(
  channels: ReportChannelRow[],
  statusLabels: ChannelStatusLabels
): ReportChartSpec | null {
  if (channels.length === 0) return null

  const values = channels.map((item) => ({
    name: item.channel_name,
    quota: item.quota,
    status:
      item.error_rate > CHANNEL_ERROR_RATE_THRESHOLD ? statusLabels.degraded : statusLabels.healthy,
  }))

  return {
    type: 'bar',
    data: [{ id: 'reportChannelCost', values }],
    xField: 'quota',
    yField: 'name',
    seriesField: 'status',
    direction: 'horizontal',
    legends: { visible: true, orient: 'bottom' },
    // Pin the ordinal domain: left to itself VChart derives it from data order,
    // so the healthy and degraded hues swap whenever the highest-cost channel
    // changes status between two time ranges. The legend then names both
    // states, so status never rests on hue alone.
    color: {
      type: 'ordinal',
      domain: [statusLabels.healthy, statusLabels.degraded],
      range: getDashboardChartColors(2).slice(0, 2),
    },
    axes: [
      { orient: 'left', type: 'band' },
      { orient: 'bottom', type: 'linear', visible: false },
    ],
    tooltip: valueTooltip('name', 'quota', formatQuota),
    background: 'transparent',
    animation: true,
  }
}

export function buildStreamSpec(
  stream: ReportStreamComparison,
  streamLabel: string,
  nonStreamLabel: string
): ReportChartSpec | null {
  if (stream.stream_requests <= 0 && stream.non_stream_requests <= 0) return null

  return {
    type: 'bar',
    data: [
      {
        id: 'reportStream',
        values: [
          { name: streamLabel, value: stream.stream_requests },
          { name: nonStreamLabel, value: stream.non_stream_requests },
        ],
      },
    ],
    xField: 'name',
    yField: 'value',
    seriesField: 'name',
    legends: { visible: false },
    color: getDashboardChartColors(2).slice(0, 2),
    axes: [
      { orient: 'bottom', type: 'band' },
      { orient: 'left', type: 'linear', label: valueAxisLabel(formatCount) },
    ],
    tooltip: valueTooltip('name', 'value', formatCount),
    background: 'transparent',
    animation: true,
  }
}

export type ReportChartKind = 'bar' | 'line'

interface TimeSeriesDatum {
  time: string
  series: string
  value: number
}

function buildTimeSeriesSpec(options: {
  id: string
  values: TimeSeriesDatum[]
  seriesOrder: string[]
  chartKind: ReportChartKind
  stack: boolean
  format: ChartValueFormat
  showLegend: boolean
}): ReportChartSpec {
  const { chartKind } = options
  return {
    type: chartKind,
    data: [{ id: options.id, values: options.values }],
    xField: 'time',
    yField: 'value',
    seriesField: 'series',
    ...(chartKind === 'bar' ? { stack: options.stack } : {}),
    legends: { visible: options.showLegend, orient: 'bottom' },
    // Pin the ordinal domain so a series keeps its hue when another one has no
    // data in the selected range.
    color: {
      type: 'ordinal',
      domain: options.seriesOrder,
      range: getDashboardChartColors(options.seriesOrder.length),
    },
    ...(chartKind === 'line'
      ? { line: { style: { lineWidth: 2 } }, point: { visible: options.values.length <= 48 } }
      : {}),
    crosshair: { xField: { visible: true, line: { visible: true } } },
    axes: [
      { orient: 'bottom', type: 'band' },
      { orient: 'left', type: 'linear', label: valueAxisLabel(options.format) },
    ],
    tooltip: valueTooltip('series', 'value', options.format),
    background: 'transparent',
    animation: true,
  }
}

export function buildVolumeSpec(
  points: ReportTrendPoint[],
  labels: { success: string; failures: string },
  chartKind: ReportChartKind
): ReportChartSpec | null {
  if (points.length === 0) return null
  // Requests include the failed ones, so the split never double counts.
  const values = points.flatMap((item) => [
    { time: item.bucket_label, series: labels.success, value: Math.max(item.requests - item.failures, 0) },
    { time: item.bucket_label, series: labels.failures, value: item.failures },
  ])
  return buildTimeSeriesSpec({
    id: 'reportVolume',
    values,
    seriesOrder: [labels.success, labels.failures],
    chartKind,
    stack: true,
    format: formatCount,
    showLegend: true,
  })
}

export function buildTokensSpec(
  points: ReportTrendPoint[],
  labels: { tokens: string },
  chartKind: ReportChartKind
): ReportChartSpec | null {
  if (points.length === 0) return null
  const values = points.map((item) => ({
    time: item.bucket_label,
    series: labels.tokens,
    value: item.tokens,
  }))
  return buildTimeSeriesSpec({
    id: 'reportTokens',
    values,
    seriesOrder: [labels.tokens],
    chartKind,
    stack: false,
    format: formatCount,
    showLegend: false,
  })
}

export function buildCostByModelSpec(
  data: ReportCostSeries,
  othersLabel: string,
  chartKind: ReportChartKind
): ReportChartSpec | null {
  if (data.points.length === 0) return null
  // Series without any cost in the period would only add an empty legend entry.
  const active = data.models.filter((model) =>
    data.points.some((p) => (p.values[model] ?? 0) > 0)
  )
  if (active.length === 0) return null

  const labelOf = (model: string) => (model === REPORT_COST_OTHERS_KEY ? othersLabel : model)
  const values = data.points.flatMap((p) =>
    active.map((model) => ({
      time: p.bucket_label,
      series: labelOf(model),
      value: p.values[model] ?? 0,
    }))
  )
  return buildTimeSeriesSpec({
    id: 'reportCostByModel',
    values,
    seriesOrder: active.map(labelOf),
    chartKind,
    stack: true,
    format: formatQuota,
    showLegend: true,
  })
}

export function buildLatencySeriesSpec(
  points: ReportLatencyPoint[],
  labels: { p50: string; p95: string },
  chartKind: ReportChartKind
): ReportChartSpec | null {
  // A percentile of 0 means the bucket had no sample; plotting it would draw a
  // false drop to zero.
  const values = points.flatMap((item) => [
    { time: item.bucket_label, series: labels.p50, value: item.p50_ms },
    { time: item.bucket_label, series: labels.p95, value: item.p95_ms },
  ]).filter((item) => item.value > 0)
  if (values.length === 0) return null
  return buildTimeSeriesSpec({
    id: 'reportLatencySeries',
    values,
    seriesOrder: [labels.p50, labels.p95],
    chartKind,
    stack: false,
    format: formatLatency,
    showLegend: true,
  })
}
