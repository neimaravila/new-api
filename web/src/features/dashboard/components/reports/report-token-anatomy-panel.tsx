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
import type { ReportTokenAnatomyRow } from '../../types'

interface ReportTokenAnatomyPanelProps {
  loading: boolean
  rows: ReportTokenAnatomyRow[]
}

export function ReportTokenAnatomyPanel(props: ReportTokenAnatomyPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const max = Math.max(1, ...props.rows.map((r) => r.total))

  return (
    <PanelShell
      title={t('Token anatomy')}
      description={t('Prompt, completion and cache tokens per model')}
    >
      {props.rows.length === 0 ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No token data in this period.')} />
      ) : (
        <div className='grid gap-2'>
          {props.rows.map((r) => (
            <BarChartRow
              key={r.model_name}
              label={r.model_name}
              value={r.total}
              maxValue={max}
              displayValue={String(r.total)}
              tone='accent-2'
              sublabel={`${t('prompt')} ${r.prompt_tokens} · ${t('completion')} ${r.completion_tokens} · ${t('cache')} ${r.cache_tokens}`}
            />
          ))}
        </div>
      )}
    </PanelShell>
  )
}
