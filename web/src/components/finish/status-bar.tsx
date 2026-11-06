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
import { cn } from '@/lib/utils'

import { TONE_DOT_CLASS, type TintTone } from './tone'

export function StatusBar(props: { tone: TintTone; label: string }) {
  return (
    <span
      role='img'
      aria-label={props.label}
      title={props.label}
      data-slot='status-bar'
      data-tone={props.tone}
      className={cn(
        'inline-block h-6 w-1 shrink-0 rounded-[4px]',
        TONE_DOT_CLASS[props.tone]
      )}
    />
  )
}
