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
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { formatLogQuota } from '@/lib/format'

import { getLogContent } from '../../../api'
import {
  logContentSchema,
  type LogContentView,
} from '../../../data/log-content-schema'
import { usageLogSchema, type UsageLog } from '../../../data/schema'
import { downloadText } from '../../../lib/export'
import { getLogRowInteractionProps } from '../../../lib/row-interaction'
import { LogDetailsSheet } from '../../log-details-sheet'
import {
  UsageLogsProvider,
  useUsageLogsContext,
} from '../../usage-logs-provider'
import { LOG_CONTENT_PENDING_REFETCH_MS } from '../use-log-content'
import openaiFixture from './fixtures/view-openai.json'

vi.mock('../../../api', () => ({ getLogContent: vi.fn() }))
vi.mock('../../../lib/export', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../../lib/export')>()),
  downloadText: vi.fn(),
}))
const copyMock = vi.hoisted(() => vi.fn(async () => true))
vi.mock('@/lib/copy-to-clipboard', () => ({ copyToClipboard: copyMock }))

const getContent = vi.mocked(getLogContent)

function makeLog(overrides: Partial<UsageLog> = {}): UsageLog {
  return usageLogSchema.parse({
    id: 1,
    user_id: 2,
    created_at: 1788840000,
    type: 2,
    content: '',
    model_name: 'gpt-test',
    username: 'alice',
    channel: 372,
    channel_name: 'primary-channel',
    token_name: 'tok',
    group: 'vip',
    quota: 500000,
    use_time: 3,
    prompt_tokens: 111,
    completion_tokens: 222,
    ip: '10.1.2.3',
    request_id: 'req-1',
    other: JSON.stringify({
      model_ratio: 1,
      completion_ratio: 1,
      frt: 1500,
      cache_tokens: 333,
      cache_creation_tokens: 444,
    }),
    ...overrides,
  })
}

function makeView(overrides: Record<string, unknown> = {}): LogContentView {
  return logContentSchema.parse({ ...openaiFixture, ...overrides })
}

const twoAttempts = [
  { attempt: 1, channel_id: 5, channel_name: 'first-ch', status_code: 502 },
  { attempt: 2, channel_id: 6, channel_name: 'second-ch', status_code: 200 },
]

function Harness(props: {
  logs: UsageLog[]
  admin?: boolean
  resetKey?: string
}) {
  const ctx = useUsageLogsContext()
  return (
    <>
      <button type='button' onClick={() => ctx.setSensitiveVisible(false)}>
        hide-sensitive
      </button>
      {props.logs.map((log) => (
        <div
          key={log.id}
          data-testid={`row-${log.id}`}
          {...getLogRowInteractionProps(log, ctx.setSelectedLog)}
        >
          row {log.id}
        </div>
      ))}
      <input aria-label='filter' />
      <LogDetailsSheet
        isAdmin={props.admin ?? true}
        isRoot={false}
        logs={props.logs}
        resetKey={props.resetKey}
      />
    </>
  )
}

function tree(props: Parameters<typeof Harness>[0]) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return (
    <QueryClientProvider client={client}>
      <UsageLogsProvider>
        <Harness {...props} />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
}

async function openRow(id: number, props: Parameters<typeof Harness>[0]) {
  const user = userEvent.setup()
  const view = render(tree(props))
  await user.click(screen.getByTestId(`row-${id}`))
  const panel = await screen.findByRole('dialog')
  return { user, panel, within: within(panel), view }
}

beforeEach(() => {
  getContent.mockReset()
  getContent.mockResolvedValue(makeView())
  vi.mocked(downloadText).mockReset()
  copyMock.mockClear()
})
afterEach(() => vi.restoreAllMocks())

it('shows loading, then the tabs of a captured call', async () => {
  let resolve: (view: LogContentView) => void = () => {}
  getContent.mockReturnValue(new Promise((r) => (resolve = r)))
  const { within: q } = await openRow(1, { logs: [makeLog()] })
  expect(q.getByText('Loading...')).toBeVisible()
  resolve(makeView())
  expect(await q.findByRole('tab', { name: 'Conversation' })).toBeVisible()
  expect(q.getByRole('tab', { name: 'Raw JSON' })).toBeVisible()
  expect(q.queryByText('Loading...')).toBeNull()
})

