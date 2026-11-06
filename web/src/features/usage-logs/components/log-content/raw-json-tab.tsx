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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { JsonView } from '@/components/finish'
import { Alert, AlertDescription } from '@/components/ui/alert'

import type {
  LogContentBody,
  LogContentView,
} from '../../data/log-content-schema'
import {
  PRETTY_MAX_CHARS,
  prettyJsonText,
  useFormatBytes,
} from './content-format'
import { CopyButton } from './message-block'

function Notice(props: { children: string }) {
  return (
    <Alert role='status'>
      <Info />
      <AlertDescription>{props.children}</AlertDescription>
    </Alert>
  )
}

/** One stored body as plain text: indented when valid JSON and small, else as it came. */
function BodyView(props: { title: string; body: LogContentBody }) {
  const { t } = useTranslation()
  const formatBytes = useFormatBytes()
  const { body } = props
  const cut = body.truncated || body.head_only
  const shown = useMemo(
    () => (body.undecodable || cut ? body.text : prettyJsonText(body.text)),
    [body.text, body.undecodable, cut]
  )
  // A formatted body is highlighted; so are SSE and cut ones (the lexer
  // tolerates them). Above the pretty-print limit the raw text stays plain.
  const highlight = body.text.length <= PRETTY_MAX_CHARS
  return (
    <section
      data-testid='log-content-raw-body'
      className='space-y-2 rounded-(--vf-radius-inner) border p-3'
    >
      <header className='flex items-center justify-between gap-2'>
        <h3 className='text-[13px] font-medium'>{props.title}</h3>
        <div className='flex items-center gap-2'>
          {body.original_bytes > 0 && (
            <span className='text-muted-foreground text-xs'>
              {t('Original size: {{size}}', {
                size: formatBytes(body.original_bytes),
              })}
            </span>
          )}
          {!body.undecodable && body.text !== '' && (
            <CopyButton text={body.text} />
          )}
        </div>
      </header>
      {body.undecodable ? (
        <Notice>
          {t('This body could not be decoded, so no text is shown.')}
        </Notice>
      ) : (
        <>
          {cut && (
            <Notice>
              {t('Some content was shortened to fit the storage limit.')}
            </Notice>
          )}
          {body.text === '' ? (
            <p className='text-muted-foreground text-sm'>{t('Empty')}</p>
          ) : (
            <div className='bg-muted/30 max-h-96 overflow-auto rounded-(--vf-radius-control)'>
              {highlight ? (
                <JsonView text={shown} collapsedLines={12} />
              ) : (
                <pre
                  data-text-style='json'
                  data-highlighted='false'
                  className='overflow-x-auto p-3 font-mono text-[12.5px] leading-[1.5] whitespace-pre'
                >
                  {shown}
                </pre>
              )}
            </div>
          )}
        </>
      )}
    </section>
  )
}

/**
 * The stored bodies as plain text. The client side is always shown; the
 * provider side only when the view carries attempts (admins).
 */
export function RawJsonTab(props: {
  view: LogContentView
  isAdmin: boolean
  selectedAttempt: number | null
}) {
  const { t } = useTranslation()
  const { view, selectedAttempt } = props
  const attempt = props.isAdmin
    ? (view.attempts.find((item) => item.attempt === selectedAttempt) ??
      view.attempts.at(-1))
    : undefined
  return (
    <div className='space-y-3'>
      <BodyView title={t('Client request')} body={view.client.request} />
      <BodyView title={t('Delivered response')} body={view.client.response} />
      {attempt && (
        <>
          {attempt.request_unavailable ? (
            <Notice>
              {t(
                'The request sent to the provider was not captured for this attempt.'
              )}
            </Notice>
          ) : (
            <BodyView
              title={t('Request sent to the provider')}
              body={attempt.request}
            />
          )}
          <BodyView
            title={t('Response received from the provider')}
            body={attempt.response}
          />
        </>
      )}
    </div>
  )
}
