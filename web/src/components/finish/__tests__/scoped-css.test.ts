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
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const css = readFileSync(
  resolve(__dirname, '../../../styles/visual-finish.css'),
  'utf8'
).replaceAll(/\/\*[\s\S]*?\*\//g, '')

/** Selectors of every rule, with @media/@supports wrappers flattened. */
function selectors(source: string): string[] {
  const flat = source.replaceAll(/@(media|supports)[^{]*\{/g, '')
  const out: string[] = []
  const re = /([^{}]+)\{[^{}]*\}/g
  let m: RegExpExecArray | null
  while ((m = re.exec(flat))) {
    for (const part of m[1].split(',')) out.push(part.trim())
  }
  return out
}

describe('visual-finish.css', () => {
  it('every rule in visual-finish.css is scoped to the finish attribute', () => {
    const list = selectors(css)
    expect(list.length).toBeGreaterThan(0)
    for (const s of list) {
      expect(
        s.startsWith('[data-visual-finish') ||
          s.startsWith('.dark [data-visual-finish'),
        s
      ).toBe(true)
    }
  })

  it('visual-finish.css uses no fixed colors', () => {
    expect(css).not.toMatch(/#[0-9a-f]{3,8}\b/i)
    expect(css).not.toMatch(/rgb\(|hsl\(|oklch\(|oklab\(/i)
  })

  it('defines every token light and dark', () => {
    const dark = css.slice(css.indexOf('.dark [data-visual-finish'))
    const light = css.slice(0, css.indexOf('.dark [data-visual-finish'))
    for (const token of [
      '--vf-tint-fill',
      '--vf-tint-edge',
      '--vf-badge-fill',
      '--vf-badge-edge',
    ]) {
      expect(light).toContain(`${token}:`)
      expect(dark).toContain(`${token}:`)
    }
  })
})
