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
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import type {
  LogContentAttempt,
  LogContentView,
} from '../../data/log-content-schema'
import type { UsageLog } from '../../data/schema'
import { parseLogOther } from '../../lib/format'

const MASK = '••••'

function Fact(props: { label: string; value: string }) {
  return (
    <div className='min-w-0'>
      <dt className='text-muted-foreground text-xs'>{props.label}</dt>
      <dd className='font-mono text-sm break-all'>{props.value}</dd>
    </div>
  )
}

function AttemptRow(props: {
  attempt: LogContentAttempt
  sensitiveVisible: boolean
}) {
  const { t, i18n } = useTranslation()
  const { attempt, sensitiveVisible } = props
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const channel = attempt.channel_name
    ? `#${attempt.channel_id} ${attempt.channel_name}`
    : `#${attempt.channel_id}`
  const failed = attempt.status_code >= 400 || !!attempt.error_message
  return (
    <li
      data-testid='log-content-attempt'
      className='space-y-1.5 rounded-lg border p-3'
    >
      <div className='flex flex-wrap items-center gap-2'>
        <span className='text-sm font-medium'>
          {t('Attempt {{attempt}}', { attempt: attempt.attempt })}
        </span>
        <span className='font-mono text-sm'>
          {sensitiveVisible ? channel : MASK}
        </span>
        {attempt.multi_key_index >= 0 && (
          <span className='text-muted-foreground font-mono text-xs'>
            {t('Key {{index}}', {
              index: sensitiveVisible
                ? formatNumber(attempt.multi_key_index, locale)
                : MASK,
            })}
          </span>
        )}
        <StatusBadge
          label={attempt.status_code ? String(attempt.status_code) : '-'}
          variant={failed ? 'danger' : 'success'}
          size='sm'
          copyable={false}
        />
        <span className='text-muted-foreground ml-auto text-xs'>
          {t('Duration')}: {formatNumber(attempt.duration_ms, locale)} ms
        </span>
      </div>
      {attempt.error_message && (
        <p className='text-destructive text-sm break-words whitespace-pre-wrap'>
          <span className='font-medium'>{t('Reason')}: </span>
          {attempt.error_message}
        </p>
      )}
    </li>
  )
}

/** Admin-only: group, affinity, mapped model and the attempts in order. */
export function RoutingTab(props: {
  view: LogContentView
  log: UsageLog
  sensitiveVisible: boolean
}) {
  const { t } = useTranslation()
  const { view, log, sensitiveVisible } = props
  const other = parseLogOther(log.other ?? '')
  const affinity = other?.admin_info?.channel_affinity
  const upstream = other?.upstream_model_name
  const facts: { label: string; value: string }[] = []
  if (log.group) facts.push({ label: t('Group'), value: log.group })
  if (upstream && upstream !== log.model_name) {
    facts.push({ label: t('Mapped model'), value: upstream })
  }
  if (affinity?.rule_name) {
    facts.push({ label: t('Channel Affinity'), value: affinity.rule_name })
  }
  if (affinity?.selected_group) {
    facts.push({ label: t('Selected group'), value: affinity.selected_group })
  }
  const attempts = [...view.attempts].sort((a, b) => a.attempt - b.attempt)
  return (
    <div className='space-y-3'>
      {facts.length > 0 && (
        <dl
          data-testid='log-content-routing-facts'
          className='grid grid-cols-2 gap-3 sm:grid-cols-4'
        >
          {facts.map((fact) => (
            <Fact key={fact.label} {...fact} />
          ))}
        </dl>
      )}
      <ol className='space-y-2'>
        {attempts.map((attempt) => (
          <AttemptRow
            key={attempt.attempt}
            attempt={attempt}
            sensitiveVisible={sensitiveVisible}
          />
        ))}
      </ol>
    </div>
  )
}
