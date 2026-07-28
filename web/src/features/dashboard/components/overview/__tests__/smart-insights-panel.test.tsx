import '@testing-library/jest-dom/vitest'
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
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { SmartInsightsPanel } = await import('../smart-insights-panel')
const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

function renderPanel(props: React.ComponentProps<typeof SmartInsightsPanel>) {
  return render(
    <I18nextProvider i18n={i18n}>
      <SmartInsightsPanel {...props} />
    </I18nextProvider>
  )
}

const insights: DashboardInsight[] = [
  {
    id: 'user-low-runway',
    severity: 'warning',
    title: 'Balance may run out soon',
    description:
      'Your recent usage suggests the current balance may not last three days.',
    metric_label: 'Runway',
    metric_value: '2 days',
    action: { label: 'Open Wallet', path: '/wallet' },
  },
]

describe('SmartInsightsPanel', () => {
  it('renders insight title, metric, and action link when data exists', () => {
    const view = renderPanel({ insights, loading: false })

    expect(view.getByText('Balance may run out soon')).toBeInTheDocument()
    expect(view.getByText('Runway')).toBeInTheDocument()
    expect(view.getByText('2 days')).toBeInTheDocument()
    expect(view.getByRole('link', { name: 'Open Wallet' })).toHaveAttribute(
      'href',
      '/wallet'
    )
  })

  it('renders an empty state when there are no insights', () => {
    const view = renderPanel({ insights: [], loading: false })

    expect(view.getByText('No urgent insights')).toBeInTheDocument()
  })

  it('renders a loading state while insights are loading', () => {
    const view = renderPanel({ insights: [], loading: true })

    expect(view.getByLabelText('Loading smart insights')).toBeInTheDocument()
  })
})
