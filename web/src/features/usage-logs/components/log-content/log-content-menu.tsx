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
import { MoreHorizontal } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import type { LogContentView } from '../../data/log-content-schema'
import { downloadText } from '../../lib/export'

/**
 * Actions on the captured call. Both need content to be on screen: they are
 * disabled without it and while sensitive data is hidden.
 */
export function LogContentMenu(props: {
  requestId: string
  view: LogContentView | undefined
  sensitiveVisible: boolean
}) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const { view } = props
  const enabled = !!view?.available && props.sensitiveVisible

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant='ghost'
            size='icon'
            className='size-7'
            aria-label={t('More actions')}
          />
        }
      >
        <MoreHorizontal className='size-4' />
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem
          disabled={!enabled}
          onClick={() => {
            if (view) void copyToClipboard(view.client.request.text)
          }}
        >
          {t('Copy request body')}
        </DropdownMenuItem>
        <DropdownMenuItem
          disabled={!enabled}
          onClick={() => {
            if (!view) return
            downloadText(
              `log-${props.requestId}.json`,
              'application/json;charset=utf-8',
              JSON.stringify(view, null, 2)
            )
          }}
        >
          {t('Export call as JSON')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
