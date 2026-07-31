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
import { flexRender, type Row } from '@tanstack/react-table'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'

import { GroupBadge } from '@/components/group-badge'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { CHANNEL_STATUS, FIELD_DESCRIPTIONS } from '../constants'
import {
  isTagAggregateRow,
  parseChannelStatusInfo,
  parseGroupsList,
} from '../lib'
import type { Channel } from '../types'
import { ChannelRowActionsLayoutContext } from './channel-row-actions-context'
import { useChannels } from './channels-provider'

const SENSITIVE_MASK = '••••'

/**
 * Bespoke channel card for the card view. Reuses every column's existing cell
 * renderer via `flexRender`, so the table's information and interactions are
 * preserved: row selection, provider/multi-key/IO.NET type badge, id,
 * name/remark + warning icons, status (with tooltips), model badges, groups,
 * inline priority/weight spinners, balance refresh, response/test times, tag
 * expand-collapse, and the per-row (or per-tag) actions menu.
 *
 * Fields are grouped by subject instead of mirroring the table's column
 * order: Header (identity — selection, status, name, type, actions), Health
 * (response time, last tested, balance, disable reason), Serves (models),
 * and Access (groups, priority, weight).
 */
function ChannelCardComponent({
  row,
  isSelected,
}: {
  row: Row<Channel>
  isSelected: boolean
}) {
  const { t } = useTranslation()
  const { sensitiveVisible } = useChannels()
  const isTagRow = isTagAggregateRow(row.original)
  const cells = row.getAllCells()

  const renderCell = (id: string) => {
    const cell = cells.find((c) => c.column.id === id)
    if (!cell || !cell.column.columnDef.cell) {
      return null
    }
    return flexRender(cell.column.columnDef.cell, cell.getContext())
  }

  const fieldLabels: Record<string, string> = {
    balance: t('Used / Remaining'),
    response_time: t('Response'),
    test_time: t('Last Tested'),
  }

  const groups = parseGroupsList(row.original.group ?? '')

  const selectCell = renderCell('select')
  const typeCell = renderCell('type')
  const nameCell = renderCell('name')
  const statusCell = renderCell('status')
  const actionsCell = renderCell('actions')
  const priorityCell = renderCell('priority')
  const weightCell = renderCell('weight')
  const balanceCell = renderCell('balance')
  const responseCell = renderCell('response_time')
  const testCell = renderCell('test_time')
  const modelsCell = renderCell('models')

  const labelClass = 'text-muted-foreground text-[11px] font-medium select-none'
  // Fixed-width, uppercase row labels so Health/Serves/Access read as three
  // distinct subjects rather than a continuation of the header.
  const rowLabelClass = cn(labelClass, 'w-14 shrink-0 pt-0.5 uppercase')

  // In card view the enable/disable state is already conveyed by the inline
  // power toggle, so the plain "Enabled"/"Disabled" badge is redundant. Keep
  // only the informative states (e.g. auto-disabled, unknown) and tag rows.
  const showStatusBadge =
    isTagRow ||
    (row.original.status !== CHANNEL_STATUS.ENABLED &&
      row.original.status !== CHANNEL_STATUS.MANUAL_DISABLED)

  // Surface the disable reason inline in the Health row instead of leaving it
  // to the status cell's tooltip (which stays, and still carries the
  // timestamp). Meaningless for a tag aggregate row — its `other_info` is
  // inherited from an arbitrary child channel, not a real aggregate reason —
  // so this is only computed for real channel rows.
  let disabledStatusText: string | null = null
  if (!isTagRow && row.original.status !== CHANNEL_STATUS.ENABLED) {
    const { statusReason } = parseChannelStatusInfo(row.original.other_info)
    const isAutoDisabled = row.original.status === CHANNEL_STATUS.AUTO_DISABLED
    const label = isAutoDisabled ? t('Auto Disabled') : t('Disabled')
    disabledStatusText = statusReason ? `${label} · ${statusReason}` : label
  }

  return (
    <ChannelRowActionsLayoutContext.Provider value='card'>
      <div
        data-state={isSelected ? 'selected' : undefined}
        className='flex flex-col gap-3'
      >
        {/* Header row (identity): selection, status, name, type, actions */}
        <div className='flex items-start justify-between gap-2'>
          <div className='flex min-w-0 flex-1 items-start gap-2'>
            {!isTagRow && selectCell && (
              <span className='mt-0.5 shrink-0'>{selectCell}</span>
            )}
            {showStatusBadge && (
              <span className='mt-0.5 shrink-0'>{statusCell}</span>
            )}
            <div className='min-w-0 flex-1 text-sm'>
              {!isTagRow && (
                <div className={labelClass}>
                  #{sensitiveVisible ? row.original.id : SENSITIVE_MASK}
                </div>
              )}
              {nameCell}
            </div>
          </div>
          <div className='flex shrink-0 items-center gap-1.5'>
            <div className='max-w-[140px] min-w-0 overflow-hidden'>
              {typeCell}
            </div>
            {actionsCell}
          </div>
        </div>

        {/* Health row: response time, last tested, balance, disable reason */}
        <div className='flex items-start gap-2'>
          <span className={rowLabelClass}>{t('Health')}</span>
          <div className='min-w-0 flex-1'>
            <div className='grid grid-cols-3 gap-x-3 gap-y-1'>
              <div className='min-w-0'>
                <div className={cn('mb-1', labelClass)}>
                  {fieldLabels.response_time}
                </div>
                <div className='min-w-0 overflow-hidden text-sm'>
                  {responseCell ?? (
                    <span className='text-muted-foreground'>-</span>
                  )}
                </div>
              </div>
              <div className='min-w-0'>
                <div className={cn('mb-1', labelClass)}>
                  {fieldLabels.test_time}
                </div>
                <div className='min-w-0 overflow-hidden text-sm'>
                  {testCell ?? <span className='text-muted-foreground'>-</span>}
                </div>
              </div>
              <div className='min-w-0'>
                <div className={cn('mb-1', labelClass)}>
                  {fieldLabels.balance}
                </div>
                <div className='min-w-0 overflow-hidden text-sm'>
                  {balanceCell ?? (
                    <span className='text-muted-foreground'>-</span>
                  )}
                </div>
              </div>
            </div>
            {disabledStatusText && (
              <div className='text-muted-foreground mt-1.5 text-xs'>
                {disabledStatusText}
              </div>
            )}
          </div>
        </div>

        {/* Serves row: which models this channel routes */}
        <div className='flex items-start gap-2'>
          <span className={rowLabelClass}>{t('Serves')}</span>
          <div className='min-w-0 flex-1'>
            {modelsCell ?? (
              <span className='text-muted-foreground text-sm'>-</span>
            )}
          </div>
        </div>

        {/* Access row: which groups reach this channel, and how it's
          weighted among them */}
        <div className='flex items-start gap-2'>
          <span className={rowLabelClass}>{t('Access')}</span>
          <div className='min-w-0 flex-1'>
            <div className='min-w-0'>
              {groups.length > 0 ? (
                <div className='-ml-1.5 flex flex-wrap gap-1'>
                  {groups.map((g) => (
                    <GroupBadge
                      key={g}
                      group={g}
                      label={sensitiveVisible ? undefined : SENSITIVE_MASK}
                      size='sm'
                    />
                  ))}
                </div>
              ) : (
                <span className='text-muted-foreground text-sm'>-</span>
              )}
            </div>
            <div className='mt-1.5 grid grid-cols-[auto_auto] items-center gap-x-4 gap-y-1'>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <span
                      className={cn(
                        labelClass,
                        'cursor-help decoration-dotted underline-offset-2 hover:underline'
                      )}
                    />
                  }
                >
                  {t('Priority')}
                </TooltipTrigger>
                <TooltipContent>
                  {t(FIELD_DESCRIPTIONS.PRIORITY)}
                </TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <span
                      className={cn(
                        labelClass,
                        'cursor-help decoration-dotted underline-offset-2 hover:underline'
                      )}
                    />
                  }
                >
                  {t('Weight')}
                </TooltipTrigger>
                <TooltipContent>{t(FIELD_DESCRIPTIONS.WEIGHT)}</TooltipContent>
              </Tooltip>
              <div className='flex justify-start'>{priorityCell}</div>
              <div className='flex justify-start'>{weightCell}</div>
            </div>
          </div>
        </div>
      </div>
    </ChannelRowActionsLayoutContext.Provider>
  )
}

/**
 * Memoized so each card only re-renders when its own react-table row reference
 * changes, instead of every card re-rendering whenever the parent table state
 * (filters, pagination, sensitive toggle, etc.) updates.
 */
export const ChannelCard = memo(ChannelCardComponent)
