# Painel de dashboard e logs inspirados no Bifrost — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Evoluir a seção Relatórios para um painel com abas e rankings, e dar à seção Common Logs resumo, gráfico de volume, status e detalhe lateral, usando apenas dados existentes.

**Architecture:** O backend aditivo estende `service/report.go` (filtros, séries, rankings) e cria `model/log_summary.go` para o resumo de logs. O frontend reorganiza `features/dashboard/components/reports/` em abas e acrescenta componentes próprios em `features/usage-logs/components/`, com o mínimo de alteração em arquivos do upstream.

**Tech Stack:** Go, Gin, GORM; React 19, TypeScript, TanStack Query/Table, VChart, Tailwind, i18next, Vitest, Bun.

**Spec:** `docs/superpowers/specs/2026-11-05-dashboard-logs-visual-design.md`

## Global Constraints

- Não alterar referências ao new-api ou à QuantumNous, licença ou cabeçalhos. Todo arquivo novo leva o mesmo cabeçalho AGPL dos vizinhos.
- Sem dependências novas, tabelas novas ou migrações. Não alterar relay nem cobrança.
- Reutilizar componentes existentes (`PanelShell`, `ReportTable`, `ReportChart`, `components/ui/*`), conforme `AGENTS.md` e `web/AGENTS.md`.
- Gráficos em VChart, via `ReportChart`, com `getDashboardChartColors` e tokens do tema. Sem cores ou fontes fixas.
- Todo texto com `t('English key')`. Números com `@/lib/format`; dinheiro com `formatQuota`/`@/lib/currency`.
- SQL portável para SQLite, MySQL, PostgreSQL e ClickHouse. Sem função de percentil no banco.
- Requisições = `consume + error`. Tokens, custo e latência vêm só de `consume`. Reembolsos não abatem custo.
- Usuário comum só vê os próprios dados; abas Canais e Usuários e filtro de canal são só para admin, com o bloqueio aplicado no backend.
- Comandos Go na raiz do repositório; comandos `bun` em `web/`.
- Commits terminam com as linhas de atribuição da sessão.

## Review Focus

- `ERROR_LOG_ENABLED` vem `false` por padrão, então não há registros de erro: a tela deve avisar que a taxa de sucesso não considera erros, em vez de mostrar 100% sem ressalva (Tasks 5 e 8).
- Clicar em botão, link ou campo copiável dentro de uma linha de log não deve abrir o painel lateral, e a linha deve abrir com o teclado (Task 9).
- Período sem dados, ou modelo com nome vazio, não deve quebrar gráficos nem criar a série "" (Tasks 3 e 6).
- Nomes de usuário ou token começando com `=`, `+`, `-` ou `@` não podem virar fórmula no CSV baixado da aba de ranking (Task 7).
- Usuário comum que chama a API diretamente com `channel=` ou `dimension=user` não pode obter dados de outras pessoas (Tasks 2 e 4).

---

### Task 1: Corrigir totais e unidade de latência dos relatórios

`service/quota.go` grava `use_time` em segundos inteiros, mas `service/report.go` o expõe como `*_latency_ms` sem converter e a tela mostra "ms". As consultas também somam todos os tipos de log.

**Files:**
- Modify: `service/report.go`
- Test: `service/report_test.go`, `controller/report_test.go`

**Interfaces:**
- Produces: campos `*_latency_ms` passam a estar de fato em milissegundos (`use_time * 1000`). `Throughput` é `tokens de consume / segundos somados`.

- [ ] **Step 1: Criar a branch `feat/dashboard-panel-logs` a partir de `production` e commitar a spec e este plano**

- [ ] **Step 2: Escrever os testes que falham**

`TestReportTotalsCountOnlyConsumeAndError`: fixture com um `consume` (Quota 100, tokens 80, UseTime 2), um `error` e um de cada `LogTypeTopup` (Quota 5000), `LogTypeRefund` (Quota 700) e `LogTypeManage`. Assert em `Trend.Totals`: `Requests == 2`, `Failures == 1`, `Consumers == 1`, `Quota == 100`, `Tokens == 80`, `AvgLatencyMs == 2000`.
`TestReportPerformanceUsesMilliseconds`: dois `consume` do mesmo modelo com UseTime 2 e 4 e 30 tokens cada: `AvgLatencyMs == 3000`, `P95LatencyMs == 4000`, `Throughput == 10`.
Repetir o assert de exclusão de `LogTypeTopup` em `Models`, `Channels` e `StreamPerformance`.

