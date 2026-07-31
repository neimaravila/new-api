import { Window } from 'happy-dom'
import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest'

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

// Stub out the chart wrapper so a canvas-backed VChart never has to render
// under happy-dom. report-chart.test.tsx exercises the real ReportChart
// (and its own @visactor/react-vchart mock) directly; this file only needs
// to prove the table and the chart's aria-label are wired to the panel's
// spec, so a lightweight stand-in is enough.
vi.mock('../report-chart', () => ({
  ReportChart: (props: { ariaLabel: string }): React.JSX.Element => <div role='img' aria-label={props.ariaLabel} />,
}))

const { render } = await import('@testing-library/react')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { formatQuota } = await import('@/lib/format')
const { ReportModelPanel } = await import('../report-model-panel')
const { ReportErrorsPanel } = await import('../report-errors-panel')
const { ReportChannelPanel } = await import('../report-channel-panel')
const { ReportTokenAnatomyPanel } = await import('../report-token-anatomy-panel')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

function renderNode(node: React.ReactNode) {
  const utils = render(<I18nextProvider i18n={i18n}>{node}</I18nextProvider>)
  return utils
}

describe('report sub-panels', () => {
  it('renders one table row per model with its cost', () => {
    const { queryByText, getAllByRole } = renderNode(
      <ReportModelPanel
        loading={false}
        models={[
          { model_name: 'gpt-4o', quota: 350, requests: 3, tokens: 200, failures: 1, error_rate: 0.25, avg_latency_ms: 600 },
          { model_name: 'claude', quota: 100, requests: 1, tokens: 350, failures: 0, error_rate: 0, avg_latency_ms: 3000 },
        ]}
      />
    )

    expect(getAllByRole('row')).toHaveLength(3)
    expect(queryByText('gpt-4o')).not.toBeNull()
    expect(queryByText(formatQuota(350))).not.toBeNull()
  })

  it('renders an empty state when there are no models', () => {
    const { queryByText } = renderNode(<ReportModelPanel loading={false} models={[]} />)
    expect(queryByText('No model usage in this period.')).not.toBeNull()
  })

  it('renders one table row per model with its failures and share', () => {
    const { queryByText, getAllByRole } = renderNode(
      <ReportErrorsPanel
        loading={false}
        errors={[{ model_name: 'gpt-4o', failures: 4, quota: 0, share: 100 }]}
      />
    )

    expect(getAllByRole('row')).toHaveLength(2)
    expect(queryByText('gpt-4o')).not.toBeNull()
    expect(queryByText('4')).not.toBeNull()
    expect(queryByText('100%')).not.toBeNull()
  })

  it('channel panel (admin) renders channel rows', () => {
    const { queryByText } = renderNode(
      <ReportChannelPanel
        loading={false}
        channels={[
          { channel_id: 10, channel_name: 'OpenAI', quota: 500, requests: 9, failures: 1, error_rate: 0.1, avg_latency_ms: 700 },
        ]}
      />
    )
    expect(queryByText('OpenAI')).not.toBeNull()
    expect(queryByText(formatQuota(500))).not.toBeNull()
  })

  it('token anatomy panel renders prompt/completion/cache sublabels', () => {
    const { queryByText } = renderNode(
      <ReportTokenAnatomyPanel
        loading={false}
        rows={[
          { model_name: 'claude', prompt_tokens: 100, completion_tokens: 200, cache_tokens: 50, total: 350 },
        ]}
      />
    )
    expect(queryByText('claude')).not.toBeNull()
    expect(queryByText('prompt 100 · completion 200 · cache 50')).not.toBeNull()
  })
})
