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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'

import { logContentSchema } from '../../../data/log-content-schema'
import { ToolsTab } from '../tools-tab'
import openaiFixture from './fixtures/view-openai.json'

const copyMock = vi.hoisted(() => vi.fn(async () => true))
vi.mock('@/lib/copy-to-clipboard', () => ({ copyToClipboard: copyMock }))

const view = (overrides: Record<string, unknown> = {}) =>
  logContentSchema.parse({ ...openaiFixture, ...overrides })

beforeEach(() => copyMock.mockClear())

it('lists name, description, schema and call count, and copies the schema', async () => {
  const user = userEvent.setup()
  render(
    <ToolsTab
      view={view({
        tools: [
          {
            name: 'get_weather',
            description: 'Get the weather of a city',
            schema: {
              type: 'object',
              properties: { city: { type: 'string' } },
            },
            called: 3,
          },
        ],
      })}
    />
  )
  expect(screen.getByText('get_weather')).toBeVisible()
  expect(screen.getByText('Get the weather of a city')).toBeVisible()
  expect(screen.getByText('Called 3 times')).toBeVisible()
  expect(screen.getByText(/"city": \{/)).toBeVisible()
  await user.click(screen.getByRole('button', { name: 'Copy' }))
  expect(copyMock).toHaveBeenCalledWith(
    JSON.stringify(
      { type: 'object', properties: { city: { type: 'string' } } },
      null,
      2
    )
  )
})

it('shows an empty state for an empty list', () => {
  render(<ToolsTab view={view({ tools: [], tools_available: 0 })} />)
  expect(screen.getByText('This call declared no tools')).toBeVisible()
})

it('points to the raw JSON when the tools are unknown', () => {
  render(<ToolsTab view={view({ tools: [], tools_available: null })} />)
  expect(
    screen.getByText('Only raw JSON is available for this call')
  ).toBeVisible()
})

it('copes with a huge and a deeply nested schema and a long description', () => {
  let deep: unknown = 'leaf'
  for (let i = 0; i < 50000; i++) deep = [deep]
  render(
    <ToolsTab
      view={view({
        tools: [
          {
            name: 'huge',
            description: 'd'.repeat(5000),
            schema: { v: 'x'.repeat(300 * 1024) },
          },
          { name: 'deep', schema: deep },
        ],
      })}
    />
  )
  expect(screen.getByText('huge')).toBeVisible()
  expect(screen.getByText('deep')).toBeVisible()
  expect(screen.getAllByRole('button', { name: 'Show more' })[0]).toBeVisible()
})

it('says so when a schema is too deep to display', () => {
  let deep: unknown = 'leaf'
  for (let i = 0; i < 50000; i++) deep = [deep]
  render(<ToolsTab view={view({ tools: [{ name: 'deep', schema: deep }] })} />)
  expect(screen.getByText('Schema too deep to display')).toBeVisible()
  expect(screen.queryByText('Schema')).toBeNull()
})

it('formats the call count in the interface language', async () => {
  const { default: i18n } = await import('i18next')
  const before = i18n.language
  await i18n.changeLanguage('pt-BR')
  try {
    render(<ToolsTab view={view({ tools: [{ name: 'f', called: 1234.5 }] })} />)
    expect(screen.getByText('Called 1.234,5 times')).toBeVisible()
  } finally {
    await i18n.changeLanguage(before)
  }
})
