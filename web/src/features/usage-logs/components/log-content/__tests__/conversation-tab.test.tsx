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

import {
  logContentSchema,
  type LogContentView,
} from '../../../data/log-content-schema'
import { ConversationTab } from '../conversation-tab'
import claudeFixture from './fixtures/view-claude.json'
import geminiFixture from './fixtures/view-gemini.json'
import openaiFixture from './fixtures/view-openai.json'
import responsesFixture from './fixtures/view-responses.json'

const copyMock = vi.hoisted(() => vi.fn(async () => true))
vi.mock('@/lib/copy-to-clipboard', () => ({ copyToClipboard: copyMock }))

function makeView(
  base: unknown = openaiFixture,
  overrides: Record<string, unknown> = {}
): LogContentView {
  return logContentSchema.parse({ ...(base as object), ...overrides })
}

function conversation(
  messages: unknown[],
  extra: Record<string, unknown> = {}
) {
  return {
    available: true,
    response_available: true,
    messages,
    ...extra,
  }
}

function userMessages(n: number, source = 'request') {
  return Array.from({ length: n }, (_, i) => ({
    role: i % 2 ? 'assistant' : 'user',
    source,
    blocks: [{ type: 'text', text: `message ${i}` }],
  }))
}

beforeEach(() => copyMock.mockClear())

it.each([
  ['openai', openaiFixture],
  ['responses', responsesFixture],
  ['claude', claudeFixture],
  ['gemini', geminiFixture],
])('renders every part of the %s fixture', (_name, fixture) => {
  render(<ConversationTab view={makeView(fixture)} />)
  const messages = screen.getAllByTestId('log-content-message')
  expect(messages.map((m) => m.dataset.role)).toEqual([
    'system',
    'user',
    'assistant',
    'tool',
    'assistant',
  ])
  expect(screen.getByText('You are a helpful assistant.')).toBeVisible()
  expect(screen.getByText('get_weather')).toBeVisible()
  expect(screen.getByText('"Lisbon"')).toBeVisible()
  expect(screen.getByText(/Tool result/)).toBeVisible()
  expect(screen.getByText('Image · image/png · 2 KB')).toBeVisible()
  expect(screen.getByText(/A cat on a sofa/)).toBeVisible()
  expect(document.body.textContent).not.toMatch(/base64/)
})

it('filters by role and clears the filter', async () => {
  const user = userEvent.setup()
  render(<ConversationTab view={makeView()} />)
  const group = screen.getByRole('group', { name: 'Filter by role' })
  await user.click(within(group).getByRole('button', { name: 'tool' }))
  expect(screen.getAllByTestId('log-content-message')).toHaveLength(1)
  await user.click(screen.getByRole('button', { name: 'Clear filter' }))
  expect(screen.getAllByTestId('log-content-message')).toHaveLength(5)
})

it('does not break on an empty list', () => {
  render(
    <ConversationTab
      view={makeView(openaiFixture, { conversation: conversation([]) })}
    />
  )
  expect(screen.getByText('No messages to show')).toBeVisible()
})

it('folds request history beyond the 6 latest and never the response', async () => {
  const user = userEvent.setup()
  const messages = [...userMessages(12), ...userMessages(1, 'response')]
  render(
    <ConversationTab
      view={makeView(openaiFixture, { conversation: conversation(messages) })}
    />
  )
  expect(screen.getAllByTestId('log-content-message')).toHaveLength(7)
  expect(screen.queryByText('message 5')).toBeNull()
  expect(screen.getByText('message 6')).toBeVisible()
  await user.click(
    screen.getByRole('button', { name: 'Show 6 earlier messages' })
  )
  expect(screen.getAllByTestId('log-content-message')).toHaveLength(13)
  expect(screen.queryByRole('button', { name: /earlier messages/ })).toBeNull()
})

it('folds nothing with 10 messages or fewer', () => {
  render(
    <ConversationTab
      view={makeView(openaiFixture, {
        conversation: conversation(userMessages(6)),
      })}
    />
  )
  expect(screen.getAllByTestId('log-content-message')).toHaveLength(6)
  expect(screen.queryByRole('button', { name: /earlier messages/ })).toBeNull()
})

it('keeps a huge history bounded to the rows on screen', async () => {
  const user = userEvent.setup()
  render(
    <ConversationTab
      view={makeView(openaiFixture, {
        conversation: conversation(userMessages(200)),
      })}
    />
  )
  expect(screen.getAllByTestId('log-content-message')).toHaveLength(6)
  await user.click(
    screen.getByRole('button', { name: 'Show 50 earlier messages' })
  )
  expect(screen.getAllByTestId('log-content-message')).toHaveLength(56)
})

