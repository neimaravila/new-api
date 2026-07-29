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

import { cn } from '@/lib/utils'

interface BarChartRowProps {
  label: string
  value: number
  maxValue: number
  displayValue: string
  tone?: 'accent-1' | 'accent-2' | 'accent-3'
  sublabel?: string
}

const toneClass = {
  'accent-1': 'bg-primary',
  'accent-2': 'bg-success',
  'accent-3': 'bg-warning',
} as const

export function BarChartRow(props: BarChartRowProps): React.JSX.Element {
  const pct = props.maxValue > 0 ? Math.max(2, (props.value / props.maxValue) * 100) : 0
  return (
    <div className='flex items-center gap-3'>
      <div className='text-muted-foreground w-32 shrink-0 truncate text-xs' title={props.label}>
        {props.label}
      </div>
      <div className='bg-muted relative h-5 flex-1 overflow-hidden rounded-md'>
        <div
          className={cn('h-full rounded-md', toneClass[props.tone ?? 'accent-1'])}
          style={{ width: `${pct}%` }}
        />
      </div>
      <div className='w-20 shrink-0 text-right font-mono text-xs font-semibold tabular-nums'>
        {props.displayValue}
      </div>
      {props.sublabel && (
        <div className='text-muted-foreground hidden w-24 shrink-0 truncate text-right text-xs sm:block'>
          {props.sublabel}
        </div>
      )}
    </div>
  )
}

interface PanelShellProps {
  title: string
  description?: string
  actions?: React.ReactNode
  children: React.ReactNode
  emptyText?: string
  isEmpty?: boolean
}

export function PanelShell(props: PanelShellProps): React.JSX.Element {
  return (
    <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
      <div className='mb-4 flex flex-wrap items-start justify-between gap-2'>
        <div className='min-w-0'>
          <h3 className='text-sm font-semibold sm:text-base'>{props.title}</h3>
          {props.description && (
            <p className='text-muted-foreground text-xs sm:text-sm'>{props.description}</p>
          )}
        </div>
        {props.actions}
      </div>
      {props.isEmpty ? (
        <div className='text-muted-foreground rounded-xl border border-dashed p-4 text-sm'>
          {props.emptyText}
        </div>
      ) : (
        props.children
      )}
    </section>
  )
}

interface EmptyOrLoadingProps {
  loading: boolean
  emptyText: string
  loadingText?: string
}

export function EmptyOrLoading(props: EmptyOrLoadingProps): React.JSX.Element {
  const { t } = useTranslation()
  if (props.loading) {
    return <div className='text-muted-foreground p-4 text-sm'>{props.loadingText ?? t('Loading…')}</div>
  }
  return (
    <div className='text-muted-foreground rounded-xl border border-dashed p-4 text-sm'>
      {props.emptyText}
    </div>
  )
}
