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
import { Info } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

import type { LogContentView } from '../../data/log-content-schema'
import { MessageBlock } from './message-block'

/** Request messages beyond this many are folded behind a button. */
export const RECENT_MESSAGES = 6
/** Most older messages revealed per click, so a huge history stays bounded. */
export const EARLIER_PAGE = 50

export function RawOnlyNotice() {
  const { t } = useTranslation()
  return (
    <Alert role='status'>
      <Info />
      <AlertDescription>
        {t('Only raw JSON is available for this call')}
      </AlertDescription>
    </Alert>
  )
}

/** The conversation of a call: filter, folded history and message blocks. */
export function ConversationTab(props: { view: LogContentView }) {
  const { t } = useTranslation()
  const { view } = props
  const [role, setRole] = useState<string | null>(null)
  const [revealed, setRevealed] = useState(0)
  const { messages, available, response_available } = view.conversation

  if (!available) return <RawOnlyNotice />

  const roles = [...new Set(messages.map((m) => m.role))]
  const activeRole = role && roles.includes(role) ? role : null
  const filtered = activeRole
    ? messages.filter((m) => m.role === activeRole)
    : messages
  const requestMessages = filtered.filter((m) => m.source !== 'response')
  const responseMessages = filtered.filter((m) => m.source === 'response')
  const folded = Math.max(0, requestMessages.length - RECENT_MESSAGES)
  const remaining = Math.max(0, folded - revealed)
  const visibleRequest = requestMessages.slice(remaining)
  const nextPage = Math.min(remaining, EARLIER_PAGE)
  const noResponseWarning =
    !response_available &&
    view.format !== 'embedding' &&
    view.format !== 'rerank'

  return (
    <div className='space-y-3'>
      {noResponseWarning && (
        <Alert role='status'>
          <Info />
          <AlertDescription>
            {t('The response of this call could not be read.')}
          </AlertDescription>
        </Alert>
      )}
      {roles.length > 1 && (
        <div
          className='flex flex-wrap gap-1.5'
          role='group'
          aria-label={t('Filter by role')}
        >
          {roles.map((r) => (
            <Button
              key={r}
              type='button'
              size='xs'
              variant={activeRole === r ? 'default' : 'outline'}
              aria-pressed={activeRole === r}
              className='capitalize'
              onClick={() => {
                setRole(activeRole === r ? null : r)
                setRevealed(0)
              }}
            >
              {r}
            </Button>
          ))}
          {activeRole && (
            <Button
              type='button'
              size='xs'
              variant='ghost'
              onClick={() => {
                setRole(null)
                setRevealed(0)
              }}
            >
              {t('Clear filter')}
            </Button>
          )}
        </div>
      )}
      {remaining > 0 && (
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => setRevealed((n) => n + nextPage)}
        >
          {t('Show {{count}} earlier messages', { count: nextPage })}
        </Button>
      )}
      {[...visibleRequest, ...responseMessages].map((message) => (
        <MessageBlock key={messages.indexOf(message)} message={message} />
      ))}
      {filtered.length === 0 && (
        <p className='text-muted-foreground text-sm'>
          {t('No messages to show')}
        </p>
      )}
    </div>
  )
}