- [ ] **Step 3: Rodar e ver falhar**

Run: `go test ./service/ -run 'TestReportTotals|TestReportPerformance' -v`
Expected: FAIL nos asserts de `Requests` e `AvgLatencyMs`.

- [ ] **Step 4: Ajustar `queryReportTrendBuckets`, `computeReportModels`, `computeReportPerformance`, `reportGroupP95`, `computeReportTokenAnatomy`, `computeReportStreamComparison` e `computeReportChannels`**

Adicionar `Where("type in (?, ?)", model.LogTypeConsume, model.LogTypeError)` e somar quota, tokens e `use_time` com `case when type = consume`. Converter para ms ao preencher as linhas. Trocar `(created_at - start) / interval` pela forma com `FLOOR` quando `common.UsingLogDatabase(common.DatabaseTypeMySQL)`, porque `/` não é divisão inteira em MySQL.

- [ ] **Step 5: Rodar e ver passar; ajustar fixtures antigas que tratavam `UseTime` como ms**

Run: `go test ./service/ ./controller/ -run 'Report' -v`
Expected: PASS

- [ ] **Step 6: Commit** — `fix(reports): count only request logs and report latency in milliseconds`

### Task 2: Filtros de modelo e canal no relatório

**Files:**
- Modify: `dto/report.go`, `service/report.go`, `controller/report.go`
- Test: `service/report_test.go`, `controller/report_test.go`

**Interfaces:**
- Produces: `type dto.ReportFilter struct { ModelName string; ChannelID int }`
- Produces: `service.GetReportSummary(userID int, username string, role int, r dto.ReportTimeRange, granularity dto.ReportGranularity, filter dto.ReportFilter) (dto.ReportSummary, error)`
- Produces: query params `model_name` e `channel` em `/api/dashboard/report` e `/report/export`.

- [ ] **Step 1: Escrever os testes que falham**

`TestReportSummaryFiltersByModel`: fixture com `gpt` e `claude`; com `ReportFilter{ModelName: "gpt"}`, `Models` só contém `gpt` e `Trend.Totals` só soma `gpt`.
`TestReportSummaryChannelFilterIgnoredForRegularUser`: usuário comum com `ChannelID` igual ao canal de outra pessoa continua vendo só os próprios registros, e os totais são iguais aos de sem filtro.
`TestGetReportExportAppliesFilters` no controller: `?model_name=gpt-4o` faz o CSV não conter `claude`.

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./service/ ./controller/ -run 'Filter' -v` / Expected: FAIL de compilação na assinatura.

- [ ] **Step 3: Implementar**

Acrescentar `modelName` e `channelID` a `reportRoleScope` e aplicá-los em `apply`, para valerem em todas as consultas. `reportScope` zera `channelID` se não for admin. Atualizar os chamadores existentes de `GetReportSummary`, incluindo testes.

- [ ] **Step 4: Rodar e ver passar** — Run: `go test ./service/ ./controller/ -run 'Report' -v` / Expected: PASS

- [ ] **Step 5: Commit** — `feat(reports): filter reports by model and channel`

### Task 3: Séries de custo por modelo e latência

**Files:**
- Modify: `dto/report.go`, `service/report.go`
- Test: `service/report_test.go`

**Interfaces:**
- Consumes: `reportRoleScope` da Task 2.
- Produces em `dto.ReportSummary`:
  - `CostByModel dto.ReportCostSeries` `json:"cost_by_model"`, com `Models []string` e `Points []dto.ReportCostPoint{BucketLabel string, BucketTimestamp int64, Values map[string]int}`.
  - `LatencySeries []dto.ReportLatencyPoint` `json:"latency_series"`, com `BucketLabel`, `BucketTimestamp`, `P50Ms int`, `P95Ms int`.
- Produces: `P50LatencyMs int` `json:"p50_latency_ms"` em `ReportPerformanceRow`.
- Constante: `dto.ReportCostOthersKey = "__others__"`.

- [ ] **Step 1: Escrever os testes que falham**

`TestReportCostByModelGroupsTopFiveAndOthers`: 7 modelos de custos diferentes, `Models` tem os 5 mais caros mais `__others__`, e a soma de `Values` em todos os pontos é igual a `Trend.Totals.Quota`.
`TestReportCostByModelSkipsEmptyModelName`: registro com `ModelName: ""` não cria a chave `""`.
`TestReportLatencySeriesPercentiles`: UseTime 1, 2, 3, 4 e 100 no mesmo bucket: `P50Ms == 3000`, `P95Ms == 100000`.
`TestReportLatencySeriesOmitsEmptyBuckets`: bucket sem `consume` não gera ponto.
`TestReportSummaryEmptyFixtureSafe` (existente): novos campos vazios, sem erro.

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./service/ -run 'TestReportCost|TestReportLatency' -v` / Expected: FAIL de compilação.

