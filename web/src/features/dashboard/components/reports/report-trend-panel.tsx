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

import { formatNumber, formatQuota } from '@/lib/format'

import { BarChartRow, EmptyOrLoading, PanelShell } from './report-primitives'
import type { ReportTrendPoint } from '../../types'

interface ReportTrendPanelProps {
  loading: boolean
  points: ReportTrendPoint[]
}

export function ReportTrendPanel(props: ReportTrendPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const max = Math.max(1, ...props.points.map((p) => p.quota))

  return (
    <PanelShell
      title={t('Trend over time')}
      description={t('Cost, requests, tokens, failures and latency per bucket')}
    >
      {props.points.length === 0 ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No trend data in this period.')} />
      ) : (
        <div className='grid gap-2'>
          {props.points.map((p) => (
            <BarChartRow
              key={p.bucket_timestamp}
              label={p.bucket_label}
              value={p.quota}
              maxValue={max}
              displayValue={formatQuota(p.quota)}
              tone='accent-1'
              sublabel={`${formatNumber(p.requests)} ${t('requests')} · ${formatNumber(p.failures)} ${t('failures')}`}
            />
          ))}
        </div>
      )}
    </PanelShell>
  )
}
