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
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from '@tanstack/react-table'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'

import { DataTableRow } from '@/components/data-table'

import { usageLogSchema, type UsageLog } from '../../data/schema'
import {
  LOG_ROW_FOCUS_CLASS,
  buildSelectedLogResetKey,
  getLogRowInteractionProps,
} from '../../lib/row-interaction'
import { useCommonLogsColumns } from '../columns/common-logs-columns'
import { LOG_PANEL_WIDTH_CLASS } from '../log-content/log-content-panel'
import { LogDetailsSheet } from '../log-details-sheet'
import { UsageLogsProvider, useUsageLogsContext } from '../usage-logs-provider'

vi.mock('../../api', () => ({
  // Never settles: these tests are about the panel shell, not the content.
  getLogContent: vi.fn(() => new Promise(() => {})),
}))

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
    group: 'default',
    quota: 500000,
    use_time: 2,
    prompt_tokens: 10,
    completion_tokens: 20,
    ip: '10.1.2.3',
    request_id: 'req-one',
    other: JSON.stringify({ model_ratio: 1, completion_ratio: 1 }),
    ...overrides,
  })
}

function Harness(props: {
  logs: UsageLog[]
  admin?: boolean
  resetKey?: string
}) {
  const columns = useCommonLogsColumns(props.admin ?? true, false)
  const extra: ColumnDef<UsageLog> = {
    id: 'extra',
    cell: () => (
      <span>
        <button type='button'>inner-button</button>
        <a href='#x'>inner-link</a>
        <input aria-label='inner-input' />
        <span data-no-row-click>inner-marked</span>
      </span>
    ),
  }
  const table = useReactTable({
    data: props.logs,
    columns: [...columns, extra],
    getCoreRowModel: getCoreRowModel(),
  })
  const ctx = useUsageLogsContext()
  return (
    <>
      <button type='button' onClick={() => ctx.setSensitiveVisible(false)}>
        hide-sensitive
      </button>
      <table>
        <tbody>
          {table.getRowModel().rows.map((row) => (
            <DataTableRow
              key={row.id}
              row={row}
              data-testid={`row-${row.original.id}`}
              {...getLogRowInteractionProps(row.original, ctx.setSelectedLog)}
            />
          ))}
        </tbody>
      </table>
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

const testClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
})

