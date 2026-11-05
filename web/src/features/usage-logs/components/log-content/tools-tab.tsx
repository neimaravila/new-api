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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { formatNumber } from '@/lib/format'

import type {
  LogContentTool,
  LogContentView,
} from '../../data/log-content-schema'
import { prettyValue } from './content-format'
import { RawOnlyNotice } from './conversation-tab'
import { CopyButton, LongText } from './message-block'

function ToolCard(props: { tool: LogContentTool }) {
  const { t } = useTranslation()
  const { tool } = props
  const schema = useMemo(() => prettyValue(tool.schema), [tool.schema])
  return (
    <section
      data-testid='log-content-tool'
      className='space-y-2 rounded-lg border p-3'
    >
      <header className='flex items-center justify-between gap-2'>
        <span className='font-mono text-sm font-medium break-all'>
          {tool.name}
        </span>
        <Badge variant='secondary'>
          {t('Called {{count}} times', { count: formatNumber(tool.called) })}
        </Badge>
      </header>
      {tool.description && (
        <LongText text={tool.description} className='font-sans' />
      )}
      {schema && (
        <div className='bg-muted/30 rounded-md border px-3 py-2'>
          <div className='mb-1 flex items-center justify-between'>
            <span className='text-xs font-medium'>{t('Schema')}</span>
            <CopyButton text={schema} />
          </div>
          <LongText text={schema} />
        </div>
      )}
    </section>
  )
}

/** The tools declared by the request: name, description and schema. */
export function ToolsTab(props: { view: LogContentView }) {
  const { t } = useTranslation()
  const { view } = props
  if (view.tools_available == null) return <RawOnlyNotice />
  if (view.tools.length === 0) {
    return (
      <p className='text-muted-foreground text-sm'>
        {t('This call declared no tools')}
      </p>
    )
  }
  const tools = view.tools.map((tool, position) => ({ tool, position }))
  return (
    <div className='space-y-3'>
      {tools.map((entry) => (
        <ToolCard key={entry.position} tool={entry.tool} />
      ))}
    </div>
  )
}