- [ ] **Step 3: Implementar `computeReportCostByModel(scope, start, end, granularity)` e `computeReportLatencySeries(scope, start, end, granularity)`**

O custo agrega por bucket e modelo no banco. A latência lê uma única amostra de `created_at, use_time` limitada por `reportLatencySampleLimit`, mais recente primeiro, e calcula os percentis em Go com o mesmo método do p95 atual. Calcular o p50 de cada linha de desempenho na mesma amostra de `reportGroupP95`, renomeada para `reportGroupPercentiles`.

- [ ] **Step 4: Rodar e ver passar** — Run: `go test ./service/ -run 'Report' -v` / Expected: PASS

- [ ] **Step 5: Commit** — `feat(reports): add cost-by-model and latency percentile series`

### Task 4: Endpoint de ranking por dimensão

**Files:**
- Modify: `dto/report.go`, `service/report.go`, `controller/report.go`, `router/api-router.go`, `model/log.go` (exportar o nome da coluna de grupo)
- Test: `service/report_test.go`, `controller/report_test.go`

**Interfaces:**
- Consumes: `dto.ReportFilter`, `reportRoleScope`.
- Produces: `type dto.ReportDimension string`, com os valores `channel`, `model`, `user`, `token`, `group`.
- Produces: `dto.ReportRankingRow{Name string; Requests, Tokens, Quota, Failures int; ErrorRate float64; AvgLatencyMs, P50LatencyMs, P95LatencyMs int}`, com JSON em snake_case.
- Produces: `service.GetReportRanking(userID int, username string, role int, r dto.ReportTimeRange, dim dto.ReportDimension, filter dto.ReportFilter) ([]dto.ReportRankingRow, error)`.
- Produces: `GET /api/dashboard/report/ranking?range=&dimension=&model_name=&channel=`. Constantes `reportRankingLimit = 50` e `reportRankingSampleLimit = 1000`.

- [ ] **Step 1: Escrever os testes que falham**

