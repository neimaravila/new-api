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
import { ArrowRight, ShieldCheck, TerminalSquare } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { DashboardMessage, DashboardSummary } from '../../types'

interface DashboardHeroProps {
  summary: DashboardSummary
}

const statusToneClass = {
  success: 'border-success/30 bg-success/10 text-success',
  warning: 'border-warning/30 bg-warning/10 text-warning',
  destructive: 'border-destructive/30 bg-destructive/10 text-destructive',
  info: 'border-info/30 bg-info/10 text-info',
} as const

function translateDashboardMessage(
  t: ReturnType<typeof useTranslation>['t'],
  message: DashboardMessage
): string {
  const values = { ...message.values }
  if (Object.prototype.hasOwnProperty.call(values, 'quotaRaw')) {
    const raw = values.quotaRaw
    values.quotaRaw =
      typeof raw === 'number' ? formatQuota(raw) : String(raw ?? '')
  }
  return t(message.key, values)
}

export function DashboardHero(props: DashboardHeroProps): React.JSX.Element {
  const { t } = useTranslation()
  const isAdmin = props.summary.role === 'admin'
  const primaryPath = isAdmin ? '/usage-logs' : '/keys'
  const secondaryPath = isAdmin ? '/channels' : '/playground'

  return (
    <section className='relative overflow-hidden rounded-2xl border bg-[radial-gradient(ellipse_80%_120%_at_80%_0%,color-mix(in_oklch,var(--primary)_16%,transparent)_0%,transparent_60%),linear-gradient(135deg,color-mix(in_oklch,var(--card)_96%,var(--primary)_4%),var(--card))] p-4 shadow-xs sm:p-6'>
      <div className='relative grid gap-5 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end'>
        <div className='min-w-0'>
          <div className='text-muted-foreground mb-2 text-xs font-medium tracking-wider uppercase'>
            {translateDashboardMessage(t, props.summary.hero.eyebrow)}
          </div>
          <div className='flex flex-wrap items-center gap-2'>
            <h2 className='text-2xl font-semibold tracking-tight sm:text-3xl'>
              {translateDashboardMessage(t, props.summary.hero.title)}
            </h2>
            <Badge
              variant='outline'
              className={cn(
                'rounded-full',
                statusToneClass[
                  props.summary.hero.status_tone as keyof typeof statusToneClass
                ] ?? statusToneClass.info
              )}
            >
              {translateDashboardMessage(t, props.summary.hero.status_label)}
            </Badge>
          </div>
          <p className='text-muted-foreground mt-2 max-w-2xl text-sm leading-relaxed'>
            {translateDashboardMessage(t, props.summary.hero.description)}
          </p>
        </div>

        <div className='flex flex-wrap gap-2'>
          <Button render={<Link to={primaryPath} role={undefined} />}>
            {isAdmin ? t('View Logs') : t('Review API Keys')}
            <ArrowRight data-icon='inline-end' />
          </Button>
          <Button
            variant='outline'
            render={<Link to={secondaryPath} role={undefined} />}
          >
            {isAdmin ? (
              <ShieldCheck data-icon='inline-start' />
            ) : (
              <TerminalSquare data-icon='inline-start' />
            )}
            {isAdmin ? t('Review Channels') : t('Open Playground')}
          </Button>
        </div>
      </div>
    </section>
  )
}
