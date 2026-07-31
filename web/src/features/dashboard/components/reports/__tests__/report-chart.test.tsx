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
import { Window } from 'happy-dom'
import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest'

import type { ReportChartSpec } from '../../../lib/report-charts'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
  'history',
  'location',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

beforeEach(() => {
  document.body.replaceChildren()
})

afterAll(() => {
  domWindow.close()
})

interface TrendDatum {
  value?: unknown
}

interface TrendDataBlock {
  values?: TrendDatum[]
}

// Stub out @visactor/react-vchart so no canvas is required. The stub mimics the
// real VChart component's behavior of capturing its spec only on mount (it does
// not resync to a later `spec` prop without being remounted) so the test can
// prove `ReportChart` forces a remount when the spec content changes.
vi.mock('@visactor/react-vchart', async () => {
  const react = await import('react')

  return {
    VChart: (props: { spec: ReportChartSpec }): React.JSX.Element => {
      const [committedSpec] = react.useState(() => props.spec)
      const dataBlock = committedSpec.data as TrendDataBlock[] | undefined
      const values = dataBlock?.[0]?.values ?? []
      const text = values.map((item) => String(item.value)).join(',')
      return react.createElement('div', { 'data-testid': 'chart-spec' }, text)
    },
  }
})

const { render, waitFor } = await import('@testing-library/react')
const { ReportChart } = await import('../report-chart')
const { buildTrendSpec } = await import('../../../lib/report-charts')

describe('ReportChart', () => {
  it('shows the new metric data after the spec changes for the same chart type', async () => {
    const points = [
      {
        bucket_label: 't1',
        bucket_timestamp: 1,
        quota: 10,
        requests: 100,
        tokens: 5,
        failures: 1,
        avg_latency_ms: 120,
        consuming_requests: 90,
      },
      {
        bucket_label: 't2',
        bucket_timestamp: 2,
        quota: 20,
        requests: 200,
        tokens: 6,
        failures: 2,
        avg_latency_ms: 130,
        consuming_requests: 95,
      },
    ]
    const costSpec = buildTrendSpec(points, 'quota', 'Cost')
    const requestsSpec = buildTrendSpec(points, 'requests', 'Requests')

    const { getByTestId, rerender } = render(<ReportChart spec={costSpec} ariaLabel='Trend' />)

    await waitFor(() => {
      expect(getByTestId('chart-spec').textContent).toBe('10,20')
    })

    rerender(<ReportChart spec={requestsSpec} ariaLabel='Trend' />)

    await waitFor(() => {
      expect(getByTestId('chart-spec').textContent).toBe('100,200')
    })
  })
})