`TestReportRankingOrdersByRequestsAndLimits`: 60 tokens distintos retornam 50 linhas, ordenadas por `Requests` desc.
`TestReportRankingRejectsAdminDimensionsForUser`: usuário comum com `user` ou `channel` recebe erro; no controller, `success == false` sem linhas.
`TestReportRankingScopesTokensToUser`: usuário comum com `token` não vê tokens de outra pessoa.
`TestReportRankingRejectsUnknownDimension`: `dimension=foo` retorna erro, sem chegar ao SQL.
`TestReportRankingNamesChannels`: em `channel`, `Name` é o nome do canal, como em `nameChannelPerformanceRows`.

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./service/ ./controller/ -run 'Ranking' -v` / Expected: FAIL de compilação.

- [ ] **Step 3: Implementar**

Mapear a dimensão para a coluna com uma tabela fixa em Go, nunca concatenando o parâmetro. Registrar a rota no grupo `dashboardRoute` e seguir `controller/report.go`, usando `dashboardViewerRole`.

- [ ] **Step 4: Rodar os testes, incluindo a cobertura de rotas de access token**

Run: `go test ./service/ ./controller/ ./router/ -v -run 'Ranking|Report|AccessToken'`
Expected: PASS. Se `TestAccessTokenRouteRulesCoverEveryDashboardRoute` reclamar da rota nova, registrá-la com a mesma regra de `/api/dashboard/report`.

- [ ] **Step 5: Commit** — `feat(reports): add ranking endpoint by dimension`

### Task 5: Endpoint de resumo de logs

**Files:**
- Create: `model/log_summary.go`, `model/log_summary_test.go`
- Modify: `controller/log.go`, `router/api-router.go`
- Test: `controller/log_summary_test.go`

**Interfaces:**
- Produces: `model.LogSummary{Requests, Successes, Failures, Tokens, Quota int; AvgUseTimeSeconds float64; BucketSeconds int64; Buckets []model.LogSummaryBucket}` e `model.LogSummaryBucket{Timestamp int64; Requests, Failures int}`, com JSON em snake_case.
- Produces: `model.GetLogSummary(userID int, filter model.LogFilter) (model.LogSummary, error)`, em que `userID == 0` significa todos.
- Produces: `GET /api/log/summary` (`AdminAuth`) e `GET /api/log/self/summary` (`UserAuth`). A resposta inclui `error_log_enabled` (de `constant.ErrorLogEnabled`).

- [ ] **Step 1: Escrever os testes que falham**

`TestGetLogSummaryCountsConsumeAndError`: 2 `consume` (UseTime 1 e 3), 1 `error` e 1 `LogTypeTopup`: `Requests == 3`, `Successes == 2`, `Failures == 1`, `AvgUseTimeSeconds == 2`, e a quota do top-up não entra.
`TestGetLogSummaryAppliesFilterAndUser`: `ModelName` e `userID` restringem totais e buckets.
`TestGetLogSummaryNonRequestTypeIsEmpty`: `Type: LogTypeManage` retorna zeros sem erro.
`TestGetLogSummaryBucketsSumToRequests`: período de 24 h, a soma de `Buckets[].Requests` é igual a `Requests`, e `len(Buckets) <= 60`.
`TestLogSummarySelfRouteIgnoresUsernameAndChannel`: no controller, `/self/summary?username=bob` por alice retorna só os dados de alice.

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./model/ ./controller/ -run 'LogSummary' -v` / Expected: FAIL de compilação.

- [ ] **Step 3: Implementar**

Reutilizar `LogFilter.apply`, `logFilterFromQuery` e `limitLogTimeRange` (para não admin). Sem início informado, usar as últimas 24 h. `BucketSeconds` é o menor valor de `60, 300, 900, 3600, 21600, 86400` que mantém no máximo 60 buckets. Usar a expressão portável de `rankingBucketExpr`, verificando `common.UsingLogDatabase`.

- [ ] **Step 4: Rodar e ver passar** — Run: `go test ./model/ ./controller/ ./router/ -run 'LogSummary|AccessToken' -v` / Expected: PASS, registrando as rotas nas regras de access token se o teste pedir.

- [ ] **Step 5: Commit** — `feat(logs): add filtered log summary endpoint`

### Task 6: Painel com abas e grade da Visão geral

**Files:**
- Modify: `web/src/features/dashboard/types.ts`, `web/src/features/dashboard/api.ts`, `web/src/features/dashboard/lib/report-charts.ts`, `web/src/features/dashboard/components/reports/reports-section.tsx`, `report-channel-panel.tsx`
- Create: `web/src/features/dashboard/components/reports/report-overview-tab.tsx`, `report-chart-card.tsx`, `report-filters.tsx`
- Test: `web/src/features/dashboard/components/reports/__tests__/report-overview-tab.test.tsx`, `web/src/features/dashboard/lib/report-charts.test.ts`

**Interfaces:**
- Consumes: campos e query params das Tasks 2 e 3.
- Produces: `type ReportTab = 'overview' | 'channels' | 'models' | 'users' | 'tokens' | 'groups'`.
- Produces: `interface ReportFilters { range: ReportTimeRange; granularity: ReportGranularity; modelName?: string; channelId?: number }`; `getReportSummary` e `getReportExportUrl` aceitam esse formato.
- Produces: `buildVolumeSpec`, `buildTokensSpec`, `buildCostByModelSpec` e `buildLatencySeriesSpec`, todos com `(data, labels, chartKind: 'bar' | 'line') => ReportChartSpec | null`.
- Produces: `<ReportChartCard title summary loading isEmpty toggleable>` sobre `PanelShell`, com o seletor barra/linha.

- [ ] **Step 1: Escrever os testes que falham**

