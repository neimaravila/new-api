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
import { AlertTriangle, CheckCircle2, Info, Lightbulb } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import type { DashboardInsight, DashboardInsightSeverity } from '../../types'

interface SmartInsightsPanelProps {
  insights: DashboardInsight[]
  loading: boolean
  error?: boolean
}

interface InsightItemProps {
  insight: DashboardInsight
}

const severityConfig: Record<
  DashboardInsightSeverity,
  { icon: typeof Info; className: string }
> = {
  info: { icon: Info, className: 'border-info/25 bg-info/8 text-info' },
  warning: {
    icon: AlertTriangle,
    className: 'border-warning/25 bg-warning/8 text-warning',
  },
  critical: {
    icon: AlertTriangle,
    className: 'border-destructive/25 bg-destructive/8 text-destructive',
  },
}

export function SmartInsightsPanel(props: SmartInsightsPanelProps) {
  const { t } = useTranslation()

  if (props.loading) {
    return (
      <section
        className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'
        aria-label={t('Loading smart insights')}
      >
        <div className='mb-4 flex items-center gap-2'>
          <Skeleton className='size-9 rounded-xl' />
          <div className='space-y-2'>
            <Skeleton className='h-4 w-36' />
            <Skeleton className='h-3 w-56' />
          </div>
        </div>
        <div className='grid gap-2'>
          <Skeleton className='h-20 rounded-xl' />
          <Skeleton className='h-20 rounded-xl' />
        </div>
      </section>
    )
  }

  if (props.error) {
    return (
      <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
        <PanelHeader />
        <div className='text-muted-foreground rounded-xl border border-dashed p-4 text-sm'>
          {t('Smart insights are temporarily unavailable.')}
        </div>
      </section>
    )
  }

  return (
    <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
      <PanelHeader />
      {props.insights.length === 0 ? (
        <div className='rounded-xl border border-dashed p-4'>
          <div className='flex items-center gap-2 text-sm font-medium'>
            <CheckCircle2 className='text-success size-4' aria-hidden='true' />
            {t('No urgent insights')}
          </div>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t(
              'The dashboard did not find anything that needs immediate attention.'
            )}
          </p>
        </div>
      ) : (
        <div className='grid gap-2'>
          {props.insights.map((insight) => (
            <InsightItem key={insight.id} insight={insight} />
          ))}
        </div>
      )}
    </section>
  )
}

function PanelHeader() {
  const { t } = useTranslation()

  return (
    <div className='mb-4 flex items-start gap-3'>
      <span className='bg-primary/10 text-primary flex size-9 shrink-0 items-center justify-center rounded-xl'>
        <Lightbulb className='size-4' aria-hidden='true' />
      </span>
      <div className='min-w-0'>
        <h3 className='text-sm font-semibold sm:text-base'>
          {t('Smart insights')}
        </h3>
        <p className='text-muted-foreground text-xs sm:text-sm'>
          {t('Rule-based recommendations from recent platform activity')}
        </p>
      </div>
    </div>
  )
}

function InsightItem(props: InsightItemProps) {
  const { t } = useTranslation()
  const config = severityConfig[props.insight.severity]
  const Icon = config.icon

  return (
    <article className='bg-background/60 rounded-xl border p-3'>
      <div className='flex items-start gap-3'>
        <span
          className={cn(
            'flex size-8 shrink-0 items-center justify-center rounded-lg border',
            config.className
          )}
        >
          <Icon className='size-4' aria-hidden='true' />
        </span>
        <div className='min-w-0 flex-1'>
          <div className='flex flex-wrap items-start justify-between gap-2'>
            <div className='min-w-0'>
              <h4 className='text-sm font-medium'>{t(props.insight.title)}</h4>
              <p className='text-muted-foreground mt-0.5 text-xs leading-relaxed'>
                {t(props.insight.description)}
              </p>
            </div>
            {props.insight.metric_label && props.insight.metric_value && (
              <div className='bg-muted/60 rounded-lg px-2 py-1 text-right'>
                <div className='text-muted-foreground text-[10px] font-medium tracking-wide uppercase'>
                  {t(props.insight.metric_label)}
                </div>
                <div className='text-xs font-semibold tabular-nums'>
                  {props.insight.metric_value}
                </div>
              </div>
            )}
          </div>
          {props.insight.action && (
            <Button
              size='sm'
              variant='link'
              className='mt-2 h-auto px-0 text-xs'
              render={<Link to={props.insight.action.path} role={undefined} />}
            >
              {t(props.insight.action.label)}
            </Button>
          )}
        </div>
      </div>
    </article>
  )
}
