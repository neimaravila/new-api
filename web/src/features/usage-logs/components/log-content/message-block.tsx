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
import { ChevronDown, ChevronUp, Copy } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { formatBytes } from '@/lib/format'
import { cn } from '@/lib/utils'

import type {
  LogContentBlock,
  LogContentMessage,
} from '../../data/log-content-schema'

/** Text longer than this starts collapsed. */
export const LONG_TEXT_CHARS = 4000

/** A copy button that copies `text` whole, whatever is on screen. */
export function CopyButton(props: { text: string; label?: string }) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  return (
    <Button
      type='button'
      variant='ghost'
      size='xs'
      className='text-muted-foreground'
      onClick={() => void copyToClipboard(props.text)}
    >
      <Copy />
      {props.label ?? t('Copy')}
    </Button>
  )
}

/** Literal text (never HTML or Markdown), cut after LONG_TEXT_CHARS until expanded. */
export function LongText(props: { text: string; className?: string }) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)
  const long = props.text.length > LONG_TEXT_CHARS
  const shown =
    long && !expanded ? props.text.slice(0, LONG_TEXT_CHARS) : props.text
  return (
    <div>
      <div
        className={cn(
          'font-mono text-sm break-words whitespace-pre-wrap',
          props.className
        )}
      >
        {shown}
        {long && !expanded && '…'}
      </div>
      {long && (
        <Button
          type='button'
          variant='link'
          size='xs'
          className='px-0'
          onClick={() => setExpanded((v) => !v)}
        >
          {expanded ? <ChevronUp /> : <ChevronDown />}
          {expanded ? t('Show less') : t('Show more')}
        </Button>
      )}
    </div>
  )
}

/** Arguments are shown indented when they are JSON, as sent otherwise. */
function prettyArguments(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

function SubBlock(props: {
  label: string
  error?: boolean
  children: React.ReactNode
}) {
  return (
    <div
      className={cn(
        'rounded-md border px-3 py-2',
        props.error
          ? 'border-destructive/40 bg-destructive/10 text-destructive'
          : 'bg-muted/30'
      )}
    >
      <div className='mb-1 text-xs font-medium'>{props.label}</div>
      {props.children}
    </div>
  )
}

function BlockView(props: { block: LogContentBlock }) {
  const { t } = useTranslation()
  const { block } = props
  switch (block.type) {
    case 'tool_call':
      return (
        <SubBlock label={`${t('Tool call')}: ${block.name}`}>
          {block.arguments && (
            <LongText text={prettyArguments(block.arguments)} />
          )}
        </SubBlock>
      )
    case 'tool_result':
      return (
        <SubBlock
          label={
            block.name ? `${t('Tool result')}: ${block.name}` : t('Tool result')
          }
          error={block.is_error}
        >
          <LongText text={block.text} />
        </SubBlock>
      )
    case 'media': {
      const kind = block.media_type.split('/')[0]
      let label = t('Media')
      if (kind === 'audio') label = t('Audio')
      else if (kind === 'image') label = t('Image')
      return (
        <Badge variant='outline' className='font-mono'>
          {label} · {block.media_type} · {formatBytes(block.size_bytes)}
        </Badge>
      )
    }
    case 'thinking':
      return (
        <SubBlock label={t('Thinking')}>
          <LongText text={block.text} className='text-muted-foreground' />
        </SubBlock>
      )
    case 'error':
      return (
        <SubBlock label={t('Error')} error>
          <LongText text={block.text} />
        </SubBlock>
      )
    default:
      return <LongText text={block.text} />
  }
}

function messageText(message: LogContentMessage): string {
  return message.blocks
    .map((b) =>
      b.type === 'tool_call' ? `${b.name} ${b.arguments}`.trim() : b.text
    )
    .filter(Boolean)
    .join('\n\n')
}

/** One message: role label, copy action and its blocks, in a bordered box. */
export function MessageBlock(props: { message: LogContentMessage }) {
  const { t } = useTranslation()
  const { message } = props
  const hasError = message.blocks.some((b) => b.type === 'error')
  return (
    <section
      data-testid='log-content-message'
      data-role={message.role}
      className={cn(
        'space-y-2 rounded-lg border p-3',
        hasError && 'border-destructive/40'
      )}
    >
      <header className='flex items-center justify-between gap-2'>
        <div className='flex items-center gap-2'>
          <span className='text-sm font-medium capitalize'>{message.role}</span>
          {message.source === 'response' && (
            <Badge variant='secondary'>{t('Response')}</Badge>
          )}
        </div>
        <CopyButton text={messageText(message)} />
      </header>
      <div className='space-y-2'>
        {message.blocks.map((block) => (
          <BlockView key={message.blocks.indexOf(block)} block={block} />
        ))}
      </div>
    </section>
  )
}
