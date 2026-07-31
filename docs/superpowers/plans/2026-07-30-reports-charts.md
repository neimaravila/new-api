# Reports Charts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the identical bar-list in every dashboard Reports panel with a chart form chosen per panel, plus a compact table holding the exact values.

**Architecture:** Pure spec builders in `lib/report-charts.ts` turn report rows into VChart specs. A thin `ReportChart` wrapper owns theme and remount concerns. Each panel becomes `PanelShell` > `ReportChart` > `ReportTable`. No backend, DTO, or palette changes.

**Tech Stack:** React 19, TypeScript, `@visactor/react-vchart` (already a dependency), Tailwind, i18next, `bun test`.

**Spec:** `docs/superpowers/specs/2026-07-30-reports-charts-design.md`

## Global Constraints

- Every new `.tsx`/`.ts` file needs the project AGPL header; run `bun run copyright` from `web/` to add it.
- All user-facing text goes through `useTranslation()` and `t('English source string')`. English is the key.
- Do not destructure component props — access `props.xxx` directly (`web/AGENTS.md` 3.2/3.3).
- Keep files under ~200 lines; split into another component when one grows past it.
- Tests live in the module's `__tests__/` directory, import from `'vitest'`, and run with `bun test` from `web/`. There is no vitest config; Bun's runner resolves `@/` from `tsconfig.json`.
- Categorical colors come from `getDashboardChartColors` in `@/features/dashboard/lib/charts`. Do not introduce another palette.
- Never a second Y axis. Two measures at different scales become a selector, a second chart, or a table column.
- Two or more series always get a legend; one series gets none (the panel title names it).
- Run `bun run typecheck` and `bunx oxlint -c .oxlintrc.json <files>` before each commit.

---

## File Structure

**Create:**
- `web/src/features/dashboard/lib/report-charts.ts` — pure `rows -> VChart spec` builders, top-N bucketing, tooltip formatting.
- `web/src/features/dashboard/lib/__tests__/report-charts.test.ts` — builder tests.
- `web/src/features/dashboard/components/reports/report-chart.tsx` — `<VChart>` wrapper: theme manager, remount key, fixed height.
- `web/src/features/dashboard/components/reports/report-table.tsx` — compact exact-value table.
- `web/src/features/dashboard/components/reports/report-totals-panel.tsx` — extracted from `reports-section.tsx`.
- `web/src/features/dashboard/components/reports/report-stream-panel.tsx` — extracted from `reports-section.tsx`.

**Modify:**
- The six panel components in `web/src/features/dashboard/components/reports/`.
- `web/src/features/dashboard/components/reports/report-primitives.tsx` — remove `BarChartRow`.
- `web/src/features/dashboard/components/reports/reports-section.tsx` — extract two panels, apply the grid.
- `web/src/features/dashboard/components/reports/__tests__/report-panels.test.tsx` — assert tables instead of bar rows.
- `web/src/i18n/locales/*.json` — 8 locales.

---

### Task 1: Spec builder module and trend chart

**Files:**
- Create: `web/src/features/dashboard/lib/report-charts.ts`
- Create: `web/src/features/dashboard/lib/__tests__/report-charts.test.ts`

**Interfaces:**
- Consumes: `getDashboardChartColors` from `@/features/dashboard/lib/charts`; `ReportTrendPoint` from `@/features/dashboard/types`.
- Produces: `export type ReportChartSpec = Record<string, unknown>`; `export type ReportTrendMetric = 'quota' | 'requests' | 'tokens' | 'failures'`; `export function buildTrendSpec(points: ReportTrendPoint[], metric: ReportTrendMetric, seriesLabel: string): ReportChartSpec | null`.

- [ ] **Step 1: Write the failing test**

Create `web/src/features/dashboard/lib/__tests__/report-charts.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import { buildTrendSpec } from '../report-charts'
import type { ReportTrendPoint } from '../../types'

const point = (over: Partial<ReportTrendPoint>): ReportTrendPoint => ({
  bucket_label: '2026-07-30',
  bucket_timestamp: 1785400000,
  quota: 0,
  requests: 0,
  tokens: 0,
  failures: 0,
  avg_latency_ms: 0,
  consuming_requests: 0,
  ...over,
})

describe('buildTrendSpec', () => {
  it('returns null when there are no points so the panel can show its empty state', () => {
    expect(buildTrendSpec([], 'quota', 'Cost')).toBeNull()
  })

  it('plots the selected metric and leaves the other measures out of the series', () => {
    const spec = buildTrendSpec(
      [point({ bucket_label: 'a', quota: 10, requests: 4 }), point({ bucket_label: 'b', quota: 30, requests: 9 })],
      'requests',
      'Requests'
    )

    const data = spec?.data as [{ values: { time: string; value: number; series: string }[] }]
    expect(data[0].values).toEqual([
      { time: 'a', value: 4, series: 'Requests' },
      { time: 'b', value: 9, series: 'Requests' },
    ])
  })

  it('hides the legend for its single series', () => {
    const spec = buildTrendSpec([point({ quota: 1 })], 'quota', 'Cost')
    expect(spec?.legends).toEqual({ visible: false })
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run from `web/`: `bun test src/features/dashboard/lib/__tests__/report-charts.test.ts`
Expected: FAIL — cannot resolve `../report-charts`.

- [ ] **Step 3: Write the implementation**

Create `web/src/features/dashboard/lib/report-charts.ts`:

```ts
import type { ReportTrendPoint } from '../types'
import { getDashboardChartColors } from './charts'

export type ReportChartSpec = Record<string, unknown>

export type ReportTrendMetric = 'quota' | 'requests' | 'tokens' | 'failures'

