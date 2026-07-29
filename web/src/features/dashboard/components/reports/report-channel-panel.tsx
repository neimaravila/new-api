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

import { BarChartRow, EmptyOrLoading, PanelShell } from './report-primitives'
import type { ReportChannelRow } from '../../types'

interface ReportChannelPanelProps {
  loading: boolean
  channels: ReportChannelRow[]
}

export function ReportChannelPanel(props: ReportChannelPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const max = Math.max(1, ...props.channels.map((c) => c.quota))

  return (
    <PanelShell
      title={t('Channel cost and health')}
      description={t('Cost, volume, error rate and latency per upstream channel')}
    >
      {props.channels.length === 0 ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No channel data in this period.')} />
      ) : (
        <div className='grid gap-2'>
          {props.channels.map((c) => (
            <BarChartRow
              key={c.channel_id}
              label={c.channel_name}
              value={c.quota}
              maxValue={max}
              displayValue={String(c.quota)}
              tone='accent-2'
              sublabel={`${c.requests} ${t('requests')} · ${Math.round(c.error_rate * 1000) / 10}% ${t('errors')} · ${c.avg_latency_ms} ms`}
            />
          ))}
        </div>
      )}
    </PanelShell>
  )
}
