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
import { ChevronDown, Wrench } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { JsonView, TintBadge } from '@/components/finish'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import type {
  LogContentTool,
  LogContentView,
} from '../../data/log-content-schema'
import { prettyValue } from './content-format'
import { RawOnlyNotice } from './conversation-tab'
import { CopyButton, LongText } from './message-block'

function ToolCard(props: { tool: LogContentTool }) {
  const { t, i18n } = useTranslation()
  const { tool } = props
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const schema = useMemo(() => prettyValue(tool.schema), [tool.schema])
  const calls = t('Called {{calls}} times', {
    count: tool.called,
    calls: formatNumber(tool.called, locale),
  })
  return (
    <section
      data-testid='log-content-tool'
      className='min-w-0 space-y-2 rounded-(--vf-radius-inner) border p-3'
    >
      <header className='flex items-start gap-2.5'>
        <span className='flex size-7 shrink-0 items-center justify-center rounded-(--vf-radius-control) bg-(--vf-warning-fill) text-(--vf-warning-text)'>
          <Wrench aria-hidden='true' className='size-3.5' />
        </span>
        <span className='min-w-0 flex-1 pt-1 font-mono text-[13px] font-medium break-all'>
          {tool.name}
        </span>
        {tool.called === 0 ? (
          <span className='text-muted-foreground shrink-0 pt-1 text-[11px]'>
            {calls}
          </span>
        ) : (
          <TintBadge tone='info' className='shrink-0'>
            {calls}
          </TintBadge>
        )}
      </header>
      {tool.description && (
        <LongText
          text={tool.description}
          variant='prose'
          className='text-muted-foreground text-xs'
        />
      )}
      {schema === null && (
        <p className='text-muted-foreground text-sm'>
          {t('Schema too deep to display')}
        </p>
      )}
      {schema && (
        <Collapsible>
          <div className='flex items-center justify-between gap-2'>
            <CollapsibleTrigger
              aria-label={tool.name}
              className='text-muted-foreground hover:text-foreground group flex items-center gap-1 text-xs font-medium'
            >
              <ChevronDown
                aria-hidden='true'
                className='size-3.5 transition-transform group-data-[panel-open]:rotate-180'
              />
              {t('Schema')}
            </CollapsibleTrigger>
          </div>
          <CollapsibleContent className='pt-2'>
            <div className='bg-muted/30 rounded-(--vf-radius-control) border'>
              <div className='flex justify-end px-2 pt-1'>
                <CopyButton text={schema} />
              </div>
              <JsonView text={schema} collapsedLines={24} />
            </div>
          </CollapsibleContent>
        </Collapsible>
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
    <div className='grid gap-2 sm:grid-cols-2'>
      {tools.map((entry) => (
        <ToolCard key={entry.position} tool={entry.tool} />
      ))}
    </div>
  )
}
