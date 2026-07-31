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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { formatNumber } from '@/lib/format'

import { EmptyOrLoading, PanelShell } from './report-primitives'
import { ReportChart } from './report-chart'
import { ReportTable, type ReportTableColumn } from './report-table'
import { buildPerformanceSpec } from '../../lib/report-charts'
import type { ReportPerformanceRow } from '../../types'

interface ReportPerformancePanelProps {
  loading: boolean
  title: string
  rows: ReportPerformanceRow[]
}

export function ReportPerformancePanel(props: ReportPerformancePanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const avgLabel = t('Avg latency')
  const p95Label = t('p95 latency')
  const spec = useMemo(
    () => buildPerformanceSpec(props.rows, avgLabel, p95Label),
    [props.rows, avgLabel, p95Label]
  )
  const columns: ReportTableColumn<ReportPerformanceRow>[] = [
    { key: 'name', header: t('Name'), render: (row) => row.name },
    { key: 'avg', header: avgLabel, align: 'end', render: (row) => `${formatNumber(row.avg_latency_ms)} ms` },
    { key: 'p95', header: p95Label, align: 'end', render: (row) => `${formatNumber(row.p95_latency_ms)} ms` },
    { key: 'requests', header: t('Requests'), align: 'end', render: (row) => formatNumber(row.requests) },
    {
      key: 'throughput',
      header: t('Throughput'),
      align: 'end',
      render: (row) => `${formatNumber(row.throughput)} tok/s`,
    },
  ]

  return (
    <PanelShell
      title={props.title}
      description={t('Average and p95 latency, plus throughput in tokens per second')}
    >
      {spec === null ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No performance data in this period.')} />
      ) : (
        <>
          <ReportChart spec={spec} ariaLabel={props.title} />
          <ReportTable
            caption={props.title}
            columns={columns}
            rows={props.rows}
            rowKey={(row) => row.name}
          />
        </>
      )}
    </PanelShell>
  )
}
