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
import { getDashboardChartColors } from './charts'
import type { ReportChartSpec } from './report-charts'

/**
 * Reads the theme's destructive token for the chart. The chart canvas paints
 * the same plain colors the palette uses, so a modern CSS color such as oklch()
 * is converted to rgb through a canvas pixel; where that is unavailable the raw
 * token is returned.
 */
export function readDestructiveColor(): string | undefined {
  if (typeof document === 'undefined') return undefined
  const raw = getComputedStyle(document.documentElement)
    .getPropertyValue('--destructive')
    .trim()
  if (!raw) return undefined
  try {
    const context = document.createElement('canvas').getContext('2d')
    if (!context) return raw
    context.canvas.width = 1
    context.canvas.height = 1
    context.fillStyle = raw
    context.fillRect(0, 0, 1, 1)
    const [r, g, b, a] = context.getImageData(0, 0, 1, 1).data
    return a === 0 ? raw : `rgb(${r}, ${g}, ${b})`
  } catch {
    return raw
  }
}

/**
 * Paints the failures series of a volume chart with the theme error color, so
 * Reports and Logs agree. Success keeps the first palette color.
 */
export function withFailureColor(
  spec: ReportChartSpec,
  labels: { success: string; failures: string }
): ReportChartSpec {
  const palette = getDashboardChartColors(2)
  return {
    ...spec,
    color: {
      type: 'ordinal',
      domain: [labels.success, labels.failures],
      range: [palette[0], readDestructiveColor() ?? palette[1]],
    },
  }
}
