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

import { BarChartRow, EmptyOrLoading, PanelShell } from './report-primitives'
import type { ReportModelRow } from '../../types'

interface ReportModelPanelProps {
  loading: boolean
  models: ReportModelRow[]
}

export function ReportModelPanel(props: ReportModelPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const max = Math.max(1, ...props.models.map((m) => m.quota))

  return (
    <PanelShell
      title={t('Model breakdown')}
      description={t('Cost, requests, tokens, error rate and latency per model')}
    >
      {props.models.length === 0 ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No model usage in this period.')} />
      ) : (
        <div className='grid gap-2'>
          {props.models.map((m) => (
            <BarChartRow
              key={m.model_name}
              label={m.model_name}
              value={m.quota}
              maxValue={max}
              displayValue={String(m.quota)}
              tone='accent-2'
              sublabel={`${m.requests} ${t('requests')} · ${Math.round(m.error_rate * 1000) / 10}% ${t('errors')} · ${m.avg_latency_ms} ms`}
            />
          ))}
        </div>
      )}
    </PanelShell>
  )
}
