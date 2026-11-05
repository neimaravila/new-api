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

import { StubVChart } from './vchart-stub'

beforeEach(() => {
  document.body.replaceChildren()
  getReportSummary.mockClear()
  getReportRanking.mockReset()
  getReportRanking.mockResolvedValue({ success: true, data: [] })
})

vi.mock('@visactor/react-vchart', () => ({ VChart: StubVChart }))

const getReportSummary = vi.hoisted(() => vi.fn())
const getReportRanking = vi.hoisted(() => vi.fn())
vi.mock('../../../api', () => ({
  getReportSummary,
  getReportRanking,
  getReportExportUrl: () => '/export',
}))

const { fireEvent, render, waitFor } = await import('@testing-library/react')
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { formatQuota } = await import('@/lib/format')
const { useAuthStore } = await import('@/stores/auth-store')
const { ReportOverviewTab } = await import('../report-overview-tab')
const { ReportChartCard } = await import('../report-chart-card')
const { ReportsSection } = await import('../reports-section')

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

const point = {
  bucket_label: 'Jul 30',
  bucket_timestamp: 1,
  quota: 350,
  requests: 3,
  tokens: 200,
  failures: 1,
  avg_latency_ms: 600,
  consuming_requests: 3,
}

function makeSummary(over: Record<string, unknown> = {}) {
  return {
    role: 'admin',
    range: '7d',
    granularity: 'day',
    period_start: 0,
    period_end: 1,
    trend: {
      granularity: 'day',
      points: [point],
      totals: { quota: 350, requests: 3, tokens: 200, failures: 1, consumers: 1, avg_latency_ms: 600 },
    },
    models: [
      { model_name: 'gpt-4o', quota: 350, requests: 3, tokens: 200, failures: 1, error_rate: 0.25, avg_latency_ms: 600 },
    ],
    errors: [{ model_name: 'gpt-4o', failures: 1, quota: 0, share: 100 }],
    model_performance: [
      { name: 'gpt-4o', avg_latency_ms: 2000, p50_latency_ms: 0, p95_latency_ms: 0, requests: 3, tokens: 200, throughput: 10 },
    ],
    channel_performance: [
      { name: 'OpenAI', avg_latency_ms: 2000, p50_latency_ms: 2000, p95_latency_ms: 3000, requests: 3, tokens: 200, throughput: 10 },
    ],
    token_anatomy: [{ model_name: 'gpt-4o', prompt_tokens: 100, completion_tokens: 80, cache_tokens: 20, total: 200 }],
    channels: [
      { channel_id: 10, channel_name: 'OpenAI', quota: 500, requests: 9, failures: 1, error_rate: 0.1, avg_latency_ms: 700 },
    ],
    stream_performance: {
      stream_avg_latency_ms: 400,
      stream_requests: 7,
      non_stream_avg_latency_ms: 900,
      non_stream_requests: 3,
    },
    cost_by_model: {
      models: ['gpt-4o', '__others__'],
      points: [{ bucket_label: 'Jul 30', bucket_timestamp: 1, values: { 'gpt-4o': 300, __others__: 50 } }],
    },
    latency_series: [{ bucket_label: 'Jul 30', bucket_timestamp: 1, p50_ms: 2000, p95_ms: 4000 }],
    ...over,
  }
}

