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
import { expect, it } from 'vitest'

import { looksLikeJson, PRETTY_MAX_CHARS, splitFences } from '../content-format'

it('splitFences separates fenced code from prose', () => {
  const input = 'a\n```js\nx=1\n```\nb'
  const parts = splitFences(input)
  expect(parts).toEqual([
    { kind: 'prose', text: 'a\n' },
    { kind: 'code', text: '```js\nx=1\n```\n' },
    { kind: 'prose', text: 'b' },
  ])
  expect(parts.map((p) => p.text).join('')).toBe(input)
})

it('splitFences treats an unclosed fence as code to the end', () => {
  expect(splitFences('intro\n```\nx\ny')).toEqual([
    { kind: 'prose', text: 'intro\n' },
    { kind: 'code', text: '```\nx\ny' },
  ])
})

it('splitFences returns nothing for empty text', () => {
  expect(splitFences('')).toEqual([])
})

it('looksLikeJson accepts objects and arrays only', () => {
  expect(looksLikeJson('{"a":1}')).toBe(true)
  expect(looksLikeJson(' [1] ')).toBe(true)
  expect(looksLikeJson('hello')).toBe(false)
  expect(looksLikeJson('{bad')).toBe(false)
  expect(looksLikeJson('1')).toBe(false)
  expect(looksLikeJson(`["${'x'.repeat(PRETTY_MAX_CHARS)}"]`)).toBe(false)
})
