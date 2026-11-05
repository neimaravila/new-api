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
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

import type { LogContentView } from '../../data/log-content-schema'

/** The state of a call with no content to show, from the view's reason. */
export function LogContentNotice(props: {
  reason: string
  /** Offered when looking up again may still help. */
  onRetry?: () => void
}) {
  const { t } = useTranslation()
  let message: string
  switch (props.reason) {
    case 'pending':
      message = t('The content of this call is still being saved.')
      break
    case 'expired':
      message = t(
        'The content of this call was deleted after the retention period.'
      )
      break
    case 'not_captured':
      message = t('Content not captured for this type of call')
      break
    default:
      message = t('The content of this call is not available.')
  }
  return (
    <Alert role='status'>
      <Info />
      <AlertDescription>
        {message}
        {props.onRetry && (
          <Button
            variant='outline'
            size='sm'
            className='mt-2 block'
            onClick={props.onRetry}
          >
            {t('Retry')}
          </Button>
        )}
      </AlertDescription>
    </Alert>
  )
}

/** Warnings about content that was cut or may be unfinished. */
export function LogContentWarnings(props: { view: LogContentView }) {
  const { t } = useTranslation()
  const { view } = props
  const bodies = [
    view.client.request,
    view.client.response,
    ...view.attempts.flatMap((attempt) => [attempt.request, attempt.response]),
  ]
  const truncated = bodies.some((body) => body.truncated || body.head_only)
  if (!truncated && !view.incomplete) return null
  return (
    <div className='space-y-2'>
      {truncated && (
        <Alert role='status'>
          <Info />
          <AlertDescription>
            {t('Some content was shortened to fit the storage limit.')}
          </AlertDescription>
        </Alert>
      )}
      {view.incomplete && (
        <Alert role='status'>
          <Info />
          <AlertDescription>
            {t(
              'The response did not finish, so the captured content may be incomplete.'
            )}
          </AlertDescription>
        </Alert>
      )}
    </div>
  )
}