describe('ReportOverviewTab', () => {
  it('renders the four chart cards and the panels below them', async () => {
    const { findAllByTestId, queryAllByText } = render(
      wrap(<ReportOverviewTab loading={false} summary={makeSummary() as never} />)
    )
    await findAllByTestId('chart-spec')
    for (const title of ['Request volume', 'Token volume', 'Cost by model', 'Latency over time']) {
      expect(queryAllByText(title).length).toBeGreaterThan(0)
    }
    expect(queryAllByText('Failures by model').length).toBeGreaterThan(0)
    expect(queryAllByText('Token anatomy').length).toBeGreaterThan(0)
    expect(queryAllByText('Stream vs non-stream').length).toBeGreaterThan(0)
    expect(queryAllByText('Channel cost and health')).toHaveLength(0)
  })

  it('redraws the chart as a line when the selector is switched', async () => {
    const { findAllByTestId, getAllByRole } = render(
      wrap(<ReportOverviewTab loading={false} summary={makeSummary() as never} />)
    )
    const before = (await findAllByTestId('chart-spec')).map((el) => el.getAttribute('data-chart-type'))
    expect(before.filter((type) => type === 'line')).toHaveLength(1)
    fireEvent.click(getAllByRole('button', { name: 'Line' })[0])
    await waitFor(async () => {
      const types = (await findAllByTestId('chart-spec')).map((el) => el.getAttribute('data-chart-type'))
      expect(types.filter((type) => type === 'line')).toHaveLength(2)
    })
  })

  it('shows a dash instead of 0 ms for a missing percentile', async () => {
    const { findAllByTestId, getAllByRole } = render(
      wrap(<ReportOverviewTab loading={false} summary={makeSummary() as never} />)
    )
    await findAllByTestId('chart-spec')
    const cells = getAllByRole('cell').map((c) => c.textContent)
    expect(cells).toContain('2 s')
    expect(cells.some((c) => c === '0 ms' || c === '0 s')).toBe(false)
    expect(cells).toContain('-')
  })
})

describe('ReportOverviewTab figures and states', () => {
  it('shows real period figures in each chart card', async () => {
    const summary = makeSummary({
      trend: {
        granularity: 'day',
        points: [point],
        totals: { quota: 350, requests: 8, tokens: 2000, failures: 2, consumers: 1, avg_latency_ms: 600 },
      },
      cost_by_model: {
        models: ['gpt-4o', '__others__'],
        points: [{ bucket_label: 'a', bucket_timestamp: 1, values: { 'gpt-4o': 10, __others__: 90 } }],
      },
      latency_series: [
        { bucket_label: 'a', bucket_timestamp: 1, p50_ms: 1000, p95_ms: 4000 },
        { bucket_label: 'b', bucket_timestamp: 2, p50_ms: 3000, p95_ms: 6000 },
        { bucket_label: 'c', bucket_timestamp: 3, p50_ms: 0, p95_ms: 0 },
      ],
    })
    const { findAllByTestId, getAllByText } = render(
      wrap(<ReportOverviewTab loading={false} summary={summary as never} />)
    )
    await findAllByTestId('chart-spec')
    expect(getAllByText('25%').length).toBeGreaterThan(0)
    expect(getAllByText('8').length).toBeGreaterThan(0)
    expect(getAllByText('2,000').length).toBeGreaterThan(0)
    expect(getAllByText(formatQuota(350)).length).toBeGreaterThan(0)
    expect(getAllByText('Other').length).toBeGreaterThan(0)
    expect(getAllByText('2 s').length).toBeGreaterThan(0)
    expect(getAllByText('6 s').length).toBeGreaterThan(0)
  })

  it('shows dashes when there is no latency sample or cost', () => {
    const summary = makeSummary({
      cost_by_model: { models: [], points: [] },
      latency_series: [{ bucket_label: 'a', bucket_timestamp: 1, p50_ms: 0, p95_ms: 0 }],
    })
    const { queryAllByText } = render(
      wrap(<ReportOverviewTab loading={false} summary={summary as never} />)
    )
    // Empty cards hide their figures and show the empty text instead.
    expect(queryAllByText('No latency samples in this period.').length).toBeGreaterThan(0)
    expect(queryAllByText('No cost in this period.').length).toBeGreaterThan(0)
  })

  it('renders the loading state in every card before the first load', () => {
    const { getAllByText } = render(wrap(<ReportOverviewTab loading summary={undefined} />))
    expect(getAllByText('Loading…')).toHaveLength(4)
  })

  it('renders an error state in every card when the fetch failed', () => {
    const { getAllByRole } = render(
      wrap(<ReportOverviewTab loading={false} isError summary={undefined} />)
    )
    const alerts = getAllByRole('alert')
    expect(alerts).toHaveLength(4)
    expect(alerts[0].textContent).toBe('This chart could not be loaded.')
  })
})

