# Hybrid Role Dashboard Design

## Summary

Redesign the dashboard into a role-aware command center. The same dashboard route remains in place, but the overview becomes more useful and more polished for both admins and regular users.

Admins should see operation and finance first: platform health, consumption, channel issues, error/latency signals, top consumers, and recent failures. Regular users should see consumption and integration first: balance runway, API key status, recent usage, recent logs, model usage, and copy-ready integration guidance.

The first implementation should be ambitious enough to add dashboard-specific APIs and smart recommendations, but still incremental: evolve `web/src/features/dashboard` and existing backend data sources instead of replacing the dashboard with a parallel product.

## Goals

- Make the dashboard feel like a premium SaaS command center rather than a simple summary page.
- Give admins a fast operational view of health, spend, failures, and usage concentration.
- Give regular users a developer-focused home for balance, keys, usage, logs, and next actions.
- Add deterministic smart insights that feel helpful without introducing an LLM dependency.
- Keep the current `/dashboard` route and section model so existing navigation remains stable.
- Preserve existing behavior for model analytics, flow analytics, and user analytics.

## Non-goals

- Do not replace the whole dashboard with a separate route or parallel implementation.
- Do not add LLM-generated insights in this phase.
- Do not redesign unrelated pages such as channels, wallet, usage logs, or pricing beyond links/cards embedded in the dashboard.
- Do not change project branding, protected metadata, module paths, or attribution.

## Current Context

The dashboard already has these frontend areas:

- `web/src/features/dashboard/index.tsx` controls dashboard sections: overview, models, flow, and admin-only users.
- `web/src/features/dashboard/components/overview/overview-dashboard.tsx` contains the current overview, setup guide, quick actions, API info, announcements, FAQ, uptime, and admin health panel.
- `web/src/features/dashboard/components/overview/summary-cards.tsx` shows user-centric quota and request summaries using existing quota APIs.
- Existing analytics sections already cover model calls, consumption distribution, traffic flow, and user analytics.

The redesign should keep this structure and split the large overview into smaller, role-aware blocks.

## Proposed User Experience

### Admin overview

The admin overview should prioritize:

1. **Role-aware hero**
   - Platform health status.
   - Period consumption/cost.
   - Error rate and latency summary.
   - Number of active insights or alerts.

2. **Smart insights**
   - High-latency channels.
   - Increased error rate compared with a previous period.
   - Heavy usage concentration in a few users or models.
   - Channels that are disabled, unhealthy, or receiving unexpected traffic.
   - Unusually high spend or request spikes.

3. **Metric grid**
   - Requests.
   - Quota/cost.
   - Tokens.
   - Error rate.
   - Average latency.
   - Active users or top consumer count.

4. **Operational panels**
   - Channel health ranking.
   - Top users/models by spend or request volume.
   - Recent failed requests.
   - Links to channels, logs, models, users, and settings.

### Regular user overview

The regular user overview should prioritize:

1. **Role-aware hero**
   - Balance and runway.
   - Active API keys.
   - Usage today or over the selected recent period.
   - Integration readiness.

2. **Smart insights**
   - Balance likely to run out soon.
   - API key exists but has no traffic.
   - Recent failures need attention.
   - A model dominates cost.
   - Suggested cheaper model for testing when applicable.

3. **Metric grid**
   - Balance.
   - Usage today or last 7 days.
   - Requests.
   - Tokens.
   - Runway.

4. **Developer panels**
   - Copy-ready curl/request example.
   - Recent logs and recent failures.
   - Model usage and cost trend.
   - API key status and quick links.

## Frontend Architecture

Keep `Dashboard` and the current dashboard section registry. Refactor overview internals into smaller components under `web/src/features/dashboard/components/overview/`:

- `dashboard-hero.tsx`
- `smart-insights-panel.tsx`
- `dashboard-metric-grid.tsx`
- `admin-operations-panel.tsx`
- `user-developer-panel.tsx`
- `recent-activity-panel.tsx` if shared by both roles

Add dashboard-specific types to `web/src/features/dashboard/types.ts`, such as:

- `DashboardSummary`
- `DashboardMetric`
- `DashboardInsight`
- `DashboardInsightSeverity`
- `DashboardInsightAction`
- `DashboardRecentActivity`
- `DashboardChannelHealth`
- `DashboardTopEntity`

Add frontend API functions to `web/src/features/dashboard/api.ts`:

- `getDashboardSummary()`
- `getDashboardInsights()`

