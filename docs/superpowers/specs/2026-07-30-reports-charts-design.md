# Reports Charts Design

## Summary

The dashboard Reports section renders every panel with the same primitive, so all seven panels look
identical regardless of what they measure. `BarChartRow` in
`web/src/features/dashboard/components/reports/report-primitives.tsx` is a `<div>` whose width is a
percentage; there is no chart, no axis, no tooltip, and no way for one panel to read differently
from the next.

This is also a departure from the rest of the dashboard, which already charts with VChart
(`@visactor/react-vchart`) driven by spec builders in `web/src/features/dashboard/lib/charts.ts`.

Replace the shared bar primitive with a chart form chosen per panel from the shape of that panel's
data, reusing the dashboard's existing VChart setup and color scheme. Keep the exact numbers
available in a compact table under each chart.

## Goals

- Give each Reports panel a chart form that fits its data, so panels are distinguishable at a glance.
- Reuse the dashboard's existing chart stack and palette instead of adding a parallel one.
- Preserve every number the panels show today, and keep them consistent with the CSV export.
- Keep the reports files within the size and structure conventions in `web/AGENTS.md`.

## Non-goals

- No drill-down or click-to-filter interactions on the charts.
- No change to the dashboard color palette or to `lib/charts.ts`.
- No refactor of the existing `model-charts.tsx`, `user-charts.tsx`, or `flow-charts.tsx`.
- No backend or DTO changes: the report API already returns everything these charts need.

## Current Context

Frontend, under `web/src/features/dashboard/`:

- `components/reports/reports-section.tsx` (248 lines) fetches the summary, renders the range and
  granularity controls plus the CSV export button, and stacks the panels with `space-y-4`. It also
  holds two panels inline: the totals panel and the stream comparison panel.
- Six panel components (`report-trend-panel.tsx`, `report-model-panel.tsx`, `report-errors-panel.tsx`,
  `report-performance-panel.tsx`, `report-channel-panel.tsx`, `report-token-anatomy-panel.tsx`), each
  ~59 lines and each a `PanelShell` wrapping a list of `BarChartRow`.
- `components/reports/report-primitives.tsx` holds `BarChartRow`, `PanelShell`, and `EmptyOrLoading`.
- `lib/charts.ts` builds VChart specs for the other dashboard sections and exports
  `getDashboardChartColors`.
- `components/models/model-charts.tsx` shows the established VChart usage: a lazily imported
  `ThemeManager` switched on `resolvedTheme`, a `chartKey` that remounts the chart when theme or data
  identity changes, and radius from `useThemeRadiusPx`.

Backend `service/report.go` already returns, per range: a time series, per-model rows, per-model
error rows with a `share` field, avg/p95 performance rows, token anatomy rows, stream comparison,
and (admin only) per-channel rows.

## Chart Form Per Panel

One measure per axis. Panels whose data spans measures of different scales get a selector or push the
extra measures into the table; no panel uses a second Y axis.

| Panel | Form | Rationale |
|-|-|-|
| Trend | Time line/area with a metric selector (cost, requests, tokens, failures) | Four measures at different scales; showing one at a time keeps a single axis |
| Models | Horizontal bar ranked by cost | Single-measure ranking; requests, error rate, and latency belong in the table |
| Errors | Donut, top 6 plus "Other" | `share` sums to 100%, a true part-to-whole; more than 8 slices stops being readable |
| Performance | Grouped horizontal bar, avg next to p95 | Two series in the same unit (ms), so they share one axis honestly |
| Token anatomy | Stacked horizontal bar (prompt, completion, cache) | Composition of a total per model |
| Channels (admin) | Horizontal bar ranked by cost, status color once error rate exceeds 5% | Error rate as color plus an icon, never as a second axis |
| Stream | Paired bar, stream against non-stream | Two-group comparison |

Categorical hues come from `getDashboardChartColors`, assigned in fixed order so a series keeps its
color when the row count changes. Series identity is never carried by color alone: every chart with
two or more series ships a legend, and the table below repeats the labels.

## Layout

Today all seven panels are full-width in a single `space-y-4` stack, which is half of why the section
reads as uniform.

- Trend spans the full width at the top.
- Models and Errors sit side by side at `lg`, as do Performance and Token anatomy.
- Channels spans the full width at the bottom, admin only.
- Below `lg` everything collapses to one column in the current order.

## Components

- `lib/report-charts.ts` — pure `data -> VChart spec` builders, one per panel form, mirroring how
  `lib/charts.ts` is organized. Top-N bucketing into "Other", color assignment, and tooltip
  formatting live here. Pure functions so the interesting logic is directly testable.
- `components/reports/report-chart.tsx` — a thin `<VChart>` wrapper owning the light/dark
  `ThemeManager` switch and the `chartKey` remount, following `model-charts.tsx`. Used only by
  Reports.
- `components/reports/report-table.tsx` — the compact exact-value table under each chart.
- `report-primitives.tsx` — keeps `PanelShell` and `EmptyOrLoading`; `BarChartRow` is deleted once no
  panel uses it.
- `reports-section.tsx` — the inline totals and stream panels move to their own files, bringing the
  section back under the ~200-line convention and leaving it responsible for fetching, controls, and
  layout.

Each panel component keeps its current props and becomes: `PanelShell` > `ReportChart` > `ReportTable`.

## Data Flow

Unchanged. `reports-section.tsx` fetches once via React Query and passes slices of the summary to each
panel. Panels stay presentational; they call a spec builder with their rows and hand the spec to
`ReportChart`. Loading and empty states keep using `EmptyOrLoading`, so a panel with no rows shows the
dashed empty box instead of an empty chart frame.

## Error Handling

- A failed summary query keeps the existing section-level message; panels are not rendered.
- An empty row set renders `EmptyOrLoading`, never a chart with no marks.
- Spec builders must tolerate zero rows, a single row, and totals of zero without dividing by zero;
  the donut with a zero total renders as empty rather than as NaN slices.

## i18n

New user-facing strings: metric selector labels on Trend, table column headers, the "Other" bucket,
and the stream/non-stream series labels. English source strings go in `web/src/i18n/locales/en.json`
and are propagated with `bun run i18n:sync`, then verified with `bun run i18n:check-reports`.

## Testing

Under `components/reports/__tests__/` and `lib/__tests__/`, per `web/AGENTS.md`:

- Spec builders (pure, no mocks): top-N collapses the tail into "Other" and the shares still total
  100%; series order follows the ranking; zero rows and a zero total return an empty-safe spec.
- Panels: `@visactor/react-vchart` is mocked at the module boundary, since a canvas chart is exactly
  the uncontrollable boundary mocking is for. Assertions target the table values and the empty state,
  which is what a user can read.
- The existing `report-panels.test.tsx` is rewritten against the table rather than deleted, so the
  formatting coverage it added survives the redesign.

`bun run typecheck` and lint on the touched files before the work is called done.

## Risks

- VChart is already a dependency used by three other dashboard sections, so no bundle growth beyond
  the new specs.
- The theme-manager remount pattern is copied from `model-charts.tsx` rather than shared; if it later
  drifts, the two need reconciling. Accepted here to avoid refactoring working sections.
