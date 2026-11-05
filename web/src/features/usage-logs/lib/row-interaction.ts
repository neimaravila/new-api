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

/** Visible keyboard focus for clickable rows (theme focus-visible ring). */
export const LOG_ROW_FOCUS_CLASS =
  'cursor-pointer outline-none focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:ring-inset'

/**
 * Row props that open the details panel on click, Enter or Space. Clicks on
 * interactive children and text selections are ignored.
 */
export function getLogRowInteractionProps(
  log: UsageLog,
  setSelectedLog: (log: UsageLog | null, opener?: HTMLElement | null) => void
) {
  return {
    tabIndex: 0,
    className: LOG_ROW_FOCUS_CLASS,
    onClick: (event: MouseEvent<HTMLElement>) => {
      const target = event.target as Element | null
      // Portaled content (dialogs) bubbles through React but is not the row.
      if (!target || !event.currentTarget.contains(target)) return
      if (target.closest(INTERACTIVE_SELECTOR)) return
      const selection = window.getSelection?.()
      if (selection && !selection.isCollapsed) return
      setSelectedLog(log, event.currentTarget)
    },
    onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
      if (event.key !== 'Enter' && event.key !== ' ') return
      if (event.target !== event.currentTarget) return
      event.preventDefault()
      setSelectedLog(log, event.currentTarget)
    },
  }
}

/**
 * Stable key for what the table is showing. It changes with category, scope,
 * page or filters, but not when the same query is merely refetched.
 */
export function buildSelectedLogResetKey(query: {
  logCategory: string
  viewAccess: string
  pagination: unknown
  columnFilters: unknown
  searchParams: unknown
}): string {
  return JSON.stringify([
    query.logCategory,
    query.viewAccess,
    query.pagination,
    query.columnFilters,
    query.searchParams,
  ])
}