Com `vchart-stub.tsx`: a Visão geral renderiza os títulos dos 4 gráficos e os painéis de erros, composição de tokens e streaming abaixo deles; o seletor troca `chartKind`; `buildLatencySeriesSpec([])` retorna `null`; `buildCostByModelSpec` traduz `__others__` para o rótulo recebido e não gera série vazia. Usuário comum não vê abas Canais e Usuários nem o filtro de canal; admin vê. Saúde e latência dos canais só aparecem na aba Canais.

- [ ] **Step 2: Rodar e ver falhar** — Run: `bun run test src/features/dashboard` / Expected: FAIL.

- [ ] **Step 3: Implementar**

`reports-section.tsx` usa `components/ui/tabs` e guarda aba e filtros em estado local. A linha do filtro segue a barra atual; modelo vem de `summary.models` e canal de `summary.channels`. As abas de ranking mostram um espaço reservado até a Task 7. O botão Exportar mantém o CSV.

- [ ] **Step 4: Rodar e ver passar** — Run: `bun run test src/features/dashboard && bun run typecheck && bun run lint` / Expected: PASS

- [ ] **Step 5: Commit** — `feat(dashboard): add tabbed reports panel with chart grid`

### Task 7: Abas de ranking

**Files:**
- Create: `web/src/features/dashboard/components/reports/report-ranking-tab.tsx`, `web/src/features/dashboard/lib/report-ranking.ts`, `web/src/features/dashboard/lib/report-ranking.test.ts`
- Modify: `web/src/features/dashboard/api.ts`, `types.ts`, `components/reports/report-table.tsx`, `reports-section.tsx`
- Test: `web/src/features/dashboard/components/reports/__tests__/report-ranking-tab.test.tsx`

**Interfaces:**
- Consumes: rota da Task 4 e `ReportTab`/`ReportFilters` da Task 6.
- Produces: `getReportRanking(dimension: ReportDimension, filters: ReportFilters)`, `sortRankingRows(rows, key, direction)` e `rankingRowsToCsv(rows, headers): string`.
- `ReportTableColumn` ganha `sortable?: boolean`; `ReportTable`, `sort?` e `onSortChange?`, opcionais e compatíveis com os usos atuais.

- [ ] **Step 1: Escrever os testes que falham**

`rankingRowsToCsv` escapa aspas, vírgulas e quebras de linha, e prefixa com `'` células que começam com `=`, `+`, `-` ou `@`. `sortRankingRows` ordena por qualquer métrica e não altera o array recebido. A aba só busca dados quando está ativa (query `enabled`). Clicar no cabeçalho alterna a ordenação e ajusta `aria-sort`. A aba mostra estado vazio e informa que só as 50 linhas de maior volume aparecem. Exportar baixa o CSV da aba.

- [ ] **Step 2: Rodar e ver falhar** — Run: `bun run test src/features/dashboard` / Expected: FAIL.

- [ ] **Step 3: Implementar a aba e ligá-la a `reports-section.tsx`**, removendo o espaço reservado. Usar `staleTime: 60 * 1000`, como nos relatórios.

- [ ] **Step 4: Rodar e ver passar** — Run: `bun run test src/features/dashboard && bun run typecheck && bun run lint` / Expected: PASS

- [ ] **Step 5: Commit** — `feat(dashboard): add ranking tabs`

### Task 8: Resumo e gráfico de volume na tela de logs

**Files:**
- Create: `web/src/features/usage-logs/components/common-logs-summary.tsx`
- Modify: `web/src/features/usage-logs/api.ts`, `types.ts`, `components/usage-logs-table.tsx`
- Test: `web/src/features/usage-logs/components/__tests__/common-logs-summary.test.tsx`

**Interfaces:**
- Consumes: rotas da Task 5, `buildApiParams` e `useLogsViewScope`.
- Produces: `getLogSummary(params, isAdmin)` e `<CommonLogsSummary />`, sem props, lendo os filtros de `route.useSearch()` como `CommonLogsStats`.

- [ ] **Step 1: Escrever os testes que falham**

Com dados simulados: mostra requisições, taxa de sucesso, tokens, custo e latência; `Requests == 0` mostra `—` na taxa, sem `NaN`. Com `error_log_enabled: false`, aparece o aviso "Error logging is off, so the success rate does not include failures". O custo fica oculto quando os dados sensíveis estão ocultos, como em `CommonLogsStats`. Falha da consulta mostra um estado discreto e não derruba a tabela.

