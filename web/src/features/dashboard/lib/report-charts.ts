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
import type {
  ReportChannelRow,
  ReportErrorRow,
  ReportModelRow,
  ReportPerformanceRow,
  ReportStreamComparison,
  ReportTokenAnatomyRow,
  ReportTrendPoint,
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

export function buildTrendSpec(
  points: ReportTrendPoint[],
  metric: ReportTrendMetric,
  seriesLabel: string
): ReportChartSpec | null {
  if (points.length === 0) return null

  const field = TREND_METRIC_FIELD[metric]
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
    background: { fill: 'transparent' },
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
    background: { fill: 'transparent' },
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
    background: { fill: 'transparent' },
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

  const values = rows.flatMap((item) => [
    { name: item.name, series: avgLabel, value: item.avg_latency_ms },
    { name: item.name, series: p95Label, value: item.p95_latency_ms },
  ])

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
      { orient: 'bottom', type: 'linear' },
    ],
    background: { fill: 'transparent' },
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
      { orient: 'bottom', type: 'linear' },
    ],
    background: { fill: 'transparent' },
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
    background: { fill: 'transparent' },
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
    background: { fill: 'transparent' },
    animation: true,
  }
}
