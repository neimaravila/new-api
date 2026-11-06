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
import { useTranslation } from 'react-i18next'

import { MetricStrip } from '@/components/finish'
import { formatCompactNumber, formatNumber, formatQuota } from '@/lib/format'

import { PanelShell } from './report-primitives'
import { usePercentFormatter } from '../../lib/percent'
import type { ReportSummary } from '../../types'

interface ReportTotalsPanelProps {
  loading: boolean
  summary: ReportSummary
}

interface TotalsTile {
  key: string
  label: string
  value: string
  sublabel?: string
}

export function ReportTotalsPanel(props: ReportTotalsPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const formatPercent = usePercentFormatter()
  const totals = props.summary.trend.totals
  const failureRate =
    totals.requests > 0 ? totals.failures / totals.requests : null

  const tiles: TotalsTile[] = [
    { key: 'quota', label: t('Quota used'), value: formatQuota(totals.quota) },
    { key: 'requests', label: t('Requests'), value: formatNumber(totals.requests) },
    { key: 'tokens', label: t('Tokens'), value: formatCompactNumber(totals.tokens) },
    {
      key: 'failures',
      label: t('Failures'),
      value: formatNumber(totals.failures),
      sublabel: failureRate === null ? undefined : formatPercent(failureRate),
    },
  ]

  return (
    <PanelShell
      title={t('Period totals')}
      description={t('Aggregate volume across the selected period')}
      isEmpty={props.loading}
      emptyText={t('Loading…')}
    >
      <MetricStrip
        size='lg'
        columnsClassName='grid-cols-2 sm:grid-cols-4'
        items={tiles.map((tile) => ({
          key: tile.key,
          label: tile.label,
          value: tile.value,
          sub: tile.sublabel,
        }))}
      />
    </PanelShell>
  )
}
