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
import { X } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'

import { healthTileLabel, type ActiveHealthTiles } from '../lib'

export type HealthFilterDimension = 'status' | 'health'

export type ChannelHealthFilterChipsProps = {
  activeTiles: ActiveHealthTiles
  slowThresholdMs: number
  onDismiss: (dimension: HealthFilterDimension) => void
}

/**
 * Removable chips naming the `status`/`health` column filters a health-strip
 * tile click writes. Deliberately independent of the strip's own render
 * state: the strip collapses to one line (`calm`) or renders nothing
 * (`hidden`) whenever nothing is currently broken, but a filter set earlier —
 * or restored from the `channel-status-filter` localStorage fallback, or a
 * bookmarked `?health=untested` URL — stays active regardless. Without a
 * chip that also ignores strip state, that combination reads as "N channels
 * healthy" over an empty, unexplained table. Takes the same `activeTiles`
 * the strip uses to light its own tiles, so a filter's chip and its tile
 * (when the strip happens to be showing one) always agree.
 */
export function ChannelHealthFilterChips(props: ChannelHealthFilterChipsProps) {
  const { activeTiles, slowThresholdMs, onDismiss } = props
  const { t } = useTranslation()

  const chips: { dimension: HealthFilterDimension; label: string }[] = []
  if (activeTiles.status) {
    chips.push({
      dimension: 'status',
      label: healthTileLabel(activeTiles.status, slowThresholdMs, t),
    })
  }
  if (activeTiles.health) {
    chips.push({
      dimension: 'health',
      label: healthTileLabel(activeTiles.health, slowThresholdMs, t),
    })
  }

  if (chips.length === 0) {
    return null
  }

  return (
    <>
      {chips.map((chip) => (
        <Badge
          key={chip.dimension}
          variant='secondary'
          className='h-8 gap-1 rounded-sm pr-1'
        >
          <span>{chip.label}</span>
          <button
            type='button'
            className='hover:bg-muted-foreground/15 flex size-4 shrink-0 items-center justify-center rounded-sm'
            aria-label={t('Remove filter')}
            onClick={() => onDismiss(chip.dimension)}
          >
            <X aria-hidden='true' />
          </button>
        </Badge>
      ))}
    </>
  )
}
