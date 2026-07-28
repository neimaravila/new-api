import { Window } from 'happy-dom'
import { afterAll, beforeEach, describe, expect, it } from 'vitest'

import type { DashboardInsight } from '../../../types'

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

const { render } = await import('@testing-library/react')
const { createMemoryHistory, createRootRoute, createRouter, RouterProvider } =
  await import('@tanstack/react-router')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { SmartInsightsPanel } = await import('../smart-insights-panel')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

function renderPanel(props: React.ComponentProps<typeof SmartInsightsPanel>) {
  const routeTree = createRootRoute({
    component: () => (
      <I18nextProvider i18n={i18n}>
        <SmartInsightsPanel
          insights={props.insights}
          loading={props.loading}
          error={props.error}
        />
      </I18nextProvider>
    ),
  })
  const router = createRouter({
    history: createMemoryHistory({ initialEntries: ['/'] }),
    routeTree,
  })

  return render(<RouterProvider router={router} />)
}

const insights: DashboardInsight[] = [
  {
    id: 'user-low-runway',
    severity: 'warning',
    title: { key: 'Balance may run out soon' },
    description: {
      key: 'Your recent usage suggests the current balance may not last three days.',
    },
    metric_label: { key: 'Runway' },
    metric_value_message: { key: '{{days}} days', values: { days: 2 } },
    action: { label: { key: 'Open Wallet' }, path: '/wallet' },
  },
]

describe('SmartInsightsPanel', () => {
  it('renders insight title, metric, and action link when data exists', async () => {
    const view = renderPanel({ insights, loading: false })

    expect(await view.findByText('Balance may run out soon')).toBeTruthy()
    expect(view.getByText('Runway')).toBeTruthy()
    expect(view.getByText('2 days')).toBeTruthy()
    expect(
      view.getByRole('link', { name: 'Open Wallet' }).getAttribute('href')
    ).toBe('/wallet')
  })

  it('renders an empty state when there are no insights', async () => {
    const view = renderPanel({ insights: [], loading: false })

    expect(await view.findByText('No urgent insights')).toBeTruthy()
  })

  it('renders a loading state while insights are loading', async () => {
    const view = renderPanel({ insights: [], loading: true })

    expect(await view.findByLabelText('Loading smart insights')).toBeTruthy()
  })
})
