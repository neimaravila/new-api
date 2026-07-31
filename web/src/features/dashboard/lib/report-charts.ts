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
import type { ReportTrendPoint } from '../types'
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
