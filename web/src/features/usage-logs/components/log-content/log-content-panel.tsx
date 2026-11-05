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
import { ChevronDown } from 'lucide-react'
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Sheet, SheetContent, SheetDescription } from '@/components/ui/sheet'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type { LogContentView } from '../../data/log-content-schema'
import type { UsageLog } from '../../data/schema'
import { DetailsDialog } from '../dialogs/details-dialog'
import { useUsageLogsContext } from '../usage-logs-provider'
import { ConversationTab } from './conversation-tab'
import { LogContentHeader } from './log-content-header'
import { LogContentNotice, LogContentWarnings } from './log-content-notice'
import { LogContentStats } from './log-content-stats'
import { RawJsonTab } from './raw-json-tab'
import { RoutingTab } from './routing-tab'
import { ToolsTab } from './tools-tab'
import { logHasContent, useLogContent } from './use-log-content'

/** The panel covers about 60% of the screen over the list from `lg` up. */
export const LOG_PANEL_WIDTH_CLASS = 'lg:w-[60vw] lg:max-w-none'

type TabId = 'conversation' | 'tools' | 'routing' | 'raw'

/** Keys that leave the panel's arrows alone: with modifiers, or in fields and menus. */
function isNavigationBlocked(event: KeyboardEvent): boolean {
  if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) {
    return true
  }
  if (event.defaultPrevented) return true
  const target = event.target as Element | null
  if (!target?.closest) return false
  // Another open dialog owns its keys; so do fields and menus.
  const dialog = target.closest('[role="dialog"],[aria-modal="true"]')
  if (dialog && !dialog.matches('[data-slot="sheet-content"]')) return true
  // Inside a tab body the arrows scroll long content.
  if (target.closest('[data-log-content-scroll]')) return true
  return !!target.closest(
    'input,textarea,select,[contenteditable="true"],[role="menu"],[role="listbox"],[role="combobox"]'
  )
}

function rowOf(log: UsageLog): HTMLElement | null {
  return document.querySelector<HTMLElement>(`[data-log-row-id="${log.id}"]`)
}

/**
 * Wide side panel for one log: header, figures, the log's details and, for a
 * call with captured content, the tabs. `logs` is the list on screen, which
 * the arrows walk through.
 */