it('looks up a pending call again and then shows the content', async () => {
  getContent
    .mockResolvedValueOnce(makeView({ available: false, reason: 'pending' }))
    .mockResolvedValue(makeView())
  const { within: q } = await openRow(1, { logs: [makeLog()] })
  expect(
    await q.findByText('The content of this call is still being saved.')
  ).toBeVisible()
  expect(
    await q.findByRole(
      'tab',
      { name: 'Conversation' },
      { timeout: LOG_CONTENT_PENDING_REFETCH_MS + 3000 }
    )
  ).toBeVisible()
  expect(getContent).toHaveBeenCalledTimes(2)
}, 10_000)

it.each([
  [
    'expired',
    'The content of this call was deleted after the retention period.',
  ],
  ['not_captured', 'Content not captured for this type of call'],
  ['unavailable', 'The content of this call is not available.'],
])(
  'explains the %s state and keeps Details available',
  async (reason, text) => {
    getContent.mockResolvedValue(makeView({ available: false, reason }))
    const { within: q, user } = await openRow(1, { logs: [makeLog()] })
    expect(await q.findByText(text)).toBeVisible()
    expect(q.queryByRole('tab')).toBeNull()
    await user.click(q.getByRole('button', { name: 'Details' }))
    expect(q.getByText('Billing Details')).toBeVisible()
  }
)

it('shows a network error with Retry', async () => {
  getContent.mockRejectedValueOnce(new Error('boom'))
  const { within: q, user } = await openRow(1, { logs: [makeLog()] })
  expect(
    await q.findByText('Could not load the content of this call')
  ).toBeVisible()
  await user.click(q.getByRole('button', { name: 'Retry' }))
  expect(await q.findByRole('tab', { name: 'Conversation' })).toBeVisible()
})

it('warns about truncated and incomplete content', async () => {
  getContent.mockResolvedValue(
    makeView({
      incomplete: true,
      client: {
        request: { text: '{}', truncated: true },
        response: { text: '{}' },
      },
    })
  )
  const { within: q } = await openRow(1, { logs: [makeLog()] })
  expect(
    await q.findByText('Some content was shortened to fit the storage limit.')
  ).toBeVisible()
  expect(
    q.getByText(
      'The response did not finish, so the captured content may be incomplete.'
    )
  ).toBeVisible()
})

