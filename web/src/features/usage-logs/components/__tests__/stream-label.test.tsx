import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

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
import { expect, it } from 'vitest'

import { StreamTpsCell } from '../timing-metrics-cell'

const css = readFileSync(
  resolve(__dirname, '../../../../styles/visual-finish.css'),
  'utf8'
)

function ruleFor(stream: 'true' | 'false'): string {
  const selector = `[data-visual-finish='logs'] [data-slot='stream-label'][data-stream='${stream}'] {`
  const start = css.indexOf(selector)
  expect(start, selector).toBeGreaterThanOrEqual(0)
  return css.slice(start, css.indexOf('}', start))
}

it('marks the Stream label as streaming', () => {
  render(<StreamTpsCell isStream />)
  expect(screen.getByText('Stream')).toHaveAttribute('data-stream', 'true')
})

it('marks the Non-stream label as not streaming', () => {
  render(<StreamTpsCell isStream={false} />)
  expect(screen.getByText('Non-stream')).toHaveAttribute('data-stream', 'false')
})

it('colors the streaming label with the readable info text token', () => {
  expect(ruleFor('true')).toContain('color: var(--vf-info-text)')
})

it('colors the non-streaming label with the readable neutral text token', () => {
  expect(ruleFor('false')).toContain('color: var(--vf-neutral-text)')
})
