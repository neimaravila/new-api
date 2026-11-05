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
import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'

import { createAppQueryClient } from '@/lib/query-client'

import { usageLogSchema, type UsageLog } from '../../../data/schema'
import { getLogRowInteractionProps } from '../../../lib/row-interaction'
import { LogDetailsSheet } from '../../log-details-sheet'
import {
  UsageLogsProvider,
  useUsageLogsContext,
} from '../../usage-logs-provider'

const apiGet = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/api')>()),
  api: { get: apiGet },
}))
const handleServerError = vi.hoisted(() => vi.fn())
vi.mock('@/lib/handle-server-error', () => ({
  handleServerError,
  markServerErrorHandled: vi.fn(),
}))

const log: UsageLog = usageLogSchema.parse({
  id: 1,
  user_id: 2,
  created_at: 1788840000,
  type: 2,
  content: '',
  model_name: 'gpt-test',
  quota: 1,
  request_id: 'req-500',
  other: '{}',
})

function Harness() {
  const ctx = useUsageLogsContext()
  return (
    <>
      <div
        data-testid='row'
        {...getLogRowInteractionProps(log, ctx.setSelectedLog)}
      >
        row
      </div>
      <LogDetailsSheet isAdmin isRoot={false} logs={[log]} />
    </>
  )
}

it('keeps a server error of the content lookup inside the panel', async () => {
  const internalError = Object.assign(new Error('Request failed'), {
    isAxiosError: true,
    response: { status: 500, data: { success: false, message: 'boom' } },
  })
  apiGet.mockRejectedValue(internalError)
  const onInternalServerError = vi.fn()
  const client = createAppQueryClient(onInternalServerError)
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={client}>
      <UsageLogsProvider>
        <Harness />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
  await user.click(screen.getByTestId('row'))
  const panel = within(await screen.findByRole('dialog'))
  expect(
    await panel.findByText('Could not load the content of this call')
  ).toBeVisible()
  expect(panel.getByRole('button', { name: 'Retry' })).toBeVisible()
  expect(apiGet).toHaveBeenCalledWith(
    '/api/log/content/req-500',
    expect.anything()
  )
  expect(onInternalServerError).not.toHaveBeenCalled()
  expect(handleServerError).not.toHaveBeenCalled()
})
