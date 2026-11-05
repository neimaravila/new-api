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
import { cn } from '@/lib/utils'

export interface ReportTableColumn<T> {
  key: string
  header: string
  align?: 'start' | 'end'
  render: (row: T) => React.ReactNode
  /** Only takes effect when the table also receives `onSortChange`. */
  sortable?: boolean
}

export interface ReportTableSort {
  key: string
  direction: 'asc' | 'desc'
}

interface ReportTableProps<T> {
  columns: ReportTableColumn<T>[]
  rows: T[]
  rowKey: (row: T) => string
  caption: string
  sort?: ReportTableSort
  onSortChange?: (key: string) => void
}

function sortAttribute(active: boolean, direction?: 'asc' | 'desc'): 'none' | 'ascending' | 'descending' {
  if (!active) return 'none'
  return direction === 'asc' ? 'ascending' : 'descending'
}

function sortArrow(active: boolean, direction?: 'asc' | 'desc'): string {
  if (!active) return ''
  return direction === 'asc' ? '↑' : '↓'
}

export function ReportTable<T>(props: ReportTableProps<T>): React.JSX.Element {
  return (
    <div className='mt-3 overflow-x-auto'>
      <table className='w-full text-xs'>
        <caption className='sr-only'>{props.caption}</caption>
        <thead>
          <tr className='text-muted-foreground border-b'>
            {props.columns.map((column) => {
              const sortable = Boolean(column.sortable && props.onSortChange)
              const active = sortable && props.sort?.key === column.key
              const ariaSort = sortable ? sortAttribute(active, props.sort?.direction) : undefined
              return (
                <th
                  key={column.key}
                  scope='col'
                  aria-sort={ariaSort}
                  className={cn('py-1.5 font-medium', column.align === 'end' ? 'text-right' : 'text-left')}
                >
                  {sortable ? (
                    <button
                      type='button'
                      className='hover:text-foreground inline-flex items-center gap-1'
                      onClick={() => props.onSortChange?.(column.key)}
                    >
                      {column.header}
                      <span aria-hidden='true'>{sortArrow(active, props.sort?.direction)}</span>
                    </button>
                  ) : (
                    column.header
                  )}
                </th>
              )
            })}
          </tr>
        </thead>
        <tbody>
          {props.rows.map((row) => (
            <tr key={props.rowKey(row)} className='border-b last:border-0'>
              {props.columns.map((column) => (
                <td
                  key={column.key}
                  className={cn(
                    'py-1.5',
                    column.align === 'end' ? 'text-right font-mono tabular-nums' : 'text-left'
                  )}
                >
                  {column.render(row)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
