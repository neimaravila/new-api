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
import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest'

import type { ActiveHealthTiles, HealthStripState } from '../../lib'

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

const { fireEvent, render, screen } = await import('@testing-library/react')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ChannelHealthStrip } = await import('../channel-health-strip')
const { ChannelHealthFilterChips } =
  await import('../channel-health-filter-chips')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

function renderStrip(
  props: Omit<React.ComponentProps<typeof ChannelHealthStrip>, 'onSelect'> & {
    onSelect?: React.ComponentProps<typeof ChannelHealthStrip>['onSelect']
  }
) {
  const queryClient = new QueryClient()
  const onSelect = props.onSelect ?? vi.fn()
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ChannelHealthStrip
          state={props.state}
          slowThresholdMs={props.slowThresholdMs}
          activeTiles={props.activeTiles}
          onSelect={onSelect}
        />
      </I18nextProvider>
    </QueryClientProvider>
  )
  return { ...utils, onSelect }
}

function renderChips(
  props: Omit<
    React.ComponentProps<typeof ChannelHealthFilterChips>,
    'onDismiss'
  > & {
    onDismiss?: React.ComponentProps<
      typeof ChannelHealthFilterChips
    >['onDismiss']
  }
) {
  const onDismiss = props.onDismiss ?? vi.fn()
  const utils = render(
    <I18nextProvider i18n={i18n}>
      <ChannelHealthFilterChips
        activeTiles={props.activeTiles}
        slowThresholdMs={props.slowThresholdMs}
        onDismiss={onDismiss}
      />
    </I18nextProvider>
  )
  return { ...utils, onDismiss }
}

const alertState: HealthStripState = {
  kind: 'alert',
  tiles: [
    { id: 'active', count: 12 },
    { id: 'disabled', count: 1 },
    { id: 'slow', count: 2 },
    { id: 'untested', count: 3 },
  ],
}

const noTiles: ActiveHealthTiles = { status: null, health: null }

describe('ChannelHealthStrip', () => {
  it('renders nothing when the strip is hidden', () => {
    const { container } = renderStrip({
      state: { kind: 'hidden' },
      slowThresholdMs: 1000,
      activeTiles: noTiles,
    })
    expect(container.childElementCount).toBe(0)
  })

  it('renders one line with a test-all affordance when calm', () => {
    renderStrip({
      state: { kind: 'calm', total: 18 },
      slowThresholdMs: 1000,
      activeTiles: noTiles,
    })
    expect(screen.getByText('18 channel(s) healthy')).toBeTruthy()
    expect(
      screen.getByRole('button', { name: /Test All Channels/i })
    ).toBeTruthy()
  })

  it('renders four real buttons, one per tile, none pressed by default', () => {
    renderStrip({
      state: alertState,
      slowThresholdMs: 1000,
      activeTiles: noTiles,
    })
    const buttons = screen.getAllByRole('button')
    expect(buttons).toHaveLength(4)
    for (const button of buttons) {
      expect(button.getAttribute('aria-pressed')).toBe('false')
    }
  })

  it('marks the active tile pressed and reads the slow label off the server threshold', () => {
    renderStrip({
      state: alertState,
      slowThresholdMs: 1500,
      activeTiles: { status: null, health: 'slow' },
    })
    const slowButton = screen.getByRole('button', {
      name: /Slower than 1\.50s/i,
    })
    expect(slowButton.getAttribute('aria-pressed')).toBe('true')
    const activeButton = screen.getByRole('button', { name: /^Active/i })
    expect(activeButton.getAttribute('aria-pressed')).toBe('false')
  })

  it('lights up a status tile and a health tile together', () => {
    renderStrip({
      state: alertState,
      slowThresholdMs: 1000,
      activeTiles: { status: 'disabled', health: 'untested' },
    })
    const disabledButton = screen.getByRole('button', { name: /^Disabled/i })
    const untestedButton = screen.getByRole('button', {
      name: /Never tested/i,
    })
    expect(disabledButton.getAttribute('aria-pressed')).toBe('true')
    expect(untestedButton.getAttribute('aria-pressed')).toBe('true')
    const activeButton = screen.getByRole('button', { name: /^Active/i })
    const slowButton = screen.getByRole('button', { name: /^Slower/i })
    expect(activeButton.getAttribute('aria-pressed')).toBe('false')
    expect(slowButton.getAttribute('aria-pressed')).toBe('false')
  })

  it('calls onSelect with the clicked tile id', () => {
    const { getByRole, onSelect } = renderStrip({
      state: alertState,
      slowThresholdMs: 1000,
      activeTiles: noTiles,
    })
    fireEvent.click(getByRole('button', { name: /Never tested/i }))
    expect(onSelect).toHaveBeenCalledWith('untested')
  })
})

describe('ChannelHealthFilterChips', () => {
  it('renders nothing when no status/health filter is active', () => {
    const { container } = renderChips({
      activeTiles: noTiles,
      slowThresholdMs: 1000,
    })
    expect(container.childElementCount).toBe(0)
  })

  // This is the scenario the health strip's `calm` state hides: a status
  // filter left over from an earlier click (or restored from localStorage)
  // with nothing currently disabled, slow, or untested. The chip takes only
  // `activeTiles` as input — never the strip's render state — so it has no
  // way to go missing just because the strip collapsed to "N channels
  // healthy". If a future change re-couples chip visibility to the strip's
  // state, this test's props (an active filter, nothing else) still produce
  // the render this asserts.
  it('names the disabled filter even though nothing is currently unhealthy', () => {
    renderChips({
      activeTiles: { status: 'disabled', health: null },
      slowThresholdMs: 1000,
    })
    expect(screen.getByText('Disabled')).toBeTruthy()
  })

  it('reads the slow chip label off the server threshold, matching the tile', () => {
    renderChips({
      activeTiles: { status: null, health: 'slow' },
      slowThresholdMs: 1500,
    })
    expect(screen.getByText('Slower than 1.50s')).toBeTruthy()
  })

  it('renders both chips when a status filter and a health filter are both active', () => {
    renderChips({
      activeTiles: { status: 'disabled', health: 'untested' },
      slowThresholdMs: 1000,
    })
    expect(screen.getByText('Disabled')).toBeTruthy()
    expect(screen.getByText('Never tested')).toBeTruthy()
  })

  it('dismisses the status chip independently of the health chip', () => {
    const { onDismiss } = renderChips({
      activeTiles: { status: 'disabled', health: 'untested' },
      slowThresholdMs: 1000,
    })
    const buttons = screen.getAllByRole('button', { name: /Remove filter/i })
    expect(buttons).toHaveLength(2)
    fireEvent.click(buttons[0])
    expect(onDismiss).toHaveBeenCalledTimes(1)
    expect(onDismiss).toHaveBeenCalledWith('status')
  })

  it('dismisses the health chip', () => {
    const { onDismiss } = renderChips({
      activeTiles: { status: null, health: 'slow' },
      slowThresholdMs: 1000,
    })
    fireEvent.click(screen.getByRole('button', { name: /Remove filter/i }))
    expect(onDismiss).toHaveBeenCalledTimes(1)
    expect(onDismiss).toHaveBeenCalledWith('health')
  })
})
