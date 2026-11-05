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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import type { ReportChartKind } from '../../lib/report-charts'
import { EmptyOrLoading, PanelShell } from './report-primitives'

interface ReportChartCardProps {
  title: string
  summary: string
  loading: boolean
  isEmpty: boolean
  emptyText?: string
  /** Shows the bar/line selector. */
  toggleable?: boolean
  defaultKind?: ReportChartKind
  children: (chartKind: ReportChartKind) => React.ReactNode
}

const KIND_OPTIONS: { value: ReportChartKind; labelKey: string }[] = [
  { value: 'bar', labelKey: 'Bar' },
  { value: 'line', labelKey: 'Line' },
]

/** One chart of the Overview grid, on the same shell as every other panel. */
export function ReportChartCard(props: ReportChartCardProps): React.JSX.Element {
  const { t } = useTranslation()
  const [chartKind, setChartKind] = useState<ReportChartKind>(props.defaultKind ?? 'bar')

  return (
    <PanelShell
      title={props.title}
      description={props.summary}
      actions={
        props.toggleable ? (
          <div className='flex gap-1' role='group' aria-label={t('Chart type')}>
            {KIND_OPTIONS.map((option) => (
              <Button
                key={option.value}
                size='sm'
                variant={chartKind === option.value ? 'default' : 'outline'}
                aria-pressed={chartKind === option.value}
                onClick={() => setChartKind(option.value)}
              >
                {t(option.labelKey)}
              </Button>
            ))}
          </div>
        ) : undefined
      }
    >
      {props.loading || props.isEmpty ? (
        <EmptyOrLoading
          loading={props.loading}
          emptyText={props.emptyText ?? t('No data in this period.')}
        />
      ) : (
        props.children(chartKind)
      )}
    </PanelShell>
  )
}
