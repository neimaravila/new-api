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

import { TintBadge } from '../tint-badge'

describe('TintBadge', () => {
  it('shows its text with the tone and an optional hidden dot', () => {
    const { rerender } = render(<TintBadge tone='success'>OK · 200</TintBadge>)
    const badge = screen.getByText('OK · 200').closest('[data-slot=tint-badge]')
    expect(badge).toHaveAttribute('data-tone', 'success')
    expect(badge?.querySelector('[aria-hidden=true]')).toBeNull()

    rerender(
      <TintBadge tone='success' dot>
        OK · 200
      </TintBadge>
    )
    const withDot = screen
      .getByText('OK · 200')
      .closest('[data-slot=tint-badge]')
    expect(withDot?.querySelector('[aria-hidden=true]')).not.toBeNull()
  })
})
