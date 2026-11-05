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
import { REPORT_COST_OTHERS_KEY, type ReportCostSeries, type ReportLatencyPoint } from '../types'

/** The model with the highest cost over the whole period, or null without cost. */
export function getTopCostModel(data: ReportCostSeries): { model: string; quota: number } | null {
  const sums = new Map<string, number>()
  for (const point of data.points) {
    for (const [model, quota] of Object.entries(point.values)) {
      sums.set(model, (sums.get(model) ?? 0) + quota)
    }
  }
  let top: { model: string; quota: number } | null = null
  for (const [model, quota] of sums) {
    if (quota > 0 && (top === null || quota > top.quota)) top = { model, quota }
  }
  return top
}

export function isOthersModel(model: string): boolean {
  return model === REPORT_COST_OTHERS_KEY
}

/**
 * Period latency from the per-bucket series. The API gives no percentile over
 * the whole period, and percentiles cannot be averaged, so this reports the
 * typical bucket (median of the bucket p50s) and the worst bucket (highest
 * bucket p95). Buckets with 0 had no sample and are ignored; 0 is returned
 * when no bucket has one.
 */
export function getPeriodLatency(series: ReportLatencyPoint[]): { p50: number; p95: number } {
  const p50s = series.map((p) => p.p50_ms).filter((v) => v > 0).sort((a, b) => a - b)
  const p95s = series.map((p) => p.p95_ms).filter((v) => v > 0)
  const mid = Math.floor(p50s.length / 2)
  let p50 = 0
  if (p50s.length % 2 === 1) p50 = p50s[mid]
  else if (p50s.length > 0) p50 = (p50s[mid - 1] + p50s[mid]) / 2
  return { p50, p95: p95s.length === 0 ? 0 : Math.max(...p95s) }
}
