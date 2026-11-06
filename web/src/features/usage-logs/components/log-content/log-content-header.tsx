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
import { ChevronDown, ChevronUp } from 'lucide-react'
import { Fragment } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import {
  TintBadge,
  toneFromStatusVariant,
  type TintTone,
} from '@/components/finish'
import type { StatusBadgeProps } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { SheetTitle } from '@/components/ui/sheet'
import { cn } from '@/lib/utils'

import type { LogContentView } from '../../data/log-content-schema'
import type { UsageLog } from '../../data/schema'
import { getLogStatusConfig } from '../../lib/utils'
import { LogContentMenu } from './log-content-menu'

const FORMAT_LABELS: Record<string, string> = {
  openai: 'OpenAI Chat',
  openai_responses: 'OpenAI Responses',
  claude: 'Claude',
  gemini: 'Gemini',
  embedding: 'Embedding',
  rerank: 'Rerank',
}

function httpVariant(status: number): StatusBadgeProps['variant'] {
  if (status >= 500) return 'danger'
  if (status >= 400) return 'warning'
  return 'success'
}

/** Badges for the call, the previous/next log buttons and the actions menu. */
export function LogContentHeader(props: {
  log: UsageLog
  view: LogContentView | undefined
  isAdmin: boolean
  sensitiveVisible: boolean
  hasPrevious: boolean
  hasNext: boolean
  onPrevious: () => void
  onNext: () => void
  selectedAttempt: number
  onSelectAttempt: (attempt: number) => void
}) {
  const { t } = useTranslation()
  const { log, view } = props
  const status = getLogStatusConfig(log.type)
  const attempts = view?.attempts ?? []
  const channel = log.channel
    ? `#${log.channel}${log.channel_name ? ` ${log.channel_name}` : ''}`
    : ''
  const httpStatus = view?.http_status ?? 0
  const isStream = view ? view.is_stream : log.is_stream
  const statusLabel = t(status.label)
  const statusTone: TintTone =
    httpStatus >= 400
      ? toneFromStatusVariant(httpVariant(httpStatus))
      : toneFromStatusVariant(status.color as StatusBadgeProps['variant'])

  return (
    <div className='min-w-0 space-y-2 pr-12' data-testid='log-content-header'>
      <div className='flex flex-wrap items-center gap-1.5'>
        <SheetTitle className='mr-1 text-[13px] font-medium'>
          {t('Log Details')}
        </SheetTitle>
        <TintBadge tone={statusTone} strong dot>
          {httpStatus > 0 ? `${statusLabel} · ${httpStatus}` : statusLabel}
        </TintBadge>
        {view && FORMAT_LABELS[view.format] && (
          <TintBadge tone='neutral'>{FORMAT_LABELS[view.format]}</TintBadge>
        )}
        {isStream && <TintBadge tone='info'>{t('Stream')}</TintBadge>}
        {props.isAdmin && channel && (
          <TintBadge tone='neutral'>
            {props.sensitiveVisible ? channel : '••••'}
          </TintBadge>
        )}
        <div className='ml-auto flex items-center gap-0.5'>
          <Button
            variant='ghost'
            size='icon'
            className='size-7'
            disabled={!props.hasPrevious}
            onClick={props.onPrevious}
            aria-label={t('Previous log')}
          >
            <ChevronUp className='size-4' />
          </Button>
          <Button
            variant='ghost'
            size='icon'
            className='size-7'
            disabled={!props.hasNext}
            onClick={props.onNext}
            aria-label={t('Next log')}
          >
            <ChevronDown className='size-4' />
          </Button>
          <LogContentMenu
            requestId={log.request_id}
            view={view}
            sensitiveVisible={props.sensitiveVisible}
          />
        </div>
      </div>
      {log.request_id && (
        <div className='flex min-w-0 items-center gap-2'>
          <span className='text-muted-foreground text-[11px] font-semibold tracking-[0.05em] uppercase'>
            {t('Request ID')}
          </span>
          <span className='text-foreground truncate font-mono text-[13px]'>
            {log.request_id}
          </span>
          <CopyButton
            value={log.request_id}
            aria-label={t('Copy')}
            className='size-4 p-0'
            iconClassName='size-3.5'
          />
        </div>
      )}
      {props.isAdmin && attempts.length > 1 && (
        <div
          className='flex flex-wrap items-center gap-1.5'
          role='group'
          aria-label={t('Attempts')}
        >
          {attempts.map((attempt, position) => {
            const selected = attempt.attempt === props.selectedAttempt
            const failed = (attempt.status_code ?? 0) >= 400
            return (
              <Fragment key={attempt.attempt}>
                {position > 0 && (
                  <span aria-hidden='true' className='text-muted-foreground'>
                    →
                  </span>
                )}
                <Button
                  variant={selected ? 'secondary' : 'outline'}
                  size='sm'
                  className='h-7 gap-1.5 px-2 text-xs'
                  aria-pressed={selected}
                  onClick={() => props.onSelectAttempt(attempt.attempt)}
                >
                  <span>#{attempt.attempt}</span>
                  <span className='font-mono'>
                    {props.sensitiveVisible
                      ? attempt.channel_name || `#${attempt.channel_id}`
                      : '••••'}
                  </span>
                  <span
                    className={cn(
                      'font-mono',
                      attempt.status_code &&
                        (failed
                          ? 'text-(--vf-danger-text)'
                          : 'text-(--vf-success-text)')
                    )}
                  >
                    {attempt.status_code || '-'}
                  </span>
                </Button>
              </Fragment>
            )
          })}
        </div>
      )}
    </div>
  )
}
