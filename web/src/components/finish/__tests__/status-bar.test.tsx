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

import { StatusBar } from '../status-bar'

describe('StatusBar', () => {
  it('is an image named by its label', () => {
    render(<StatusBar tone='danger' label='Error' />)
    expect(screen.getByRole('img', { name: 'Error' })).toHaveAttribute(
      'data-tone',
      'danger'
    )
    expect(screen.queryByText('Error')).toBeNull()
  })
})
