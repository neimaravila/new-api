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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'

import { logContentSchema } from '../../../data/log-content-schema'
import { PRETTY_MAX_CHARS } from '../content-format'
import { RawJsonTab } from '../raw-json-tab'
import openaiFixture from './fixtures/view-openai.json'

const copyMock = vi.hoisted(() => vi.fn(async () => true))
vi.mock('@/lib/copy-to-clipboard', () => ({ copyToClipboard: copyMock }))

beforeEach(() => copyMock.mockClear())

const body = (o: Record<string, unknown> = {}) => ({ text: '', ...o })
const view = (client: Record<string, unknown>, attempts: unknown[] = []) =>
  logContentSchema.parse({
    ...openaiFixture,
    client: {
      request: body(client.request as Record<string, unknown>),
      response: body(client.response as Record<string, unknown>),
    },
    attempts,
  })

it('hides provider bodies from a non-admin even if attempts are present', () => {
  render(
    <RawJsonTab
      view={view({}, [
        {
          attempt: 1,
          request: { text: '{"p":1}' },
          response: { text: '{"q":2}' },
        },
      ])}
      isAdmin={false}
      selectedAttempt={1}
    />
  )
  expect(screen.getAllByRole('heading').map((h) => h.textContent)).toEqual([
    'Client request',
    'Delivered response',
  ])
  expect(screen.queryByText(/"p"/)).toBeNull()
})

it('shows only the client side for a regular user', () => {
  render(
    <RawJsonTab
      view={view({
        request: { text: '{"a":1}' },
        response: { text: '{"b":2}' },
      })}
      isAdmin
      selectedAttempt={null}
    />
  )
  const titles = screen.getAllByRole('heading').map((h) => h.textContent)
  expect(titles).toEqual(['Client request', 'Delivered response'])
  expect(screen.queryByRole('group')).toBeNull()
})

it('formats valid JSON and leaves SSE, cut and invalid text as it came', () => {
  const sse = 'data: {"a":1}\n\ndata: [DONE]\n'
  render(
    <RawJsonTab
      view={view({ request: { text: '{"a":1}' }, response: { text: sse } }, [
        {
          attempt: 1,
          request: { text: '{"x":', truncated: true },
          response: { text: 'not json' },
        },
      ])}
      isAdmin
      selectedAttempt={1}
    />
  )
  const pres = screen
    .getAllByTestId('log-content-raw-body')
    .map((s) => s.querySelector('pre')?.textContent)
  expect(pres[0]).toBe('{\n  "a": 1\n}')
  expect(pres[1]).toBe(sse)
  expect(pres[2]).toBe('{"x":')
  expect(pres[3]).toBe('not json')
  expect(
    screen.getByText('Some content was shortened to fit the storage limit.')
  ).toBeVisible()
})

it('shows the selected attempt and a notice when its request is unavailable', () => {
  const attempts = [
    {
      attempt: 1,
      request: { text: '{"one":1}' },
      response: { text: '{"r1":1}' },
    },
    { attempt: 2, request_unavailable: true, response: { text: '{"r2":2}' } },
  ]
  const { rerender } = render(
    <RawJsonTab view={view({}, attempts)} isAdmin selectedAttempt={1} />
  )
  expect(screen.getByText(/"one": 1/)).toBeVisible()
  rerender(<RawJsonTab view={view({}, attempts)} isAdmin selectedAttempt={2} />)
  expect(screen.queryByText(/"one": 1/)).toBeNull()
  expect(screen.getByText(/"r2": 2/)).toBeVisible()
  expect(
    screen.getByText(
      'The request sent to the provider was not captured for this attempt.'
    )
  ).toBeVisible()
})

it('does not parse above 256 KiB', () => {
  const big = `{"k":"${'x'.repeat(PRETTY_MAX_CHARS)}"}`
  const spy = vi.spyOn(JSON, 'parse')
  render(
    <RawJsonTab
      view={view({ request: { text: big } })}
      isAdmin
      selectedAttempt={null}
    />
  )
  expect(spy.mock.calls.some(([arg]) => arg === big)).toBe(false)
  spy.mockRestore()
  const pre = screen
    .getAllByTestId('log-content-raw-body')[0]
    .querySelector('pre')
  expect(pre?.textContent).toBe(big)
})

it('warns about head-only bodies and shows no text for undecodable ones', () => {
  render(
    <RawJsonTab
      view={view({
        request: { text: 'head', head_only: true },
        response: { text: 'secret-bytes', undecodable: true },
      })}
      isAdmin
      selectedAttempt={null}
    />
  )
  expect(
    screen.getByText('Some content was shortened to fit the storage limit.')
  ).toBeVisible()
  expect(
    screen.getByText('This body could not be decoded, so no text is shown.')
  ).toBeVisible()
  expect(screen.queryByText('secret-bytes')).toBeNull()
  const sections = screen.getAllByTestId('log-content-raw-body')
  expect(within(sections[1]).queryByRole('button', { name: 'Copy' })).toBeNull()
})

it('copies the original text, not the formatted one', async () => {
  const user = userEvent.setup()
  render(
    <RawJsonTab
      view={view({ request: { text: '{"a":1}' } })}
      isAdmin
      selectedAttempt={null}
    />
  )
  await user.click(
    within(screen.getAllByTestId('log-content-raw-body')[0]).getByRole(
      'button',
      { name: 'Copy' }
    )
  )
  expect(copyMock).toHaveBeenCalledWith('{"a":1}')
})
