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

import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

export interface ReportChannelOption {
  id: number
  name: string
}

interface ReportFiltersBarProps {
  modelOptions: string[]
  channelOptions: ReportChannelOption[]
  modelName?: string
  channelId?: number
  /** The channel filter is admin-only. */
  showChannel: boolean
  onModelChange: (modelName: string | undefined) => void
  onChannelChange: (channelId: number | undefined) => void
}

export function ReportFiltersBar(props: ReportFiltersBarProps): React.JSX.Element {
  const { t } = useTranslation()
  return (
    <div className='flex flex-wrap items-center gap-2'>
      <NativeSelect
        size='sm'
        aria-label={t('Model')}
        value={props.modelName ?? ''}
        onChange={(event) => props.onModelChange(event.target.value || undefined)}
      >
        <NativeSelectOption value=''>{t('All models')}</NativeSelectOption>
        {props.modelOptions.map((name) => (
          <NativeSelectOption key={name} value={name}>
            {name}
          </NativeSelectOption>
        ))}
      </NativeSelect>
      {props.showChannel && (
        <NativeSelect
          size='sm'
          aria-label={t('Channel')}
          value={props.channelId ? String(props.channelId) : ''}
          onChange={(event) => props.onChannelChange(Number(event.target.value) || undefined)}
        >
          <NativeSelectOption value=''>{t('All channels')}</NativeSelectOption>
          {props.channelOptions.map((channel) => (
            <NativeSelectOption key={channel.id} value={String(channel.id)}>
              {channel.name}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      )}
    </div>
  )
}
