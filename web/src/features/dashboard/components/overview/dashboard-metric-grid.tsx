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
import {
  Activity,
  AlertTriangle,
  Coins,
  KeyRound,
  RadioTower,
  Timer,
  Zap,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { DashboardMetric } from '../../types'
import { StatCard } from '../ui/stat-card'

interface DashboardMetricGridProps {
  metrics: DashboardMetric[]
  loading?: boolean
}

const metricIcons = {
  balance: Coins,
  quota: Coins,
  requests: Activity,
  tokens: Zap,
  failures: AlertTriangle,
  keys: KeyRound,
  latency: Timer,
  channels: RadioTower,
} as const

const metricTones = {
  info: 'accent-1',
  success: 'accent-2',
  warning: 'accent-3',
  destructive: 'accent-3',
} as const

export function DashboardMetricGrid(
  props: DashboardMetricGridProps
): React.JSX.Element {
  return (
    <div className='grid gap-2 sm:grid-cols-2 xl:grid-cols-4'>
      {props.metrics.map((metric) => (
        <DashboardMetricItem
          key={metric.key}
          metric={metric}
          loading={props.loading}
        />
      ))}
    </div>
  )
}

function DashboardMetricItem(props: {
  metric: DashboardMetric
  loading?: boolean
}): React.JSX.Element {
  const { t } = useTranslation()
  const Icon =
    metricIcons[props.metric.key as keyof typeof metricIcons] ?? Activity

  return (
    <div className='bg-card rounded-2xl border p-3 shadow-xs'>
      <StatCard
        title={t(props.metric.title)}
        value={props.metric.value}
        description={
          props.metric.description ? t(props.metric.description) : ''
        }
        icon={Icon}
        tone={metricTones[props.metric.tone] ?? 'accent-1'}
        sparkline={props.metric.trend}
        sparklineVariant='line'
        loading={props.loading}
      />
    </div>
  )
}
