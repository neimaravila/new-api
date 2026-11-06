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
import { beforeAll, expect, it } from 'vitest'

import { logContentSchema } from '../../../data/log-content-schema'
import { usageLogSchema } from '../../../data/schema'
import { RetryBanner, RoutingTab } from '../routing-tab'
import openaiFixture from './fixtures/view-openai.json'

const log = usageLogSchema.parse({
  id: 1,
  user_id: 2,
  created_at: 1788840000,
  type: 2,
  content: '',
  model_name: 'gpt-test',
  group: 'vip',
  request_id: 'req-1',
  other: JSON.stringify({
    upstream_model_name: 'gpt-upstream',
    admin_info: {
      channel_affinity: { rule_name: 'sticky-rule', selected_group: 'edge' },
    },
  }),
})

const view = logContentSchema.parse({
  ...openaiFixture,
  attempts: [
    {
      attempt: 2,
      channel_id: 6,
      channel_name: 'second-ch',
      multi_key_index: 0,
      status_code: 200,
      duration_ms: 1234,
    },
    {
      attempt: 1,
      channel_id: 5,
      channel_name: 'first-ch',
      multi_key_index: -1,
      status_code: 502,
      error_message: 'bad gateway',
      duration_ms: 80,
    },
  ],
})

it('lists the attempts in order with channel, key index, status, reason and duration', () => {
  render(<RoutingTab view={view} log={log} sensitiveVisible />)
  const rows = screen.getAllByTestId('log-content-attempt')
  expect(rows).toHaveLength(2)
  expect(within(rows[0]).getByText('#5 first-ch')).toBeVisible()
  expect(within(rows[0]).getByText('502')).toBeVisible()
  expect(within(rows[0]).getByText(/bad gateway/)).toBeVisible()
  expect(within(rows[0]).getByText(/80 ms/)).toBeVisible()
  expect(within(rows[0]).queryByText(/Key /)).toBeNull()
  expect(within(rows[1]).getByText('#6 second-ch')).toBeVisible()
  expect(within(rows[1]).getByText('Key 0')).toBeVisible()
  expect(within(rows[1]).getByText(/1,234 ms/)).toBeVisible()
})

it('shows group, affinity and mapped model', () => {
  render(<RoutingTab view={view} log={log} sensitiveVisible />)
  const facts = within(screen.getByTestId('log-content-routing-facts'))
  expect(facts.getByText('vip')).toBeVisible()
  expect(facts.getByText('sticky-rule')).toBeVisible()
  expect(facts.getByText('edge')).toBeVisible()
  expect(facts.getByText('gpt-upstream')).toBeVisible()
})

it('masks channel names and key indexes when sensitive data is hidden', () => {
  render(<RoutingTab view={view} log={log} sensitiveVisible={false} />)
  expect(screen.queryByText(/first-ch|second-ch/)).toBeNull()
  expect(screen.queryByText(/bad gateway/)).toBeNull()
  const facts = within(screen.getByTestId('log-content-routing-facts'))
  for (const hidden of ['vip', 'gpt-upstream', 'sticky-rule', 'edge']) {
    expect(facts.queryByText(hidden)).toBeNull()
  }
  expect(facts.getAllByText('••••')).toHaveLength(4)
  expect(screen.queryByText('Key 0')).toBeNull()
  expect(screen.getByText('Key ••••')).toBeVisible()
  expect(screen.getAllByText('••••')).toHaveLength(7)
})

it('lists attempts with a tinted status and the duration aligned right', () => {
  render(<RoutingTab view={view} log={log} sensitiveVisible />)
  expect(
    screen.getByText('502').closest('[data-slot=tint-badge]')
  ).toHaveAttribute('data-tone', 'danger')
  expect(
    screen.getByText('200').closest('[data-slot=tint-badge]')
  ).toHaveAttribute('data-tone', 'success')
  expect(screen.getByText('80 ms')).toHaveClass('font-mono', 'ml-auto')
})

it('shows the failure reason in muted text', () => {
  render(<RoutingTab view={view} log={log} sensitiveVisible />)
  const reason = screen.getByText(/bad gateway/).closest('p')
  expect(reason).not.toHaveClass('text-destructive')
  expect(reason).toHaveClass('text-muted-foreground')
  expect(within(reason as HTMLElement).getByText(/Reason/)).toBeVisible()
})

beforeAll(async () => {
  const { default: i18n } = await import('i18next')
  const { default: en } = await import('@/i18n/locales/en.json')
  i18n.addResourceBundle('en', 'translation', en.translation, true, true)
})

const attemptsOf = (codes: number[]) =>
  codes.map((status_code, i) => ({
    attempt: i + 1,
    channel_id: i + 1,
    multi_key_index: -1,
    status_code,
    duration_ms: 10,
  }))
const withCodes = (codes: number[]) =>
  logContentSchema.parse({ ...openaiFixture, attempts: attemptsOf(codes) })
    .attempts

it('summarizes a retry that ended well', () => {
  render(<RetryBanner attempts={withCodes([500, 200])} />)
  const text = screen.getByText('Final attempt succeeded after 1 failure')
  expect(text.closest('[data-slot=tint-badge]')).toHaveAttribute(
    'data-tone',
    'success'
  )
})

it('summarizes a call where every attempt failed', () => {
  render(<RetryBanner attempts={withCodes([500, 502])} />)
  const text = screen.getByText('All 2 attempts failed')
  expect(text.closest('[data-slot=tint-badge]')).toHaveAttribute(
    'data-tone',
    'danger'
  )
})

it('renders no banner for a single attempt', () => {
  const { container } = render(<RetryBanner attempts={withCodes([200])} />)
  expect(container).toBeEmptyDOMElement()
})

it('keeps routing details masked when sensitive data is hidden', () => {
  render(<RoutingTab view={view} log={log} sensitiveVisible={false} />)
  expect(screen.queryByText(/bad gateway|first-ch/)).toBeNull()
  expect(screen.getAllByText('••••').length).toBeGreaterThanOrEqual(7)
})
