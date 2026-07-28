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
import { Link } from '@tanstack/react-router'
import { ArrowRight, Code2, FileText, KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { formatQuota } from '@/lib/format'

import type { DashboardSummary } from '../../types'

export function UserDeveloperPanel(props: {
  summary: DashboardSummary
}): React.JSX.Element {
  const { t } = useTranslation()
  const recentActivity = props.summary.recent_activity ?? []
  const topModels = props.summary.top_models ?? []

  return (
    <div className='grid gap-4 xl:grid-cols-[.9fr_1.1fr]'>
      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
        <div className='mb-4 flex items-center gap-2'>
          <Code2 className='text-muted-foreground size-4' aria-hidden='true' />
          <h3 className='text-sm font-semibold'>{t('Ready-to-run request')}</h3>
        </div>
        <div className='bg-foreground/[0.035] rounded-xl p-3 font-mono text-xs'>
          <code className='block truncate'>curl /v1/chat/completions</code>
          <code className='text-muted-foreground block truncate'>
            -H "Authorization: Bearer sk-..."
          </code>
          <code className='text-muted-foreground block truncate'>
            -d {'{'}"model":"gpt-4o-mini"{'}'}
          </code>
        </div>
        <div className='mt-4 flex flex-wrap gap-2'>
          <Button render={<Link to='/keys' role={undefined} />}>
            <KeyRound data-icon='inline-start' />
            {t('Review API Keys')}
          </Button>
          <Button
            variant='outline'
            render={<Link to='/playground' role={undefined} />}
          >
            {t('Open Playground')}
            <ArrowRight data-icon='inline-end' />
          </Button>
        </div>
      </section>

      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
        <div className='mb-4 flex items-center justify-between gap-3'>
          <div className='flex items-center gap-2'>
            <FileText
              className='text-muted-foreground size-4'
              aria-hidden='true'
            />
            <h3 className='text-sm font-semibold'>{t('Recent requests')}</h3>
          </div>
          <Button
            variant='ghost'
            size='sm'
            render={<Link to='/usage-logs' role={undefined} />}
          >
            {t('View Logs')}
            <ArrowRight data-icon='inline-end' />
          </Button>
        </div>
        <div className='grid gap-2'>
          {recentActivity.length === 0 ? (
            <div className='text-muted-foreground rounded-xl border border-dashed p-3 text-sm'>
              {t('Send your first request to see activity here.')}
            </div>
          ) : (
            recentActivity.slice(0, 5).map((activity) => (
              <div
                key={activity.id}
                className='bg-background/60 flex items-center justify-between gap-3 rounded-xl border px-3 py-2'
              >
                <div className='min-w-0'>
                  <div className='truncate text-sm font-medium'>
                    {activity.model_name || t('Unknown model')}
                  </div>
                  <div className='text-muted-foreground text-xs'>
                    {activity.content || t('Request log')}
                  </div>
                </div>
                <div className='font-mono text-xs font-semibold'>
                  {formatQuota(activity.quota ?? 0)}
                </div>
              </div>
            ))
          )}
        </div>
      </section>

      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5 xl:col-span-2'>
        <h3 className='mb-4 text-sm font-semibold'>{t('Model usage')}</h3>
        <div className='grid gap-2 md:grid-cols-2 xl:grid-cols-3'>
          {topModels.length === 0 ? (
            <div className='text-muted-foreground rounded-xl border border-dashed p-3 text-sm'>
              {t('No model usage data yet.')}
            </div>
          ) : (
            topModels.map((model) => (
              <div
                key={model.id}
                className='bg-background/60 rounded-xl border px-3 py-2'
              >
                <div className='truncate text-sm font-medium'>{model.name}</div>
                <div className='text-muted-foreground mt-0.5 text-xs'>
                  {model.requests} {t('requests')} · {formatQuota(model.quota)}
                </div>
              </div>
            ))
          )}
        </div>
      </section>
    </div>
  )
}