describe('ReportChartCard', () => {
  it('switches the chart kind from the bar/line selector', () => {
    const { getByRole, getByTestId } = render(
      wrap(
        <ReportChartCard title='Requests' summary='x' loading={false} isEmpty={false} toggleable>
          {(kind) => <div data-testid='kind'>{kind}</div>}
        </ReportChartCard>
      )
    )
    expect(getByTestId('kind').textContent).toBe('bar')
    fireEvent.click(getByRole('button', { name: 'Line' }))
    expect(getByTestId('kind').textContent).toBe('line')
    expect(getByRole('button', { name: 'Line' }).getAttribute('aria-pressed')).toBe('true')
  })

  it('has no selector when not toggleable and shows the empty text when empty', () => {
    const { queryByRole, queryAllByText } = render(
      wrap(
        <ReportChartCard title='Requests' summary='x' loading={false} isEmpty emptyText='Nothing'>
          {() => <div />}
        </ReportChartCard>
      )
    )
    expect(queryByRole('button', { name: 'Line' })).toBeNull()
    expect(queryAllByText('Nothing').length).toBeGreaterThan(0)
  })
})

describe('ReportsSection tabs and filters', () => {
  function setRole(role: number) {
    useAuthStore.setState((state) => ({
      auth: { ...state.auth, user: { ...state.auth.user, role } as never },
    }))
  }

  it('admin sees every tab and the channel filter; Channels panels live only in the Channels tab', async () => {
    setRole(10)
    getReportSummary.mockResolvedValue({ success: true, data: makeSummary() })
    const { findByRole, getByRole, queryByLabelText, queryAllByText } = render(wrap(<ReportsSection />))
    await findByRole('tab', { name: 'Channels' })
    expect(getByRole('tab', { name: 'Users' })).not.toBeNull()
    await waitFor(() => expect(queryByLabelText('Channel')).not.toBeNull())
    expect(queryByLabelText('Model')).not.toBeNull()
    expect(queryAllByText('Channel cost and health')).toHaveLength(0)

    fireEvent.click(getByRole('tab', { name: 'Channels' }))
    await waitFor(() => expect(queryAllByText('Channel cost and health').length).toBeGreaterThan(0))
    expect(queryAllByText('Channel latency').length).toBeGreaterThan(0)
  })

  it('shows the error state in the cards when the report fails to load', async () => {
    setRole(1)
    getReportSummary.mockRejectedValue(new Error('boom'))
    const { findAllByRole } = render(wrap(<ReportsSection />))
    const alerts = await findAllByRole('alert')
    expect(alerts.length).toBeGreaterThanOrEqual(4)
  })

  it('keeps the chart kind and the tab content mounted while a filter refetches', async () => {
    setRole(1)
    const full = makeSummary({
      models: [
        { model_name: 'gpt-4o', quota: 1, requests: 1, tokens: 1, failures: 0, error_rate: 0, avg_latency_ms: 1 },
      ],
    })
    let release: (() => void) | undefined
    getReportSummary.mockImplementation(async (p: { modelName?: string }) => {
      if (p.modelName) await new Promise<void>((resolve) => (release = resolve))
      return { success: true, data: full }
    })
    const { findAllByTestId, getAllByRole, findByLabelText, getAllByTestId } = render(
      wrap(<ReportsSection />)
    )
    await findAllByTestId('chart-spec')
    fireEvent.click(getAllByRole('button', { name: 'Line' })[0])
    await waitFor(() =>
      expect(getAllByTestId('chart-spec').filter((el) => el.getAttribute('data-chart-type') === 'line')).toHaveLength(2)
    )
    const select = (await findByLabelText('Model')) as HTMLSelectElement
    await waitFor(() => expect(select.options.length).toBe(2))
    fireEvent.change(select, { target: { value: 'gpt-4o' } })
    await waitFor(() => expect(release).toBeDefined())
    // Refetch is pending: the previous data and the line choice stay on screen.
    expect(getAllByRole('button', { name: 'Line' })[0].getAttribute('aria-pressed')).toBe('true')
    release?.()
    await waitFor(() =>
      expect(getAllByRole('button', { name: 'Line' })[0].getAttribute('aria-pressed')).toBe('true')
    )
    expect(getAllByTestId('chart-spec').filter((el) => el.getAttribute('data-chart-type') === 'line')).toHaveLength(2)
  })

  it('regular users see neither the Channels and Users tabs nor the channel filter', async () => {
    setRole(1)
    getReportSummary.mockResolvedValue({
      success: true,
      data: makeSummary({ channels: undefined, channel_performance: undefined }),
    })
    const { findByRole, queryByRole, queryByLabelText } = render(wrap(<ReportsSection />))
    await findByRole('tab', { name: 'Overview' })
    expect(queryByRole('tab', { name: 'Channels' })).toBeNull()
    expect(queryByRole('tab', { name: 'Users' })).toBeNull()
    expect(queryByRole('tab', { name: 'Models' })).not.toBeNull()
    expect(queryByLabelText('Channel')).toBeNull()
  })

  it('keeps the model options when a model filter is applied and sends model_name', async () => {
    setRole(1)
    const full = makeSummary({
      models: [
        { model_name: 'gpt-4o', quota: 1, requests: 1, tokens: 1, failures: 0, error_rate: 0, avg_latency_ms: 1 },
        { model_name: 'claude', quota: 1, requests: 1, tokens: 1, failures: 0, error_rate: 0, avg_latency_ms: 1 },
      ],
    })
    const filtered = makeSummary({ models: [(full.models as unknown[])[0]] })
    getReportSummary.mockImplementation(async (p: { modelName?: string }) => ({
      success: true,
      data: p.modelName ? filtered : full,
    }))
    const { findByLabelText } = render(wrap(<ReportsSection />))
    const select = (await findByLabelText('Model')) as HTMLSelectElement
    await waitFor(() => expect(select.options.length).toBe(3))
    fireEvent.change(select, { target: { value: 'gpt-4o' } })
    await waitFor(() =>
      expect(getReportSummary).toHaveBeenCalledWith(expect.objectContaining({ modelName: 'gpt-4o' }))
    )
    expect((await findByLabelText('Model') as HTMLSelectElement).options.length).toBe(3)
  })
})

