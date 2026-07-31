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
import { useState } from 'react'

import type { ReportChartSpec } from '../../../lib/report-charts'

interface StubChartDatum {
  value?: unknown
}

interface StubChartDataBlock {
  values?: StubChartDatum[]
}

interface StubVChartProps {
  spec: ReportChartSpec
}

/**
 * Stand-in for `@visactor/react-vchart`'s `VChart`, which needs a real canvas
 * and throws under happy-dom. Every report test that renders a chart mocks that
 * boundary with this one component, so the mock a `bun test` process ends up
 * with is the same no matter which test file registered it last.
 *
 * It mirrors the real component's mount-only spec capture: the rendered text
 * comes from the spec committed on mount and does not resync when a later
 * `spec` prop arrives without a remount.
 *
 * The mock factories using it must stay synchronous — under `bun test` an async
 * `vi.mock` factory for a module another test file also imports makes the whole
 * run spin forever.
 */
export function StubVChart(props: StubVChartProps): React.JSX.Element {
  const [committedSpec] = useState(() => props.spec)
  const dataBlock = committedSpec.data as StubChartDataBlock[] | undefined
  const values = dataBlock?.[0]?.values ?? []
  return (
    <div data-testid='chart-spec'>
      {values.map((item) => String(item.value)).join(',')}
    </div>
  )
}
