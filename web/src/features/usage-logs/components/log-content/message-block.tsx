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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton as IconCopyButton } from '@/components/copy-button'
import { JsonView, TintBadge } from '@/components/finish'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { cn } from '@/lib/utils'

import type {
  LogContentBlock,
  LogContentMessage,
} from '../../data/log-content-schema'
import {
  looksLikeJson,
  prettyJsonText,
  splitFences,
  useFormatBytes,
} from './content-format'
import { ROLE_DOT, ROLE_STYLE, visualRole } from './role-style'

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

/** The first LONG_TEXT_CHARS characters, never ending inside a surrogate pair. */
function cutText(text: string): string {
  let end = LONG_TEXT_CHARS
  const last = text.charCodeAt(end - 1)
  if (last >= 0xd800 && last <= 0xdbff) end--
  return text.slice(0, end)
}

const PROSE_CLASS =
  'font-sans text-[13px] leading-[1.6] [overflow-wrap:anywhere] whitespace-pre-wrap'
const CODE_CLASS =
  'bg-muted/50 rounded-(--vf-radius-control) px-2 py-1 font-mono text-[12.5px] leading-[1.5] [overflow-wrap:anywhere] whitespace-pre-wrap'

/**
 * Literal text (never HTML or Markdown), cut after LONG_TEXT_CHARS until
 * expanded. `prose` sets text in the interface font with fenced code in mono;
 * without a variant the text stays mono, as before.
 */
export function LongText(props: {
  text: string
  variant?: 'prose' | 'mono'
  className?: string
}) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)
  const long = props.text.length > LONG_TEXT_CHARS
  const cut = long && !expanded
  const shown = cut ? `${cutText(props.text)}…` : props.text
  const segments = useMemo(() => {
    if (props.variant !== 'prose') return null
    let offset = 0
    return splitFences(shown).map((segment) => {
      const keyed = { ...segment, offset }
      offset += segment.text.length
      return keyed
    })
  }, [props.variant, shown])
  return (
    <div>
      {segments ? (
        <div className='space-y-1.5'>
          {segments.map((segment) => (
            <div
              key={segment.offset}
              data-text-style={segment.kind === 'code' ? 'mono' : 'prose'}
              className={cn(
                segment.kind === 'code' ? CODE_CLASS : PROSE_CLASS,
                props.className
              )}
            >
              {segment.text}
            </div>
          ))}
        </div>
      ) : (
        <div
          data-text-style='mono'
          className={cn(
            'font-mono text-sm break-words whitespace-pre-wrap',
            props.className
          )}
        >
          {shown}
        </div>
      )}
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

/** Text of a block: JSON highlighted, otherwise prose with fenced code in mono. */
function TextView(props: { text: string; className?: string }) {
  const json = useMemo(() => looksLikeJson(props.text), [props.text])
  const pretty = useMemo(
    () => (json ? prettyJsonText(props.text) : ''),
    [json, props.text]
  )
  if (json) return <JsonView text={pretty} collapsedLines={12} />
  return (
    <LongText text={props.text} variant='prose' className={props.className} />
  )
}

function ToolCallBox(props: { block: LogContentBlock }) {
  const { block } = props
  const { t } = useTranslation()
  const pretty = useMemo(
    () => prettyJsonText(block.arguments),
    [block.arguments]
  )
  return (
    <div
      data-testid='log-content-tool-call'
      className='bg-background overflow-hidden rounded-(--vf-radius-control) border'
    >
      <div
        className={cn(
          'flex items-center gap-2 px-3 py-1.5',
          block.arguments && 'border-b'
        )}
      >
        <span className='sr-only'>{t('Tool call')}</span>
        <span className='font-mono text-[13px] font-semibold'>
          {block.name}
        </span>
        {block.id && (
          <span className='text-muted-foreground font-mono text-[11px]'>
            {block.id}
          </span>
        )}
        <IconCopyButton
          value={block.arguments}
          aria-label={t('Copy')}
          className='ml-auto size-5 p-0'
          iconClassName='size-3.5'
        />
      </div>
      {block.arguments && <JsonView text={pretty} collapsedLines={12} />}
    </div>
  )
}

function BlockView(props: { block: LogContentBlock }) {
  const { t } = useTranslation()
  const formatSize = useFormatBytes()
  const { block } = props
  switch (block.type) {
    case 'tool_call':
      return <ToolCallBox block={block} />
    case 'tool_result':
      return (
        <SubBlock
          label={
            block.name ? `${t('Tool result')}: ${block.name}` : t('Tool result')
          }
          error={block.is_error}
        >
          {looksLikeJson(block.text) ? (
            <JsonView text={prettyJsonText(block.text)} collapsedLines={12} />
          ) : (
            <LongText
              text={block.text}
              variant='mono'
              className='text-[12.5px] leading-[1.5]'
            />
          )}
        </SubBlock>
      )
    case 'media': {
      const kind = block.media_type.split('/')[0]
      let label = t('Media')
      if (kind === 'audio') label = t('Audio')
      else if (kind === 'image') label = t('Image')
      return (
        <Badge variant='outline' className='font-mono'>
          {label} · {block.media_type} · {formatSize(block.size_bytes)}
        </Badge>
      )
    }
    case 'thinking':
      return (
        <SubBlock label={t('Thinking')}>
          <TextView text={block.text} className='text-muted-foreground' />
        </SubBlock>
      )
    case 'error':
      return (
        <SubBlock label={t('Error')} error>
          <TextView text={block.text} />
        </SubBlock>
      )
    default:
      return <TextView text={block.text} />
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

/** One message on the timeline: role label above a tinted box with its blocks. */
export function MessageBlock(props: { message: LogContentMessage }) {
  const { t } = useTranslation()
  const { message } = props
  const blocks = message.blocks.map((block, position) => ({ block, position }))
  const hasError = message.blocks.some((b) => b.type === 'error')
  const visual = visualRole(message.role, hasError)
  const lines = messageText(message).split('\n').length
  return (
    <div className='relative'>
      <span
        aria-hidden='true'
        className={cn(
          'absolute top-[7px] -left-5 size-1.5 rounded-full',
          ROLE_DOT[visual]
        )}
      />
      <div className='mb-1 flex items-center gap-2'>
        <span className='text-xs font-semibold capitalize'>{message.role}</span>
        <span className='text-muted-foreground text-[11px]'>
          {t('{{count}} lines', { count: lines })}
        </span>
        {message.source === 'response' && (
          <TintBadge tone='primary'>{t('Response')}</TintBadge>
        )}
      </div>
      <section
        data-testid='log-content-message'
        data-role={message.role}
        className={cn(
          'group relative space-y-2 rounded-(--vf-radius-inner) border p-3',
          ROLE_STYLE[visual]
        )}
      >
        <IconCopyButton
          value={messageText(message)}
          aria-label={t('Copy')}
          className='absolute top-2 right-2 size-5 p-0 opacity-0 group-hover:opacity-100 focus-visible:opacity-100'
          iconClassName='size-3.5'
        />
        <div className='space-y-2'>
          {blocks.map((entry) => (
            <BlockView key={entry.position} block={entry.block} />
          ))}
        </div>
      </section>
    </div>
  )
}
