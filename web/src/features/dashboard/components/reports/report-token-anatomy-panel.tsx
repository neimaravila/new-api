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
import { buildTokenAnatomySpec } from '../../lib/report-charts'
import type { ReportTokenAnatomyRow } from '../../types'

interface ReportTokenAnatomyPanelProps {
  loading: boolean
  rows: ReportTokenAnatomyRow[]
}

export function ReportTokenAnatomyPanel(props: ReportTokenAnatomyPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const promptLabel = t('Prompt')
  const completionLabel = t('Completion')
  const cacheLabel = t('Cache')
  const spec = useMemo(
    () =>
      buildTokenAnatomySpec(props.rows, {
        prompt: promptLabel,
        completion: completionLabel,
        cache: cacheLabel,
      }),
    [props.rows, promptLabel, completionLabel, cacheLabel]
  )
  const columns: ReportTableColumn<ReportTokenAnatomyRow>[] = [
    { key: 'model', header: t('Model'), render: (row) => row.model_name },
    { key: 'prompt', header: promptLabel, align: 'end', render: (row) => formatNumber(row.prompt_tokens) },
    {
      key: 'completion',
      header: completionLabel,
      align: 'end',
      render: (row) => formatNumber(row.completion_tokens),
    },
    { key: 'cache', header: cacheLabel, align: 'end', render: (row) => formatNumber(row.cache_tokens) },
    { key: 'total', header: t('Total'), align: 'end', render: (row) => formatNumber(row.total) },
  ]

  return (
    <PanelShell
      title={t('Token anatomy')}
      description={t('Prompt, completion and cache tokens per model')}
    >
      {spec === null ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No token data in this period.')} />
      ) : (
        <>
          <ReportChart spec={spec} ariaLabel={t('Token anatomy')} />
          <ReportTable
            caption={t('Token anatomy')}
            columns={columns}
            rows={props.rows}
            rowKey={(row) => row.model_name}
          />
        </>
      )}
    </PanelShell>
  )
}
