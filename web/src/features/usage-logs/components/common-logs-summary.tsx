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
import { getRouteApi } from '@tanstack/react-router'
import dayjs from 'dayjs'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { ReportChart } from '@/features/dashboard/components/reports/report-chart'
import { getDashboardChartColors } from '@/features/dashboard/lib/charts'
import { withFailureColor } from '@/features/dashboard/lib/failure-color'
import { buildVolumeSpec } from '@/features/dashboard/lib/report-charts'
import type { ReportTrendPoint } from '@/features/dashboard/types'
import { useTheme } from '@/context/theme-provider'
import { Skeleton } from '@/components/ui/skeleton'
import {
  formatLogQuota,
  formatNumber,
  formatPercent,
  formatUseTime,
} from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getLogSummary } from '../api'
import { fillSummaryBuckets } from '../lib/summary-buckets'
import { buildApiParams } from '../lib/utils'
import type { LogSummary } from '../types'
import { useLogsViewScope, useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

const CHART_HEIGHT = 120

function bucketLabel(timestamp: number, bucketSeconds: number): string {
  return dayjs
    .unix(timestamp)
    .format(bucketSeconds >= 86400 ? 'MM-DD' : 'MM-DD HH:mm')
}

function SummaryStat(props: {
  label: string
  value: string
  testId?: string
}) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div
        data-testid={props.testId}
        className='text-foreground/90 font-mono text-sm font-semibold tabular-nums'
      >
        {props.value}
      </div>
    </div>
  )
}

export function CommonLogsSummary() {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const { isAdminView: isAdmin } = useLogsViewScope()
  const searchParams = route.useSearch()
  const { sensitiveVisible } = useUsageLogsContext()

  const { data, isLoading, isError } = useQuery({
    queryKey: ['usage-logs-summary', isAdmin, searchParams],
    queryFn: async () => {
      const { p: _page, page_size: _pageSize, ...params } = buildApiParams({
        page: 1,
        pageSize: 1,
        searchParams,
        columnFilters: [],
        isAdmin,
      })
      const result = requireServerSuccess(await getLogSummary(params, isAdmin))
      return {
        summary: result.data as LogSummary,
        start: params.start_timestamp,
        end: params.end_timestamp,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const summary = data?.summary
  const spec = useMemo(() => {
    if (!summary) return null
    const points: ReportTrendPoint[] = fillSummaryBuckets(
      summary.buckets ?? [],
      summary.bucket_seconds,
      data?.start,
      data?.end
    ).map((bucket) => ({
      bucket_label: bucketLabel(bucket.timestamp, summary.bucket_seconds),
      bucket_timestamp: bucket.timestamp,
      quota: 0,
      requests: bucket.requests,
      tokens: 0,
      failures: bucket.failures,
      avg_latency_ms: 0,
      consuming_requests: 0,
    }))
    const labels = { success: t('Success'), failures: t('Failures') }
    const base = buildVolumeSpec(points, labels, 'bar')
    if (!base) return null
    return {
      ...withFailureColor(base, labels),
      legends: { visible: false },
    }
    // resolvedTheme: the destructive token differs between light and dark.
    // oxlint-disable-next-line react-hooks/exhaustive-deps
  }, [summary, data?.start, data?.end, t, resolvedTheme])

  if (isError && !summary) {
    return (
      <div className='text-muted-foreground text-xs'>
        {t('Summary unavailable')}
      </div>
    )
  }

  if (isLoading || !summary) {
    return (
      <div className='space-y-2'>
        <Skeleton className='h-9 w-full rounded-md' />
        <Skeleton className='h-[120px] w-full rounded-md' />
      </div>
    )
  }

  const successRate =
    summary.requests > 0 ? (summary.successes / summary.requests) * 100 : null

  return (
    <section className='border-border/60 bg-card/50 space-y-2 rounded-lg border p-3'>
      <div className='grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-5'>
        <SummaryStat
          label={t('Requests')}
          value={formatNumber(summary.requests)}
        />
        <SummaryStat
          label={t('Success rate')}
          value={successRate == null ? '—' : formatPercent(successRate)}
          testId='success-rate'
        />
        <SummaryStat
          label={t('Tokens')}
          value={formatNumber(summary.tokens)}
          testId='summary-tokens'
        />
        <SummaryStat
          label={t('Cost')}
          value={sensitiveVisible ? formatLogQuota(summary.quota) : '••••'}
          testId='summary-cost'
        />
        <SummaryStat
          label={t('Avg latency')}
          value={formatUseTime(summary.avg_use_time_seconds)}
        />
      </div>
      {!summary.error_log_enabled && (
        <p className='text-muted-foreground text-xs'>
          {t(
            'Error logging is off, so the success rate does not include failures'
          )}
        </p>
      )}
      {spec && (
        <>
          <ReportChart
            spec={spec}
            height={CHART_HEIGHT}
            ariaLabel={t('Requests over time')}
          />
          <div className='text-muted-foreground flex items-center gap-3 text-xs'>
            <span className='inline-flex items-center gap-1'>
              <span
                className='size-2 rounded-full'
                style={{ backgroundColor: getDashboardChartColors(2)[0] }}
              />
              {t('Success')}
            </span>
            <span className='inline-flex items-center gap-1'>
              <span className='bg-destructive size-2 rounded-full' />
              {t('Failures')}
            </span>
          </div>
        </>
      )}
    </section>
  )
}