function tree(props: Parameters<typeof Harness>[0]) {
  return (
    <QueryClientProvider client={testClient}>
      <UsageLogsProvider>
        <Harness {...props} />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
}

function renderHarness(props: Parameters<typeof Harness>[0]) {
  return render(tree(props))
}

async function openDetails(user: ReturnType<typeof userEvent.setup>) {
  const dialog = await screen.findByRole('dialog')
  await user.click(within(dialog).getByRole('button', { name: 'Details' }))
  return dialog
}

function before(a: HTMLElement, b: HTMLElement): boolean {
  return !!(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING)
}

afterEach(() => {
  vi.restoreAllMocks()
})

it('shows OK, Error and the type label in the status column', () => {
  renderHarness({
    logs: [
      makeLog({ id: 1, type: 2 }),
      makeLog({ id: 2, type: 5 }),
      makeLog({ id: 3, type: 6 }),
    ],
  })
  const status = (id: number) =>
    screen
      .getByTestId(`row-${id}`)
      .querySelector('[data-column-id="status"]') as HTMLElement
  const bar = (id: number) =>
    status(id).querySelector('[data-slot="status-bar"]')
  expect(bar(1)).toHaveAttribute('aria-label', 'OK')
  expect(bar(2)).toHaveAttribute('aria-label', 'Error')
  expect(bar(3)).toHaveAttribute('aria-label', 'Refund')
  expect(screen.getByTestId('row-1').querySelector('td:first-child')).toBe(
    status(1)
  )
})

it('opens the panel on row click with sections in the required order', async () => {
  const user = userEvent.setup()
  renderHarness({
    logs: [
      makeLog({ id: 1, type: 2 }),
      makeLog({
        id: 2,
        type: 5,
        request_id: 'req-err',
        content: 'upstream exploded',
      }),
    ],
  })
  await user.click(screen.getByTestId('row-1'))
  const dialog = await openDetails(user)
  const q = within(dialog)
  // The header labels it too; the Details one is the last.
  const ids = q.getAllByText('Request ID').at(-1) as HTMLElement
  const channel = q.getByText('Channel')
  const consumption = q.getByText('Consumption')
  const timing = q.getByText('Response Time')
  const billing = q.getByText('Billing Details')
  expect(q.getByText('OK')).toBeVisible()
  expect(before(q.getByText('OK'), ids)).toBe(true)
  expect(before(ids, channel)).toBe(true)
  expect(before(channel, consumption)).toBe(true)
  expect(before(consumption, timing)).toBe(true)
  expect(before(timing, billing)).toBe(true)

  // Switching rows updates the open panel instead of closing it.
  await user.click(screen.getByTestId('row-2'))
  const same = screen.getByRole('dialog')
  expect(same).toBe(dialog)
  await user.click(within(same).getByRole('button', { name: 'Details' }))
  expect(within(same).getAllByText('req-err').length).toBeGreaterThan(0)
  const error = within(same).getByText('upstream exploded')
  expect(before(within(same).getByText('Response Time'), error)).toBe(true)
  expect(within(same).getAllByText('upstream exploded')).toHaveLength(1)
})

it('opens on Enter only when the row is focused', async () => {
  const user = userEvent.setup()
  renderHarness({ logs: [makeLog()] })
  const row = screen.getByTestId('row-1')
  expect(row).toHaveAttribute('tabindex', '0')
  row.focus()
  await user.keyboard('{Enter}')
  expect(await screen.findByRole('dialog')).toBeVisible()
})

it('ignores clicks on interactive children and text selection', async () => {
  const user = userEvent.setup()
  renderHarness({ logs: [makeLog()] })
  await user.click(screen.getByText('inner-button'))
  await user.click(screen.getByText('inner-link'))
  await user.click(screen.getByLabelText('inner-input'))
  await user.click(screen.getByText('inner-marked'))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

  vi.spyOn(window, 'getSelection').mockReturnValue({
    isCollapsed: false,
    toString: () => 'abc',
  } as unknown as Selection)
  fireEvent.click(screen.getByTestId('row-1'))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

it('opens the same panel from the Details column button', async () => {
  const user = userEvent.setup()
  renderHarness({ logs: [makeLog({ content: 'hello' })] })
  await user.click(screen.getByTitle('Click to view full details'))
  expect(await screen.findByRole('dialog')).toBeVisible()
  expect(screen.getAllByRole('dialog')).toHaveLength(1)
})

it('hides channel for regular users and consumption when sensitive data is hidden', async () => {
  const user = userEvent.setup()
  renderHarness({ logs: [makeLog()], admin: false })
  await user.click(screen.getByTestId('row-1'))
  const dialog = await openDetails(user)
  expect(within(dialog).queryByText('Channel')).not.toBeInTheDocument()
  expect(within(dialog).queryByText('primary-channel')).toBeNull()
  expect(within(dialog).getByText('Consumption')).toBeVisible()
  expect(within(dialog).queryByText('••••')).not.toBeInTheDocument()

  await user.click(screen.getByText('hide-sensitive'))
  expect(
    within(screen.getByRole('dialog')).getAllByText('••••').length
  ).toBeGreaterThan(0)
})

it('closes the panel with Escape', async () => {
  const user = userEvent.setup()
  renderHarness({ logs: [makeLog()] })
  await user.click(screen.getByTestId('row-1'))
  await screen.findByRole('dialog')
  await user.keyboard('{Escape}')
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
})

const baseQuery = {
  logCategory: 'common',
  viewAccess: 'admin',
  pagination: { pageIndex: 0, pageSize: 100 },
  columnFilters: [{ id: 'model_name', value: 'a' }],
  searchParams: { model: 'a' },
}

it('builds a reset key that changes with page, filter and category only', () => {
  const key = buildSelectedLogResetKey(baseQuery)
  expect(
    buildSelectedLogResetKey({
      ...baseQuery,
      pagination: { pageIndex: 1, pageSize: 100 },
    })
  ).not.toBe(key)
  expect(
    buildSelectedLogResetKey({
      ...baseQuery,
      columnFilters: [{ id: 'model_name', value: 'b' }],
      searchParams: { model: 'b' },
    })
  ).not.toBe(key)
  expect(
    buildSelectedLogResetKey({ ...baseQuery, logCategory: 'drawing' })
  ).not.toBe(key)
  // Same values in fresh objects (a refetch) keep the key.
  expect(
    buildSelectedLogResetKey({
      ...baseQuery,
      pagination: { ...baseQuery.pagination },
      columnFilters: baseQuery.columnFilters.map((f) => ({ ...f })),
      searchParams: { ...baseQuery.searchParams },
    })
  ).toBe(key)
})

it('closes the panel when the reset key changes and keeps it on a refetch', async () => {
  const user = userEvent.setup()
  const key = buildSelectedLogResetKey(baseQuery)
  const { rerender } = renderHarness({ logs: [makeLog()], resetKey: key })
  await user.click(screen.getByTestId('row-1'))
  await screen.findByRole('dialog')

  // Refetch: new log objects, same query.
  rerender(tree({ logs: [makeLog()], resetKey: key }))
  expect(screen.getByRole('dialog')).toBeVisible()

  rerender(
    tree({
      logs: [makeLog()],
      resetKey: buildSelectedLogResetKey({
        ...baseQuery,
        pagination: { pageIndex: 1, pageSize: 100 },
      }),
    })
  )
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
})

it('moves focus into the panel on open and back to the row on close', async () => {
  const user = userEvent.setup()
  renderHarness({ logs: [makeLog()] })
  const row = screen.getByTestId('row-1')
  await user.click(row)
  const dialog = await screen.findByRole('dialog')
  await waitFor(() =>
    expect(dialog).toContainElement(document.activeElement as HTMLElement)
  )
  await user.keyboard('{Escape}')
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  await waitFor(() => expect(row).toHaveFocus())
})

it('moves focus into the panel when opened from the Details button', async () => {
  const user = userEvent.setup()
  renderHarness({ logs: [makeLog({ content: 'hello' })] })
  await user.click(screen.getByTitle('Click to view full details'))
  const dialog = await screen.findByRole('dialog')
  await waitFor(() =>
    expect(dialog).toContainElement(document.activeElement as HTMLElement)
  )
  await user.click(within(dialog).getByRole('button', { name: 'Close' }))
  await waitFor(() => expect(screen.getByTestId('row-1')).toHaveFocus())
})

it('ignores Escape typed in an unrelated input', async () => {
  const user = userEvent.setup()
  renderHarness({ logs: [makeLog()] })
  await user.click(screen.getByTestId('row-1'))
  await screen.findByRole('dialog')
  await user.click(screen.getByLabelText('filter'))
  await user.keyboard('abc{Escape}')
  expect(screen.getByRole('dialog')).toBeVisible()
})

it('opens with Space on the row itself, preventing scroll, with a focus ring', async () => {
  const user = userEvent.setup()
  renderHarness({ logs: [makeLog()] })
  const row = screen.getByTestId('row-1')
  expect(row.className).toContain('focus-visible:ring')
  expect(LOG_ROW_FOCUS_CLASS).toContain('focus-visible:ring')
  // Space on a child control is not hijacked.
  screen.getByText('inner-button').focus()
  await user.keyboard(' ')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

  row.focus()
  const event = new KeyboardEvent('keydown', {
    key: ' ',
    bubbles: true,
    cancelable: true,
  })
  row.dispatchEvent(event)
  expect(event.defaultPrevented).toBe(true)
  expect(await screen.findByRole('dialog')).toBeVisible()
})

it('is 60vw wide from lg up and reserves no room on the page', async () => {
  const user = userEvent.setup()
  renderHarness({ logs: [makeLog({ id: 1, type: 2 })] })
  await user.click(screen.getByTestId('row-1'))
  const panel = await waitFor(() => {
    const el = document.querySelector('[data-slot="sheet-content"]')
    if (!el) throw new Error('panel not open')
    return el
  })
  expect(panel).toHaveClass('lg:w-[60vw]')
  expect(LOG_PANEL_WIDTH_CLASS).toContain('lg:w-[60vw]')
})
