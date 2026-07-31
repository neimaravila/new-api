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

import { EmptyOrLoading, PanelShell } from './report-primitives'
import { ReportChart } from './report-chart'
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

export function ReportTrendPanel(props: ReportTrendPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const [metric, setMetric] = useState<ReportTrendMetric>('quota')
  const metricLabel = t(METRIC_OPTIONS.find((option) => option.value === metric)?.labelKey ?? 'Cost')
  const spec = useMemo(
    () => buildTrendSpec(props.points, metric, metricLabel),
    [props.points, metric, metricLabel]
  )

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
        <ReportChart spec={spec} height={280} ariaLabel={t('Usage over time')} />
      )}
    </PanelShell>
  )
}
