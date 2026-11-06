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
import type { StatusVariant } from '@/components/status-badge'

export type TintTone =
  | 'success'
  | 'danger'
  | 'warning'
  | 'info'
  | 'neutral'
  | 'primary'

const VARIANT_TONES: Partial<Record<StatusVariant, TintTone>> = {
  success: 'success',
  green: 'success',
  'light-green': 'success',
  teal: 'success',
  danger: 'danger',
  red: 'danger',
  warning: 'warning',
  amber: 'warning',
  orange: 'warning',
  yellow: 'warning',
  info: 'info',
  blue: 'info',
  'light-blue': 'info',
  indigo: 'info',
  cyan: 'info',
}

export function toneFromStatusVariant(
  variant: StatusVariant | null | undefined
): TintTone {
  return (variant && VARIANT_TONES[variant]) || 'neutral'
}

/** Static class maps: Tailwind needs the whole strings to generate them. */
export const TONE_BADGE_CLASS: Record<TintTone, string> = {
  success:
    'border-(--vf-success-edge) bg-(--vf-success-fill) text-(--vf-success-text)',
  danger:
    'border-(--vf-danger-edge) bg-(--vf-danger-fill) text-(--vf-danger-text)',
  warning:
    'border-(--vf-warning-edge) bg-(--vf-warning-fill) text-(--vf-warning-text)',
  info: 'border-(--vf-info-edge) bg-(--vf-info-fill) text-(--vf-info-text)',
  neutral:
    'border-(--vf-neutral-edge) bg-(--vf-neutral-fill) text-(--vf-neutral-text)',
  primary:
    'border-(--vf-primary-edge) bg-(--vf-primary-fill) text-(--vf-primary-text)',
}

export const TONE_DOT_CLASS: Record<TintTone, string> = {
  success: 'bg-(--vf-success)',
  danger: 'bg-(--vf-danger)',
  warning: 'bg-(--vf-warning)',
  info: 'bg-(--vf-info)',
  neutral: 'bg-(--vf-neutral)',
  primary: 'bg-(--vf-primary)',
}

export const TONE_TEXT_CLASS: Record<TintTone, string> = {
  success: 'text-(--vf-success-text)',
  danger: 'text-(--vf-danger-text)',
  warning: 'text-(--vf-warning-text)',
  info: 'text-(--vf-info-text)',
  neutral: 'text-(--vf-neutral-text)',
  primary: 'text-(--vf-primary-text)',
}
