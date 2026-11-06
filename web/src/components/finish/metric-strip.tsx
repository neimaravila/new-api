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
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

import { splitUnit } from './split-unit'
import { TONE_TEXT_CLASS, type TintTone } from './tone'

export type MetricItem = {
  key: string
  label: string
  value: string
  sub?: ReactNode
  testId?: string
  mono?: boolean
  tone?: TintTone
}

const CELL_SIZE = {
  md: 'px-3 py-2.5',
  lg: 'px-[18px] py-4',
} as const

const VALUE_SIZE = {
  md: 'text-[18px] font-semibold',
  lg: 'text-2xl leading-8 font-semibold tracking-[-0.02em]',
} as const

export function MetricStrip(props: {
  items: MetricItem[]
  size: 'md' | 'lg'
  columnsClassName: string
  'data-testid'?: string
}) {
  return (
    <div
      data-slot='metric-strip'
      data-testid={props['data-testid']}
      className={cn(
        'grid overflow-hidden rounded-(--vf-radius-inner) border bg-(--vf-surface)',
        props.columnsClassName
      )}
    >
      {props.items.map((item) => {
        const { value, unit } = splitUnit(item.value)
        return (
          <div
            key={item.key}
            data-slot='metric-cell'
            className={cn(
              'min-w-0 shadow-[1px_0_0_var(--vf-divider),0_1px_0_var(--vf-divider)]',
              CELL_SIZE[props.size]
            )}
          >
            <div className='text-muted-foreground truncate text-[11px] font-medium tracking-[0.05em] uppercase'>
              {item.label}
            </div>
            <span
              data-slot='metric-value'
              data-testid={item.testId}
              className={cn(
                'block truncate tabular-nums',
                VALUE_SIZE[props.size],
                item.mono && 'font-mono',
                item.tone && TONE_TEXT_CLASS[item.tone]
              )}
            >
              {value}
              {unit && (
                <span
                  data-slot='metric-unit'
                  className='text-muted-foreground text-[0.6em]'
                >
                  {unit}
                </span>
              )}
            </span>
            {item.sub != null && (
              <div className='text-muted-foreground truncate text-[11px]'>
                {item.sub}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