it('shows the badges, the channel only to admins and attempts only when several', async () => {
  getContent.mockResolvedValue(
    makeView({ is_stream: true, http_status: 200, attempts: twoAttempts })
  )
  const { within: q, user } = await openRow(1, { logs: [makeLog()] })
  const header = within(await screen.findByTestId('log-content-header'))
  expect(await header.findByText('OpenAI Chat')).toBeVisible()
  expect(header.getByText('OK')).toBeVisible()
  expect(header.getByText('HTTP 200')).toBeVisible()
  expect(header.getByText('Stream')).toBeVisible()
  expect(header.getByText('#372 primary-channel')).toBeVisible()
  const first = q.getByRole('button', { name: /#1/ })
  const second = q.getByRole('button', { name: /#2/ })
  expect(second).toHaveAttribute('aria-pressed', 'true')
  await user.click(first)
  expect(first).toHaveAttribute('aria-pressed', 'true')
  expect(second).toHaveAttribute('aria-pressed', 'false')
})

it('hides channel and attempts from regular users, and a single attempt strip', async () => {
  getContent.mockResolvedValue(makeView({ attempts: [twoAttempts[0]] }))
  const { within: q } = await openRow(1, { logs: [makeLog()], admin: false })
  await q.findByRole('tab', { name: 'Conversation' })
  expect(q.queryByText('#372 primary-channel')).toBeNull()
  expect(q.queryByRole('group', { name: 'Attempts' })).toBeNull()
  expect(q.queryByRole('tab', { name: 'Routing' })).toBeNull()
})

it('shows the Routing and Raw JSON tabs to admins and Raw JSON only to users', async () => {
  getContent.mockResolvedValue(makeView({ attempts: twoAttempts }))
  const admin = await openRow(1, { logs: [makeLog()] })
  await admin.user.click(
    await admin.within.findByRole('tab', { name: 'Routing' })
  )
  expect(await screen.findByTestId('log-content-tab-routing')).toBeVisible()
  await admin.user.click(admin.within.getByRole('tab', { name: 'Raw JSON' }))
  expect(await screen.findByTestId('log-content-tab-raw')).toBeVisible()
})

it('shows only the client bodies in Raw JSON to a regular user', async () => {
  getContent.mockResolvedValue(makeView({ attempts: [] }))
  const { within: q, user } = await openRow(1, {
    logs: [makeLog()],
    admin: false,
  })
  await user.click(await q.findByRole('tab', { name: 'Raw JSON' }))
  expect(q.queryByRole('tab', { name: 'Routing' })).toBeNull()
  const headings = within(await screen.findByTestId('log-content-tab-raw'))
    .getAllByRole('heading')
    .map((h) => h.textContent)
  expect(headings).toEqual(['Client request', 'Delivered response'])
})

it('shows the five figures with cache tokens, cost and tools', async () => {
  getContent.mockResolvedValue(makeView({ tools_available: 4, tool_calls: 2 }))
  await openRow(1, { logs: [makeLog()] })
  const stats = within(await screen.findByTestId('log-content-stats'))
  expect(stats.getByText('3.0s')).toBeVisible()
  expect(stats.getByText('First token: 1.5 s')).toBeVisible()
  expect(stats.getByText('gpt-test')).toBeVisible()
  expect(stats.getByText('Group: vip')).toBeVisible()
  expect(stats.getByText('111')).toBeVisible()
  expect(stats.getByText('Output Tokens: 222')).toBeVisible()
  expect(stats.getByText('Cache Read: 333')).toBeVisible()
  expect(stats.getByText('Cache Write: 444')).toBeVisible()
  expect(stats.getByText(formatLogQuota(500000))).toBeVisible()
  await waitFor(() => expect(stats.getByText('Calls: 2')).toBeVisible())
  expect(stats.getByText('4')).toBeVisible()
})

it('shows - and never 0 when the tools offered are unknown', async () => {
  getContent.mockResolvedValue(
    makeView({ tools_available: null, tool_calls: null, tools: [] })
  )
  await openRow(1, { logs: [makeLog()] })
  const stats = within(await screen.findByTestId('log-content-stats'))
  await waitFor(() => expect(stats.getByText('Calls: -')).toBeVisible())
  const cell = stats.getByText('Tools').parentElement as HTMLElement
  expect(within(cell).getByText('-')).toBeVisible()
  expect(within(cell).queryByText('0')).toBeNull()
})

it('keeps Details closed until opened, with the current body', async () => {
  const { within: q, user } = await openRow(1, { logs: [makeLog()] })
  await q.findByRole('tab', { name: 'Conversation' })
  expect(q.queryByText('Billing Details')).toBeNull()
  await user.click(q.getByRole('button', { name: 'Details' }))
  expect(q.getByText('Billing Details')).toBeVisible()
  expect(q.getByText('Consumption')).toBeVisible()
  expect(q.getByText('Channel')).toBeVisible()
})

it('walks through the logs with the buttons and the arrow keys', async () => {
  const logs = [
    makeLog({ id: 1, request_id: 'a' }),
    makeLog({ id: 2, request_id: 'b' }),
    makeLog({ id: 3, request_id: 'c' }),
  ]
  const { within: q, user } = await openRow(1, { logs })
  const prev = q.getByRole('button', { name: 'Previous log' })
  const next = q.getByRole('button', { name: 'Next log' })
  expect(prev).toBeDisabled()
  await user.keyboard('{ArrowDown}')
  await waitFor(() => expect(q.getByText('b')).toBeVisible())
  await user.keyboard('{ArrowUp}')
  await waitFor(() => expect(q.getByText('a')).toBeVisible())
  await user.keyboard('{ArrowUp}')
  expect(q.getByText('a')).toBeVisible()
  await user.click(next)
  await user.click(next)
  await waitFor(() => expect(q.getByText('c')).toBeVisible())
  expect(next).toBeDisabled()
  await user.keyboard('{ArrowDown}')
  expect(q.getByText('c')).toBeVisible()
  await user.click(prev)
  await waitFor(() => expect(q.getByText('b')).toBeVisible())
})

it('ignores the arrows in the filter and with Ctrl, and keeps Tab and Escape', async () => {
  const logs = [
    makeLog({ id: 1, request_id: 'a' }),
    makeLog({ id: 2, request_id: 'b' }),
  ]
  const { user, panel } = await openRow(1, { logs })
  await user.click(screen.getByLabelText('filter'))
  await user.keyboard('{ArrowDown}')
  expect(within(panel).getByText('a')).toBeVisible()
  fireEvent.keyDown(panel, { key: 'ArrowDown', ctrlKey: true })
  expect(within(panel).getByText('a')).toBeVisible()
  fireEvent.keyDown(panel, { key: 'Tab' })
  expect(screen.getByRole('dialog')).toBeVisible()
  await user.keyboard('{Escape}')
  expect(screen.getByRole('dialog')).toBeVisible()
  await user.click(within(panel).getByRole('button', { name: 'Close' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})

it('returns focus to the row selected now when closing, and closes on a page change', async () => {
  const logs = [
    makeLog({ id: 1, request_id: 'a' }),
    makeLog({ id: 2, request_id: 'b' }),
  ]
  const { user, panel, view } = await openRow(1, { logs })
  await user.click(within(panel).getByRole('button', { name: 'Next log' }))
  await waitFor(() => expect(within(panel).getByText('b')).toBeVisible())
  await user.click(within(panel).getByRole('button', { name: 'Close' }))
  await waitFor(() => expect(screen.getByTestId('row-2')).toHaveFocus())

  await user.click(screen.getByTestId('row-1'))
  await screen.findByRole('dialog')
  view.rerender(tree({ logs, resetKey: 'page-2' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})

it('copies the request body and exports the call as JSON', async () => {
  const view = makeView({
    client: { request: { text: '{"model":"x"}' }, response: { text: '{}' } },
  })
  getContent.mockResolvedValue(view)
  const { user, within: q } = await openRow(1, { logs: [makeLog()] })
  await q.findByRole('tab', { name: 'Conversation' })
  await user.click(q.getByRole('button', { name: 'More actions' }))
  await user.click(
    await screen.findByRole('menuitem', { name: 'Copy request body' })
  )
  expect(copyMock).toHaveBeenCalledWith('{"model":"x"}')

  await user.click(q.getByRole('button', { name: 'More actions' }))
  await user.click(
    await screen.findByRole('menuitem', { name: 'Export call as JSON' })
  )
  expect(downloadText).toHaveBeenCalledOnce()
  const [name, , content] = vi.mocked(downloadText).mock.calls[0]
  expect(name).toBe('log-req-1.json')
  expect(JSON.parse(content)).toEqual(view)
})

it('disables the menu actions without content and while sensitive data is hidden', async () => {
  getContent.mockResolvedValue(
    makeView({ available: false, reason: 'expired' })
  )
  const first = await openRow(1, { logs: [makeLog()] })
  await first.within.findByText(/retention period/)
  await first.user.click(
    first.within.getByRole('button', { name: 'More actions' })
  )
  expect(
    await screen.findByRole('menuitem', { name: 'Copy request body' })
  ).toHaveAttribute('aria-disabled', 'true')
  expect(
    screen.getByRole('menuitem', { name: 'Export call as JSON' })
  ).toHaveAttribute('aria-disabled', 'true')
})

it('disables the menu actions when sensitive data is hidden', async () => {
  const { user, within: q } = await openRow(1, { logs: [makeLog()] })
  await q.findByRole('tab', { name: 'Conversation' })
  await user.click(screen.getByText('hide-sensitive'))
  await user.click(q.getByRole('button', { name: 'More actions' }))
  expect(
    await screen.findByRole('menuitem', { name: 'Copy request body' })
  ).toHaveAttribute('aria-disabled', 'true')
  expect(
    screen.getByRole('menuitem', { name: 'Export call as JSON' })
  ).toHaveAttribute('aria-disabled', 'true')
})

it('warns in the content tabs and masks the channel in sensitive mode', async () => {
  getContent.mockResolvedValue(
    makeView({
      tools_available: 1,
      tools: [{ name: 'f', called: 0 }],
      attempts: twoAttempts,
    })
  )
  const { user, within: q } = await openRow(1, { logs: [makeLog()] })
  await q.findByRole('tab', { name: 'Conversation' })
  await user.click(screen.getByText('hide-sensitive'))
  for (const name of ['Conversation', 'Tools', 'Raw JSON']) {
    await user.click(q.getByRole('tab', { name }))
    expect(q.getByText('Sensitive data is hidden.')).toBeVisible()
  }
  const header = within(screen.getByTestId('log-content-header'))
  expect(header.queryByText('#372 primary-channel')).toBeNull()
  expect(header.getAllByText('••••').length).toBeGreaterThan(0)
})

it('looks up once on open, by the role-specific route, and never for a top-up row', async () => {
  const admin = await openRow(1, { logs: [makeLog()] })
  await admin.within.findByRole('tab', { name: 'Conversation' })
  expect(getContent).toHaveBeenCalledTimes(1)
  expect(getContent).toHaveBeenCalledWith('req-1', true)
  admin.view.unmount()

  getContent.mockClear()
  const self = await openRow(1, { logs: [makeLog()], admin: false })
  await self.within.findByRole('tab', { name: 'Conversation' })
  expect(getContent).toHaveBeenCalledTimes(1)
  expect(getContent).toHaveBeenCalledWith('req-1', false)
  self.view.unmount()

  getContent.mockClear()
  const topup = await openRow(1, { logs: [makeLog({ type: 1 })] })
  expect(topup.within.getByText('Request ID')).toBeVisible()
  expect(getContent).not.toHaveBeenCalled()
})

it('makes no lookup before a row is opened', () => {
  render(tree({ logs: [makeLog()] }))
  expect(getContent).not.toHaveBeenCalled()
})

it('is 60vw wide on large screens', async () => {
  await openRow(1, { logs: [makeLog()] })
  const el = document.querySelector('[data-slot="sheet-content"]')
  expect(el).toHaveClass('lg:w-[60vw]')
  expect(el?.className ?? '').not.toMatch(/lg:pr-/)
})

it('leaves the arrows to another open dialog', async () => {
  const logs = [
    makeLog({ id: 1, request_id: 'a' }),
    makeLog({ id: 2, request_id: 'b' }),
  ]
  const { panel } = await openRow(1, { logs })
  const other = document.createElement('div')
  other.setAttribute('role', 'dialog')
  const field = document.createElement('button')
  other.appendChild(field)
  document.body.appendChild(other)
  fireEvent.keyDown(field, { key: 'ArrowDown' })
  expect(within(panel).getByText('a')).toBeVisible()
  const modal = document.createElement('div')
  modal.setAttribute('aria-modal', 'true')
  const inner = document.createElement('button')
  modal.appendChild(inner)
  document.body.appendChild(modal)
  fireEvent.keyDown(inner, { key: 'ArrowDown' })
  expect(within(panel).getByText('a')).toBeVisible()
  const prevented = new KeyboardEvent('keydown', {
    key: 'ArrowDown',
    bubbles: true,
    cancelable: true,
  })
  prevented.preventDefault()
  document.body.dispatchEvent(prevented)
  expect(within(panel).getByText('a')).toBeVisible()
  other.remove()
  modal.remove()
})

it('keeps the arrows for scrolling inside a tab body but navigates from the header and the page', async () => {
  const logs = [
    makeLog({ id: 1, request_id: 'a' }),
    makeLog({ id: 2, request_id: 'b' }),
  ]
  const { panel, within: q } = await openRow(1, { logs })
  const body = await q.findByTestId('log-content-tab-conversation')
  fireEvent.keyDown(body, { key: 'ArrowDown' })
  expect(q.getByText('a')).toBeVisible()
  fireEvent.keyDown(within(panel).getByTestId('log-content-header'), {
    key: 'ArrowDown',
  })
  await waitFor(() => expect(q.getByText('b')).toBeVisible())
  fireEvent.keyDown(screen.getByTestId('row-2'), { key: 'ArrowUp' })
  await waitFor(() => expect(q.getByText('a')).toBeVisible())
})

it('offers Retry once the pending lookups run out', async () => {
  getContent.mockResolvedValue(
    makeView({ available: false, reason: 'pending' })
  )
  const { within: q, user } = await openRow(1, { logs: [makeLog()] })
  const retry = await q.findByRole(
    'button',
    { name: 'Retry' },
    { timeout: LOG_CONTENT_PENDING_REFETCH_MS * 7 }
  )
  getContent.mockResolvedValue(makeView())
  await user.click(retry)
  expect(await q.findByRole('tab', { name: 'Conversation' })).toBeVisible()
}, 20_000)

it('shows the last attempt of a newly selected log at once', async () => {
  getContent.mockResolvedValue(makeView({ attempts: twoAttempts }))
  const logs = [
    makeLog({ id: 1, request_id: 'a' }),
    makeLog({ id: 2, request_id: 'a' }),
  ]
  const { user, within: q } = await openRow(1, { logs })
  await user.click(await q.findByRole('button', { name: /#1/ }))
  expect(q.getByRole('button', { name: /#1/ })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
  await user.click(q.getByRole('button', { name: 'Next log' }))
  expect(await q.findByRole('button', { name: /#2/ })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
})
