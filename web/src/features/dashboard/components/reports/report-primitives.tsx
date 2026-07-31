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

interface PanelShellProps {
  title: string
  description?: string
  actions?: React.ReactNode
  children: React.ReactNode
  emptyText?: string
  isEmpty?: boolean
}

export function PanelShell(props: PanelShellProps): React.JSX.Element {
  return (
    <section className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'>
      <div className='mb-4 flex flex-wrap items-start justify-between gap-2'>
        <div className='min-w-0'>
          <h3 className='text-sm font-semibold sm:text-base'>{props.title}</h3>
          {props.description && (
            <p className='text-muted-foreground text-xs sm:text-sm'>{props.description}</p>
          )}
        </div>
        {props.actions}
      </div>
      {props.isEmpty ? (
        <div className='text-muted-foreground rounded-xl border border-dashed p-4 text-sm'>
          {props.emptyText}
        </div>
      ) : (
        props.children
      )}
    </section>
  )
}

interface EmptyOrLoadingProps {
  loading: boolean
  emptyText: string
  loadingText?: string
}

export function EmptyOrLoading(props: EmptyOrLoadingProps): React.JSX.Element {
  const { t } = useTranslation()
  if (props.loading) {
    return <div className='text-muted-foreground p-4 text-sm'>{props.loadingText ?? t('Loading…')}</div>
  }
  return (
    <div className='text-muted-foreground rounded-xl border border-dashed p-4 text-sm'>
      {props.emptyText}
    </div>
  )
}