it('collapses long text, shows more, and copies the whole text', async () => {
  const user = userEvent.setup()
  const long = `${'a'.repeat(4000)}TAIL`
  render(
    <ConversationTab
      view={makeView(openaiFixture, {
        conversation: conversation([
          {
            role: 'user',
            source: 'request',
            blocks: [{ type: 'text', text: long }],
          },
        ]),
      })}
    />
  )
  expect(document.body.textContent).not.toContain('TAIL')
  await user.click(screen.getByRole('button', { name: 'Copy' }))
  expect(copyMock).toHaveBeenCalledWith(long)
  await user.click(screen.getByRole('button', { name: 'Show more' }))
  expect(document.body.textContent).toContain('TAIL')
  expect(screen.getByRole('button', { name: 'Show less' })).toBeVisible()
})

it('highlights error blocks and error tool results with the theme token', () => {
  render(
    <ConversationTab
      view={makeView(openaiFixture, {
        conversation: conversation([
          {
            role: 'assistant',
            source: 'response',
            blocks: [{ type: 'error', text: 'upstream failed' }],
          },
          {
            role: 'tool',
            source: 'request',
            blocks: [
              { type: 'tool_result', text: 'boom', is_error: true },
              { type: 'tool_result', text: 'fine' },
            ],
          },
        ]),
      })}
    />
  )
  expect(
    screen.getByText('upstream failed').closest('.text-destructive')
  ).not.toBeNull()
  expect(screen.getByText('boom').closest('.text-destructive')).not.toBeNull()
  expect(screen.getByText('fine').closest('.text-destructive')).toBeNull()
})

it('renders text literally, with no HTML or Markdown', () => {
  const { container } = render(
    <ConversationTab
      view={makeView(openaiFixture, {
        conversation: conversation([
          {
            role: 'user',
            source: 'request',
            blocks: [
              { type: 'text', text: '<script>alert(1)</script> **negrito**' },
            ],
          },
        ]),
      })}
    />
  )
  expect(
    screen.getByText('<script>alert(1)</script> **negrito**')
  ).toBeVisible()
  expect(container.querySelector('script')).toBeNull()
  expect(container.querySelector('strong')).toBeNull()
})

it('explains a call with no conversation', () => {
  render(
    <ConversationTab
      view={makeView(openaiFixture, {
        conversation: conversation([], { available: false }),
      })}
    />
  )
  expect(
    screen.getByText('Only raw JSON is available for this call')
  ).toBeVisible()
})

it('warns when the response could not be read, except for embeddings', () => {
  const messages = userMessages(2)
  const { unmount } = render(
    <ConversationTab
      view={makeView(openaiFixture, {
        conversation: conversation(messages, { response_available: false }),
      })}
    />
  )
  expect(screen.getAllByTestId('log-content-message')).toHaveLength(2)
  expect(
    screen.getByText('The response of this call could not be read.')
  ).toBeVisible()
  unmount()
  render(
    <ConversationTab
      view={makeView(openaiFixture, {
        format: 'embedding',
        conversation: conversation(messages, { response_available: false }),
      })}
    />
  )
  expect(
    screen.queryByText('The response of this call could not be read.')
  ).toBeNull()
})

it('formats sizes in the interface language', async () => {
  const { default: i18n } = await import('i18next')
  const before = i18n.language
  await i18n.changeLanguage('pt-BR')
  try {
    const view = makeView(openaiFixture, {
      conversation: conversation([
        {
          role: 'user',
          source: 'request',
          blocks: [
            { type: 'media', media_type: 'image/png', size_bytes: 2560 },
          ],
        },
      ]),
    })
    render(<ConversationTab view={view} />)
    expect(screen.getByText(/2,5 KB/)).toBeVisible()
  } finally {
    await i18n.changeLanguage(before)
  }
})

it('does not split a surrogate pair when cutting long text', () => {
  const text = `${'a'.repeat(3999)}😀${'b'.repeat(10)}`
  render(
    <ConversationTab
      view={makeView(openaiFixture, {
        conversation: conversation([
          { role: 'user', source: 'request', blocks: [{ type: 'text', text }] },
        ]),
      })}
    />
  )
  expect(document.body.textContent).not.toContain('😀')
  expect(document.body.textContent).not.toMatch(
    /[\ud800-\udbff](?![\udc00-\udfff])/
  )
})

