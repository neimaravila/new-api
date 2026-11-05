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
import type { KeyboardEvent, MouseEvent } from 'react'

import type { UsageLog } from '../data/schema'

const INTERACTIVE_SELECTOR =
  'button,a,input,select,textarea,[role="button"],[role="checkbox"],[role="menuitem"],[data-no-row-click]'

/**
 * Row props that open the details panel on click or Enter. Clicks on
 * interactive children and text selections are ignored.
 */
export function getLogRowInteractionProps(
  log: UsageLog,
  setSelectedLog: (log: UsageLog | null) => void
) {
  return {
    tabIndex: 0,
    onClick: (event: MouseEvent<HTMLElement>) => {
      const target = event.target as Element | null
      if (target?.closest(INTERACTIVE_SELECTOR)) return
      const selection = window.getSelection?.()
      if (selection && !selection.isCollapsed) return
      setSelectedLog(log)
    },
    onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
      if (event.key !== 'Enter' || event.target !== event.currentTarget) return
      event.preventDefault()
      setSelectedLog(log)
    },
  }
}
