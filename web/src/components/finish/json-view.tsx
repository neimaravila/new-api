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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { tokenizeJson, type JsonToken, type JsonTokenKind } from './json-tokens'

const TOKEN_CLASS: Partial<Record<JsonTokenKind, string>> = {
  key: 'text-(--vf-json-key)',
  string: 'text-(--vf-json-string)',
  number: 'text-(--vf-json-number)',
  literal: 'text-(--vf-json-literal)',
  punct: 'text-(--vf-json-punct)',
}

/** Same cut LongText applies (LONG_TEXT_CHARS) until "Show more". */
export const JSON_CUT_CHARS = 4000
/** Above this many spans the remainder renders as plain text. */
export const JSON_MAX_TOKENS = 20000

type ViewToken = { kind: JsonTokenKind; text: string; offset: number }

/** Merge adjacent punctuation and whitespace tokens into one span. */
function mergeTokens(raw: JsonToken[]): JsonToken[] {
  const out: JsonToken[] = []
  for (const token of raw) {
    const last = out.at(-1)
    const glue = (k: JsonTokenKind) => k === 'punct' || k === 'space'
    if (last && glue(last.kind) && glue(token.kind)) {
      last.kind =
        last.kind === 'punct' || token.kind === 'punct' ? 'punct' : 'space'
      last.text += token.text
    } else {
      out.push({ ...token })
    }
  }
  return out
}

/** Cut `text` to about JSON_CUT_CHARS on a line or token boundary. */
function cutJson(text: string): string {
  if (text.length <= JSON_CUT_CHARS) return text
  const head = text.slice(0, JSON_CUT_CHARS)
  const newline = head.lastIndexOf('\n')
  if (newline > 0) return head.slice(0, newline)
  const raw = tokenizeJson(head)
  if (raw && raw.length > 1) {
    raw.pop() // the last token may be partial
    return raw.map((tk) => tk.text).join('')
  }
  const last = head.charCodeAt(head.length - 1)
  return last >= 0xd800 && last <= 0xdbff ? head.slice(0, -1) : head
}

export function JsonView(props: {
  text: string
  collapsedLines?: number
  className?: string
}) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)

  const limit = props.collapsedLines
  const lines = useMemo(() => props.text.split('\n'), [props.text])
  const longByLines = limit != null && lines.length > limit
  const longByChars = props.text.length > JSON_CUT_CHARS
  const foldable = longByLines || longByChars
  const folded = foldable && !expanded
  const shown = useMemo(() => {
    if (!folded) return props.text
    const byLines = longByLines ? lines.slice(0, limit).join('\n') : props.text
    return cutJson(byLines)
  }, [folded, longByLines, lines, limit, props.text])
  const view = useMemo(() => {
    const raw = tokenizeJson(shown)
    if (!raw) return null
    const merged = mergeTokens(raw)
    const capped = merged.slice(0, JSON_MAX_TOKENS)
    let offset = 0
    const tokens: ViewToken[] = capped.map((token) => {
      const keyed = { ...token, offset }
      offset += token.text.length
      return keyed
    })
    return { tokens, rest: shown.slice(offset) }
  }, [shown])

  return (
    <div className='min-w-0 overflow-hidden rounded-(--vf-radius-control)'>
      <pre
        data-text-style='json'
        data-highlighted={view ? 'true' : 'false'}
        className={cn(
          'overflow-x-auto p-3 font-mono text-[12.5px] leading-[1.5] whitespace-pre',
          props.className
        )}
      >
        {view ? (
          <>
            {view.tokens.map((token) => (
              <span
                key={token.offset}
                data-token={token.kind}
                className={TOKEN_CLASS[token.kind]}
              >
                {token.text}
              </span>
            ))}
            {view.rest}
          </>
        ) : (
          shown
        )}
        {folded && longByChars ? '…' : null}
      </pre>
      {foldable && (
        <div className='bg-muted/50 flex h-7 items-center px-2 text-xs'>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            className='h-6 px-2 text-xs'
            onClick={() => setExpanded((v) => !v)}
          >
            {expanded ? t('Show less') : t('Show more')}
          </Button>
        </div>
      )}
    </div>
  )
}
