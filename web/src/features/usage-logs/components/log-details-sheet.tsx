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

import type { UsageLog } from '../data/schema'
import { LogContentPanel } from './log-content/log-content-panel'
import { useUsageLogsContext } from './usage-logs-provider'

/** Single side panel showing the log selected in the common logs table. */
export function LogDetailsSheet(props: {
  isAdmin: boolean
  isRoot: boolean
  /** The logs on screen, which the panel's arrows walk through. */
  logs: UsageLog[]
  /** Changes when page, filters or category change; closes the panel. */
  resetKey?: string
}) {
  const { selectedLog, setSelectedLog, selectedLogOpener } =
    useUsageLogsContext()
  const { resetKey } = props

  useEffect(() => {
    setSelectedLog(null)
  }, [resetKey, setSelectedLog])

  if (!selectedLog) return null

  const close = () => {
    const opener = selectedLogOpener.current
    const id = selectedLog.id
    setSelectedLog(null)
    // Return focus to the row now selected, if it is still there.
    setTimeout(() => {
      const row = document.querySelector<HTMLElement>(
        `[data-log-row-id="${id}"]`
      )
      const target = row?.isConnected ? row : opener
      if (target?.isConnected) target.focus()
    }, 0)
  }

  return (
    <LogContentPanel
      log={selectedLog}
      logs={props.logs}
      isAdmin={props.isAdmin}
      isRoot={props.isRoot}
      onClose={close}
    />
  )
}
