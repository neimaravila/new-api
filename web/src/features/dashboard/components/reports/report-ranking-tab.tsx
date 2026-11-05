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
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { formatLatencyMs, formatNumber, formatQuota } from '@/lib/format'

import { getReportRanking } from '../../api'
import { rankingRowsToCsv, sortRankingRows } from '../../lib/report-ranking'
import type {
  ReportDimension,
  ReportFilters,
  ReportRankingRow,
  ReportRankingSortKey,
  ReportSortDirection,
} from '../../types'
import { EmptyOrLoading, PanelShell } from './report-primitives'
import { ReportTable, type ReportTableColumn } from './report-table'

interface ReportRankingTabProps {
  dimension: ReportDimension
  title: string
  /** The query only runs while the tab is the active one. */
  enabled: boolean
  filters: ReportFilters
  /** Receives the CSV of the displayed ranking, or null while there is none. */
  onCsvChange?: (csv: string | null) => void
}

function formatErrorRate(rate: number): string {
  return `${Math.round(rate * 1000) / 10}%`
}

export function ReportRankingTab(props: ReportRankingTabProps): React.JSX.Element {
  const { t } = useTranslation()
  const { dimension, filters, enabled, onCsvChange } = props
  const [sort, setSort] = useState<{ key: ReportRankingSortKey; direction: ReportSortDirection }>({
    key: 'requests',
    direction: 'desc',
  })

  const query = useQuery({
    queryKey: [
      'dashboard',
      'reports',
      'ranking',
      dimension,
      filters.range,
      filters.modelName ?? '',
      filters.channelId ?? 0,
    ],
    queryFn: async () => {
      const result = await getReportRanking(dimension, filters)
      if (!result.success) {
        throw new Error(result.message || 'Failed to load ranking')
      }
      return result.data ?? []
    },
    enabled,
    staleTime: 60 * 1000,
    placeholderData: keepPreviousData,
  })

  const rows = useMemo(
    () => sortRankingRows(query.data ?? [], sort.key, sort.direction),
    [query.data, sort]
  )

  const columns: ReportTableColumn<ReportRankingRow>[] = [
    { key: 'name', header: t('Name'), render: (row) => row.name },
    {
      key: 'requests',
      header: t('Requests'),
      align: 'end',
      sortable: true,
      render: (row) => formatNumber(row.requests),
    },
    {
      key: 'tokens',
      header: t('Tokens'),
      align: 'end',
      sortable: true,
      render: (row) => formatNumber(row.tokens),
    },
    {
      key: 'quota',
      header: t('Cost'),
      align: 'end',
      sortable: true,
      render: (row) => formatQuota(row.quota),
    },
    {
      key: 'error_rate',
      header: t('Error rate'),
      align: 'end',
      sortable: true,
      render: (row) => formatErrorRate(row.error_rate),
    },
    {
      key: 'avg_latency_ms',
      header: t('Avg latency'),
      align: 'end',
      sortable: true,
      render: (row) => formatLatencyMs(row.avg_latency_ms),
    },
    {
      key: 'p50_latency_ms',
      header: t('P50 latency'),
      align: 'end',
      sortable: true,
      render: (row) => formatLatencyMs(row.p50_latency_ms),
    },
    {
      key: 'p95_latency_ms',
      header: t('P95 latency'),
      align: 'end',
      sortable: true,
      render: (row) => formatLatencyMs(row.p95_latency_ms),
    },
  ]

  const csv = useMemo(() => {
    if (rows.length === 0) return null
    return rankingRowsToCsv(
      rows.map((row) => [
        row.name,
        row.requests,
        row.tokens,
        formatQuota(row.quota),
        formatErrorRate(row.error_rate),
        formatLatencyMs(row.avg_latency_ms),
        formatLatencyMs(row.p50_latency_ms),
        formatLatencyMs(row.p95_latency_ms),
      ]),
      columns.map((column) => column.header)
    )
    // The column headers only change with the language.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, t])

  useEffect(() => {
    if (!enabled) return
    onCsvChange?.(csv)
    return () => onCsvChange?.(null)
  }, [csv, enabled, onCsvChange])

  const toggleSort = (key: string) => {
    const next = key as ReportRankingSortKey
    setSort((current) =>
      current.key === next
        ? { key: next, direction: current.direction === 'desc' ? 'asc' : 'desc' }
        : { key: next, direction: 'desc' }
    )
  }

  let body: React.ReactNode
  if (query.isError && !query.data) {
    body = (
      <div role='alert' className='text-muted-foreground rounded-xl border border-dashed p-4 text-sm'>
        {t('The ranking could not be loaded.')}
      </div>
    )
  } else if (rows.length === 0) {
    body = <EmptyOrLoading loading={query.isLoading} emptyText={t('No data in this period.')} />
  } else {
    body = (
      <ReportTable
        caption={props.title}
        columns={columns}
        rows={rows}
        rowKey={(row) => row.name}
        sort={sort}
        onSortChange={toggleSort}
      />
    )
  }

  return (
    <PanelShell
      title={props.title}
      description={t('Only the 50 highest-volume rows are shown.')}
    >
      {body}
    </PanelShell>
  )
}
