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
  isError?: boolean
  /** Key figures for the period, shown above the chart. */
  figures?: { label: string; value: string }[]
  emptyText?: string
  /** Shows the bar/line selector. */
  toggleable?: boolean
  defaultKind?: ReportChartKind
  /** Controlled kind: lets the parent keep it when this card unmounts. */
  kind?: ReportChartKind
  onKindChange?: (kind: ReportChartKind) => void
  children: (chartKind: ReportChartKind) => React.ReactNode
}

const KIND_OPTIONS: { value: ReportChartKind; labelKey: string }[] = [
  { value: 'bar', labelKey: 'Bar' },
  { value: 'line', labelKey: 'Line' },
]

/** One chart of the Overview grid, on the same shell as every other panel. */
export function ReportChartCard(props: ReportChartCardProps): React.JSX.Element {
  const { t } = useTranslation()
  const [localKind, setLocalKind] = useState<ReportChartKind>(props.defaultKind ?? 'bar')
  const chartKind = props.kind ?? localKind
  const setChartKind = (next: ReportChartKind) => {
    setLocalKind(next)
    props.onKindChange?.(next)
  }

  let body: React.ReactNode
  if (props.isError) {
    body = (
      <div role='alert' className='text-muted-foreground rounded-xl border border-dashed p-4 text-sm'>
        {t('This chart could not be loaded.')}
      </div>
    )
  } else if (props.loading || props.isEmpty) {
    body = (
      <EmptyOrLoading
        loading={props.loading}
        emptyText={props.emptyText ?? t('No data in this period.')}
      />
    )
  } else {
    body = (
      <>
        {props.figures && props.figures.length > 0 && (
          <dl className='mb-3 flex flex-wrap gap-x-6 gap-y-2'>
            {props.figures.map((figure) => (
              <div key={figure.label}>
                <dt className='text-muted-foreground text-xs font-medium'>{figure.label}</dt>
                <dd className='font-mono text-base font-semibold tabular-nums'>{figure.value}</dd>
              </div>
            ))}
          </dl>
        )}
        {props.children(chartKind)}
      </>
    )
  }

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
      {body}
    </PanelShell>
  )
}
