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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  formatLatencyMs,
  formatLogQuota,
  formatNumber,
  formatUseTime,
} from '@/lib/format'

import type { LogContentView } from '../../data/log-content-schema'
import type { UsageLog } from '../../data/schema'
import { parseLogOther } from '../../lib/format'

function Cell(props: { label: string; value: string; children?: ReactNode }) {
  return (
    <div className='bg-muted/30 min-w-0 rounded-md border px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='truncate font-mono text-sm font-medium'>
        {props.value}
      </div>
      {props.children != null && (
        <div className='text-muted-foreground mt-0.5 text-xs'>
          {props.children}
        </div>
      )}
    </div>
  )
}

function countOrDash(value: number | null | undefined): string {
  return value == null ? '-' : formatNumber(value)
}

/** The five figures of a call: latency, model, tokens, cost and tools. */
export function LogContentStats(props: {
  log: UsageLog
  view: LogContentView | undefined
  sensitiveVisible: boolean
}) {
  const { t } = useTranslation()
  const { log, view } = props
  const other = parseLogOther(log.other ?? '')
  const cacheWrite =
    (other?.cache_creation_tokens_5m ?? 0) +
      (other?.cache_creation_tokens_1h ?? 0) ||
    other?.cache_creation_tokens ||
    0
  return (
    <div
      className='grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5'
      data-testid='log-content-stats'
    >
      <Cell label={t('Latency')} value={formatUseTime(log.use_time ?? 0)}>
        {t('First token')}: {formatLatencyMs(other?.frt)}
      </Cell>
      <Cell label={t('Model')} value={log.model_name || '-'}>
        {t('Group')}: {log.group || other?.group || '-'}
      </Cell>
      <Cell
        label={t('Input Tokens')}
        value={formatNumber(log.prompt_tokens ?? 0)}
      >
        <span className='block'>
          {t('Output Tokens')}: {formatNumber(log.completion_tokens ?? 0)}
        </span>
        <span className='block'>
          {t('Cache Read')}: {formatNumber(other?.cache_tokens ?? 0)}
        </span>
        <span className='block'>
          {t('Cache Write')}: {formatNumber(cacheWrite)}
        </span>
      </Cell>
      <Cell
        label={t('Cost')}
        value={props.sensitiveVisible ? formatLogQuota(log.quota ?? 0) : '••••'}
      />
      <Cell label={t('Tools')} value={countOrDash(view?.tools_available)}>
        {t('Calls')}: {countOrDash(view?.tool_calls)}
      </Cell>
    </div>
  )
}
