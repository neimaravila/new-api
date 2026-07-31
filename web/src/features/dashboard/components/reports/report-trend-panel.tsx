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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { formatNumber, formatQuota } from '@/lib/format'

import { EmptyOrLoading, PanelShell } from './report-primitives'
import { ReportChart } from './report-chart'
import { ReportTable, type ReportTableColumn } from './report-table'
import { buildTrendSpec, type ReportTrendMetric } from '../../lib/report-charts'
import type { ReportTrendPoint } from '../../types'

interface ReportTrendPanelProps {
  loading: boolean
  points: ReportTrendPoint[]
}

const METRIC_OPTIONS: { value: ReportTrendMetric; labelKey: string }[] = [
  { value: 'quota', labelKey: 'Cost' },
  { value: 'requests', labelKey: 'Requests' },
  { value: 'tokens', labelKey: 'Tokens' },
  { value: 'failures', labelKey: 'Failures' },
]

/**
 * The table exists so the trend is readable as text, not to mirror the whole
 * series: a 30-day hourly range returns ~720 buckets. Every daily range and the
 * hourly today/yesterday ranges fit under this cap in full; longer hourly ranges
 * show the most recent buckets and say so, with the CSV export for the rest.
 */
export const TREND_TABLE_MAX_ROWS = 48

export function ReportTrendPanel(props: ReportTrendPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const [metric, setMetric] = useState<ReportTrendMetric>('quota')
  const metricLabel = t(METRIC_OPTIONS.find((option) => option.value === metric)?.labelKey ?? 'Cost')
  const spec = useMemo(
    () => buildTrendSpec(props.points, metric, metricLabel),
    [props.points, metric, metricLabel]
  )
  const chartLabel = t('Usage over time: {{metric}}', { metric: metricLabel })
  const tableRows = props.points.slice(-TREND_TABLE_MAX_ROWS)
  const columns: ReportTableColumn<ReportTrendPoint>[] = [
    { key: 'bucket', header: t('Period'), render: (row) => row.bucket_label },
    {
      key: 'value',
      header: metricLabel,
      align: 'end',
      render: (row) => (metric === 'quota' ? formatQuota(row.quota) : formatNumber(row[metric])),
    },
  ]

  return (
    <PanelShell
      title={t('Usage over time')}
      description={t('One measure at a time, across the selected period')}
      actions={
        <div className='flex flex-wrap gap-1'>
          {METRIC_OPTIONS.map((option) => (
            <Button
              key={option.value}
              size='sm'
              variant={metric === option.value ? 'default' : 'outline'}
              onClick={() => setMetric(option.value)}
            >
              {t(option.labelKey)}
            </Button>
          ))}
        </div>
      }
    >
      {spec === null ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No usage in this period.')} />
      ) : (
        <>
          <ReportChart spec={spec} height={280} ariaLabel={chartLabel} />
          <ReportTable
            caption={chartLabel}
            columns={columns}
            rows={tableRows}
            rowKey={(row) => String(row.bucket_timestamp)}
          />
          {props.points.length > TREND_TABLE_MAX_ROWS && (
            <p className='text-muted-foreground mt-2 text-xs'>
              {t(
                'Showing the most recent {{shown}} of {{total}} periods. Export the CSV for the full series.',
                { shown: TREND_TABLE_MAX_ROWS, total: props.points.length }
              )}
            </p>
          )}
        </>
      )}
    </PanelShell>
  )
}
