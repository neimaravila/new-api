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
import type { ReportErrorRow } from '../../types'

interface ReportErrorsPanelProps {
  loading: boolean
  errors: ReportErrorRow[]
}

export function ReportErrorsPanel(props: ReportErrorsPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const max = Math.max(1, ...props.errors.map((e) => e.failures))

  return (
    <PanelShell
      title={t('Error analysis')}
      description={t('Top models by failure count and their share of total failures')}
    >
      {props.errors.length === 0 ? (
        <EmptyOrLoading loading={false} emptyText={t('No errors recorded in this period.')} />
      ) : (
        <div className='grid gap-2'>
          {props.errors.map((e) => (
            <BarChartRow
              key={e.model_name}
              label={e.model_name}
              value={e.failures}
              maxValue={max}
              displayValue={formatNumber(e.failures)}
              tone='accent-3'
              sublabel={`${e.share}% ${t('share')}`}
            />
          ))}
        </div>
      )}
    </PanelShell>
  )
}
