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
import {
  Activity,
  AlertTriangle,
  ArrowRight,
  RadioTower,
  Users,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { formatQuota } from '@/lib/format'

import type { DashboardSummary } from '../../types'

export function AdminOperationsPanel(props: {
  summary: DashboardSummary
}): React.JSX.Element {
  const { t } = useTranslation()
  const channels = props.summary.channels ?? []
  const topUsers = props.summary.top_users ?? []
  const topModels = props.summary.top_models ?? []
  const recentActivity = props.summary.recent_activity ?? []

  return (
    <div className='grid gap-4 xl:grid-cols-[1.15fr_.85fr]'>
      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
        <PanelTitle
          icon={RadioTower}
          title={t('Channel health')}
          actionPath='/channels'
          actionLabel={t('Manage channels')}
        />
        <div className='grid gap-2'>
          {channels.length === 0 ? (
            <EmptyLine text={t('No channel health data yet.')} />
          ) : (
            channels.map((channel) => (
              <div
                key={channel.id}
                className='bg-background/60 flex items-center justify-between gap-3 rounded-xl border px-3 py-2'
              >
                <div className='min-w-0'>
                  <div className='truncate text-sm font-medium'>
                    {channel.name}
                  </div>
                  <div className='text-muted-foreground text-xs'>
                    {channel.response_time} {t('ms')}
                  </div>
                </div>
                <Badge variant='outline'>
                  {t('Status')} {channel.status}
                </Badge>
              </div>
            ))
          )}
        </div>
      </section>

      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
        <PanelTitle
          icon={AlertTriangle}
          title={t('Recent activity')}
          actionPath='/usage-logs'
          actionLabel={t('View all')}
        />
        <div className='grid gap-2'>
          {recentActivity.length === 0 ? (
            <EmptyLine text={t('No recent activity yet.')} />
          ) : (
            recentActivity.slice(0, 5).map((activity) => (
              <div
                key={activity.id}
                className='bg-background/60 rounded-xl border px-3 py-2'
              >
                <div className='line-clamp-1 text-sm font-medium'>
                  {activity.content || t('Request log')}
                </div>
                <div className='text-muted-foreground mt-0.5 text-xs'>
                  {activity.model_name || t('Unknown model')} ·{' '}
                  {formatQuota(activity.quota ?? 0)}
                </div>
              </div>
            ))
          )}
        </div>
      </section>

      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5 xl:col-span-2'>
        <div className='grid gap-4 md:grid-cols-2'>
          <TopList
            icon={Users}
            title={t('Top users')}
            items={topUsers}
            emptyText={t('No user usage data yet.')}
          />
          <TopList
            icon={Activity}
            title={t('Top models')}
            items={topModels}
            emptyText={t('No model usage data yet.')}
          />
        </div>
      </section>
    </div>
  )
}

function PanelTitle(props: {
  icon: typeof Activity
  title: string
  actionPath: string
  actionLabel: string
}): React.JSX.Element {
  const Icon = props.icon
  return (
    <div className='mb-4 flex items-center justify-between gap-3'>
      <div className='flex items-center gap-2'>
        <Icon className='text-muted-foreground size-4' aria-hidden='true' />
        <h3 className='text-sm font-semibold'>{props.title}</h3>
      </div>
      <Button
        variant='ghost'
        size='sm'
        render={<Link to={props.actionPath} role={undefined} />}
      >
        {props.actionLabel}
        <ArrowRight data-icon='inline-end' />
      </Button>
    </div>
  )
}

function TopList(props: {
  icon: typeof Activity
  title: string
  items: NonNullable<DashboardSummary['top_models']>
  emptyText: string
}): React.JSX.Element {
  const { t } = useTranslation()
  const Icon = props.icon
  return (
    <div>
      <div className='mb-3 flex items-center gap-2'>
        <Icon className='text-muted-foreground size-4' aria-hidden='true' />
        <h4 className='text-sm font-semibold'>{props.title}</h4>
      </div>
      <div className='grid gap-2'>
        {props.items.length === 0 ? (
          <EmptyLine text={props.emptyText} />
        ) : (
          props.items.map((item) => (
            <div
              key={item.id}
              className='bg-background/60 flex items-center justify-between gap-3 rounded-xl border px-3 py-2'
            >
              <div className='min-w-0'>
                <div className='truncate text-sm font-medium'>
                  {item.display_name || item.name}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {item.requests} {t('requests')} · {item.tokens} {t('tokens')}
                </div>
              </div>
              <div className='font-mono text-xs font-semibold'>
                {formatQuota(item.quota)}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function EmptyLine(props: { text: string }): React.JSX.Element {
  return (
    <div className='text-muted-foreground rounded-xl border border-dashed p-3 text-sm'>
      {props.text}
    </div>
  )
}
