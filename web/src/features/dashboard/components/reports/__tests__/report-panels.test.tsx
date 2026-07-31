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

import { StubVChart } from './vchart-stub'

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

// Stub out @visactor/react-vchart so the canvas-backed chart never mounts under
// happy-dom once `ReportChart`'s async theme import resolves. Only that
// third-party boundary is mocked: the panels and `ReportChart` itself stay real.
vi.mock('@visactor/react-vchart', () => ({ VChart: StubVChart }))

const { render, within } = await import('@testing-library/react')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { formatQuota } = await import('@/lib/format')
const { ReportModelPanel } = await import('../report-model-panel')
const { ReportErrorsPanel } = await import('../report-errors-panel')
const { ReportChannelPanel } = await import('../report-channel-panel')
const { ReportTokenAnatomyPanel } = await import('../report-token-anatomy-panel')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

function renderNode(node: React.ReactNode) {
  const utils = render(<I18nextProvider i18n={i18n}>{node}</I18nextProvider>)
  return utils
}

describe('report sub-panels', () => {
  it('renders one table row per model with its cost', async () => {
    const { queryByText, getAllByRole, findByTestId } = renderNode(
      <ReportModelPanel
        loading={false}
        models={[
          { model_name: 'gpt-4o', quota: 350, requests: 3, tokens: 200, failures: 1, error_rate: 0.25, avg_latency_ms: 600 },
          { model_name: 'claude', quota: 100, requests: 1, tokens: 350, failures: 0, error_rate: 0, avg_latency_ms: 3000 },
        ]}
      />
    )

    // The panel renders a chart next to the table; waiting for it also lets the
    // chart's async theme setup settle before the test ends.
    expect(await findByTestId('chart-spec')).not.toBeNull()
    expect(getAllByRole('row')).toHaveLength(3)
    expect(queryByText('gpt-4o')).not.toBeNull()
    expect(queryByText(formatQuota(350))).not.toBeNull()
  })

  it('renders an empty state when there are no models', () => {
    const { queryByText } = renderNode(<ReportModelPanel loading={false} models={[]} />)
    expect(queryByText('No model usage in this period.')).not.toBeNull()
  })

  it('renders one table row per model with its failures and share', async () => {
    const { getAllByRole, findByTestId } = renderNode(
      <ReportErrorsPanel
        loading={false}
        errors={[{ model_name: 'gpt-4o', failures: 4, quota: 0, share: 100 }]}
      />
    )

    expect(await findByTestId('chart-spec')).not.toBeNull()
    const rows = getAllByRole('row')
    expect(rows).toHaveLength(2)
    // Scoped to the data row: the chart stub also prints the failure count, so a
    // document-wide text query for '4' would be ambiguous.
    const cells = within(rows[1]).getAllByRole('cell')
    expect(cells.map((cell) => cell.textContent)).toEqual(['gpt-4o', '4', '100%'])
  })

  it('channel panel (admin) lists channels with cost and error rate', () => {
    const { queryByText, getAllByRole } = renderNode(
      <ReportChannelPanel
        loading={false}
        channels={[
          { channel_id: 10, channel_name: 'OpenAI', quota: 500, requests: 9, failures: 1, error_rate: 0.1, avg_latency_ms: 700 },
        ]}
      />
    )
    expect(queryByText('OpenAI')).not.toBeNull()
    expect(queryByText(formatQuota(500))).not.toBeNull()
    expect(getAllByRole('row')).toHaveLength(2)
  })

  it('token anatomy panel lists prompt, completion and cache columns', () => {
    const { queryByText } = renderNode(
      <ReportTokenAnatomyPanel
        loading={false}
        rows={[{ model_name: 'claude', prompt_tokens: 100, completion_tokens: 200, cache_tokens: 50, total: 350 }]}
      />
    )
    expect(queryByText('claude')).not.toBeNull()
    expect(queryByText('100')).not.toBeNull()
    expect(queryByText('200')).not.toBeNull()
    expect(queryByText('50')).not.toBeNull()
  })
})
