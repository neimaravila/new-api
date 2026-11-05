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

const getReportRanking = vi.hoisted(() => vi.fn())
vi.mock('../../../api', () => ({
  getReportRanking,
  getReportSummary: vi.fn(),
  getReportExportUrl: () => '/export',
}))

beforeEach(() => {
  document.body.replaceChildren()
  getReportRanking.mockReset()
})

const { fireEvent, render, waitFor } = await import('@testing-library/react')
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { formatQuota } = await import('@/lib/format')
const { ReportRankingTab } = await import('../report-ranking-tab')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en', resources: { en: { translation: {} } } })

function wrap(node: React.ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return (
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>{node}</I18nextProvider>
    </QueryClientProvider>
  )
}

const filters = { range: '7d', granularity: 'day' } as const
const row = (name: string, over: Record<string, number> = {}) => ({
  name,
  requests: 1,
  tokens: 10,
  quota: 500000,
  failures: 0,
  error_rate: 0.125,
  avg_latency_ms: 1500,
  p50_latency_ms: 0,
  p95_latency_ms: 800,
  ...over,
})

function names(container: HTMLElement): string[] {
  return [...container.querySelectorAll('tbody tr')].map(
    (tr) => tr.querySelector('td')?.textContent ?? ''
  )
}

describe('ReportRankingTab', () => {
  it('does not fetch while inactive and fetches the dimension when active', async () => {
    getReportRanking.mockResolvedValue({ success: true, data: [] })
    const view = render(wrap(<ReportRankingTab dimension='model' title='Models' enabled={false} filters={filters} />))
    await new Promise((r) => setTimeout(r, 20))
    expect(getReportRanking).not.toHaveBeenCalled()
    view.rerender(wrap(<ReportRankingTab dimension='model' title='Models' enabled filters={filters} />))
    await waitFor(() => expect(getReportRanking).toHaveBeenCalledWith('model', filters))
  })

  it('renders formatted metrics sorted by requests descending, with dashes for zero latency', async () => {
    getReportRanking.mockResolvedValue({
      success: true,
      data: [row('a', { requests: 2 }), row('b', { requests: 9 })],
    })
    const { findByText, container, getAllByText } = render(
      wrap(<ReportRankingTab dimension='model' title='Models' enabled filters={filters} />)
    )
    await findByText('b')
    expect(names(container)).toEqual(['b', 'a'])
    expect(getAllByText(formatQuota(500000)).length).toBe(2)
    expect(getAllByText('12.5%').length).toBe(2)
    expect(getAllByText('1.5 s').length).toBe(2)
    expect(getAllByText('-').length).toBe(2)
  })

  it('toggles sorting from the header and updates aria-sort', async () => {
    getReportRanking.mockResolvedValue({
      success: true,
      data: [row('a', { tokens: 5 }), row('b', { tokens: 50 })],
    })
    const { findByText, container, getByRole } = render(
      wrap(<ReportRankingTab dimension='model' title='Models' enabled filters={filters} />)
    )
    await findByText('a')
    expect(getByRole('columnheader', { name: /Requests/ }).getAttribute('aria-sort')).toBe('descending')
    fireEvent.click(getByRole('button', { name: 'Tokens' }))
    expect(names(container)).toEqual(['b', 'a'])
    expect(getByRole('columnheader', { name: /Tokens/ }).getAttribute('aria-sort')).toBe('descending')
    expect(getByRole('columnheader', { name: /Requests/ }).getAttribute('aria-sort')).toBe('none')
    fireEvent.click(getByRole('button', { name: 'Tokens' }))
    expect(names(container)).toEqual(['a', 'b'])
    expect(getByRole('columnheader', { name: /Tokens/ }).getAttribute('aria-sort')).toBe('ascending')
  })

  it('shows the empty, error and top-50 states', async () => {
    getReportRanking.mockResolvedValue({ success: true, data: [] })
    const empty = render(wrap(<ReportRankingTab dimension='user' title='Users' enabled filters={filters} />))
    await empty.findByText('No data in this period.')
    expect(empty.getByText('Only the 50 highest-volume rows are shown.')).not.toBeNull()
    empty.unmount()

    getReportRanking.mockResolvedValue({ success: false, message: 'boom' })
    const failed = render(wrap(<ReportRankingTab dimension='user' title='Users' enabled filters={filters} />))
    await failed.findByRole('alert')
  })

  it('reports the displayed CSV, in the displayed order and with formatted cost', async () => {
    getReportRanking.mockResolvedValue({
      success: true,
      data: [row('=evil', { requests: 1 }), row('ok', { requests: 5 })],
    })
    const onCsvChange = vi.fn()
    render(
      wrap(
        <ReportRankingTab dimension='model' title='Models' enabled filters={filters} onCsvChange={onCsvChange} />
      )
    )
    await waitFor(() => expect(onCsvChange.mock.calls.some(([csv]) => typeof csv === 'string')).toBe(true))
    const csv = onCsvChange.mock.calls.map(([c]) => c).filter(Boolean).pop() as string
    const lines = csv.split('\r\n')
    expect(lines[0]).toBe('Name,Requests,Tokens,Cost,Error rate,Avg latency,P50 latency,P95 latency')
    expect(lines[1].startsWith('ok,5,10,')).toBe(true)
    expect(lines[1]).toContain(formatQuota(500000).includes(',') ? `"${formatQuota(500000)}"` : formatQuota(500000))
    expect(lines[2].startsWith("'=evil,")).toBe(true)
    // Zero latency means no sample: an empty cell, not "'-".
    expect(lines[1].endsWith(',1.5 s,,800 ms')).toBe(true)
    expect(csv).not.toContain("'-")
  })
})
