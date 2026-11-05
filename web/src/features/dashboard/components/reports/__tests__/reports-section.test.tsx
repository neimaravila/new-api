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
import { beforeEach, describe, expect, it, vi } from 'vitest'

const getReportSummary = vi.hoisted(() => vi.fn())
const getReportRanking = vi.hoisted(() => vi.fn())
vi.mock('../../../api', () => ({
  getReportSummary,
  getReportRanking,
  getReportExportUrl: () => '/export',
}))
vi.mock('@visactor/react-vchart', () => ({ VChart: () => null }))

beforeEach(() => {
  document.body.replaceChildren()
  getReportSummary.mockReset()
  getReportRanking.mockReset()
  getReportRanking.mockResolvedValue({ success: true, data: [] })
})

const { cleanup, fireEvent, render, waitFor } = await import('@testing-library/react')
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { useAuthStore } = await import('@/stores/auth-store')
const { ReportsSection } = await import('../reports-section')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en', resources: { en: { translation: {} } } })

const summary = {
  range: '7d',
  role: 'admin',
  granularity: 'day',
  period_start: 0,
  period_end: 1,
  models: [],
  errors: [],
  model_performance: [],
  channel_performance: [],
  token_anatomy: [],
  channels: [],
  cost_by_model: { models: [], points: [] },
  latency_series: [],
  trend: {
    granularity: 'day',
    points: [],
    totals: { quota: 0, requests: 0, tokens: 0, failures: 0, consumers: 0, avg_latency_ms: 0 },
  },
}

describe('ReportsSection summary query', () => {
  it('does not reload the summary on a ranking tab when only the granularity changes', async () => {
    useAuthStore.setState((state) => ({
      auth: { ...state.auth, user: { id: 1, username: 'u', role: 1 } },
    }))
    getReportSummary.mockResolvedValue({ success: true, data: summary })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const view = render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <ReportsSection />
        </I18nextProvider>
      </QueryClientProvider>
    )
    await waitFor(() => expect(getReportSummary).toHaveBeenCalledTimes(1))
    // Wait for the filter options to be taken from the first load.
    await new Promise((r) => setTimeout(r, 30))

    fireEvent.mouseDown(view.getByRole('tab', { name: 'Models' }))
    fireEvent.click(view.getByRole('tab', { name: 'Models' }))
    await waitFor(() => expect(getReportRanking).toHaveBeenCalled())
    fireEvent.click(view.getByRole('button', { name: 'Hourly' }))
    await waitFor(() => expect(getReportRanking.mock.calls.length).toBeGreaterThan(0))
    await new Promise((r) => setTimeout(r, 50))

    expect(getReportSummary).toHaveBeenCalledTimes(1)
    cleanup()
  })
})
