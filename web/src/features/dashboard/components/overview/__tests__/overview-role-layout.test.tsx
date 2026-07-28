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
import { Window } from 'happy-dom'
import type { ReactNode } from 'react'
import { afterAll, beforeEach, describe, expect, it } from 'vitest'

import type { DashboardSummary } from '../../../types'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
  'history',
  'location',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

beforeEach(() => {
  document.body.replaceChildren()
})

afterAll(() => {
  domWindow.close()
})

const { render, screen } = await import('@testing-library/react')
const { createMemoryHistory, createRootRoute, createRouter, RouterProvider } =
  await import('@tanstack/react-router')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { AdminOperationsPanel } = await import('../admin-operations-panel')
const { DashboardHero } = await import('../dashboard-hero')
const { UserDeveloperPanel } = await import('../user-developer-panel')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

function renderWithProviders(children: ReactNode): ReturnType<typeof render> {
  const routeTree = createRootRoute({
    component: () => <I18nextProvider i18n={i18n}>{children}</I18nextProvider>,
  })
  const router = createRouter({
    history: createMemoryHistory({ initialEntries: ['/'] }),
    routeTree,
  })

  return render(<RouterProvider router={router} />)
}

const baseSummary: DashboardSummary = {
  role: 'admin',
  generated_at: 1,
  period_start: 1,
  period_end: 2,
  hero: {
    eyebrow: { key: 'Platform command center' },
    title: {
      key: '{{requests}} requests · {{quota}} quota used',
      values: { requests: 10, quota: 20 },
    },
    description: {
      key: '{{failures}} recent failures · {{channels}} channels need review',
      values: { failures: 1, channels: 1 },
    },
    status_label: { key: 'Needs attention' },
    status_tone: 'warning',
  },
  metrics: [],
}

describe('dashboard role panels', () => {
  it('renders admin operational links and channel data', async () => {
    renderWithProviders(
      <AdminOperationsPanel
        summary={{
          ...baseSummary,
          channels: [
            {
              id: 1,
              name: 'OpenAI',
              status: 1,
              response_time: 800,
              used_quota: 10,
            },
          ],
          top_users: [
            { id: '1', name: 'alice', quota: 10, requests: 2, tokens: 20 },
          ],
          top_models: [
            {
              id: 'gpt-4o',
              name: 'gpt-4o',
              quota: 10,
              requests: 2,
              tokens: 20,
            },
          ],
        }}
      />
    )

    expect(await screen.findByText('Channel health')).toBeTruthy()
    expect(screen.getByText('OpenAI')).toBeTruthy()
    expect(
      screen
        .getByRole('link', { name: /Manage channels/i })
        .getAttribute('href')
    ).toBe('/channels')
  })

  it('renders user developer actions and hides admin channel management', async () => {
    renderWithProviders(
      <UserDeveloperPanel summary={{ ...baseSummary, role: 'user' }} />
    )

    expect(await screen.findByText('Ready-to-run request')).toBeTruthy()
    expect(
      screen
        .getByRole('link', { name: /Review API Keys/i })
        .getAttribute('href')
    ).toBe('/keys')
    expect(screen.queryByText('Channel health')).toBeNull()
  })

  it('renders admin hero action based on role', async () => {
    renderWithProviders(<DashboardHero summary={baseSummary} />)

    expect(await screen.findByText('Platform command center')).toBeTruthy()
    expect(screen.getByText('10 requests · 20 quota used')).toBeTruthy()
    expect(
      screen.getByText('1 recent failures · 1 channels need review')
    ).toBeTruthy()
    expect(
      screen
        .getByRole('link', { name: /Review Channels/i })
        .getAttribute('href')
    ).toBe('/channels')
  })

  it('renders user hero action based on role', async () => {
    renderWithProviders(
      <DashboardHero summary={{ ...baseSummary, role: 'user' }} />
    )

    expect(await screen.findByText('Platform command center')).toBeTruthy()
    expect(
      screen
        .getByRole('link', { name: /Review API Keys/i })
        .getAttribute('href')
    ).toBe('/keys')
    expect(
      screen
        .getByRole('link', { name: /Open Playground/i })
        .getAttribute('href')
    ).toBe('/playground')
  })
})
