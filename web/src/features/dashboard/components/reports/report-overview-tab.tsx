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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { formatLatencyMs, formatNumber, formatQuota } from '@/lib/format'

import {
  buildCostByModelSpec,
  buildLatencySeriesSpec,
  buildTokensSpec,
  buildVolumeSpec,
  type ReportChartKind,
  type ReportChartSpec,
} from '../../lib/report-charts'
import { getPeriodLatency, getTopCostModel, isOthersModel } from '../../lib/report-figures'
import type { ReportCostSeries, ReportLatencyPoint, ReportSummary, ReportTrendPoint } from '../../types'
import { ReportChart } from './report-chart'
import { ReportChartCard } from './report-chart-card'
import { ReportErrorsPanel } from './report-errors-panel'
import { ReportModelPanel } from './report-model-panel'
import { ReportPerformancePanel } from './report-performance-panel'
import { ReportStreamPanel } from './report-stream-panel'
import { ReportTokenAnatomyPanel } from './report-token-anatomy-panel'
import { ReportTotalsPanel } from './report-totals-panel'
import { ReportTrendPanel } from './report-trend-panel'

const EMPTY_COST: ReportCostSeries = { models: [], points: [] }
const EMPTY_LATENCY: ReportLatencyPoint[] = []
const EMPTY_POINTS: ReportTrendPoint[] = []

interface ReportOverviewTabProps {
  loading: boolean
  isError?: boolean
  /** Undefined until the first load (or after a failed one). */
  summary: ReportSummary | undefined
}

export function ReportOverviewTab(props: ReportOverviewTabProps): React.JSX.Element {
  const { t } = useTranslation()
  const { summary, loading } = props
  const successLabel = t('Successful')
  const failuresLabel = t('Failures')
  const tokensLabel = t('Tokens')
  const othersLabel = t('Other')
  const p50Label = t('p50 latency')
  const p95Label = t('p95 latency')

  const points = summary?.trend.points ?? EMPTY_POINTS
  const costByModel = summary?.cost_by_model ?? EMPTY_COST
  const latencySeries = summary?.latency_series ?? EMPTY_LATENCY

  // Each builder returns null when there is nothing to draw, which is also what
  // the card reads as its empty state.
  const builders = useMemo(
    () => ({
      volume: (kind: ReportChartKind) =>
        buildVolumeSpec(points, { success: successLabel, failures: failuresLabel }, kind),
      tokens: (kind: ReportChartKind) => buildTokensSpec(points, { tokens: tokensLabel }, kind),
      cost: (kind: ReportChartKind) => buildCostByModelSpec(costByModel, othersLabel, kind),
      latency: (kind: ReportChartKind) =>
        buildLatencySeriesSpec(latencySeries, { p50: p50Label, p95: p95Label }, kind),
    }),
    [points, costByModel, latencySeries, successLabel, failuresLabel, tokensLabel, othersLabel, p50Label, p95Label]
  )

  const totals = summary?.trend.totals
  const topCost = getTopCostModel(costByModel)
  let topModelLabel = '-'
  if (topCost) topModelLabel = isOthersModel(topCost.model) ? othersLabel : topCost.model
  const periodLatency = getPeriodLatency(latencySeries)
  const figures: Record<keyof typeof builders, { label: string; value: string }[]> = {
    volume: totals
      ? [
          { label: t('Requests'), value: formatNumber(totals.requests) },
          {
            label: t('Failure rate'),
            value:
              totals.requests > 0
                ? `${Math.round((totals.failures / totals.requests) * 1000) / 10}%`
                : '-',
          },
        ]
      : [],
    tokens: totals ? [{ label: t('Tokens'), value: formatNumber(totals.tokens) }] : [],
    cost: totals
      ? [
          { label: t('Total cost'), value: formatQuota(totals.quota) },
          {
            label: t('Top model'),
            value: topModelLabel,
          },
        ]
      : [],
    latency: [
      { label: t('Typical p50'), value: formatLatencyMs(periodLatency.p50) },
      { label: t('Peak p95'), value: formatLatencyMs(periodLatency.p95) },
    ],
  }

  const cards: {
    key: keyof typeof builders
    title: string
    summary: string
    emptyText: string
    toggleable: boolean
    defaultKind: ReportChartKind
  }[] = [
    {
      key: 'volume',
      title: t('Request volume'),
      summary: t('Successful and failed requests over time'),
      emptyText: t('No usage in this period.'),
      toggleable: true,
      defaultKind: 'bar',
    },
    {
      key: 'tokens',
      title: t('Token volume'),
      summary: t('Tokens processed over time'),
      emptyText: t('No usage in this period.'),
      toggleable: true,
      defaultKind: 'bar',
    },
    {
      key: 'cost',
      title: t('Cost by model'),
      summary: t('Cost over time, split by the top models'),
      emptyText: t('No cost in this period.'),
      toggleable: true,
      defaultKind: 'bar',
    },
    {
      key: 'latency',
      title: t('Latency over time'),
      summary: t('Median and p95 latency per period'),
      emptyText: t('No latency samples in this period.'),
      toggleable: true,
      defaultKind: 'line',
    },
  ]

  return (
    <div className='space-y-4'>
      {summary && <ReportTotalsPanel loading={loading} summary={summary} />}
      <div className='grid gap-4 lg:grid-cols-2'>
        {cards.map((card) => {
          const isEmpty = builders[card.key]('bar') === null
          return (
            <ReportChartCard
              key={card.key}
              title={card.title}
              summary={card.summary}
              emptyText={card.emptyText}
              loading={loading}
              isError={props.isError}
              figures={figures[card.key]}
              isEmpty={isEmpty}
              toggleable={card.toggleable}
              defaultKind={card.defaultKind}
            >
              {(kind) => (
                <ReportChart
                  spec={builders[card.key](kind) as ReportChartSpec | null}
                  ariaLabel={card.title}
                />
              )}
            </ReportChartCard>
          )
        })}
      </div>
      {summary && (
        <>
          <ReportTrendPanel loading={loading} points={points} />
          <div className='grid gap-4 lg:grid-cols-2'>
            <ReportModelPanel loading={loading} models={summary.models} />
            <ReportErrorsPanel loading={loading} errors={summary.errors} />
          </div>
          <div className='grid gap-4 lg:grid-cols-2'>
            <ReportPerformancePanel
              loading={loading}
              title={t('Model latency')}
              rows={summary.model_performance}
            />
            <ReportTokenAnatomyPanel loading={loading} rows={summary.token_anatomy} />
          </div>
          {summary.stream_performance && (
            <ReportStreamPanel loading={loading} stream={summary.stream_performance} />
          )}
        </>
      )}
    </div>
  )
}