describe('ReportsSection ranking tabs', () => {
  it('maps tabs to dimensions and exports the ranking CSV of the active tab', async () => {
    useAuthStore.setState((state) => ({
      auth: { ...state.auth, user: { ...state.auth.user, role: 10 } as never },
    }))
    getReportSummary.mockResolvedValue({ success: true, data: makeSummary() })
    getReportRanking.mockResolvedValue({
      success: true,
      data: [
        {
          name: 'gpt-4o', requests: 3, tokens: 7, quota: 500000, failures: 0,
          error_rate: 0, avg_latency_ms: 0, p50_latency_ms: 0, p95_latency_ms: 0,
        },
      ],
    })
    const blobs: Blob[] = []
    URL.createObjectURL = ((blob: Blob) => (blobs.push(blob), 'blob:x')) as never
    URL.revokeObjectURL = (() => {}) as never
    const { findByRole, findByText, getByRole } = render(wrap(<ReportsSection />))
    expect(getReportRanking).not.toHaveBeenCalled()

    fireEvent.click(await findByRole('tab', { name: 'Models' }))
    await findByText('gpt-4o')
    expect(getReportRanking).toHaveBeenCalledWith('model', expect.objectContaining({ range: '7d' }))
    await waitFor(() => expect((getByRole('button', { name: 'Export CSV' }) as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(getByRole('button', { name: 'Export CSV' }))
    expect(blobs).toHaveLength(1)
    expect(await blobs[0].text()).toContain('"gpt-4o","3","7",')

    fireEvent.click(getByRole('tab', { name: 'Users' }))
    await waitFor(() => expect(getReportRanking).toHaveBeenCalledWith('user', expect.anything()))
  })
})
