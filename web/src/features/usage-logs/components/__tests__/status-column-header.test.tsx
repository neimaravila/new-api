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
import {
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from '@tanstack/react-table'
import { render, renderHook, screen } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { expect, it } from 'vitest'

import { DataTableView } from '@/components/data-table'

import type { UsageLog } from '../../data/schema'
import { useCommonLogsColumns } from '../columns/common-logs-columns'

function LogsTable(props: { columns: ColumnDef<UsageLog>[] }) {
  const table = useReactTable({
    data: [],
    columns: props.columns,
    getCoreRowModel: getCoreRowModel(),
  })
  return <DataTableView table={table} applyHeaderSize />
}

function renderTable() {
  const { result } = renderHook(() => useCommonLogsColumns(false, false))
  render(<LogsTable columns={result.current} />)
  return screen.getByRole('columnheader', { name: 'Status' })
}

it('renders the Status header as plain text so the table shows its name', () => {
  const header = renderTable()
  expect(header).toHaveAttribute('data-column-id', 'status')
  expect(header).toHaveTextContent('Status')
})

it('sizes the Status column at 28px', () => {
  expect(renderTable().style.width).toBe('28px')
})

it('hides the Status header text visually inside the logs scope', () => {
  const css = readFileSync(
    resolve(__dirname, '../../../../styles/visual-finish.css'),
    'utf8'
  )
  const rule = /\[data-visual-finish='logs'\] th\[data-column-id='status'\][^{]*\{([^}]*)\}/.exec(
    css
  )
  expect(rule).not.toBeNull()
  expect(rule?.[1]).toMatch(/position:\s*absolute/)
  expect(rule?.[1]).toMatch(/clip(-path)?:/)
  expect(rule?.[1]).toMatch(/width:\s*1px/)
})
