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
import { useQueryClient } from '@tanstack/react-query'
import { TestTube } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { textColorMap, type StatusVariant } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import {
  formatResponseTime,
  handleTestAllChannels,
  isHealthTileActive,
  type ActiveHealthTiles,
  type HealthStripState,
  type HealthTile,
  type HealthTileId,
} from '../lib'

export type ChannelHealthStripProps = {
  state: HealthStripState
  slowThresholdMs: number
  activeTiles: ActiveHealthTiles
  onSelect: (tile: HealthTileId) => void
}

const TILE_VARIANT: Record<HealthTileId, StatusVariant> = {
  active: 'success',
  disabled: 'danger',
  slow: 'warning',
  untested: 'neutral',
}

/**
 * Health summary for the whole channel instance, rendered above the table.
 *
 * Takes the already-resolved strip state and a click handler; it holds no
 * query of its own, so the page owns the data (the `channel-ops` query it
 * already runs for the Max Retries badge) and this component stays
 * renderable in isolation.
 */
export function ChannelHealthStrip(props: ChannelHealthStripProps) {
  const { state, slowThresholdMs, activeTiles, onSelect } = props
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  if (state.kind === 'hidden') {
    return null
  }

  if (state.kind === 'calm') {
    return (
      <div className='flex flex-wrap items-center justify-between gap-2 rounded-xl border p-3'>
        <p className='text-muted-foreground text-sm'>
          {t('{{count}} channel(s) healthy', { count: state.total })}
        </p>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => {
            handleTestAllChannels(queryClient)
          }}
        >
          <TestTube data-icon='inline-start' />
          {t('Test All Channels')}
        </Button>
      </div>
    )
  }

  const tileLabel = (tile: HealthTile) => {
    switch (tile.id) {
      case 'active':
        return t('Active')
      case 'disabled':
        return t('Disabled')
      case 'slow':
        return t('Slower than {{threshold}}', {
          threshold: formatResponseTime(slowThresholdMs, t),
        })
      case 'untested':
        return t('Never tested')
    }
  }

  return (
    <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
      {state.tiles.map((tile) => {
        const isActive = isHealthTileActive(tile.id, activeTiles)
        return (
          <button
            key={tile.id}
            type='button'
            aria-pressed={isActive}
            onClick={() => onSelect(tile.id)}
            className={cn(
              'rounded-xl border p-3 text-left transition-colors',
              'hover:bg-muted/60 focus-visible:ring-ring focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none',
              isActive ? 'border-primary bg-muted/60' : 'bg-muted/40'
            )}
          >
            <div className='text-muted-foreground text-xs font-medium'>
              {tileLabel(tile)}
            </div>
            <div
              className={cn(
                'mt-1 font-mono text-lg font-semibold tabular-nums sm:text-xl',
                textColorMap[TILE_VARIANT[tile.id]]
              )}
            >
              {tile.count}
            </div>
          </button>
        )
      })}
    </div>
  )
}