- [ ] **Step 2: Rodar e ver falhar** — Run: `bun run test src/features/usage-logs` / Expected: FAIL.

- [ ] **Step 3: Implementar**

Renderizar a faixa e o gráfico (barras empilhadas de sucesso e erro, via `ReportChart`, com a cor `destructive` do tema) acima da tabela, somente quando `logCategory === 'common'`. Importar `ReportChart` do dashboard, sem duplicá-lo. `CommonLogsStats` (RPM/TPM) permanece.

- [ ] **Step 4: Rodar e ver passar** — Run: `bun run test src/features/usage-logs && bun run typecheck && bun run lint` / Expected: PASS

- [ ] **Step 5: Commit** — `feat(logs): add summary strip and volume chart`

### Task 9: Coluna de status e detalhe lateral

**Files:**
- Modify: `web/src/features/usage-logs/components/dialogs/details-dialog.tsx`, `columns/common-logs-columns.tsx`, `usage-logs-provider.tsx`, `usage-logs-table.tsx`, `web/src/components/data-table/core/data-table-row.tsx` (`onClick` opcional, se ainda não existir)
- Test: `web/src/features/usage-logs/components/__tests__/log-details-sheet.test.tsx`

**Interfaces:**
- Produces: `DetailsDialog` ganha `presentation?: 'dialog' | 'sheet'` (padrão `'dialog'`), trocando só o invólucro externo das linhas 671 e 1412. O corpo atual segue único.
- Produces no contexto: `selectedLog: UsageLog | null` e `setSelectedLog`.

- [ ] **Step 1: Escrever os testes que falham**

Clicar na linha abre o painel com status, IDs, origem, consumo, tempos e erro, nessa ordem, seguidos de cobrança, recarga e auditoria quando existirem. Enter em uma linha focada abre o painel; clicar em `button`, `a`, `input` ou `[data-no-row-click]` dentro da linha não abre. Usuário comum não vê canal nem IP restritos, e `sensitiveVisible=false` continua ocultando os mesmos campos. Status: `type 2` → OK, `type 5` → Erro, demais → rótulo de `getLogTypeConfig`. Trocar de linha atualiza o painel sem fechá-lo. Drawing e Task Logs não mudam.

- [ ] **Step 2: Rodar e ver falhar** — Run: `bun run test src/features/usage-logs` / Expected: FAIL.

- [ ] **Step 3: Implementar**

Adicionar a coluna de status como primeira de `common-logs-columns.tsx`. O botão da coluna Details passa a usar `setSelectedLog` e deixa de renderizar seu próprio diálogo. Renderizar um único `DetailsDialog presentation='sheet'` em `usage-logs-table.tsx`, com largura total no celular. A lista móvel abre o mesmo painel.

- [ ] **Step 4: Rodar e ver passar** — Run: `bun run test src/features/usage-logs && bun run typecheck && bun run lint` / Expected: PASS, incluindo `detail-preview` e `mobile-card`.

- [ ] **Step 5: Commit** — `feat(logs): add status column and side details panel`

### Task 10: Traduções e verificação final

**Files:**
- Modify: `web/src/i18n/locales/*.json`, `web/src/i18n/locales/_reports/*`

- [ ] **Step 1: Rodar a sincronização e traduzir as chaves novas para pt-BR**, seguindo o processo dos outros idiomas em `_reports/`.

Run: `bun run i18n:sync && bun run i18n:check-reports`
Expected: sem chaves faltando.

- [ ] **Step 2: Rodar todas as verificações**

Run: `go test ./... && (cd web && bun run typecheck && bun run lint && bun run test && bun run build)`
Expected: tudo passa. Registrar qualquer falha já existente antes desta branch, sem escondê-la.

- [ ] **Step 3: Verificar no app rodando**

Conferir tema claro e escuro, um preset, celular, admin e usuário comum. Com os mesmos período e filtro, confirmar que requisições, tokens e custo batem entre o painel e a tela de logs. Testar com `ERROR_LOG_ENABLED=true` e `false`.

- [ ] **Step 4: Commit** — `i18n: add dashboard panel and log summary strings`
