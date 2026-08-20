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
import { beforeEach, describe, expect, it } from 'vitest'

beforeEach(() => {
  document.body.replaceChildren()
})

const { render } = await import('@testing-library/react')
const { ReportTable } = await import('../report-table')

describe('ReportTable', () => {
  it('renders one row per datum with the column headers as accessible names', () => {
    const { getByRole, getAllByRole } = render(
      <ReportTable
        caption='Models'
        rows={[{ id: 'a', cost: 10 }, { id: 'b', cost: 20 }]}
        rowKey={(row) => row.id}
        columns={[
          { key: 'name', header: 'Model', render: (row) => row.id },
          { key: 'cost', header: 'Cost', align: 'end', render: (row) => String(row.cost) },
        ]}
      />
    )

    expect(getByRole('columnheader', { name: 'Model' })).toBeDefined()
    expect(getAllByRole('row')).toHaveLength(3)
  })

  it('renders only the header row when there is no data', () => {
    const { getAllByRole } = render(
      <ReportTable
        caption='Models'
        rows={[]}
        rowKey={(row: { id: string }) => row.id}
        columns={[{ key: 'name', header: 'Model', render: (row: { id: string }) => row.id }]}
      />
    )
    expect(getAllByRole('row')).toHaveLength(1)
  })
})
