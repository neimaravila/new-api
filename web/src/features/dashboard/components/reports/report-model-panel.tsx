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

import { formatNumber, formatQuota } from '@/lib/format'

import { EmptyOrLoading, PanelShell } from './report-primitives'
import { ReportChart } from './report-chart'
import { ReportTable, type ReportTableColumn } from './report-table'
import { buildModelCostSpec } from '../../lib/report-charts'
import type { ReportModelRow } from '../../types'

interface ReportModelPanelProps {
  loading: boolean
  models: ReportModelRow[]
}

export function ReportModelPanel(props: ReportModelPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const spec = useMemo(() => buildModelCostSpec(props.models), [props.models])
  const columns: ReportTableColumn<ReportModelRow>[] = [
    { key: 'model', header: t('Model'), render: (row) => row.model_name },
    { key: 'cost', header: t('Cost'), align: 'end', render: (row) => formatQuota(row.quota) },
    { key: 'requests', header: t('Requests'), align: 'end', render: (row) => formatNumber(row.requests) },
    {
      key: 'errors',
      header: t('Error rate'),
      align: 'end',
      render: (row) => `${Math.round(row.error_rate * 1000) / 10}%`,
    },
    {
      key: 'latency',
      header: t('Avg latency'),
      align: 'end',
      render: (row) => `${formatNumber(row.avg_latency_ms)} ms`,
    },
  ]

  return (
    <PanelShell title={t('Model breakdown')} description={t('Cost per model, with requests, errors and latency')}>
      {spec === null ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No model usage in this period.')} />
      ) : (
        <>
          <ReportChart spec={spec} ariaLabel={t('Cost per model')} />
          <ReportTable
            caption={t('Model breakdown')}
            columns={columns}
            rows={props.models}
            rowKey={(row) => row.model_name}
          />
        </>
      )}
    </PanelShell>
  )
}
