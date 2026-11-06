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
import { flexRender } from '@tanstack/react-table'
import { cleanup, render, renderHook, screen } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'

import { useCommonLogsColumns } from '../columns/common-logs-columns'

afterEach(cleanup)

function statusColumn() {
  const { result } = renderHook(() => useCommonLogsColumns(false, false))
  const column = result.current.find((c) => c.id === 'status')
  if (!column) throw new Error('status column missing')
  return column
}

function renderCell(type: number) {
  const column = statusColumn()
  const cell = column.cell as (ctx: unknown) => React.ReactNode
  render(<>{cell({ row: { original: { type } } })}</>)
}

it('shows a status bar named OK for a consume log', () => {
  renderCell(2)
  expect(screen.getByRole('img', { name: 'OK' })).toHaveAttribute(
    'data-tone',
    'success'
  )
  expect(screen.queryByText('OK')).toBeNull()
})

it('shows a status bar named Error for an error log', () => {
  renderCell(5)
  expect(screen.getByRole('img', { name: 'Error' })).toHaveAttribute(
    'data-tone',
    'danger'
  )
})

it('keeps a visually hidden Status header', () => {
  const header = statusColumn().header as () => React.ReactNode
  render(<>{flexRender(header, {} as never)}</>)
  const label = screen.getByText('Status')
  expect(label).toHaveClass('sr-only')
})
