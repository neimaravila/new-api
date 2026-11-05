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
import { useEffect } from 'react'

import { DetailsDialog } from './dialogs/details-dialog'
import { useUsageLogsContext } from './usage-logs-provider'

/** Single side panel showing the log selected in the common logs table. */
export function LogDetailsSheet(props: {
  isAdmin: boolean
  isRoot: boolean
  /** Changes when page, filters or category change; closes the panel. */
  resetKey?: string
}) {
  const { selectedLog, setSelectedLog, selectedLogOpener, sensitiveVisible } =
    useUsageLogsContext()
  const { resetKey } = props

  useEffect(() => {
    setSelectedLog(null)
  }, [resetKey, setSelectedLog])

  if (!selectedLog) return null

  const close = () => {
    const opener = selectedLogOpener.current
    setSelectedLog(null)
    // Return focus to the row that opened the panel, if it is still there.
    setTimeout(() => {
      if (opener?.isConnected) opener.focus()
    }, 0)
  }

  return (
    <DetailsDialog
      log={selectedLog}
      isAdmin={props.isAdmin}
      isRoot={props.isRoot}
      open
      onOpenChange={(open, eventDetails) => {
        // The panel is non-modal so rows stay clickable: only the close
        // button and Escape dismiss it. Outside presses and focus moves are
        // row clicks that switch the log, not requests to close.
        if (open) return
        const reason = eventDetails?.reason
        if (reason === 'close-press') {
          close()
        } else if (reason === 'escape-key') {
          // Escape elsewhere (e.g. typing in a filter) must not close it.
          const active = document.activeElement
          const inPanel = !!active?.closest('[data-slot="sheet-content"]')
          const inRow = !!selectedLogOpener.current?.contains(active)
          if (inPanel || inRow) close()
        }
      }}
      presentation='sheet'
      hideQuota={!sensitiveVisible}
    />
  )
}
