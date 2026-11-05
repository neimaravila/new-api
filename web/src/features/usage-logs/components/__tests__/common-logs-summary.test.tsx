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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { StubVChart } from '@/features/dashboard/components/reports/__tests__/vchart-stub'
import * as api from '../../api'
import { CommonLogsSummary } from '../common-logs-summary'
import { UsageLogsProvider, useUsageLogsContext } from '../usage-logs-provider'

vi.mock('@visactor/react-vchart', () => ({
  VChart: (props: { spec: Record<string, unknown> }) => (
    <StubVChart spec={props.spec} />
  ),
}))

vi.mock('../../api', () => ({ getLogSummary: vi.fn() }))

const summary = {
  requests: 1200,
  successes: 1100,
  failures: 100,
  tokens: 4500000,
  quota: 0,
  avg_use_time_seconds: 2.5,
  bucket_seconds: 3600,
  buckets: [{ timestamp: 3600, requests: 5, failures: 1 }],
  error_log_enabled: true,
}

beforeEach(() => {
  useAuthStore.setState((state) => ({
    auth: {
      ...state.auth,
      user: { id: 7, username: 'alice', role: ROLE.USER },
      accessToken: 'test-token',
      accessExpiresAt: Math.floor(Date.now() / 1000) + 3600,
    },
  }))
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
  useAuthStore.getState().auth.reset()
})

function HideToggle() {
  const { setSensitiveVisible } = useUsageLogsContext()
  return <button type='button' onClick={() => setSensitiveVisible(false)}>hide</button>
}

function renderSummary(search = 'startTime=1000000&endTime=1010000&type=%5B2%5D') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const root = createRootRoute()
  const auth = createRoute({ getParentRoute: () => root, id: '_authenticated' })
  const logs = createRoute({
    getParentRoute: () => auth,
    path: '/usage-logs/$section',
    component: () => (
      <UsageLogsProvider>
        <HideToggle />
        <CommonLogsSummary />
      </UsageLogsProvider>
    ),
    validateSearch: (s: Record<string, unknown>) => s,
  })
  const router = createRouter({
    routeTree: root.addChildren([auth.addChildren([logs])]),
    history: createMemoryHistory({
      initialEntries: [`/usage-logs/common?${search}`],
    }),
  })
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

it('shows requests, success rate, tokens, cost and latency', async () => {
  vi.mocked(api.getLogSummary).mockResolvedValue({
    success: true,
    data: { ...summary, quota: 500000 },
  })
  renderSummary()

  expect(await screen.findByText('1,200')).toBeVisible()
  expect(screen.getByText('91.67%')).toBeVisible()
  expect(screen.getByText('Success rate')).toBeVisible()
  expect(screen.getByText('Tokens')).toBeVisible()
  expect(screen.getByText('Cost')).toBeVisible()
  expect(screen.getByText('2.5s')).toBeVisible()
})

it('requests the summary with the table filters and no paging', async () => {
  vi.mocked(api.getLogSummary).mockResolvedValue({ success: true, data: summary })
  renderSummary()

  await screen.findByText('1,200')
  const [params, isAdmin] = vi.mocked(api.getLogSummary).mock.calls[0]
  expect(isAdmin).toBe(false)
  expect(params).toMatchObject({
    type: 2,
    start_timestamp: 1000,
    end_timestamp: 1010,
  })
  expect(params).not.toHaveProperty('p')
  expect(params).not.toHaveProperty('page_size')
})

it('shows a dash instead of NaN when there are no requests', async () => {
  vi.mocked(api.getLogSummary).mockResolvedValue({
    success: true,
    data: { ...summary, requests: 0, successes: 0, failures: 0, buckets: [] },
  })
  renderSummary()

  expect(await screen.findByText('Success rate')).toBeVisible()
  expect(screen.queryByText(/NaN/)).toBeNull()
  expect(screen.getByTestId('success-rate')).toHaveTextContent('—')
})

it('warns that the success rate excludes failures when error logging is off', async () => {
  vi.mocked(api.getLogSummary).mockResolvedValue({
    success: true,
    data: { ...summary, error_log_enabled: false },
  })
  renderSummary()

  expect(
    await screen.findByText(
      'Error logging is off, so the success rate does not include failures'
    )
  ).toBeVisible()
})

it('hides the cost when sensitive data is hidden', async () => {
  vi.mocked(api.getLogSummary).mockResolvedValue({
    success: true,
    data: { ...summary, quota: 500000 },
  })
  const user = userEvent.setup()
  renderSummary()

  await screen.findByText('1,200')
  await user.click(screen.getByRole('button', { name: 'hide' }))
  await waitFor(() => expect(screen.getByTestId('summary-cost')).toHaveTextContent('••••'))
})

it('fills buckets without rows so the time axis stays even', async () => {
  vi.mocked(api.getLogSummary).mockResolvedValue({
    success: true,
    data: {
      ...summary,
      bucket_seconds: 3600,
      buckets: [
        { timestamp: 3600, requests: 5, failures: 1 },
        { timestamp: 10800, requests: 3, failures: 0 },
      ],
    },
  })
  renderSummary('startTime=3600000&endTime=10810000')

  const chart = await screen.findByTestId('chart-spec')
  // success values per bucket 0, 1h, 2h then failures: 4,0,3,1,0,0
  expect(chart).toHaveTextContent('4,1,0,0,3,0')
})

it('shows a quiet notice when the summary fails', async () => {
  vi.mocked(api.getLogSummary).mockRejectedValue(new Error('boom'))
  renderSummary()

  expect(await screen.findByText('Summary unavailable')).toBeVisible()
})
