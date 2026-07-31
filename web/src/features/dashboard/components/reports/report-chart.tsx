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
import { VChart } from '@visactor/react-vchart'
import { useEffect, useRef, useState } from 'react'

import { useTheme } from '@/context/theme-provider'
import { VCHART_OPTION } from '@/lib/vchart'

import type { ReportChartSpec } from '../../lib/report-charts'

let themeManagerPromise: Promise<(typeof import('@visactor/vchart'))['ThemeManager']> | null = null

interface ReportChartProps {
  spec: ReportChartSpec | null
  height?: number
  ariaLabel: string
}

export function ReportChart(props: ReportChartProps): React.JSX.Element | null {
  const { resolvedTheme } = useTheme()
  const [themeReady, setThemeReady] = useState(false)
  const themeManagerRef = useRef<(typeof import('@visactor/vchart'))['ThemeManager'] | null>(null)

  useEffect(() => {
    const updateTheme = async () => {
      setThemeReady(false)
      if (!themeManagerPromise) {
        themeManagerPromise = import('@visactor/vchart').then((m) => m.ThemeManager)
      }
      const ThemeManager = await themeManagerPromise
      themeManagerRef.current = ThemeManager
      ThemeManager.setCurrentTheme(resolvedTheme === 'dark' ? 'dark' : 'light')
      setThemeReady(true)
    }
    updateTheme()
  }, [resolvedTheme])

  if (!props.spec) return null

  const height = props.height ?? 240
  // VChart only reads `spec` on mount, so the key has to change whenever the
  // spec content does; the serialized spec already covers its `type`, and
  // `themeReady` is always true below, since `<VChart>` is gated behind it.
  const chartKey = `${resolvedTheme}-${JSON.stringify(props.spec)}`

  return (
    <div role='img' aria-label={props.ariaLabel} style={{ height }}>
      {themeReady && <VChart key={chartKey} spec={props.spec} option={VCHART_OPTION} />}
    </div>
  )
}
