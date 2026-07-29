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
import { formatCompactNumber, formatNumber, formatQuota } from '@/lib/format'
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
import { ReportTokenAnatomyPanel } from './report-token-anatomy-panel'
import { ReportTrendPanel } from './report-trend-panel'
import { BarChartRow, PanelShell } from './report-primitives'

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
          <ReportTotalsPanel
            loading={summaryQuery.isLoading}
            summary={summary}
          />
          <ReportTrendPanel
            loading={summaryQuery.isLoading}
            points={summary.trend.points}
          />
          <ReportModelPanel
            loading={summaryQuery.isLoading}
            models={summary.models}
          />
          <ReportErrorsPanel
            loading={summaryQuery.isLoading}
            errors={summary.errors}
          />
          <ReportPerformancePanel
            loading={summaryQuery.isLoading}
            title={t('Model performance')}
            rows={summary.model_performance}
          />
          <ReportTokenAnatomyPanel
            loading={summaryQuery.isLoading}
            rows={summary.token_anatomy}
          />
          {summary.stream_performance && (
            <ReportStreamPanel
              stream={summary.stream_performance}
              loading={summaryQuery.isLoading}
            />
          )}
          {isAdmin && summary.channel_performance && (
            <ReportPerformancePanel
              loading={summaryQuery.isLoading}
              title={t('Channel performance')}
              rows={summary.channel_performance}
            />
          )}
          {isAdmin && summary.channels && (
            <ReportChannelPanel
              loading={summaryQuery.isLoading}
              channels={summary.channels}
            />
          )}
        </>
      )}
    </div>
  )
}

interface ReportTotalsPanelProps {
  loading: boolean
  summary: {
    trend: { totals: { quota: number; requests: number; tokens: number; failures: number; avg_latency_ms: number } }
  }
}

function ReportTotalsPanel(props: ReportTotalsPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const totals = props.summary.trend.totals
  const max = Math.max(totals.quota, 1)
  const rows = [
    { label: t('Quota used'), value: totals.quota, display: formatQuota(totals.quota), tone: 'accent-2' as const },
    { label: t('Requests'), value: totals.requests, display: formatNumber(totals.requests), tone: 'accent-1' as const },
    { label: t('Tokens'), value: totals.tokens, display: formatCompactNumber(totals.tokens), tone: 'accent-1' as const },
    { label: t('Failures'), value: totals.failures, display: formatNumber(totals.failures), tone: 'accent-3' as const },
  ]
  return (
    <PanelShell
      title={t('Period totals')}
      description={t('Aggregate volume across the selected period')}
      isEmpty={props.loading}
      emptyText={t('Loading…')}
    >
      <div className='grid gap-2'>
        {rows.map((r) => (
          <BarChartRow
            key={r.label}
            label={r.label}
            value={r.value}
            maxValue={max}
            displayValue={r.display}
            tone={r.tone}
            sublabel={r.label === t('Failures') && totals.requests > 0 ? `${Math.round((totals.failures / totals.requests) * 1000) / 10}%` : undefined}
          />
        ))}
      </div>
    </PanelShell>
  )
}

interface ReportStreamPanelProps {
  loading: boolean
  stream: {
    stream_avg_latency_ms: number
    stream_requests: number
    non_stream_avg_latency_ms: number
    non_stream_requests: number
  }
}

function ReportStreamPanel(props: ReportStreamPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const maxLatency = Math.max(props.stream.stream_avg_latency_ms, props.stream.non_stream_avg_latency_ms, 1)
  return (
    <PanelShell title={t('Stream vs non-stream')} description={t('Average latency and request volume by streaming mode')}>
      <div className='grid gap-2'>
        <BarChartRow
          label={t('Stream')}
          value={props.stream.stream_avg_latency_ms}
          maxValue={maxLatency}
          displayValue={`${formatNumber(props.stream.stream_avg_latency_ms)} ms`}
          sublabel={`${formatNumber(props.stream.stream_requests)} ${t('requests')}`}
          tone='accent-1'
        />
        <BarChartRow
          label={t('Non-stream')}
          value={props.stream.non_stream_avg_latency_ms}
          maxValue={maxLatency}
          displayValue={`${formatNumber(props.stream.non_stream_avg_latency_ms)} ms`}
          sublabel={`${formatNumber(props.stream.non_stream_requests)} ${t('requests')}`}
          tone='accent-2'
        />
      </div>
    </PanelShell>
  )
}
