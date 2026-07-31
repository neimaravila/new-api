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
import { buildErrorShareSpec } from '../../lib/report-charts'
import type { ReportErrorRow } from '../../types'

interface ReportErrorsPanelProps {
  loading: boolean
  errors: ReportErrorRow[]
}

export function ReportErrorsPanel(props: ReportErrorsPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const otherLabel = t('Other')
  const spec = useMemo(() => buildErrorShareSpec(props.errors, otherLabel), [props.errors, otherLabel])
  const columns: ReportTableColumn<ReportErrorRow>[] = [
    { key: 'model', header: t('Model'), render: (row) => row.model_name },
    { key: 'failures', header: t('Failures'), align: 'end', render: (row) => formatNumber(row.failures) },
    { key: 'share', header: t('Share'), align: 'end', render: (row) => `${Math.round(row.share * 10) / 10}%` },
  ]

  return (
    <PanelShell title={t('Failures by model')} description={t('Where the errors in this period came from')}>
      {spec === null ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No failures in this period.')} />
      ) : (
        <>
          <ReportChart spec={spec} ariaLabel={t('Failures by model')} />
          <ReportTable
            caption={t('Failures by model')}
            columns={columns}
            rows={props.errors}
            rowKey={(row) => row.model_name}
          />
        </>
      )}
    </PanelShell>
  )
}