const TREND_METRIC_FIELD: Record<ReportTrendMetric, keyof ReportTrendPoint> = {
  quota: 'quota',
  requests: 'requests',
  tokens: 'tokens',
  failures: 'failures',
}

export function buildTrendSpec(
  points: ReportTrendPoint[],
  metric: ReportTrendMetric,
  seriesLabel: string
): ReportChartSpec | null {
  if (points.length === 0) return null

  const field = TREND_METRIC_FIELD[metric]
  const values = points.map((item) => ({
    time: item.bucket_label,
    value: Number(item[field]) || 0,
    series: seriesLabel,
  }))

  return {
    type: 'area',
    data: [{ id: 'reportTrend', values }],
    xField: 'time',
    yField: 'value',
    seriesField: 'series',
    stack: false,
    legends: { visible: false },
    color: getDashboardChartColors(1).slice(0, 1),
    line: { style: { lineWidth: 2 } },
    point: { visible: false },
    crosshair: { xField: { visible: true, line: { visible: true } } },
    background: { fill: 'transparent' },
    animation: true,
  }
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bun test src/features/dashboard/lib/__tests__/report-charts.test.ts`
Expected: PASS, 3 tests.

- [ ] **Step 5: Header, typecheck, lint, commit**

```bash
cd web
bun run copyright
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/dashboard/lib/report-charts.ts src/features/dashboard/lib/__tests__/report-charts.test.ts
cd ..
git add web/src/features/dashboard/lib/report-charts.ts web/src/features/dashboard/lib/__tests__/report-charts.test.ts
git commit -m "feat(reports): add trend chart spec builder"
```

---

### Task 2: Model cost bar and error share donut

**Files:**
- Modify: `web/src/features/dashboard/lib/report-charts.ts`
- Modify: `web/src/features/dashboard/lib/__tests__/report-charts.test.ts`

**Interfaces:**
- Consumes: `ReportChartSpec` from Task 1.
- Produces: `export const ERROR_DONUT_MAX_SLICES = 6`; `export function buildModelCostSpec(models: ReportModelRow[]): ReportChartSpec | null`; `export function buildErrorShareSpec(errors: ReportErrorRow[], otherLabel: string): ReportChartSpec | null`.

- [ ] **Step 1: Write the failing tests**

Append to `web/src/features/dashboard/lib/__tests__/report-charts.test.ts` (and extend the import to `import { ERROR_DONUT_MAX_SLICES, buildErrorShareSpec, buildModelCostSpec, buildTrendSpec } from '../report-charts'`, plus `import type { ReportErrorRow, ReportModelRow } from '../../types'`):

```ts
const model = (name: string, quota: number): ReportModelRow => ({
  model_name: name,
  quota,
  requests: 1,
  tokens: 1,
  failures: 0,
  error_rate: 0,
  avg_latency_ms: 100,
})

const errorRow = (name: string, failures: number): ReportErrorRow => ({
  model_name: name,
  failures,
  quota: 0,
  share: 0,
})

describe('buildModelCostSpec', () => {
  it('returns null for no models', () => {
    expect(buildModelCostSpec([])).toBeNull()
  })

  it('ranks models by cost descending so the widest bar is on top', () => {
    const spec = buildModelCostSpec([model('cheap', 10), model('expensive', 90)])
    const data = spec?.data as [{ values: { name: string; quota: number }[] }]
    expect(data[0].values.map((row) => row.name)).toEqual(['expensive', 'cheap'])
  })
})

describe('buildErrorShareSpec', () => {
  it('returns null when every model has zero failures', () => {
    expect(buildErrorShareSpec([errorRow('gpt-4o', 0)], 'Other')).toBeNull()
  })

  it('collapses the tail past the slice cap into a single Other slice that keeps the total', () => {
    const rows = Array.from({ length: ERROR_DONUT_MAX_SLICES + 3 }, (_, i) => errorRow(`m${i}`, 10 - i))
    const spec = buildErrorShareSpec(rows, 'Other')

    const values = (spec?.data as [{ values: { type: string; value: number }[] }])[0].values
    expect(values).toHaveLength(ERROR_DONUT_MAX_SLICES + 1)
    expect(values[values.length - 1]).toEqual({ type: 'Other', value: 4 + 3 + 2 })
    expect(values.reduce((sum, item) => sum + item.value, 0)).toBe(
      rows.reduce((sum, row) => sum + row.failures, 0)
    )
  })

  it('keeps every model when the count is at the cap', () => {
    const rows = Array.from({ length: ERROR_DONUT_MAX_SLICES }, (_, i) => errorRow(`m${i}`, 5))
    const values = (buildErrorShareSpec(rows, 'Other')?.data as [{ values: unknown[] }])[0].values
    expect(values).toHaveLength(ERROR_DONUT_MAX_SLICES)
  })
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bun test src/features/dashboard/lib/__tests__/report-charts.test.ts`
Expected: FAIL — `buildModelCostSpec` / `buildErrorShareSpec` are not exported.

- [ ] **Step 3: Write the implementation**

Add to `web/src/features/dashboard/lib/report-charts.ts` (extend the type import to `import type { ReportErrorRow, ReportModelRow, ReportTrendPoint } from '../types'`):

```ts
export const ERROR_DONUT_MAX_SLICES = 6

export function buildModelCostSpec(models: ReportModelRow[]): ReportChartSpec | null {
  if (models.length === 0) return null

  const values = [...models]
    .sort((a, b) => b.quota - a.quota)
    .map((item) => ({ name: item.model_name, quota: item.quota }))

  return {
    type: 'bar',
    data: [{ id: 'reportModelCost', values }],
    xField: 'quota',
    yField: 'name',
    direction: 'horizontal',
    legends: { visible: false },
    color: getDashboardChartColors(1).slice(0, 1),
    bar: { state: { hover: { stroke: '#000', lineWidth: 1 } } },
    axes: [
      { orient: 'left', type: 'band' },
      { orient: 'bottom', type: 'linear', visible: false },
    ],
    background: { fill: 'transparent' },
    animation: true,
  }
}

export function buildErrorShareSpec(
  errors: ReportErrorRow[],
  otherLabel: string
): ReportChartSpec | null {
  if (errors.length === 0) return null

  const ranked = [...errors].sort((a, b) => b.failures - a.failures)
  const values = ranked
    .slice(0, ERROR_DONUT_MAX_SLICES)
    .map((item) => ({ type: item.model_name, value: item.failures }))
  const tail = ranked.slice(ERROR_DONUT_MAX_SLICES)
  if (tail.length > 0) {
    values.push({
      type: otherLabel,
      value: tail.reduce((sum, item) => sum + item.failures, 0),
    })
  }

  const total = values.reduce((sum, item) => sum + item.value, 0)
  if (total <= 0) return null

  return {
    type: 'pie',
    data: [{ id: 'reportErrorShare', values }],
    outerRadius: 0.8,
    innerRadius: 0.55,
    padAngle: 0.6,
    valueField: 'value',
    categoryField: 'type',
    legends: { visible: true, orient: 'bottom' },
    label: { visible: true },
    color: getDashboardChartColors(values.length),
    pie: { state: { hover: { outerRadius: 0.85, stroke: '#000', lineWidth: 1 } } },
    background: { fill: 'transparent' },
    animation: true,
  }
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bun test src/features/dashboard/lib/__tests__/report-charts.test.ts`
Expected: PASS, 8 tests.

- [ ] **Step 5: Typecheck, lint, commit**

```bash
cd web && bun run typecheck && bunx oxlint -c .oxlintrc.json src/features/dashboard/lib/report-charts.ts src/features/dashboard/lib/__tests__/report-charts.test.ts && cd ..
git add web/src/features/dashboard/lib/
git commit -m "feat(reports): add model cost and error share chart specs"
```

---

### Task 3: Performance, token anatomy, channel, and stream specs

**Files:**
- Modify: `web/src/features/dashboard/lib/report-charts.ts`
- Modify: `web/src/features/dashboard/lib/__tests__/report-charts.test.ts`

**Interfaces:**
- Produces: `export const CHANNEL_ERROR_RATE_THRESHOLD = 0.05`; `export function buildPerformanceSpec(rows: ReportPerformanceRow[], avgLabel: string, p95Label: string): ReportChartSpec | null`; `export function buildTokenAnatomySpec(rows: ReportTokenAnatomyRow[], labels: { prompt: string; completion: string; cache: string }): ReportChartSpec | null`; `export function buildChannelCostSpec(channels: ReportChannelRow[]): ReportChartSpec | null`; `export function buildStreamSpec(stream: ReportStreamComparison, streamLabel: string, nonStreamLabel: string): ReportChartSpec | null`.

- [ ] **Step 1: Write the failing tests**

Append to the same test file (extend imports accordingly):

```ts
const perfRow = (name: string, avg: number, p95: number): ReportPerformanceRow => ({
  name,
  avg_latency_ms: avg,
  p95_latency_ms: p95,
  requests: 5,
  tokens: 100,
  throughput: 20,
})

describe('buildPerformanceSpec', () => {
  it('returns null for no rows', () => {
    expect(buildPerformanceSpec([], 'Avg', 'p95')).toBeNull()
  })

  it('emits avg and p95 as two series in the same millisecond scale', () => {
    const spec = buildPerformanceSpec([perfRow('gpt-4o', 500, 900)], 'Avg', 'p95')
    const values = (spec?.data as [{ values: { name: string; series: string; value: number }[] }])[0].values
    expect(values).toEqual([
      { name: 'gpt-4o', series: 'Avg', value: 500 },
      { name: 'gpt-4o', series: 'p95', value: 900 },
    ])
    expect(spec?.legends).toEqual({ visible: true, orient: 'bottom' })
  })
})

describe('buildTokenAnatomySpec', () => {
  it('stacks prompt, completion and cache per model', () => {
    const spec = buildTokenAnatomySpec(
      [{ model_name: 'claude', prompt_tokens: 100, completion_tokens: 200, cache_tokens: 50, total: 350 }],
      { prompt: 'Prompt', completion: 'Completion', cache: 'Cache' }
    )
    expect(spec?.stack).toBe(true)
    const values = (spec?.data as [{ values: { series: string; value: number }[] }])[0].values
    expect(values).toEqual([
      { name: 'claude', series: 'Prompt', value: 100 },
      { name: 'claude', series: 'Completion', value: 200 },
      { name: 'claude', series: 'Cache', value: 50 },
    ])
  })

  it('returns null for no rows', () => {
    expect(buildTokenAnatomySpec([], { prompt: 'p', completion: 'c', cache: 'k' })).toBeNull()
  })
})

describe('buildChannelCostSpec', () => {
  it('marks channels above the error threshold so the bar carries status, not a second axis', () => {
    const spec = buildChannelCostSpec([
      { channel_id: 1, channel_name: 'ok', quota: 100, requests: 10, failures: 0, error_rate: 0, avg_latency_ms: 100 },
      { channel_id: 2, channel_name: 'bad', quota: 90, requests: 10, failures: 5, error_rate: 0.5, avg_latency_ms: 100 },
    ])
    const values = (spec?.data as [{ values: { name: string; degraded: boolean }[] }])[0].values
    expect(values).toEqual([
      { name: 'ok', quota: 100, degraded: false },
      { name: 'bad', quota: 90, degraded: true },
    ])
  })
})

describe('buildStreamSpec', () => {
  it('returns null when neither mode had requests', () => {
    const spec = buildStreamSpec(
      { stream_avg_latency_ms: 0, stream_requests: 0, non_stream_avg_latency_ms: 0, non_stream_requests: 0 },
      'Stream',
      'Non-stream'
    )
    expect(spec).toBeNull()
  })

  it('pairs stream against non-stream request counts', () => {
    const spec = buildStreamSpec(
      { stream_avg_latency_ms: 400, stream_requests: 7, non_stream_avg_latency_ms: 900, non_stream_requests: 3 },
      'Stream',
      'Non-stream'
    )
    const values = (spec?.data as [{ values: { name: string; value: number }[] }])[0].values
    expect(values).toEqual([
      { name: 'Stream', value: 7 },
      { name: 'Non-stream', value: 3 },
    ])
  })
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bun test src/features/dashboard/lib/__tests__/report-charts.test.ts`
Expected: FAIL — the four builders are not exported.

- [ ] **Step 3: Write the implementation**

Add to `report-charts.ts`:

```ts
export const CHANNEL_ERROR_RATE_THRESHOLD = 0.05

export function buildPerformanceSpec(
  rows: ReportPerformanceRow[],
  avgLabel: string,
  p95Label: string
): ReportChartSpec | null {
  if (rows.length === 0) return null

  const values = rows.flatMap((item) => [
    { name: item.name, series: avgLabel, value: item.avg_latency_ms },
    { name: item.name, series: p95Label, value: item.p95_latency_ms },
  ])

  return {
    type: 'bar',
    data: [{ id: 'reportPerformance', values }],
    xField: 'value',
    yField: ['name', 'series'],
    seriesField: 'series',
    direction: 'horizontal',
    legends: { visible: true, orient: 'bottom' },
    color: getDashboardChartColors(2).slice(0, 2),
    axes: [
      { orient: 'left', type: 'band' },
      { orient: 'bottom', type: 'linear' },
    ],
    background: { fill: 'transparent' },
    animation: true,
  }
}

export function buildTokenAnatomySpec(
  rows: ReportTokenAnatomyRow[],
  labels: { prompt: string; completion: string; cache: string }
): ReportChartSpec | null {
  if (rows.length === 0) return null

  const values = rows.flatMap((item) => [
    { name: item.model_name, series: labels.prompt, value: item.prompt_tokens },
    { name: item.model_name, series: labels.completion, value: item.completion_tokens },
    { name: item.model_name, series: labels.cache, value: item.cache_tokens },
  ])

  return {
    type: 'bar',
    data: [{ id: 'reportTokenAnatomy', values }],
    xField: 'value',
    yField: 'name',
    seriesField: 'series',
    direction: 'horizontal',
    stack: true,
    legends: { visible: true, orient: 'bottom' },
    color: getDashboardChartColors(3).slice(0, 3),
    bar: { style: { stroke: 'transparent', lineWidth: 2 } },
    axes: [
      { orient: 'left', type: 'band' },
      { orient: 'bottom', type: 'linear' },
    ],
    background: { fill: 'transparent' },
    animation: true,
  }
}

export function buildChannelCostSpec(channels: ReportChannelRow[]): ReportChartSpec | null {
  if (channels.length === 0) return null

  const values = channels.map((item) => ({
    name: item.channel_name,
    quota: item.quota,
    degraded: item.error_rate > CHANNEL_ERROR_RATE_THRESHOLD,
  }))

  return {
    type: 'bar',
    data: [{ id: 'reportChannelCost', values }],
    xField: 'quota',
    yField: 'name',
    seriesField: 'degraded',
    direction: 'horizontal',
    legends: { visible: false },
    color: getDashboardChartColors(2).slice(0, 2),
    axes: [
      { orient: 'left', type: 'band' },
      { orient: 'bottom', type: 'linear', visible: false },
    ],
    background: { fill: 'transparent' },
    animation: true,
  }
}

export function buildStreamSpec(
  stream: ReportStreamComparison,
  streamLabel: string,
  nonStreamLabel: string
): ReportChartSpec | null {
  if (stream.stream_requests <= 0 && stream.non_stream_requests <= 0) return null

  return {
    type: 'bar',
    data: [
      {
        id: 'reportStream',
        values: [
          { name: streamLabel, value: stream.stream_requests },
          { name: nonStreamLabel, value: stream.non_stream_requests },
        ],
      },
    ],
    xField: 'name',
    yField: 'value',
    seriesField: 'name',
    legends: { visible: false },
    color: getDashboardChartColors(2).slice(0, 2),
    background: { fill: 'transparent' },
    animation: true,
  }
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bun test src/features/dashboard/lib/__tests__/report-charts.test.ts`
Expected: PASS, 15 tests.

- [ ] **Step 5: Typecheck, lint, commit**

```bash
cd web && bun run typecheck && bunx oxlint -c .oxlintrc.json src/features/dashboard/lib/report-charts.ts src/features/dashboard/lib/__tests__/report-charts.test.ts && cd ..
git add web/src/features/dashboard/lib/
git commit -m "feat(reports): add performance, token, channel and stream chart specs"
```

---

### Task 4: ReportChart wrapper and ReportTable

**Files:**
- Create: `web/src/features/dashboard/components/reports/report-chart.tsx`
- Create: `web/src/features/dashboard/components/reports/report-table.tsx`
- Create: `web/src/features/dashboard/components/reports/__tests__/report-table.test.tsx`

**Interfaces:**
- Consumes: `ReportChartSpec` from Task 1; `VCHART_OPTION` from `@/lib/vchart`; `useTheme` from `@/context/theme-provider`.
- Produces: `export function ReportChart(props: { spec: ReportChartSpec | null; height?: number; ariaLabel: string }): React.JSX.Element | null`; `export interface ReportTableColumn<T> { key: string; header: string; align?: 'start' | 'end'; render: (row: T) => React.ReactNode }`; `export function ReportTable<T>(props: { columns: ReportTableColumn<T>[]; rows: T[]; rowKey: (row: T) => string; caption: string }): React.JSX.Element`.

- [ ] **Step 1: Write the failing test**

Create `web/src/features/dashboard/components/reports/__tests__/report-table.test.tsx`, reusing the happy-dom bootstrap already at the top of `report-panels.test.tsx` (copy lines 1–36 verbatim), then:

```tsx
const { render } = await import('@testing-library/react')
const { ReportTable } = await import('../report-table')

describe('ReportTable', () => {
  it('renders one row per datum with the column headers as accessible names', () => {
    const { getByRole, getAllByRole } = render(
      <ReportTable
        caption='Models'
        rows={[{ id: 'a', cost: 10 }, { id: 'b', cost: 20 }]}
        rowKey={(row) => row.id}
        columns={[
          { key: 'name', header: 'Model', render: (row) => row.id },
          { key: 'cost', header: 'Cost', align: 'end', render: (row) => String(row.cost) },
        ]}
      />
    )

    expect(getByRole('columnheader', { name: 'Model' })).toBeDefined()
    expect(getAllByRole('row')).toHaveLength(3)
  })

  it('renders only the header row when there is no data', () => {
    const { getAllByRole } = render(
      <ReportTable
        caption='Models'
        rows={[]}
        rowKey={(row: { id: string }) => row.id}
        columns={[{ key: 'name', header: 'Model', render: (row: { id: string }) => row.id }]}
      />
    )
    expect(getAllByRole('row')).toHaveLength(1)
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bun test src/features/dashboard/components/reports/__tests__/report-table.test.tsx`
Expected: FAIL — cannot resolve `../report-table`.

- [ ] **Step 3: Write ReportTable**

Create `web/src/features/dashboard/components/reports/report-table.tsx`:

```tsx
import { cn } from '@/lib/utils'

export interface ReportTableColumn<T> {
  key: string
  header: string
  align?: 'start' | 'end'
  render: (row: T) => React.ReactNode
}

interface ReportTableProps<T> {
  columns: ReportTableColumn<T>[]
  rows: T[]
  rowKey: (row: T) => string
  caption: string
}

export function ReportTable<T>(props: ReportTableProps<T>): React.JSX.Element {
  return (
    <div className='mt-3 overflow-x-auto'>
      <table className='w-full text-xs'>
        <caption className='sr-only'>{props.caption}</caption>
        <thead>
          <tr className='text-muted-foreground border-b'>
            {props.columns.map((column) => (
              <th
                key={column.key}
                scope='col'
                className={cn('py-1.5 font-medium', column.align === 'end' ? 'text-right' : 'text-left')}
              >
                {column.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {props.rows.map((row) => (
            <tr key={props.rowKey(row)} className='border-b last:border-0'>
              {props.columns.map((column) => (
                <td
                  key={column.key}
                  className={cn(
                    'py-1.5',
                    column.align === 'end' ? 'text-right font-mono tabular-nums' : 'text-left'
                  )}
                >
                  {column.render(row)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bun test src/features/dashboard/components/reports/__tests__/report-table.test.tsx`
Expected: PASS, 2 tests.

- [ ] **Step 5: Write ReportChart**

Create `web/src/features/dashboard/components/reports/report-chart.tsx`. The theme-manager pattern is copied from `components/models/model-charts.tsx:40-97`:

```tsx
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
  const chartKey = [String(props.spec.type), resolvedTheme, themeReady ? 'ready' : 'pending'].join('-')

  return (
    <div role='img' aria-label={props.ariaLabel} style={{ height }}>
      {themeReady && <VChart key={chartKey} spec={props.spec} option={VCHART_OPTION} />}
    </div>
  )
}
```

- [ ] **Step 6: Typecheck, lint, commit**

```bash
cd web && bun run copyright && bun run typecheck && bunx oxlint -c .oxlintrc.json src/features/dashboard/components/reports/ && cd ..
git add web/src/features/dashboard/components/reports/
git commit -m "feat(reports): add chart wrapper and value table primitives"
```

---

### Task 5: Trend panel with metric selector

**Files:**
- Modify: `web/src/features/dashboard/components/reports/report-trend-panel.tsx`

**Interfaces:**
- Consumes: `buildTrendSpec`, `ReportTrendMetric` (Task 1); `ReportChart` (Task 4).
- Produces: unchanged props `{ loading: boolean; points: ReportTrendPoint[] }`.

- [ ] **Step 1: Replace the panel body**

Rewrite the component body of `report-trend-panel.tsx`, keeping the existing copyright header and props interface:

```tsx
const METRIC_OPTIONS: { value: ReportTrendMetric; labelKey: string }[] = [
  { value: 'quota', labelKey: 'Cost' },
  { value: 'requests', labelKey: 'Requests' },
  { value: 'tokens', labelKey: 'Tokens' },
  { value: 'failures', labelKey: 'Failures' },
]

export function ReportTrendPanel(props: ReportTrendPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const [metric, setMetric] = useState<ReportTrendMetric>('quota')
  const metricLabel = t(METRIC_OPTIONS.find((option) => option.value === metric)?.labelKey ?? 'Cost')
  const spec = useMemo(
    () => buildTrendSpec(props.points, metric, metricLabel),
    [props.points, metric, metricLabel]
  )

  return (
    <PanelShell
      title={t('Usage over time')}
      description={t('One measure at a time, across the selected period')}
      actions={
        <div className='flex flex-wrap gap-1'>
          {METRIC_OPTIONS.map((option) => (
            <Button
              key={option.value}
              size='sm'
              variant={metric === option.value ? 'default' : 'outline'}
              onClick={() => setMetric(option.value)}
            >
              {t(option.labelKey)}
            </Button>
          ))}
        </div>
      }
    >
      {spec === null ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No usage in this period.')} />
      ) : (
        <ReportChart spec={spec} height={280} ariaLabel={t('Usage over time')} />
      )}
    </PanelShell>
  )
}
```

Imports to add: `useMemo`, `useState` from `react`; `Button` from `@/components/ui/button`; `buildTrendSpec` and `type ReportTrendMetric` from `../../lib/report-charts`; `ReportChart` from `./report-chart`.

- [ ] **Step 2: Verify it compiles and the suite still passes**

```bash
cd web && bun run typecheck && bun test src/features/dashboard/
```
Expected: typecheck clean; existing report tests still pass (this panel has no test yet).

- [ ] **Step 3: Look at it in the browser**

Run `bun run dev`, open the dashboard Reports section, and confirm: the area chart renders, each metric button swaps the series, the crosshair tooltip follows the cursor, and dark mode redraws rather than inverting.

- [ ] **Step 4: Lint and commit**

```bash
cd web && bunx oxlint -c .oxlintrc.json src/features/dashboard/components/reports/report-trend-panel.tsx && cd ..
git add web/src/features/dashboard/components/reports/report-trend-panel.tsx
git commit -m "feat(reports): chart the usage trend with a metric selector"
```

---

### Task 6: Models and Errors panels

**Files:**
- Modify: `web/src/features/dashboard/components/reports/report-model-panel.tsx`
- Modify: `web/src/features/dashboard/components/reports/report-errors-panel.tsx`
- Modify: `web/src/features/dashboard/components/reports/__tests__/report-panels.test.tsx`

**Interfaces:**
- Consumes: `buildModelCostSpec`, `buildErrorShareSpec` (Task 2); `ReportChart`, `ReportTable`, `ReportTableColumn` (Task 4).

- [ ] **Step 1: Update the failing tests first**

In `report-panels.test.tsx`, replace the model and errors cases. The model case now asserts the table, and a new case asserts the "Other" bucket reaches the UI:

```tsx
it('renders one table row per model with its cost', () => {
  const { queryByText, getAllByRole } = renderNode(
    <ReportModelPanel
      loading={false}
      models={[
        { model_name: 'gpt-4o', quota: 350, requests: 3, tokens: 200, failures: 1, error_rate: 0.25, avg_latency_ms: 600 },
        { model_name: 'claude', quota: 100, requests: 1, tokens: 350, failures: 0, error_rate: 0, avg_latency_ms: 3000 },
      ]}
    />
  )

  expect(getAllByRole('row')).toHaveLength(3)
  expect(queryByText('gpt-4o')).not.toBeNull()
  expect(queryByText(formatQuota(350))).not.toBeNull()
})
```

Mock the chart module at the top of the test file, next to the other dynamic imports, because a canvas chart cannot render under happy-dom:

```tsx
const { mock } = await import('bun:test')
mock.module('../report-chart', () => ({
  ReportChart: (props: { ariaLabel: string }) => <div role='img' aria-label={props.ariaLabel} />,
}))
```

- [ ] **Step 2: Run to verify the model test fails**

Run: `bun test src/features/dashboard/components/reports/__tests__/report-panels.test.tsx`
Expected: FAIL — no `row` elements, the panel still renders bar divs.

- [ ] **Step 3: Rewrite ReportModelPanel**

Body of `report-model-panel.tsx`:

```tsx
export function ReportModelPanel(props: ReportModelPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const spec = useMemo(() => buildModelCostSpec(props.models), [props.models])
  const columns: ReportTableColumn<ReportModelRow>[] = [
    { key: 'model', header: t('Model'), render: (row) => row.model_name },
    { key: 'cost', header: t('Cost'), align: 'end', render: (row) => formatQuota(row.quota) },
    { key: 'requests', header: t('Requests'), align: 'end', render: (row) => formatNumber(row.requests) },
    {
      key: 'errors',
      header: t('Error rate'),
      align: 'end',
      render: (row) => `${Math.round(row.error_rate * 1000) / 10}%`,
    },
    {
      key: 'latency',
      header: t('Avg latency'),
      align: 'end',
      render: (row) => `${formatNumber(row.avg_latency_ms)} ms`,
    },
  ]

  return (
    <PanelShell title={t('Model breakdown')} description={t('Cost per model, with requests, errors and latency')}>
      {spec === null ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No model usage in this period.')} />
      ) : (
        <>
          <ReportChart spec={spec} ariaLabel={t('Cost per model')} />
          <ReportTable
            caption={t('Model breakdown')}
            columns={columns}
            rows={props.models}
            rowKey={(row) => row.model_name}
          />
        </>
      )}
    </PanelShell>
  )
}
```

- [ ] **Step 4: Rewrite ReportErrorsPanel the same way**

```tsx
export function ReportErrorsPanel(props: ReportErrorsPanelProps): React.JSX.Element {
  const { t } = useTranslation()
  const otherLabel = t('Other')
  const spec = useMemo(() => buildErrorShareSpec(props.errors, otherLabel), [props.errors, otherLabel])
  const columns: ReportTableColumn<ReportErrorRow>[] = [
    { key: 'model', header: t('Model'), render: (row) => row.model_name },
    { key: 'failures', header: t('Failures'), align: 'end', render: (row) => formatNumber(row.failures) },
    { key: 'share', header: t('Share'), align: 'end', render: (row) => `${Math.round(row.share * 10) / 10}%` },
  ]

  return (
    <PanelShell title={t('Failures by model')} description={t('Where the errors in this period came from')}>
      {spec === null ? (
        <EmptyOrLoading loading={props.loading} emptyText={t('No failures in this period.')} />
      ) : (
        <>
          <ReportChart spec={spec} ariaLabel={t('Failures by model')} />
          <ReportTable
            caption={t('Failures by model')}
            columns={columns}
            rows={props.errors}
            rowKey={(row) => row.model_name}
          />
        </>
      )}
    </PanelShell>
  )
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bun test src/features/dashboard/components/reports/`
Expected: PASS.

- [ ] **Step 6: Typecheck, lint, commit**

```bash
cd web && bun run typecheck && bunx oxlint -c .oxlintrc.json src/features/dashboard/components/reports/ && cd ..
git add web/src/features/dashboard/components/reports/
git commit -m "feat(reports): chart model cost and failure share"
```

---

### Task 7: Performance, Token anatomy and Channel panels

**Files:**
- Modify: `web/src/features/dashboard/components/reports/report-performance-panel.tsx`
- Modify: `web/src/features/dashboard/components/reports/report-token-anatomy-panel.tsx`
- Modify: `web/src/features/dashboard/components/reports/report-channel-panel.tsx`
- Modify: `web/src/features/dashboard/components/reports/report-primitives.tsx`
- Modify: `web/src/features/dashboard/components/reports/__tests__/report-panels.test.tsx`

**Interfaces:**
- Consumes: `buildPerformanceSpec`, `buildTokenAnatomySpec`, `buildChannelCostSpec`, `CHANNEL_ERROR_RATE_THRESHOLD` (Task 3).

- [ ] **Step 1: Update the channel and token tests**

Replace the two remaining bar-row cases in `report-panels.test.tsx`:

```tsx
it('channel panel (admin) lists channels with cost and error rate', () => {
  const { queryByText, getAllByRole } = renderNode(
    <ReportChannelPanel
      loading={false}
      channels={[
        { channel_id: 10, channel_name: 'OpenAI', quota: 500, requests: 9, failures: 1, error_rate: 0.1, avg_latency_ms: 700 },
      ]}
    />
  )
  expect(queryByText('OpenAI')).not.toBeNull()
  expect(queryByText(formatQuota(500))).not.toBeNull()
  expect(getAllByRole('row')).toHaveLength(2)
})

it('token anatomy panel lists prompt, completion and cache columns', () => {
  const { queryByText } = renderNode(
    <ReportTokenAnatomyPanel
      loading={false}
      rows={[{ model_name: 'claude', prompt_tokens: 100, completion_tokens: 200, cache_tokens: 50, total: 350 }]}
    />
  )
  expect(queryByText('claude')).not.toBeNull()
  expect(queryByText('100')).not.toBeNull()
  expect(queryByText('200')).not.toBeNull()
  expect(queryByText('50')).not.toBeNull()
})
```

- [ ] **Step 2: Run to verify they fail**

Run: `bun test src/features/dashboard/components/reports/__tests__/report-panels.test.tsx`
Expected: FAIL on the row count and column values.

- [ ] **Step 3: Rewrite the three panels**

Each follows the Task 6 shape: `useMemo` over its builder, `EmptyOrLoading` when the spec is `null`, then `ReportChart` plus `ReportTable`. Columns:

- Performance: `t('Name')` (`row.name`), `t('Avg latency')` (`${formatNumber(row.avg_latency_ms)} ms`), `t('p95 latency')` (`${formatNumber(row.p95_latency_ms)} ms`), `t('Requests')` (`formatNumber(row.requests)`), `t('Throughput')` (`${formatNumber(row.throughput)} tok/s`). Row key: `row.name`. Chart aria-label: `props.title`.
- Token anatomy: `t('Model')`, `t('Prompt')`, `t('Completion')`, `t('Cache')`, `t('Total')` — all counts through `formatNumber`. Row key: `row.model_name`.
- Channels: `t('Channel')` (`row.channel_name`), `t('Cost')` (`formatQuota(row.quota)`), `t('Requests')`, `t('Error rate')` rendered as `${Math.round(row.error_rate * 1000) / 10}%` wrapped in `<span className='text-destructive'>` when `row.error_rate > CHANNEL_ERROR_RATE_THRESHOLD`, `t('Avg latency')`. Row key: `String(row.channel_id)`.

- [ ] **Step 4: Delete BarChartRow**

Remove the `BarChartRowProps` interface, the `toneClass` map, and the `BarChartRow` export from `report-primitives.tsx`. Confirm nothing still imports it:

```bash
cd web && grep -rn "BarChartRow" src/ ; cd ..
```
Expected: no matches (the last consumer, `reports-section.tsx`, is handled in Task 8 — if it still imports `BarChartRow`, do Task 8 first).

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bun test src/features/dashboard/components/reports/`
Expected: PASS.

- [ ] **Step 6: Typecheck, lint, commit**

```bash
cd web && bun run typecheck && bunx oxlint -c .oxlintrc.json src/features/dashboard/components/reports/ && cd ..
git add web/src/features/dashboard/components/reports/
git commit -m "feat(reports): chart latency, token anatomy and channel cost"
```

---

### Task 8: Section layout and panel extraction

**Files:**
- Create: `web/src/features/dashboard/components/reports/report-totals-panel.tsx`
- Create: `web/src/features/dashboard/components/reports/report-stream-panel.tsx`
- Modify: `web/src/features/dashboard/components/reports/reports-section.tsx`

**Interfaces:**
- Produces: `export function ReportTotalsPanel(props: { loading: boolean; summary: ReportSummary }): React.JSX.Element`; `export function ReportStreamPanel(props: { loading: boolean; stream: ReportStreamComparison }): React.JSX.Element`.

- [ ] **Step 1: Move the two inline panels into their own files**

Cut `ReportTotalsPanel` and the inline stream panel out of `reports-section.tsx` into the two new files, exporting them by the names above and adding the copyright header. The stream panel gains a chart: `buildStreamSpec(props.stream, t('Streaming'), t('Non-streaming'))` into `ReportChart`, with a two-row `ReportTable` carrying requests and avg latency for each mode.

- [ ] **Step 2: Apply the responsive grid**

Replace the flat panel stack in `reports-section.tsx` with:

```tsx
<ReportTotalsPanel loading={summaryQuery.isLoading} summary={summary} />
<ReportTrendPanel loading={summaryQuery.isLoading} points={summary.trend.points} />
<div className='grid gap-4 lg:grid-cols-2'>
  <ReportModelPanel loading={summaryQuery.isLoading} models={summary.models} />
  <ReportErrorsPanel loading={summaryQuery.isLoading} errors={summary.errors} />
</div>
<div className='grid gap-4 lg:grid-cols-2'>
  <ReportPerformancePanel
    loading={summaryQuery.isLoading}
    title={t('Model latency')}
    rows={summary.model_performance}
  />
  <ReportTokenAnatomyPanel loading={summaryQuery.isLoading} rows={summary.token_anatomy} />
</div>
{summary.stream_performance && (
  <ReportStreamPanel loading={summaryQuery.isLoading} stream={summary.stream_performance} />
)}
{isAdmin && summary.channels && (
  <ReportChannelPanel loading={summaryQuery.isLoading} channels={summary.channels} />
)}
{isAdmin && summary.channel_performance && (
  <ReportPerformancePanel
    loading={summaryQuery.isLoading}
    title={t('Channel latency')}
    rows={summary.channel_performance}
  />
)}
```

Keep the surrounding `<div className='space-y-4'>`, the range/granularity controls, and the export button untouched.

- [ ] **Step 3: Confirm the file shrank below the size limit**

```bash
cd web && wc -l src/features/dashboard/components/reports/*.tsx ; cd ..
```
Expected: every file under ~200 lines.

- [ ] **Step 4: Typecheck, test, lint, commit**

```bash
cd web && bun run copyright && bun run typecheck && bun test src/features/dashboard/ && bunx oxlint -c .oxlintrc.json src/features/dashboard/components/reports/ && cd ..
git add web/src/features/dashboard/components/reports/
git commit -m "refactor(reports): split section panels and lay them out on a grid"
```

---

### Task 9: Translations and final verification

**Files:**
- Modify: `web/src/i18n/locales/en.json` and the seven other locale files.

- [ ] **Step 1: Add the English source strings**

New keys introduced across Tasks 5–8: `Usage over time`, `One measure at a time, across the selected period`, `No usage in this period.`, `Cost`, `Requests`, `Tokens`, `Failures`, `Model`, `Error rate`, `Avg latency`, `p95 latency`, `Throughput`, `Name`, `Prompt`, `Completion`, `Cache`, `Total`, `Share`, `Other`, `Channel`, `Model latency`, `Channel latency`, `Streaming`, `Non-streaming`, `Failures by model`, `Where the errors in this period came from`, `No failures in this period.`, `Cost per model`, `Cost, requests, tokens, error rate and latency per model` (replaced by `Cost per model, with requests, errors and latency`).

Add any that are not already present to `web/src/i18n/locales/en.json`, then translate them in the other seven locale files. Do not translate `p95`.

- [ ] **Step 2: Sync and verify locale parity**

```bash
cd web
bun run i18n:sync
bun run i18n:check-reports
```
Expected: `i18n:check-reports` prints `All N report UI keys exist in 8 locales.` and exits 0.

- [ ] **Step 3: Validate the categorical palette**

Run the dataviz skill's validator against the first eight colors of `getDashboardChartColors(8)` for both surfaces. If a pair FAILs the CVD separation check, do not repaint the dashboard palette — that is out of scope per the spec. Record the result in the commit message, and confirm the affected charts already carry secondary encoding (legend plus the table below).

- [ ] **Step 4: Full check and production build**

```bash
cd web
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/dashboard/
bun test src/features/dashboard/
bun run build
```
Expected: all clean; build succeeds.

- [ ] **Step 5: Look at every panel**

Run `bun run dev` and check the Reports section as both a regular user and an admin, in light and dark mode, at a narrow width and at `lg`. Confirm: no two panels read alike, no axis label collisions, no horizontal page scroll, and every empty panel shows its dashed empty state rather than a blank chart frame.

- [ ] **Step 6: Commit**

```bash
git add web/src/i18n/locales/
git commit -m "i18n(reports): translate the chart panel strings"
```

---

## Self-Review

- **Spec coverage:** trend selector (Task 5), models bar (6), errors donut (2/6), performance grouped bar (3/7), token anatomy stacked (3/7), channels status bar (3/7), stream paired bar (3/8), layout grid (8), `report-charts.ts` (1–3), `report-chart.tsx` and `report-table.tsx` (4), `BarChartRow` removal (7), `reports-section.tsx` split (8), i18n (9), tests (1–7), empty-safe builders (1–3).
- **Type consistency:** `ReportChartSpec` is defined in Task 1 and used by every later builder; `ReportTableColumn<T>` is defined in Task 4 and used in Tasks 6–8; panel props are unchanged throughout, so `reports-section.tsx` in Task 8 still matches.
- **Known risk:** the grouped-bar `yField: ['name', 'series']` and stacked-bar forms in Task 3 follow the horizontal-bar pattern at `lib/charts.ts:821-846` but are not yet rendered anywhere; Task 5 step 3 and Task 9 step 5 are the checks that catch a wrong axis binding.
