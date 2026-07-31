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
import { buildStreamSpec } from '../../lib/report-charts'
import type { ReportStreamComparison } from '../../types'

interface ReportStreamPanelProps {
  loading: boolean
  stream: ReportStreamComparison
}

interface StreamModeRow {
  mode: string
  requests: number
  avgLatencyMs: number
}

export function ReportStreamPanel(props: ReportStreamPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const streamLabel = t('Streaming')
  const nonStreamLabel = t('Non-streaming')
  const spec = useMemo(
    () => buildStreamSpec(props.stream, streamLabel, nonStreamLabel),
    [props.stream, streamLabel, nonStreamLabel]
  )
  const rows: StreamModeRow[] = [
    {
      mode: streamLabel,
      requests: props.stream.stream_requests,
      avgLatencyMs: props.stream.stream_avg_latency_ms,
    },
    {
      mode: nonStreamLabel,
      requests: props.stream.non_stream_requests,
      avgLatencyMs: props.stream.non_stream_avg_latency_ms,
    },
  ]
  const columns: ReportTableColumn<StreamModeRow>[] = [
    { key: 'mode', header: t('Mode'), render: (row) => row.mode },
    { key: 'requests', header: t('Requests'), align: 'end', render: (row) => formatNumber(row.requests) },
    {
      key: 'latency',
      header: t('Avg latency'),
      align: 'end',
      render: (row) => `${formatNumber(row.avgLatencyMs)} ms`,
    },
  ]

  return (
    <PanelShell
      title={t('Stream vs non-stream')}
      description={t('Average latency and request volume by streaming mode')}
    >
      {spec === null ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No stream comparison data in this period.')} />
      ) : (
        <>
          <ReportChart spec={spec} ariaLabel={t('Stream vs non-stream requests')} />
          <ReportTable
            caption={t('Stream vs non-stream')}
            columns={columns}
            rows={rows}
            rowKey={(row) => row.mode}
          />
        </>
      )}
    </PanelShell>
  )
}
