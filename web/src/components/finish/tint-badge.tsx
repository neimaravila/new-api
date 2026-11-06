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
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

import { TONE_BADGE_CLASS, TONE_DOT_CLASS, type TintTone } from './tone'

export function TintBadge(props: {
  tone: TintTone
  children: ReactNode
  dot?: boolean
  strong?: boolean
  className?: string
}) {
  return (
    <span
      data-slot='tint-badge'
      data-tone={props.tone}
      className={cn(
        'inline-flex w-fit items-center gap-1.5 rounded-(--vf-radius-badge) border whitespace-nowrap',
        props.strong
          ? 'px-2 py-0.5 text-[11px] font-semibold tracking-[0.04em] uppercase'
          : 'px-1.5 py-px text-xs font-medium',
        TONE_BADGE_CLASS[props.tone],
        props.className
      )}
    >
      {props.dot && (
        <span
          aria-hidden='true'
          className={cn(
            'size-1.5 shrink-0 rounded-full',
            TONE_DOT_CLASS[props.tone]
          )}
        />
      )}
      {props.children}
    </span>
  )
}
