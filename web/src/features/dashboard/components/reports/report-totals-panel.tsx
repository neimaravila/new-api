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

import { formatCompactNumber, formatNumber, formatQuota } from '@/lib/format'

import { PanelShell } from './report-primitives'
import type { ReportSummary } from '../../types'

interface ReportTotalsPanelProps {
  loading: boolean
  summary: ReportSummary
}

interface TotalsTile {
  key: string
  label: string
  value: string
  sublabel?: string
}

export function ReportTotalsPanel(props: ReportTotalsPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const totals = props.summary.trend.totals
  const failureRate =
    totals.requests > 0 ? Math.round((totals.failures / totals.requests) * 1000) / 10 : null

  const tiles: TotalsTile[] = [
    { key: 'quota', label: t('Quota used'), value: formatQuota(totals.quota) },
    { key: 'requests', label: t('Requests'), value: formatNumber(totals.requests) },
    { key: 'tokens', label: t('Tokens'), value: formatCompactNumber(totals.tokens) },
    {
      key: 'failures',
      label: t('Failures'),
      value: formatNumber(totals.failures),
      sublabel: failureRate === null ? undefined : `${failureRate}%`,
    },
  ]

  return (
    <PanelShell
      title={t('Period totals')}
      description={t('Aggregate volume across the selected period')}
      isEmpty={props.loading}
      emptyText={t('Loading…')}
    >
      <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
        {tiles.map((tile) => (
          <div key={tile.key} className='bg-muted/40 rounded-xl border p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{tile.label}</div>
            <div className='mt-1 font-mono text-lg font-semibold tabular-nums sm:text-xl'>
              {tile.value}
            </div>
            {tile.sublabel && (
              <div className='text-muted-foreground mt-0.5 text-xs'>{tile.sublabel}</div>
            )}
          </div>
        ))}
      </div>
    </PanelShell>
  )
}