export function LogContentPanel(props: {
  log: UsageLog
  logs: UsageLog[]
  isAdmin: boolean
  isRoot: boolean
  onClose: () => void
}) {
  const { t } = useTranslation()
  const { log, logs, isAdmin, isRoot, onClose } = props
  const { setSelectedLog, selectedLogOpener, sensitiveVisible } =
    useUsageLogsContext()
  const hasContent = logHasContent(log)
  const query = useLogContent(log, isAdmin)
  // Provider-side data is for admins: a self view never carries attempts.
  const view = useMemo(
    () =>
      query.data && !isAdmin ? { ...query.data, attempts: [] } : query.data,
    [query.data, isAdmin]
  )

  const index = logs.findIndex((item) => item.id === log.id)
  const previous = index > 0 ? logs[index - 1] : undefined
  const next = index >= 0 ? logs[index + 1] : undefined
  const goTo = useCallback(
    (target: UsageLog | undefined) => {
      if (target) setSelectedLog(target, rowOf(target))
    },
    [setSelectedLog]
  )

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
      if (isNavigationBlocked(event)) return
      const target = event.key === 'ArrowDown' ? next : previous
      if (!target) return
      event.preventDefault()
      goTo(target)
    }
    // Captured, because the dialog popup stops arrow keys from bubbling.
    document.addEventListener('keydown', onKeyDown, true)
    return () => document.removeEventListener('keydown', onKeyDown, true)
  }, [goTo, next, previous])

  // The attempt shown defaults to the last one, which answered the call.
  const attempts = view?.attempts ?? []
  const [choice, setChoice] = useState<{ logId: number; attempt: number }>()
  const chosenAttempt = choice?.logId === log.id ? choice.attempt : undefined
  const selectedAttempt = chosenAttempt ?? attempts.at(-1)?.attempt ?? 0

  const [tab, setTab] = useState<TabId>('conversation')
  const showTools = !!view && (view.tools.length > 0 || !!view.tools_available)
  const showRouting = isAdmin && attempts.length > 0
  const tabs: { id: TabId; label: string }[] = [
    { id: 'conversation', label: t('Conversation') },
    ...(showTools ? [{ id: 'tools' as const, label: t('Tools') }] : []),
    ...(showRouting ? [{ id: 'routing' as const, label: t('Routing') }] : []),
    { id: 'raw', label: t('Raw JSON') },
  ]
  const activeTab = tabs.some((item) => item.id === tab) ? tab : 'conversation'

  const handleOpenChange = (
    open: boolean,
    eventDetails?: { reason?: string }
  ) => {
    // The panel is non-modal so rows stay clickable: only the close button
    // and Escape dismiss it. Outside presses and focus moves are row clicks
    // that switch the log, not requests to close.
    if (open) return
    const reason = eventDetails?.reason
    if (reason === 'close-press') {
      onClose()
    } else if (reason === 'escape-key') {
      // Escape elsewhere (e.g. typing in a filter) must not close it.
      const active = document.activeElement
      const inPanel = !!active?.closest('[data-slot="sheet-content"]')
      const inRow = !!selectedLogOpener.current?.contains(active)
      if (inPanel || inRow) onClose()
    }
  }

  const tabBody = (id: TabId, current: LogContentView): ReactNode => {
    if (!sensitiveVisible && id !== 'routing') {
      return (
        <p className='text-muted-foreground text-sm' role='status'>
          {t('Sensitive data is hidden.')}
        </p>
      )
    }
    return (
      <div data-testid={`log-content-tab-${id}`}>
        {id === 'conversation' && <ConversationTab view={current} />}
        {id === 'tools' && <ToolsTab view={current} />}
        {id === 'routing' && (
          <RoutingTab
            view={current}
            log={log}
            sensitiveVisible={sensitiveVisible}
          />
        )}
        {id === 'raw' && (
          <RawJsonTab
            view={current}
            isAdmin={isAdmin}
            selectedAttempt={selectedAttempt}
          />
        )}
      </div>
    )
  }

  let body: ReactNode = null
  if (hasContent) {
    if (query.isError) {
      body = (
        <ErrorState
          className='min-h-[160px]'
          title={t('Could not load the content of this call')}
          onRetry={() => void query.refetch()}
        />
      )
    } else if (!view) {
      body = <LoadingState className='min-h-[160px]' />
    } else if (!view.available) {
      body = (
        <LogContentNotice
          reason={view.reason}
          onRetry={
            query.pendingExhausted ? () => void query.refetch() : undefined
          }
        />
      )
    } else {
      body = (
        <>
          <LogContentWarnings view={view} />
          <Tabs value={activeTab} onValueChange={(v) => setTab(v as TabId)}>
            <TabsList variant='line'>
              {tabs.map((item) => (
                <TabsTrigger key={item.id} value={item.id}>
                  {item.label}
                </TabsTrigger>
              ))}
            </TabsList>
            {tabs.map((item) => (
              <TabsContent
                key={item.id}
                value={item.id}
                data-log-content-scroll=''
              >
                {tabBody(item.id, view)}
              </TabsContent>
            ))}
          </Tabs>
        </>
      )
    }
  }

  return (
    <Sheet open onOpenChange={handleOpenChange} modal={false}>
      <SheetContent
        showOverlay={false}
        className={`w-full max-sm:max-w-full sm:max-w-lg ${LOG_PANEL_WIDTH_CLASS}`}
      >
        <SheetDescription className='sr-only'>
          {t('View the complete details for this log entry')}
        </SheetDescription>
        <div className='px-4 pt-4'>
          <LogContentHeader
            log={log}
            view={view}
            isAdmin={isAdmin}
            sensitiveVisible={sensitiveVisible}
            hasPrevious={!!previous}
            hasNext={!!next}
            onPrevious={() => goTo(previous)}
            onNext={() => goTo(next)}
            selectedAttempt={selectedAttempt}
            onSelectAttempt={(attempt) => setChoice({ logId: log.id, attempt })}
          />
        </div>
        <div className='min-h-0 flex-1 space-y-3 overflow-x-hidden overflow-y-auto overscroll-contain px-4 pb-4'>
          {hasContent && (
            <LogContentStats
              log={log}
              view={view}
              sensitiveVisible={sensitiveVisible}
            />
          )}
          <Collapsible defaultOpen={!hasContent} key={log.id}>
            <CollapsibleTrigger className='text-muted-foreground hover:text-foreground group flex w-full items-center gap-1.5 text-sm font-medium'>
              <ChevronDown className='size-4 transition-transform group-data-[panel-open]:rotate-180' />
              {t('Details')}
            </CollapsibleTrigger>
            <CollapsibleContent className='pt-2'>
              <DetailsDialog
                log={log}
                isAdmin={isAdmin}
                isRoot={isRoot}
                open
                onOpenChange={() => {}}
                presentation='embedded'
                hideQuota={!sensitiveVisible}
              />
            </CollapsibleContent>
          </Collapsible>
          {body}
        </div>
      </SheetContent>
    </Sheet>
  )
}
