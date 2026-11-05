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
import { afterEach, expect, it } from 'vitest'

import { buildVolumeSpec, type ReportChartSpec } from '../report-charts'
import { readDestructiveColor, withFailureColor } from '../failure-color'

afterEach(() => {
  document.documentElement.style.removeProperty('--destructive')
})

const labels = { success: 'Successful', failures: 'Failures' }

it('returns undefined without a destructive token', () => {
  expect(readDestructiveColor()).toBeUndefined()
})

it('paints the failures series with the destructive color and pins the domain', () => {
  document.documentElement.style.setProperty('--destructive', '#ff0000')
  const base = buildVolumeSpec(
    [
      {
        bucket_label: 'a',
        bucket_timestamp: 1,
        quota: 0,
        requests: 3,
        tokens: 0,
        failures: 1,
        avg_latency_ms: 0,
        consuming_requests: 2,
      },
    ],
    labels,
    'bar'
  )
  const spec = withFailureColor(base as ReportChartSpec, labels) as {
    color: { domain: string[]; range: string[] }
  }
  expect(spec.color.domain).toEqual(['Successful', 'Failures'])
  expect(spec.color.range[1]).toBe('#ff0000')
  expect(spec.color.range[0]).not.toBe('#ff0000')
})