Use TanStack Query with stable query keys and sensible stale times. Existing setup guide, API info, announcements, FAQ, and uptime panels can remain, but the redesigned hero and insights should appear above them.

All user-facing frontend text must use `useTranslation()` and `t('English key')`. New keys must be synced across supported locale files.

## Backend Architecture

Add dashboard-specific endpoints rather than forcing the frontend to aggregate many unrelated endpoints:

- `GET /api/dashboard/summary`
- `GET /api/dashboard/insights`

Both endpoints should infer the current user's role from auth context:

- Admin responses include global operational and financial data.
- Regular user responses include only that user's own data.

Suggested backend layering:

- Router registers dashboard endpoints under existing authenticated API routes.
- Controller validates the request and calls service functions.
- Service builds summaries and insights from existing model/log/channel data.
- Model layer uses GORM-compatible queries that work on SQLite, MySQL, and PostgreSQL.

Avoid database-specific SQL unless there is an explicit fallback for every supported database.

## Insight Rules

Insights are deterministic in this phase. Each insight should include:

- `id`
- `severity`: `info`, `warning`, or `critical`
- `title`
- `description`
- `metricLabel` and `metricValue` when useful
- Optional `actionLabel` and `actionPath`
- Optional entity references, such as channel ID, user ID, model name, or token/key ID

Initial rules:

### Admin rules

- High channel latency when recent average response time crosses a defined threshold.
- High error rate when recent failures exceed a threshold.
- Usage spike when the current period is materially higher than the previous period.
- Cost concentration when a small number of users or models represent most quota usage.
- Channel attention when a channel is disabled, recently failed, or has stale test/balance data.

### User rules

- Low balance/runway when estimated remaining days are below a threshold.
- No API key or key with no recent traffic.
- Recent failed requests exist.
- Model cost concentration when one model dominates recent usage.
- High usage growth compared with the previous period.

Rules should be implemented in small service-level functions that are easy to test. They should not be over-abstracted into a complex rules engine unless repeated patterns justify it later.

## Data Flow

1. Frontend loads `/dashboard/overview`.
2. `OverviewDashboard` fetches summary and insights using TanStack Query.
3. Backend checks the authenticated user and role.
4. Service builds role-specific summary data from quota logs, request logs, channel state, API keys, and user quota.
5. Frontend renders role-specific hero, metric grid, insights, and panels.
6. Existing panels such as API info, uptime, announcements, and FAQ continue to load independently.

## Error Handling and Empty States

- If summary fails, show an error state with retry and keep existing static/quick-action content visible.
- If insights fail, show a compact non-blocking error or empty state, not a full-page failure.
- If there is no usage data, show onboarding and integration guidance rather than empty charts.
- If admin-only data is unavailable, show partial dashboard data and a warning insight where appropriate.
- Sensitive fields such as token/key values must remain masked; do not expose secrets through dashboard summary endpoints.

## Security and Permissions

- Admin-only metrics must never be returned to regular users.
- User summary endpoints must scope all usage/log data to the current user.
- Do not include raw API keys, channel keys, or other provider credentials in dashboard responses.
- For links to logs, channels, users, or settings, the frontend should only render actions available for the current role.

## Testing Strategy

### Backend

Add deterministic tests for summary and insight builders:

- Admin receives global metrics; regular user receives scoped metrics.
- Low balance insight triggers below the threshold and does not trigger above it.
- High error/latency insights trigger based on explicit fixture data.
- Cost concentration insight triggers when one user/model dominates usage.
- Empty data returns useful defaults and no panic.

Use `require` for setup/fatal assertions and `assert` for non-fatal checks.

### Frontend

Add focused tests where practical:

- Admin overview renders admin panels and does not show user-only primary copy.
- Regular user overview renders developer panels and hides admin-only operational links.
- Insight panel renders severity, action links, empty state, and loading state.

Run frontend build after changes.

## Rollout Plan

1. Add backend DTOs/services/endpoints for summary and insights.
2. Add frontend API functions and types.
3. Split overview into role-aware components.
4. Build admin and user layouts using the new summary/insight data.
5. Preserve existing setup guide and supporting panels, repositioned below the new role-aware sections.
6. Add i18n keys for all new UI text.
7. Add backend and frontend tests.
8. Run verification commands.

## Open Implementation Details

- Exact thresholds should start conservative and be constants near the insight builder.
- Period comparison can begin with a default recent window, such as 24 hours for admin operational signals and 7 days for user consumption signals.
- The first version can use current backend quota/log/channel data. If a metric is expensive to compute, defer it or compute a simpler approximation rather than blocking the redesign.
