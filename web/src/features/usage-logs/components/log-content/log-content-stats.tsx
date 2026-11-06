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

import { MetricStrip, type MetricItem } from '@/components/finish'
import {
  formatLatencyMs,
  formatLogQuota,
  formatNumber,
  formatUseTime,
} from '@/lib/format'

import type { LogContentView } from '../../data/log-content-schema'
import type { UsageLog } from '../../data/schema'
import { parseLogOther } from '../../lib/format'

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
  const tokenSub = [
    `${t('Output Tokens')}: ${formatNumber(log.completion_tokens ?? 0)}`,
    (other?.cache_tokens ?? 0) > 0 &&
      `${t('Cache Read')}: ${formatNumber(other?.cache_tokens ?? 0)}`,
    cacheWrite > 0 && `${t('Cache Write')}: ${formatNumber(cacheWrite)}`,
  ]
    .filter(Boolean)
    .join(' · ')
  const items: MetricItem[] = [
    {
      key: 'latency',
      label: t('Latency'),
      value: formatUseTime(log.use_time ?? 0),
      sub: `${t('First token')}: ${formatLatencyMs(other?.frt)}`,
      tone: log.type === 2 ? 'success' : undefined,
    },
    {
      key: 'model',
      label: t('Model'),
      value: log.model_name || '-',
      mono: true,
      sub: `${t('Group')}: ${log.group || other?.group || '-'}`,
    },
    {
      key: 'tokens',
      label: t('Input Tokens'),
      value: formatNumber(log.prompt_tokens ?? 0),
      sub: tokenSub,
    },
    {
      key: 'cost',
      label: t('Cost'),
      value: props.sensitiveVisible ? formatLogQuota(log.quota ?? 0) : '••••',
    },
    {
      key: 'tools',
      label: t('Tools'),
      value: countOrDash(view?.tools_available),
      sub: `${t('Calls')}: ${countOrDash(view?.tool_calls)}`,
    },
  ]
  return (
    <MetricStrip
      items={items}
      size='md'
      columnsClassName='grid-cols-2 sm:grid-cols-3 lg:grid-cols-5'
      data-testid='log-content-stats'
    />
  )
}