it('shows huge or deeply nested arguments raw without failing', () => {
  const huge = JSON.stringify({ v: 'x'.repeat(300 * 1024) })
  const deep = '['.repeat(100000) + ']'.repeat(100000)
  render(
    <ConversationTab
      view={makeView(openaiFixture, {
        conversation: conversation([
          {
            role: 'assistant',
            source: 'request',
            blocks: [
              { type: 'tool_call', name: 'big', arguments: huge },
              { type: 'tool_call', name: 'deep', arguments: deep },
            ],
          },
        ]),
      })}
    />
  )
  expect(screen.getByText('big')).toBeVisible()
  expect(screen.getByText('deep')).toBeVisible()
})

function single(blocks: unknown[], role = 'user') {
  return makeView(openaiFixture, {
    conversation: conversation([{ role, source: 'request', blocks }]),
  })
}

it('renders user and assistant text in the prose font', () => {
  render(<ConversationTab view={makeView()} />)
  const el = screen.getByText(
    'What is in this picture, and what is the weather in Lisbon?'
  )
  expect(el).toHaveAttribute('data-text-style', 'prose')
  expect(el).toHaveClass('font-sans')
  expect(el).not.toHaveClass('font-mono')
})

it('renders a fenced code segment inside prose in mono', () => {
  render(
    <ConversationTab
      view={single([{ type: 'text', text: 'before\n```js\nx=1\n```\nafter' }])}
    />
  )
  const mono = screen.getByText(/x=1/)
  expect(mono).toHaveAttribute('data-text-style', 'mono')
  expect(screen.getByText(/before/)).toHaveAttribute('data-text-style', 'prose')
  expect(screen.getByText(/after/)).toHaveAttribute('data-text-style', 'prose')
})

it('renders tool call arguments as highlighted JSON', () => {
  render(<ConversationTab view={makeView()} />)
  const box = screen.getByTestId('log-content-tool-call')
  expect(
    box.querySelector('[data-text-style=json][data-highlighted=true]')
  ).not.toBeNull()
  expect(within(box).getByText('get_weather')).toHaveClass('font-mono')
})

it('renders a JSON tool result as JSON and a plain one in mono', () => {
  const { container } = render(
    <ConversationTab
      view={single(
        [
          { type: 'tool_result', text: '{"temp":21}' },
          { type: 'tool_result', text: 'plain result' },
        ],
        'tool'
      )}
    />
  )
  expect(container.querySelector('[data-text-style=json]')).not.toBeNull()
  expect(screen.getByText('plain result')).toHaveAttribute(
    'data-text-style',
    'mono'
  )
})

it('gives each role its own style and falls back to system', () => {
  const messages = [
    { role: 'user', source: 'request', blocks: [{ type: 'text', text: 'u' }] },
    {
      role: 'developer',
      source: 'request',
      blocks: [{ type: 'text', text: 'd' }],
    },
    { role: 'weird', source: 'request', blocks: [{ type: 'text', text: 'w' }] },
    {
      role: 'assistant',
      source: 'response',
      blocks: [{ type: 'error', text: 'e' }],
    },
  ]
  render(
    <ConversationTab
      view={makeView(openaiFixture, { conversation: conversation(messages) })}
    />
  )
  const [user, dev, weird, err] = screen.getAllByTestId('log-content-message')
  expect(user).toHaveClass('bg-(--vf-role-user-bg)')
  expect(dev).toHaveClass('bg-(--vf-role-system-bg)')
  expect(weird).toHaveClass('bg-(--vf-role-system-bg)')
  expect(err).toHaveClass('bg-(--vf-role-error-bg)')
})

it('shows the line count beside the role', () => {
  render(<ConversationTab view={single([{ type: 'text', text: 'a\nb\nc' }])} />)
  expect(screen.getByText('3 lines')).toBeVisible()
})

it('keeps HTML in message text literal', () => {
  const { container } = render(
    <ConversationTab view={single([{ type: 'text', text: '<b>x</b>' }])} />
  )
  expect(container.querySelector('b')).toBeNull()
  expect(screen.getByText('<b>x</b>')).toBeVisible()
})

it('wraps a long unbroken token inside the box', () => {
  render(
    <ConversationTab view={single([{ type: 'text', text: 'x'.repeat(500) }])} />
  )
  expect(screen.getByText('x'.repeat(500))).toHaveClass(
    '[overflow-wrap:anywhere]'
  )
})
