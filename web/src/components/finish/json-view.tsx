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

import { tokenizeJson, type JsonTokenKind } from './json-tokens'

const TOKEN_CLASS: Partial<Record<JsonTokenKind, string>> = {
  key: 'text-(--vf-json-key)',
  string: 'text-(--vf-json-string)',
  number: 'text-(--vf-json-number)',
  literal: 'text-(--vf-json-literal)',
  punct: 'text-(--vf-json-punct)',
}

export function JsonView(props: {
  text: string
  collapsedLines?: number
  className?: string
}) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)

  const lines = useMemo(() => props.text.split('\n'), [props.text])
  const limit = props.collapsedLines
  const foldable = limit != null && lines.length > limit
  const folded = foldable && !expanded
  const shown = useMemo(
    () => (folded ? lines.slice(0, limit).join('\n') : props.text),
    [folded, lines, limit, props.text]
  )
  const tokens = useMemo(() => {
    const raw = tokenizeJson(shown)
    if (!raw) return null
    let offset = 0
    return raw.map((token) => {
      const keyed = { ...token, offset }
      offset += token.text.length
      return keyed
    })
  }, [shown])

  return (
    <div className='min-w-0 overflow-hidden rounded-(--vf-radius-control)'>
      <pre
        data-text-style='json'
        data-highlighted={tokens ? 'true' : 'false'}
        className={cn(
          'overflow-x-auto p-3 font-mono text-[12.5px] leading-[1.5] whitespace-pre',
          props.className
        )}
      >
        {tokens
          ? tokens.map((token) => (
              <span
                key={token.offset}
                data-token={token.kind}
                className={TOKEN_CLASS[token.kind]}
              >
                {token.text}
              </span>
            ))
          : shown}
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
