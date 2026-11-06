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
import { describe, expect, it } from 'vitest'

import { JSON_HIGHLIGHT_MAX_CHARS, tokenizeJson } from '../json-tokens'

const join = (tokens: { text: string }[] | null) =>
  (tokens ?? []).map((t) => t.text).join('')

describe('tokenizeJson', () => {
  it('is lossless', () => {
    const inputs = [
      '{"a\\"b": [1, -1.5e+3, true, null], "c": "é ok"}\n',
      'data: {"x":1}\n\n',
      '{"cut": "abc',
    ]
    for (const input of inputs) {
      expect(join(tokenizeJson(input))).toBe(input)
    }
  })

  it('tells keys from string values', () => {
    const tokens = tokenizeJson('{"k": "v"}') ?? []
    expect(tokens.map((t) => t.kind)).toEqual([
      'punct',
      'key',
      'punct',
      'space',
      'string',
      'punct',
    ])
  })

  it('reads numbers and literals', () => {
    const tokens = tokenizeJson('[-0.5e2,true,false,null]') ?? []
    expect(
      tokens.filter((t) => t.kind !== 'punct').map((t) => t.kind)
    ).toEqual(['number', 'literal', 'literal', 'literal'])
  })

  it('keeps an unterminated string as one token', () => {
    const tokens = tokenizeJson('{"cut": "abc') ?? []
    expect(tokens.at(-1)).toEqual({
      kind: 'string',
      text: '"abc',
    })
  })

  it('stops at the size limit', () => {
    expect(tokenizeJson('a'.repeat(JSON_HIGHLIGHT_MAX_CHARS))).not.toBeNull()
    expect(tokenizeJson('a'.repeat(JSON_HIGHLIGHT_MAX_CHARS + 1))).toBeNull()
  })
})
