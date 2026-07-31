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
}

interface ReportTableProps<T> {
  columns: ReportTableColumn<T>[]
  rows: T[]
  rowKey: (row: T) => string
  caption: string
}

export function ReportTable<T>(props: ReportTableProps<T>): React.JSX.Element {
  return (
    <div className='mt-3 overflow-x-auto'>
      <table className='w-full text-xs'>
        <caption className='sr-only'>{props.caption}</caption>
        <thead>
          <tr className='text-muted-foreground border-b'>
            {props.columns.map((column) => (
              <th
                key={column.key}
                scope='col'
                className={cn('py-1.5 font-medium', column.align === 'end' ? 'text-right' : 'text-left')}
              >
                {column.header}
              </th>
            ))}
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
