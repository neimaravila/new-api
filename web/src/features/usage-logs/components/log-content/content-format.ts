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

import { toIntlLocale } from '@/i18n/languages'
import { formatBytes } from '@/lib/format'

/** Pretty-printing is skipped above this size; the raw text is shown. */
export const PRETTY_MAX_CHARS = 256 * 1024

/** Indents JSON text; falls back to the text itself when large or invalid. */
export function prettyJsonText(raw: string): string {
  if (raw.length > PRETTY_MAX_CHARS) return raw
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

/** Indents an already parsed value; empty when absent, compact when large, null when it cannot be serialized (too deep). */
export function prettyValue(value: unknown): string | null {
  if (value === undefined || value === null) return ''
  try {
    const compact = JSON.stringify(value)
    if (compact === undefined) return ''
    if (compact.length > PRETTY_MAX_CHARS) return compact
    return JSON.stringify(value, null, 2)
  } catch {
    return null
  }
}

export type TextSegment = { kind: 'prose' | 'code'; text: string }

/** Splits text into prose and fenced code (``` at line start); an unclosed fence runs to the end. */
export function splitFences(text: string): TextSegment[] {
  const segments: TextSegment[] = []
  let inCode = false
  const push = (kind: TextSegment['kind'], chunk: string) => {
    const last = segments.at(-1)
    if (last && last.kind === kind) last.text += chunk
    else segments.push({ kind, text: chunk })
  }
  for (const line of text.split(/(?<=\n)/)) {
    if (!line) continue
    const fence = line.startsWith('```')
    if (!inCode && fence) {
      inCode = true
      push('code', line)
    } else if (inCode) {
      push('code', line)
      if (fence) inCode = false
    } else {
      push('prose', line)
    }
  }
  return segments
}

/** True for text that is a JSON object or array within the pretty-print size limit. */
export function looksLikeJson(text: string): boolean {
  if (text.length > PRETTY_MAX_CHARS) return false
  const trimmed = text.trim()
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return false
  try {
    JSON.parse(trimmed)
    return true
  } catch {
    return false
  }
}

/** Byte sizes in the interface language, with translated units. */
export function useFormatBytes(): (bytes: number) => string {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  return (bytes) =>
    formatBytes(bytes, locale, [t('B'), t('KB'), t('MB'), t('GB')])
}
