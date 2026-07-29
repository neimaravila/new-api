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

import { formatNumber } from '@/lib/format'

import { BarChartRow, EmptyOrLoading, PanelShell } from './report-primitives'
import type { ReportPerformanceRow } from '../../types'

interface ReportPerformancePanelProps {
  loading: boolean
  title: string
  rows: ReportPerformanceRow[]
}

export function ReportPerformancePanel(props: ReportPerformancePanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const max = Math.max(1, ...props.rows.map((r) => r.p95_latency_ms))

  return (
    <PanelShell
      title={props.title}
      description={t('Average and p95 latency, plus throughput in tokens per second')}
    >
      {props.rows.length === 0 ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No performance data in this period.')} />
      ) : (
        <div className='grid gap-2'>
          {props.rows.map((r) => (
            <BarChartRow
              key={r.name}
              label={r.name}
              value={r.p95_latency_ms}
              maxValue={max}
              displayValue={`p95 ${formatNumber(r.p95_latency_ms)} ms`}
              tone='accent-1'
              sublabel={`${t('avg')} ${formatNumber(r.avg_latency_ms)} ms · ${formatNumber(r.throughput)} ${t('tok/s')}`}
            />
          ))}
        </div>
      )}
    </PanelShell>
  )
}
