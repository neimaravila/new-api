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
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { MetricStrip, type MetricItem } from '../metric-strip'

const items: MetricItem[] = [
  { key: 'lat', label: 'Latency', value: '1.2s', testId: 'lat' },
  { key: 'a', label: 'A', value: '1' },
  { key: 'b', label: 'B', value: '2' },
  { key: 'c', label: 'C', value: '3' },
  { key: 'd', label: 'D', value: '4' },
]

describe('MetricStrip', () => {
  it('renders one cell per item with value and unit apart', () => {
    const { container } = render(
      <MetricStrip items={items} size='md' columnsClassName='grid-cols-5' />
    )
    expect(container.querySelectorAll('[data-slot=metric-cell]')).toHaveLength(
      5
    )
    const value = screen.getByTestId('lat')
    expect(value).toHaveTextContent('1.2s')
    expect(value.querySelector('[data-slot=metric-unit]')).toHaveTextContent(
      /^s$/
    )
    expect(screen.getByText('Latency')).toBeVisible()
  })

  it('keeps a masked value whole', () => {
    const { container } = render(
      <MetricStrip
        items={[{ key: 'cost', label: 'Cost', value: '••••', testId: 'cost' }]}
        size='lg'
        columnsClassName='grid-cols-1'
      />
    )
    expect(container.querySelector('[data-slot=metric-unit]')).toBeNull()
    expect(screen.getByTestId('cost')).toHaveTextContent('••••')
  })

  it('puts the full label in a title so a cut label stays readable', () => {
    render(
      <MetricStrip
        items={[{ key: 'tin', label: 'Tokens de entrada', value: '1' }]}
        size='md'
        columnsClassName='grid-cols-1'
      />
    )
    expect(screen.getByText('Tokens de entrada')).toHaveAttribute(
      'title',
      'Tokens de entrada'
    )
  })

  it('makes the strip a size container so columns follow its width, not the viewport', () => {
    render(
      <MetricStrip
        items={items}
        size='md'
        columnsClassName='grid-cols-5'
        data-testid='strip'
      />
    )
    expect(screen.getByTestId('strip').parentElement).toHaveClass('@container')
  })
})
