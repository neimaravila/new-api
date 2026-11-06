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
export const JSON_HIGHLIGHT_MAX_CHARS = 256 * 1024

export type JsonTokenKind =
  | 'key'
  | 'string'
  | 'number'
  | 'literal'
  | 'punct'
  | 'space'
  | 'other'

export type JsonToken = { kind: JsonTokenKind; text: string }

const NUMBER_RE = /-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?/y
const LITERAL_RE = /true|false|null/y
const SPACE_RE = /\s+/y
const PUNCT = '{}[],:'

function matchAt(re: RegExp, text: string, index: number): string | null {
  re.lastIndex = index
  const m = re.exec(text)
  return m ? m[0] : null
}

function stringEnd(text: string, start: number): number {
  let i = start + 1
  while (i < text.length) {
    const ch = text[i]
    if (ch === '\\') i += 2
    else if (ch === '"') return i + 1
    else i++
  }
  return text.length
}

function isRecognizableStart(text: string, i: number): boolean {
  const ch = text[i]
  return (
    ch === '"' ||
    PUNCT.includes(ch) ||
    /[\s\d-]/.test(ch) ||
    matchAt(LITERAL_RE, text, i) !== null
  )
}

/**
 * Single-pass, total and lossless JSON lexer. Never throws; returns null when
 * the text is above JSON_HIGHLIGHT_MAX_CHARS.
 */
export function tokenizeJson(text: string): JsonToken[] | null {
  if (text.length > JSON_HIGHLIGHT_MAX_CHARS) return null
  const tokens: JsonToken[] = []
  let i = 0
  while (i < text.length) {
    const ch = text[i]
    if (ch === '"') {
      const end = stringEnd(text, i)
      let j = end
      while (j < text.length && /\s/.test(text[j])) j++
      const kind: JsonTokenKind = text[j] === ':' ? 'key' : 'string'
      tokens.push({ kind, text: text.slice(i, end) })
      i = end
      continue
    }
    if (PUNCT.includes(ch)) {
      tokens.push({ kind: 'punct', text: ch })
      i++
      continue
    }
    const space = matchAt(SPACE_RE, text, i)
    if (space) {
      tokens.push({ kind: 'space', text: space })
      i += space.length
      continue
    }
    const num = matchAt(NUMBER_RE, text, i)
    if (num) {
      tokens.push({ kind: 'number', text: num })
      i += num.length
      continue
    }
    const lit = matchAt(LITERAL_RE, text, i)
    if (lit) {
      tokens.push({ kind: 'literal', text: lit })
      i += lit.length
      continue
    }
    let j = i + 1
    while (j < text.length && !isRecognizableStart(text, j)) j++
    tokens.push({ kind: 'other', text: text.slice(i, j) })
    i = j
  }
  return tokens
}
