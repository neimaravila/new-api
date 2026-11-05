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
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Download } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { getReportExportUrl, getReportSummary } from '../../api'
import type {
  ReportDimension,
  ReportFilters,
  ReportGranularity,
  ReportTab,
  ReportTimeRange,
} from '../../types'
import { ReportChannelPanel } from './report-channel-panel'
import { ReportFiltersBar, type ReportChannelOption } from './report-filters'
import { ReportOverviewTab } from './report-overview-tab'
import { ReportPerformancePanel } from './report-performance-panel'
import { ReportRankingTab } from './report-ranking-tab'

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

const TAB_OPTIONS: { value: ReportTab; labelKey: string; adminOnly?: boolean }[] = [
  { value: 'overview', labelKey: 'Overview' },
  { value: 'channels', labelKey: 'Channels', adminOnly: true },
  { value: 'models', labelKey: 'Models' },
  { value: 'users', labelKey: 'Users', adminOnly: true },
  { value: 'tokens', labelKey: 'Tokens' },
  { value: 'groups', labelKey: 'Groups' },
]

const TAB_DIMENSIONS: Record<Exclude<ReportTab, 'overview'>, ReportDimension> = {
  channels: 'channel',
  models: 'model',
  users: 'user',
  tokens: 'token',
  groups: 'group',
}

function downloadCsv(csv: string, name: string): void {
  // The BOM keeps spreadsheet apps from misreading UTF-8 names.
  const url = URL.createObjectURL(new Blob(['\uFEFF', csv], { type: 'text/csv;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = name
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

interface FilterOptions {
  range: ReportTimeRange
  models: string[]
  channels: ReportChannelOption[]
}

export function ReportsSection(): React.JSX.Element {
  const { t } = useTranslation()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const isAdmin = Boolean(userRole && userRole >= ROLE.ADMIN)

  const [tab, setTab] = useState<ReportTab>('overview')
  const [range, setRange] = useState<ReportTimeRange>('7d')
  const [granularity, setGranularity] = useState<ReportGranularity>('day')
  const [modelName, setModelName] = useState<string | undefined>()
  const [channelId, setChannelId] = useState<number | undefined>()
  const [options, setOptions] = useState<FilterOptions | null>(null)
  const [rankingCsv, setRankingCsv] = useState<string | null>(null)
  const handleCsvChange = useCallback((csv: string | null) => setRankingCsv(csv), [])

  const visibleTabs = TAB_OPTIONS.filter((option) => isAdmin || !option.adminOnly)
  const activeTab = visibleTabs.some((option) => option.value === tab) ? tab : 'overview'
  const filters: ReportFilters = {
    range,
    granularity,
    modelName,
    channelId: isAdmin ? channelId : undefined,
  }
  const isFiltered = Boolean(filters.modelName || filters.channelId)

  const summaryQuery = useQuery({
    queryKey: ['dashboard', 'reports', range, granularity, modelName ?? '', filters.channelId ?? 0],
    queryFn: async () => {
      const result = await getReportSummary(filters)
      if (!result.success) {
        throw new Error(result.message || 'Failed to load report')
      }
      return result.data
    },
    staleTime: 60 * 1000,
    // Keep the previous data on screen while a filter refetches, so the tab
    // content (and its bar/line choice) is not unmounted.
    placeholderData: keepPreviousData,
  })

  const summary = summaryQuery.data

  // Filter options come from the first unfiltered load of the range, so
  // picking a model or channel never shrinks the lists to that one entry.
  useEffect(() => {
    if (!summary || isFiltered || summary.range !== range) return
    setOptions((current) => {
      if (current?.range === range) return current
      return {
        range,
        models: summary.models.map((row) => row.model_name),
        channels: (summary.channels ?? []).map((row) => ({
          id: row.channel_id,
          name: row.channel_name,
        })),
      }
    })
  }, [summary, isFiltered, range])

  const changeRange = (next: ReportTimeRange) => {
    // The options belong to a range, so a filter chosen in the old one is dropped.
    setRange(next)
    setModelName(undefined)
    setChannelId(undefined)
  }

  const exportUrl = useMemo(
    () => getReportExportUrl({ range, granularity, modelName, channelId: filters.channelId }),
    [range, granularity, modelName, filters.channelId]
  )
  const loading = summaryQuery.isLoading

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
                onClick={() => changeRange(opt.value)}
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
          <div className='bg-border mx-1 hidden h-5 w-px sm:block' />
          <ReportFiltersBar
            modelOptions={options?.models ?? []}
            channelOptions={options?.channels ?? []}
            modelName={modelName}
            channelId={filters.channelId}
            showChannel={isAdmin}
            onModelChange={setModelName}
            onChannelChange={setChannelId}
          />
        </div>
        {activeTab === 'overview' ? (
          <Button size='sm' variant='outline' render={<a href={exportUrl} download />}>
            <Download data-icon='inline-start' />
            {t('Export CSV')}
          </Button>
        ) : (
          <Button
            size='sm'
            variant='outline'
            disabled={!rankingCsv}
            onClick={() => rankingCsv && downloadCsv(rankingCsv, `report-${activeTab}-${range}.csv`)}
          >
            <Download data-icon='inline-start' />
            {t('Export CSV')}
          </Button>
        )}
      </div>

      {summaryQuery.isError && (
        <div className='bg-card text-muted-foreground rounded-2xl border border-dashed p-4 text-sm shadow-xs'>
          {t('The reports data is temporarily unavailable.')}
        </div>
      )}

      <Tabs value={activeTab} onValueChange={(value) => setTab(value as ReportTab)}>
        <TabsList className='max-w-full overflow-x-auto'>
          {visibleTabs.map((option) => (
            <TabsTrigger key={option.value} value={option.value}>
              {t(option.labelKey)}
            </TabsTrigger>
          ))}
        </TabsList>

        <TabsContent value='overview'>
          <ReportOverviewTab loading={loading} isError={summaryQuery.isError} summary={summary} />
        </TabsContent>
        {visibleTabs
          .filter((option) => option.value !== 'overview')
          .map((option) => {
            const value = option.value as Exclude<ReportTab, 'overview'>
            return (
              <TabsContent key={value} value={value} className='space-y-4'>
                <ReportRankingTab
                  dimension={TAB_DIMENSIONS[value]}
                  title={t(option.labelKey)}
                  enabled={activeTab === value}
                  filters={filters}
                  onCsvChange={handleCsvChange}
                />
                {value === 'channels' && summary?.channels && (
                  <ReportChannelPanel loading={loading} channels={summary.channels} />
                )}
                {value === 'channels' && summary?.channel_performance && (
                  <ReportPerformancePanel
                    loading={loading}
                    title={t('Channel latency')}
                    rows={summary.channel_performance}
                  />
                )}
              </TabsContent>
            )
          })}
      </Tabs>
    </div>
  )
}
