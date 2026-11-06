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
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { JSON_HIGHLIGHT_MAX_CHARS } from '../json-tokens'
import { JsonView } from '../json-view'

describe('JsonView', () => {
  it('colors small JSON with token spans', () => {
    const { container } = render(<JsonView text='{"k":1}' />)
    expect(container.querySelectorAll('[data-token=key]').length).toBe(1)
    const pre = container.querySelector('pre')
    expect(pre).toHaveAttribute('data-highlighted', 'true')
    expect(pre).toHaveAttribute('data-text-style', 'json')
  })

  it('shows plain text above 256 KiB', () => {
    const text = 'a'.repeat(JSON_HIGHLIGHT_MAX_CHARS + 1)
    const { container } = render(<JsonView text={text} />)
    fireEvent.click(screen.getByRole('button', { name: 'Show more' }))
    expect(container.querySelector('[data-token]')).toBeNull()
    expect(container.querySelector('pre')).toHaveAttribute(
      'data-highlighted',
      'false'
    )
    expect(container.querySelector('pre')?.textContent).toBe(text)
  })

  it('keeps markup in strings as text', () => {
    const text = '{"x":"<script>alert(1)</script><img src=x onerror=alert(1)>"}'
    const { container } = render(<JsonView text={text} />)
    expect(container.querySelector('script, img')).toBeNull()
    expect(container.textContent).toContain('<script>')
  })

  it('scrolls long lines instead of breaking words', () => {
    const { container } = render(<JsonView text='{"k":1}' />)
    const pre = container.querySelector('pre')
    expect(pre?.classList.contains('break-all')).toBe(false)
    expect(pre?.classList.contains('overflow-x-auto')).toBe(true)
  })

  it('folds after collapsedLines and expands', () => {
    const text = Array.from({ length: 20 }, (_, i) => `line${i + 1}`).join('\n')
    const { container } = render(<JsonView text={text} collapsedLines={12} />)
    expect(container.textContent).not.toContain('line13')
    fireEvent.click(screen.getByRole('button', { name: 'Show more' }))
    expect(container.textContent).toContain('line20')
    expect(screen.getByRole('button', { name: 'Show less' })).toBeTruthy()
  })

  it('cuts a long one-line JSON at 4000 characters until Show more', () => {
    const text = JSON.stringify({ k: 'a b '.repeat(2000), tail: 'TAIL' })
    expect(text.length).toBeGreaterThan(4000)
    const { container } = render(<JsonView text={text} />)
    expect(container.textContent).not.toContain('TAIL')
    expect(container.querySelector('pre')!.textContent!.length).toBeLessThan(
      4100
    )
    fireEvent.click(screen.getByRole('button', { name: 'Show more' }))
    expect(container.querySelector('pre')!.textContent).toBe(text)
    expect(screen.getByRole('button', { name: 'Show less' })).toBeTruthy()
  })

  it('cuts multi-line JSON on a line boundary', () => {
    const lines = Array.from(
      { length: 400 },
      (_, i) => `  "key${i}": "${'x'.repeat(20)}",`
    )
    const text = `{\n${lines.join('\n')}\n}`
    const { container } = render(<JsonView text={text} />)
    const shown = container.querySelector('pre')!.textContent!
    expect(shown.length).toBeLessThanOrEqual(4001)
    expect(text.startsWith(shown.replace(/…$/, ''))).toBe(true)
    expect(shown.replace(/…$/, '').endsWith(',')).toBe(true)
  })

  it('renders 256 KiB of [ in fewer than 25000 elements', () => {
    const text = '['.repeat(JSON_HIGHLIGHT_MAX_CHARS)
    const { container } = render(<JsonView text={text} />)
    fireEvent.click(screen.getByRole('button', { name: 'Show more' }))
    expect(container.querySelectorAll('*').length).toBeLessThan(25000)
    expect(container.querySelector('pre')!.textContent).toBe(text)
  })

  it('merges adjacent punctuation and whitespace into one span', () => {
    const { container } = render(<JsonView text='{ "a": [ 1 ] }' />)
    const spans = [...container.querySelectorAll('[data-token]')]
    for (let i = 1; i < spans.length; i++) {
      const glue = (e: Element) =>
        ['punct', 'space'].includes(e.getAttribute('data-token')!)
      expect(glue(spans[i - 1]) && glue(spans[i])).toBe(false)
    }
    expect(container.querySelector('pre')!.textContent).toBe('{ "a": [ 1 ] }')
  })

  it('renders the remainder as plain text above the token cap', () => {
    const text = '1,'.repeat(30000).slice(0, JSON_HIGHLIGHT_MAX_CHARS)
    const { container } = render(<JsonView text={text} />)
    fireEvent.click(screen.getByRole('button', { name: 'Show more' }))
    expect(
      container.querySelectorAll('[data-token]').length
    ).toBeLessThanOrEqual(20000)
    expect(container.querySelector('pre')!.textContent).toBe(text)
  })
})
