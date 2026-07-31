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

import type { HealthStripState } from '../../lib'

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
const { QueryClient, QueryClientProvider } = await import(
  '@tanstack/react-query'
)
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ChannelHealthStrip } = await import('../channel-health-strip')

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
          activeTile={props.activeTile}
          onSelect={onSelect}
        />
      </I18nextProvider>
    </QueryClientProvider>
  )
  return { ...utils, onSelect }
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

describe('ChannelHealthStrip', () => {
  it('renders nothing when the strip is hidden', () => {
    const { container } = renderStrip({
      state: { kind: 'hidden' },
      slowThresholdMs: 1000,
      activeTile: null,
    })
    expect(container.childElementCount).toBe(0)
  })

  it('renders one line with a test-all affordance when calm', () => {
    renderStrip({
      state: { kind: 'calm', total: 18 },
      slowThresholdMs: 1000,
      activeTile: null,
    })
    expect(screen.getByText('18 channels healthy')).toBeTruthy()
    expect(
      screen.getByRole('button', { name: /Test All Channels/i })
    ).toBeTruthy()
  })

  it('renders four real buttons, one per tile, none pressed by default', () => {
    renderStrip({ state: alertState, slowThresholdMs: 1000, activeTile: null })
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
      activeTile: 'slow',
    })
    const slowButton = screen.getByRole('button', {
      name: /Slower than 1\.50s/i,
    })
    expect(slowButton.getAttribute('aria-pressed')).toBe('true')
    const activeButton = screen.getByRole('button', { name: /^Active/i })
    expect(activeButton.getAttribute('aria-pressed')).toBe('false')
  })

  it('calls onSelect with the clicked tile id', () => {
    const { getByRole, onSelect } = renderStrip({
      state: alertState,
      slowThresholdMs: 1000,
      activeTile: null,
    })
    fireEvent.click(getByRole('button', { name: /Never tested/i }))
    expect(onSelect).toHaveBeenCalledWith('untested')
  })
})
