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
import { useQuery } from '@tanstack/react-query'
import { Download } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { getReportExportUrl, getReportSummary } from '../../api'
import type {
  ReportGranularity,
  ReportTimeRange,
} from '../../types'
import { ReportChannelPanel } from './report-channel-panel'
import { ReportErrorsPanel } from './report-errors-panel'
import { ReportModelPanel } from './report-model-panel'
import { ReportPerformancePanel } from './report-performance-panel'
import { ReportStreamPanel } from './report-stream-panel'
import { ReportTokenAnatomyPanel } from './report-token-anatomy-panel'
import { ReportTotalsPanel } from './report-totals-panel'
import { ReportTrendPanel } from './report-trend-panel'

const RANGE_OPTIONS: { value: ReportTimeRange; labelKey: string }[] = [
  { value: 'today', labelKey: 'Today' },
  { value: 'yesterday', labelKey: 'Yesterday' },
  { value: '7d', labelKey: 'Last 7 days' },
  { value: '30d', labelKey: 'Last 30 days' },
  { value: 'month', labelKey: 'This month' },
]

const GRANULARITY_OPTIONS: { value: ReportGranularity; labelKey: string }[] = [
  { value: 'hour', labelKey: 'Hourly' },
  { value: 'day', labelKey: 'Daily' },
]

export function ReportsSection(): React.JSX.Element {
  const { t } = useTranslation()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const isAdmin = Boolean(userRole && userRole >= ROLE.ADMIN)

  const [range, setRange] = useState<ReportTimeRange>('7d')
  const [granularity, setGranularity] = useState<ReportGranularity>('day')

  const summaryQuery = useQuery({
    queryKey: ['dashboard', 'reports', range, granularity],
    queryFn: async () => {
      const result = await getReportSummary({ range, granularity })
      if (!result.success) {
        throw new Error(result.message || 'Failed to load report')
      }
      return result.data
    },
    staleTime: 60 * 1000,
  })

  const summary = summaryQuery.data
  const exportUrl = useMemo(() => getReportExportUrl({ range, granularity }), [range, granularity])

  return (
    <div className='space-y-4'>
      <div className='bg-card flex flex-wrap items-center justify-between gap-2 rounded-2xl border p-3 shadow-xs'>
        <div className='flex flex-wrap items-center gap-2'>
          <div className='flex flex-wrap gap-1'>
            {RANGE_OPTIONS.map((opt) => (
              <Button
                key={opt.value}
                size='sm'
                variant={range === opt.value ? 'default' : 'outline'}
                onClick={() => setRange(opt.value)}
              >
                {t(opt.labelKey)}
              </Button>
            ))}
          </div>
          <div className='bg-border mx-1 hidden h-5 w-px sm:block' />
          <div className='flex flex-wrap gap-1'>
            {GRANULARITY_OPTIONS.map((opt) => (
              <Button
                key={opt.value}
                size='sm'
                variant={granularity === opt.value ? 'default' : 'outline'}
                onClick={() => setGranularity(opt.value)}
              >
                {t(opt.labelKey)}
              </Button>
            ))}
          </div>
        </div>
        <Button size='sm' variant='outline' render={<a href={exportUrl} download />}>
          <Download data-icon='inline-start' />
          {t('Export CSV')}
        </Button>
      </div>

      {summaryQuery.isError && (
        <div className='bg-card text-muted-foreground rounded-2xl border border-dashed p-4 text-sm shadow-xs'>
          {t('The reports data is temporarily unavailable.')}
        </div>
      )}

      {summary && (
        <>
          <ReportTotalsPanel loading={summaryQuery.isLoading} summary={summary} />
          <ReportTrendPanel loading={summaryQuery.isLoading} points={summary.trend.points} />
          <div className='grid gap-4 lg:grid-cols-2'>
            <ReportModelPanel loading={summaryQuery.isLoading} models={summary.models} />
            <ReportErrorsPanel loading={summaryQuery.isLoading} errors={summary.errors} />
          </div>
          <div className='grid gap-4 lg:grid-cols-2'>
            <ReportPerformancePanel
              loading={summaryQuery.isLoading}
              title={t('Model latency')}
              rows={summary.model_performance}
            />
            <ReportTokenAnatomyPanel loading={summaryQuery.isLoading} rows={summary.token_anatomy} />
          </div>
          {summary.stream_performance && (
            <ReportStreamPanel loading={summaryQuery.isLoading} stream={summary.stream_performance} />
          )}
          {isAdmin && summary.channels && (
            <ReportChannelPanel loading={summaryQuery.isLoading} channels={summary.channels} />
          )}
          {isAdmin && summary.channel_performance && (
            <ReportPerformancePanel
              loading={summaryQuery.isLoading}
              title={t('Channel latency')}
              rows={summary.channel_performance}
            />
          )}
        </>
      )}
    </div>
  )
}
