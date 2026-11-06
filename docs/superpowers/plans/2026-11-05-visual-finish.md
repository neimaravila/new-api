# Acabamento visual no estilo do Bifrost — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dar à seção Relatórios do dashboard, à seção Common Logs e ao painel de detalhe da chamada o acabamento medido no Bifrost (hierarquia de números, prosa em sans, cor por papel, JSON colorido, tabela densa, badges tingidos, cantos internos menores), sem mudar dados, comportamento nem a identidade do new-api, e sem mudar nenhuma outra tela.

**Architecture:** Um atributo `data-visual-finish` (`reports`, `logs`, `log-detail`) marca a raiz de cada tela do escopo. Um único arquivo, `web/src/styles/visual-finish.css`, define todos os tokens derivados do tema (`--vf-*`, por `color-mix`) e as poucas regras que mexem em componentes compartilhados, sempre com seletor iniciado por esse atributo. Primitivas novas e pequenas em `web/src/components/finish/` (badge tingido, faixa de métricas, barra de status, visualizador de JSON com tokenizador próprio) só são usadas pelas telas do escopo. O resto muda por classes passadas a partir das features.

**Tech Stack:** React 19, TypeScript, Tailwind CSS 4 (`bg-(--var)`), Base UI, i18next, Vitest + Testing Library, Bun; Playwright e ImageMagick (`compare`) só na verificação, a partir do scratchpad.

**Spec:** `docs/superpowers/specs/2026-11-05-visual-finish-design.md`, com as medidas em `docs/superpowers/specs/2026-11-05-visual-finish-measurements.md` (citado abaixo como "M §x.y").

## Global Constraints

- Branch de execução: `feat/visual-finish`, a partir de `production`.
- Só as três telas mudam. Fora: demais seções do dashboard (Visão geral incluída), Drawing e Task Logs, menu lateral, cabeçalho, logo, qualquer outra tela. Componentes de `web/src/components/` não mudam para outras telas.
- Identidade: primária e link do new-api, `--font-ui-sans` e o mono do tema. Nada de Geist nem do verde do Bifrost. O raio do tema (`--radius`, 16px) continua no contorno de páginas e cards de seção.
- Cores novas só por mistura ou opacidade de `--chart-*`, `--success`, `--destructive`, `--warning`, `--info`, `--muted`, `--muted-foreground`, `--border`, `--primary`, `--foreground`. Sem hex, sem `rose-*`/`sky-*` novos. Todas em `web/src/styles/visual-finish.css`.
- Textos e dados não mudam. As únicas chaves de i18n novas são as 4 listadas em "Chaves novas" (ver "Conflitos com a spec", item 4).
- Contraste de texto 4.5:1. Status nunca só por cor: badge com texto, barra com `role='img'` e `aria-label`.
- Desempenho: realce de JSON só até `256 * 1024` caracteres (mesmo limite de `PRETTY_MAX_CHARS`); acima, texto puro. Blocos longos continuam recolhidos.
- Sem dependência nova. O realce não usa `shiki` nem `yace` (os dois geram HTML para `innerHTML`); é um tokenizador próprio que só gera nós de texto. Nunca `dangerouslySetInnerHTML`.
- Arquivos do upstream com edição mínima: `web/src/styles/index.css` (1 linha), `usage-logs/index.tsx` (1 atributo), `usage-logs-table.tsx` (props do `DataTablePage`), `logs-filter-toolbar.tsx` (1 atributo), `timing-metrics-cell.tsx` (1 atributo), `common-logs-filter-bar.tsx` (remover `stats`), `common-logs-columns.tsx` (só a coluna `status`, que é nossa). Nenhum arquivo de `web/src/components/ui/` ou `data-table/` muda.
- Cabeçalho AGPL nos arquivos novos, `t('English key')`, números por `@/lib/format`, props sem desestruturação (`web/AGENTS.md` §3.2), testes em `__tests__/` do módulo.
- Comandos, sempre em `web/`: tipos `bunx -p typescript@5.9 tsc -p tsconfig.app.json --noEmit`; lint `bunx oxlint -c .oxlintrc.json <arquivos alterados>`; testes `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test [caminho]`. `bun run typecheck` e `bun run lint` estão quebrados por erros antigos.
- i18n: chaves direto nos 8 arquivos (`en, fr, ja, pt-BR, ru, vi, zh, zh-TW`), com plurais na forma que cada arquivo já usa (pt-BR e fr com `_many`), depois `bun run i18n:check-reports`. Não usar `i18n:sync`.
- Commits terminam com as linhas de atribuição da sessão.

## Review Focus

- Token longo sem espaço (URL, marcador de base64, string JSON de 2 KB) na prosa ou no JSON: quebra dentro da caixa, sem rolagem horizontal da página; o JSON rola dentro do próprio bloco (Tasks 3 e 5).
- JSON cortado ou que não é JSON (`head_only`, SSE `data: {...}`, string sem fechar): o realce não lança erro e não perde nem duplica caracteres (Task 3).
- Papel fora dos quatro conhecidos (`developer`, vazio, desconhecido): usa o estilo de sistema, nunca fica sem fundo nem quebra (Task 5).
- Modo sensível desligado: a faixa de métricas, os badges e as tentativas continuam mascarando custo, canal e motivo com `••••` (Tasks 4 e 6).
- Drawing Logs e Task Logs usam o mesmo `UsageLogsTable` e o mesmo `LogsFilterToolbar`: ficam sem o atributo e sem nenhuma mudança visual (Tasks 1, 7 e 9).

---

## Decisões fixadas

### Escopo por atributo

- `web/src/components/finish/scope.ts`: `export type FinishScope = 'reports' | 'logs' | 'log-detail'`; `export const FINISH_ATTR = 'data-visual-finish'`.
- Onde o atributo entra:
  - `reports`: `<div>` raiz de `ReportsSection`;
  - `logs`: o `<div className='min-h-0 flex-1'>` de `usage-logs/index.tsx`, com `data-visual-finish={logsFinishScope(activeCategory)}`, que vale `'logs'` só para `common` (React omite `undefined`);
  - `log-detail`: o `SheetContent` de `LogContentPanel`. O painel é portal, então não herda o atributo da lista.
- **Garantia de isolamento:** (1) toda regra de `visual-finish.css` começa por `[data-visual-finish` ou por `.dark [data-visual-finish`, e um teste lê o arquivo e falha se alguma não começar; (2) os tokens `--vf-*` só existem dentro do atributo, então uma classe `bg-(--vf-*)` fora do escopo vira transparente, nunca uma cor errada; (3) os componentes compartilhados só recebem classes por props a partir das features do escopo; (4) a Task 9 compara pixels de Visão geral, Canais e Drawing Logs antes e depois.
- **Camada:** `visual-finish.css` fica fora de `@layer`, importado depois de `theme-presets.css` em `index.css`. Assim as poucas regras de componente compartilhado vencem os utilitários do Tailwind sem `!important`. Por isso as regras são poucas e sempre com seletor de coluna ou `data-slot`.

### Tokens (um só lugar: `web/src/styles/visual-finish.css`)

Tudo sob `[data-visual-finish]`; os percentuais do escuro sob `.dark [data-visual-finish]`.

| Token | Valor claro | Escuro |
|-|-|-|
| `--vf-radius-inner` | `min(var(--radius), 0.625rem)` (10px) | igual |
| `--vf-radius-control` | `min(var(--radius), 0.5rem)` (8px) | igual |
| `--vf-radius-badge` | `min(var(--radius), 0.375rem)` (6px) | igual |
| `--vf-surface` | `var(--card)` | igual (fica uma camada acima de `--background`) |
| `--vf-divider` | `var(--border)` | igual |
| `--vf-tint-fill` / `--vf-tint-edge` | `6%` / `25%` | `16%` / `40%` |
| `--vf-badge-fill` / `--vf-badge-edge` | `10%` / `30%` | `18%` / `40%` |
| `--vf-<tone>` | `success→--success`, `danger→--destructive`, `warning→--warning`, `info→--info`, `primary→--primary`, `neutral→--muted-foreground` | igual |
| `--vf-<tone>-fill` | `color-mix(in oklab, var(--vf-<tone>) var(--vf-badge-fill), transparent)` | igual |
| `--vf-<tone>-edge` | `color-mix(in oklab, var(--vf-<tone>) var(--vf-badge-edge), transparent)` | igual |
| `--vf-<tone>-text` | `color-mix(in oklch, var(--vf-<tone>) 60%, var(--foreground))` | igual (no escuro clareia) |
| `--vf-role-user-{bg,border,dot}` | `--primary` a `--vf-tint-fill` / `--vf-tint-edge` / sólido | igual |
| `--vf-role-tool-{bg,border,dot}` | `--warning`, mesma regra | igual |
| `--vf-role-system-{bg,border,dot}` | `color-mix(in oklab, var(--muted) 50%, transparent)` / `var(--border)` / `var(--muted-foreground)` | igual |
| `--vf-role-assistant-{bg,border,dot}` | `transparent` / `var(--border)` / `var(--chart-3)` | igual |
| `--vf-role-error-{bg,border,dot}` | `--destructive`, mesma regra de user | igual |
| `--vf-json-{key,string,number,literal}` | `color-mix(in oklch, var(--chart-1\|5\|4\|3) 55%, var(--foreground))` | igual |
| `--vf-json-punct` | `var(--muted-foreground)` | igual |

A mistura com `--foreground` escurece no claro e clareia no escuro, então a mesma fórmula atende os dois temas e os presets. Se a medição de contraste da Task 9 der menos de 4.5:1, só os percentuais desta tabela mudam.

### Regras de componente compartilhado em `visual-finish.css`

- `[data-visual-finish='logs'] :is([data-slot='input'], [data-slot='select-trigger'], [data-slot='button'])`: `border-radius: var(--vf-radius-control)`; nos controles da barra de filtros, `font-size: 13px` (M §2.7, §2.8). A regra de 16px no celular de `index.css` continua valendo, porque tem `!important`.
- `[data-visual-finish='logs'] [data-action='reset']`: sem borda, sem fundo e sem sombra (Reset como ghost).
- `[data-visual-finish='logs'] tbody td`: `font-weight: 400`; `td[data-column-id='model_name'] [data-slot='status-badge']` e `td[data-column-id='token_name'] [data-slot='status-badge']`: fundo e borda transparentes, `padding-inline: 0` (texto simples), modelo em mono 12px; `td[data-column-id='created_at'] [data-slot='status-badge']`: 11px `var(--muted-foreground)`.
- `[data-visual-finish='logs'] [data-slot='stream-label']`: badge tingido por `currentColor` (`padding: 1px 6px`, raio `--vf-radius-badge`, fundo `color-mix(in oklab, currentColor var(--vf-badge-fill), transparent)`, borda com `--vf-badge-edge`, 11px/500).
- `[data-visual-finish='reports'] :is([data-slot='button'], [data-slot='select-trigger'], [data-slot='input'])`: raio `--vf-radius-control`.

O restante vai por classes nos componentes das features.

### Primitivas (`web/src/components/finish/`)

| Arquivo | Exporta |
|-|-|
| `scope.ts` | `FinishScope`, `FINISH_ATTR` |
| `tone.ts` | `type TintTone = 'success' \| 'danger' \| 'warning' \| 'info' \| 'neutral' \| 'primary'`; `toneFromStatusVariant(variant: StatusVariant \| null \| undefined): TintTone` (`success, green, light-green, teal → success`; `danger, red → danger`; `warning, amber, orange, yellow → warning`; `info, blue, light-blue, indigo, cyan → info`; resto → `neutral`) |
| `tint-badge.tsx` | `TintBadge(props: { tone: TintTone; children: ReactNode; dot?: boolean; strong?: boolean; className?: string })`: `<span data-slot='tint-badge' data-tone>`, 12px/500, `px-1.5 py-px`, raio `--vf-radius-badge`, borda 1px; `strong` = 11px/600 caixa alta `tracking-[0.04em] px-2 py-0.5` (M §3a.1); `dot` = ponto 6px `aria-hidden` |
| `split-unit.ts` | `splitUnit(formatted: string): { value: string; unit: string }` |
| `metric-strip.tsx` | `MetricStrip(props: { items: MetricItem[]; size: 'md' \| 'lg'; columnsClassName: string; 'data-testid'?: string })`; `type MetricItem = { key: string; label: string; value: string; sub?: ReactNode; testId?: string; mono?: boolean; tone?: TintTone }` |
| `status-bar.tsx` | `StatusBar(props: { tone: TintTone; label: string })`: `<span role='img' aria-label data-tone>` 4x24px, raio 4px, `bg-(--vf-<tone>)` |
| `json-tokens.ts` | `JSON_HIGHLIGHT_MAX_CHARS = 256 * 1024`; `type JsonTokenKind = 'key' \| 'string' \| 'number' \| 'literal' \| 'punct' \| 'space' \| 'other'`; `type JsonToken = { kind: JsonTokenKind; text: string }`; `tokenizeJson(text: string): JsonToken[] \| null` |
| `json-view.tsx` | `JsonView(props: { text: string; collapsedLines?: number; className?: string })` |

- `MetricStrip`: contêiner `grid overflow-hidden rounded-(--vf-radius-inner) border bg-(--vf-surface)` + `columnsClassName`. Cada célula (`data-slot='metric-cell'`) desenha a própria divisória com `shadow-[1px_0_0_var(--vf-divider),0_1px_0_var(--vf-divider)]`; as da borda externa são cortadas pelo `overflow-hidden`, então a grade pode quebrar em 2, 3 ou 5 colunas sem buracos. Rótulo 11px/500 caixa alta `tracking-[0.05em] text-muted-foreground`. Valor `tabular-nums` sans (mono quando `mono`), `md` = 18px/600 com célula `px-3 py-2.5`, `lg` = 24px/600 `leading-8 tracking-[-0.02em]` com célula `px-[18px] py-4` (M §2.6, §3a.3). Valor em `<span data-slot='metric-value' data-testid={testId}>`, com a unidade de `splitUnit` em `<span data-slot='metric-unit'>` a `0.6em text-muted-foreground` dentro dele. `sub` em uma linha 11px muted `truncate`. `tone` pinta só o valor com `text-(--vf-<tone>-text)`.
- `splitUnit` separa só um sufixo `ms`, `s` ou `%` precedido de dígito e espaço opcional (`\s`, ` `, ` `), e só se o resto não tem letras. Exemplos fixos nos testes.
- `tokenizeJson`: lexer de uma passada, total (nunca lança) e sem perdas (`tokens.map(t => t.text).join('') === text`). String vai até a aspa sem escape ou até o fim do texto. Uma string seguida de espaços e `:` é `key`. Número por `-?\d+(\.\d+)?([eE][+-]?\d+)?`; `true`, `false`, `null` são `literal`; `{}[],:` são `punct`; espaço em branco é `space`; o resto, um caractere por vez agrupado até o próximo reconhecível, é `other`. Retorna `null` com `text.length > JSON_HIGHLIGHT_MAX_CHARS`.
- `JsonView`: `<pre data-text-style='json' data-highlighted={'true'|'false'}>` em mono `text-[12.5px] leading-[1.5]`, `p-3`, raio `--vf-radius-control`, `overflow-x-auto`, `whitespace-pre` (rolagem horizontal em vez de `break-all`, M §3g). Cada token vira `<span data-token={kind} className=...>{text}</span>`, com cor `text-(--vf-json-*)`; `space` e `other` sem cor. Com `collapsedLines` e mais linhas que isso, só as primeiras `collapsedLines` linhas são tokenizadas e mostradas, e um rodapé de 28px `bg-muted/50 text-xs` com `Button variant='ghost'` alterna `t('Show more')`/`t('Show less')`. O limite de 256 KiB vale para o texto que vai ser tokenizado.

### Prosa e mono na conversa

`content-format.ts` ganha `splitFences(text: string): { kind: 'prose' | 'code'; text: string }[]` (cercas de três crases em início de linha; cerca sem fechamento vai até o fim como `code`) e `looksLikeJson(text: string): boolean` (`trim()` começa com `{` ou `[`, tamanho até `PRETTY_MAX_CHARS` e `JSON.parse` sem erro).

| Bloco | Exibição | `data-text-style` |
|-|-|-|
| `text`, `thinking`, `error` | `looksLikeJson` → `JsonView` do texto formatado; senão segmentos de `splitFences`: prosa em `font-sans text-[13px] leading-[1.6] [overflow-wrap:anywhere] whitespace-pre-wrap`, código em mono 12.5px `leading-[1.5]` com fundo `bg-muted/50` e raio `--vf-radius-control` | `prose` / `mono` / `json` |
| `tool_call` | caixa própria: cabeçalho com nome em mono 13px/600, `id` em mono 11px muted e cópia por ícone, `border-b`; corpo `JsonView` de `prettyJsonText(arguments)` com `collapsedLines={12}`; raio 8px (M §3d) | `json` |
| `tool_result` | `looksLikeJson` → `JsonView`; senão mono 12.5px | `json` / `mono` |

A regra de recolher `LONG_TEXT_CHARS` continua igual para prosa e mono.

### Chaves novas

| Chave | Uso |
|-|-|
| `{{count}} lines` (plural) | metadado do bloco de mensagem |
| `Timing, request and billing details` | resumo à direita de "Details" |
| `Final attempt succeeded after {{count}} failures` (plural) | banner de retentativa |
| `All {{count}} attempts failed` (plural) | banner quando nenhuma tentativa respondeu 2xx |

## Conflitos com a spec

1. **Seção 1 das medidas é a Visão geral.** As medidas de hero, cartões de métrica e "Channel health" (M §1, `nx-dashboard.png`) vêm de `/dashboard/overview`, e a spec manda aplicá-las ("hero e cards do dashboard, lista de saúde dos canais como linhas"). Mas a mesma spec põe as "demais seções do dashboard" fora e exige provar que a Visão geral não mudou. **Recomendação, seguida pelo plano:** aplicar a lógica de M §1 aos equivalentes dentro de Relatórios (faixa de totais, figuras dos cards de gráfico, `PanelShell`, tabela de canais com status em badge) e não tocar em `components/overview/`.
2. **`content-format.ts` não classifica conteúdo.** M §3c.1 diz que ele "já classifica o conteúdo"; hoje ele só formata JSON. **Plano:** classificar pelo tipo do bloco mais `looksLikeJson` e `splitFences`, novos em `content-format.ts`.
3. **Prompt de sistema.** O Bifrost mostra o system em mono; a spec manda prosa em sans. Segue a spec: system em sans, com o fundo de sistema.
4. **"Textos não mudam" x textos que as medidas acrescentam.** M §3a.4, §3c.2 e §4 pedem resumo no "Details", metadado de linhas e banner de retentativa. **Recomendação:** aceitar as 4 chaves de "Chaves novas", porque descrevem dados que a tela já tem, sem mudar dado ou comportamento. O contador de tokens estimado ("~23 tokens") fica de fora, porque seria um número novo inventado pela tela.
5. **O status do cabeçalho já é `StatusBadge`.** M §3a.1 o mede como "texto solto" porque o tipo `badge` de `StatusBadge` não tem fundo. `StatusBadge` é compartilhado e não muda; o painel passa a usar `TintBadge`.
6. **Tema escuro com página mais escura (M §3c.4: `#202020` sobre `#141414`).** Escurecer a página mudaria o shell. **Plano:** só o painel sobe uma camada (`bg-(--vf-surface)`, que é `--card`).
7. **A comparação original não tem Relatórios.** As capturas de "dashboard" foram da Visão geral. A Task 9 fotografa `/dashboard/reports` antes e depois, e o Bifrost `dashboard` serve só de referência de estilo.

---

### Task 1: Escopo, tokens e CSS isolado

**Files:**
- Create: `web/src/components/finish/scope.ts`, `web/src/styles/visual-finish.css`, `web/src/components/finish/__tests__/scoped-css.test.ts`, `web/src/features/usage-logs/lib/visual-finish.ts`, `web/src/features/usage-logs/lib/__tests__/visual-finish.test.ts`
- Modify: `web/src/styles/index.css` (uma linha `@import './visual-finish.css';` depois de `theme-presets.css`)

**Interfaces:**
- Produces: `FinishScope`, `FINISH_ATTR` (acima); todos os tokens `--vf-*` da tabela; `logsFinishScope(category: LogCategory): FinishScope | undefined`

- [ ] **Step 1: Criar a branch `feat/visual-finish` a partir de `production`, commitar a spec, as medidas e este plano, e guardar o binário atual da demo**

Run: `cp /Users/neimaravila/qwen3/new-api-demo/new-api /Users/neimaravila/qwen3/new-api-demo/new-api-production`
Expected: cópia criada. Ela é o "antes" da Task 9; nenhuma task anterior à 9 recompila a demo.

- [ ] **Step 2: Escrever os testes que falham**

`scoped-css.test.ts` (lê o arquivo com `readFileSync`, remove comentários e blocos `@media`/`@supports` só para iterar os seletores internos):
- `every rule in visual-finish.css is scoped to the finish attribute`: cada seletor de cada regra, separado por vírgula, começa por `[data-visual-finish` ou `.dark [data-visual-finish`.
- `visual-finish.css uses no fixed colors`: nenhuma ocorrência de `/#[0-9a-f]{3,8}\b/i`, `rgb(`, `hsl(`, `oklch(` ou `oklab(` (as misturas usam `color-mix(in oklab, var(--…) …)`, que não casa com `oklab(`).
- `defines every token light and dark`: `--vf-tint-fill`, `--vf-tint-edge`, `--vf-badge-fill`, `--vf-badge-edge` aparecem nos dois blocos.

`visual-finish.test.ts`:
- `logsFinishScope marks only the common logs`: `common → 'logs'`, `drawing → undefined`, `task → undefined`.

- [ ] **Step 3: Rodar e ver falhar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/components/finish src/features/usage-logs/lib/__tests__/visual-finish.test.ts` / Expected: FAIL (arquivos inexistentes).

- [ ] **Step 4: Implementar** os tokens da tabela e as regras de "Regras de componente compartilhado", `scope.ts`, `logsFinishScope` e o `@import`.

- [ ] **Step 5: Rodar e ver passar** — mesmo comando / Expected: PASS. Depois tipos e lint dos arquivos novos.

- [ ] **Step 6: Commit** — `feat(finish): scoped visual tokens for the report, logs and detail screens`

### Task 2: Badge tingido, faixa de métricas e barra de status

**Files:**
- Create: `web/src/components/finish/tone.ts`, `tint-badge.tsx`, `split-unit.ts`, `metric-strip.tsx`, `status-bar.tsx`, `index.ts`
- Test: `web/src/components/finish/__tests__/tint-badge.test.tsx`, `split-unit.test.ts`, `metric-strip.test.tsx`, `status-bar.test.tsx`

**Interfaces:**
- Consumes: tokens `--vf-<tone>-{fill,edge,text}`, `--vf-radius-*`, `--vf-surface`, `--vf-divider` (Task 1)
- Produces: `TintTone`, `toneFromStatusVariant`, `TintBadge`, `splitUnit`, `MetricStrip`, `MetricItem`, `StatusBar`, exportados também por `index.ts`

- [ ] **Step 1: Escrever os testes que falham**

- `splitUnit splits only a trailing ms, s or percent`: `'1.2s' → {value:'1.2', unit:'s'}`; `'350 ms' → {'350','ms'}`; `'12.5%' → {'12.5','%'}`; `'12,5 %' → {'12,5','%'}`; `'1m 3s' → {'1m 3s',''}`; `'$0.5885' → {'$0.5885',''}`; `'-' → {'-',''}`; `'••••' → {'••••',''}`.
- `toneFromStatusVariant maps status variants to tones`: `green → success`, `red → danger`, `amber → warning`, `blue → info`, `purple → neutral`, `undefined → neutral`.
- `TintBadge shows its text with the tone and an optional hidden dot`: `getByText('OK · 200').closest('[data-slot=tint-badge]')` tem `data-tone='success'`; com `dot`, há um filho `aria-hidden='true'`; sem `dot`, nenhum.
- `MetricStrip renders one cell per item with value and unit apart`: 5 itens → 5 `[data-slot=metric-cell]`; item `{value:'1.2s', testId:'lat'}` → `getByTestId('lat')` tem `toHaveTextContent('1.2s')` e contém `[data-slot=metric-unit]` com texto `s`; rótulo visível com o texto passado.
- `MetricStrip keeps a masked value whole`: `value: '••••'` → nenhum `[data-slot=metric-unit]`.
- `StatusBar is an image named by its label`: `getByRole('img', { name: 'Error' })` com `data-tone='danger'`; `queryByText('Error')` é `null`.

- [ ] **Step 2: Rodar e ver falhar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/components/finish` / Expected: FAIL.

- [ ] **Step 3: Implementar** conforme "Primitivas". Classes de tom num mapa estático `Record<TintTone, string>` (o Tailwind precisa das strings inteiras).

- [ ] **Step 4: Rodar e ver passar** — mesmo comando / Expected: PASS. Tipos e lint.

- [ ] **Step 5: Commit** — `feat(finish): tinted badge, metric strip and status bar`

### Task 3: Realce de JSON sem HTML

**Files:**
- Create: `web/src/components/finish/json-tokens.ts`, `json-view.tsx`
- Test: `web/src/components/finish/__tests__/json-tokens.test.ts`, `json-view.test.tsx`

**Interfaces:**
- Consumes: tokens `--vf-json-*`, `--vf-radius-control` (Task 1)
- Produces: `JSON_HIGHLIGHT_MAX_CHARS`, `JsonTokenKind`, `JsonToken`, `tokenizeJson`, `JsonView` (também em `index.ts`)

- [ ] **Step 1: Escrever os testes que falham**

`json-tokens.test.ts`:
- `tokenizeJson is lossless`: para `{"a\"b": [1, -1.5e+3, true, null], "c": "é ok"}\n`, para `data: {"x":1}\n\n` e para `{"cut": "abc` → `join('')` igual à entrada.
- `tokenizeJson tells keys from string values`: `{"k": "v"}` → kinds `punct, key, punct, space, string, punct`.
- `tokenizeJson reads numbers and literals`: `[-0.5e2,true,false,null]` → `number, literal, literal, literal` entre `punct`.
- `tokenizeJson keeps an unterminated string as one token`: `{"cut": "abc` → último token `{kind:'string', text:'"abc'}`.
- `tokenizeJson stops at the size limit`: `'a'.repeat(JSON_HIGHLIGHT_MAX_CHARS)` não é `null`; `+1` caractere é `null`.

`json-view.test.tsx`:
- `JsonView colors small JSON with token spans`: `container.querySelectorAll('[data-token=key]').length === 1` para `{"k":1}`; `pre` com `data-highlighted='true'` e `data-text-style='json'`.
- `JsonView shows plain text above 256 KiB`: texto de `JSON_HIGHLIGHT_MAX_CHARS + 1` → nenhum `[data-token]`, `data-highlighted='false'`, `textContent` igual à entrada.
- `JsonView keeps markup in strings as text`: `{"x":"<script>alert(1)</script><img src=x onerror=alert(1)>"}` → `container.querySelector('script, img')` é `null` e o `textContent` contém `<script>`.
- `JsonView scrolls long lines instead of breaking words`: o `pre` não tem a classe `break-all` e tem `overflow-x-auto`.
- `JsonView folds after collapsedLines and expands`: 20 linhas com `collapsedLines={12}` → `textContent` sem a linha 13; clicar `Show more` mostra a linha 20 e o botão vira `Show less`.

- [ ] **Step 2: Rodar e ver falhar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/components/finish` / Expected: FAIL.

- [ ] **Step 3: Implementar** conforme "Primitivas". Tokenizar dentro de `useMemo` com dependência no texto mostrado.

- [ ] **Step 4: Rodar e ver passar** — mesmo comando / Expected: PASS. Tipos e lint.

- [ ] **Step 5: Commit** — `feat(finish): bounded JSON highlighting with text nodes only`

### Task 4: Painel de detalhe — escopo, cabeçalho, faixa de métricas, Details e abas

**Files:**
- Modify: `web/src/features/usage-logs/components/log-content/log-content-panel.tsx`, `log-content-header.tsx`, `log-content-stats.tsx`
- Modify: os 8 arquivos de `web/src/i18n/locales/` (`Timing, request and billing details`)
- Test: `web/src/features/usage-logs/components/log-content/__tests__/log-content-panel.test.tsx`

**Interfaces:**
- Consumes: `TintBadge`, `toneFromStatusVariant`, `MetricStrip`, `FINISH_ATTR` (Tasks 1 e 2)
- Produces: `SheetContent` com `data-visual-finish='log-detail'`; contador de aba em `<span data-slot='tab-count' aria-hidden='true'>`

Decisões:
- `SheetContent`: atributo de escopo e `bg-(--vf-surface)`.
- Cabeçalho (M §3a.1, §3a.2): status e HTTP num só `TintBadge strong dot`, com texto `` `${t(status.label)} · ${httpStatus}` `` quando houver HTTP, senão só o status; tom de `toneFromStatusVariant(status.color)`, e `danger`/`warning` por `httpVariant` quando o HTTP for 4xx/5xx. Formato e canal em `TintBadge tone='neutral'`, stream em `tone='info'`. O título continua `SheetTitle`, agora 13px/500. O request ID vira uma linha com rótulo `t('Request ID')` 11px/600 caixa alta muted, valor mono 13px `text-foreground` e `CopyButton` só ícone de 16px com `aria-label={t('Copy')}`.
- Tentativas (M §4): seta `→` `aria-hidden` entre os botões; o código HTTP dentro do botão em `text-(--vf-danger-text)` se `>= 400`, senão `text-(--vf-success-text)`; o resto do botão neutro.
- `LogContentStats` vira `MetricStrip size='md' columnsClassName='grid-cols-2 sm:grid-cols-3 lg:grid-cols-5'`, com o `data-testid='log-content-stats'` no contêiner. Latência com `tone='success'` quando o log é consume; modelo com `mono`. `sub` em uma linha unida por ` · `: latência `First token: x`; modelo `Group: x`; tokens `Output Tokens: x`, e `Cache Read`/`Cache Write` só quando `> 0`; custo sem `sub`; tools `Calls: x`.
- "Details": o `Collapsible` ganha contêiner `rounded-(--vf-radius-control) border px-4 py-2.5`, e o gatilho mostra à direita `t('Timing, request and billing details')` em 12px muted.
- Abas: `TabsTrigger` recebe `className='text-muted-foreground data-active:text-foreground'` e, em Conversa, Tools e Roteamento, um contador `ml-1.5 rounded-(--vf-radius-badge) bg-muted px-1.5 text-[11px] font-medium tabular-nums`, com `view.conversation.messages.length`, `view.tools.length` e `attempts.length`. JSON bruto sem contador. O contador é `aria-hidden`, então o nome acessível da aba continua igual e os testes atuais seguem valendo.

- [ ] **Step 1: Escrever os testes que falham** (em `log-content-panel.test.tsx`, com as fixtures que o arquivo já usa)

- `marks the panel with the log-detail finish scope`: `document.querySelector('[data-slot=sheet-content]')` tem `data-visual-finish='log-detail'`.
- `shows status and HTTP code in one tinted badge`: consume com `http_status: 200` → `getByText('OK · 200').closest('[data-slot=tint-badge]')` com `data-tone='success'`; erro com 500 → `'Error · 500'` e `data-tone='danger'`.
- `labels the request id and offers to copy it`: `getByText('Request ID')` visível; `getByRole('button', { name: 'Copy' })` dentro de `log-content-header`.
- `shows the five figures in one strip`: `getByTestId('log-content-stats')` tem `data-slot='metric-strip'` e 5 `[data-slot=metric-cell]`; o texto `Cache Read` não aparece quando `cache_tokens` é 0.
- `keeps the cost masked in the strip when sensitive data is hidden`: `sensitiveVisible=false` → a célula Cost tem `••••` e nenhum `[data-slot=metric-unit]`.
- `shows message, tool and attempt counts on the tabs`: como admin, com 5 mensagens, 2 tools e 2 tentativas → `within(getByRole('tab', { name: 'Conversation' })).getByText('5')`, `Tools` com `2`, `Routing` com `2`; `getByRole('tab', { name: 'Raw JSON' })` sem `[data-slot=tab-count]`.
- `puts an arrow between attempts and colors only the failed code`: 2 tentativas (500, 200) → um `→` entre os botões; o `500` tem `text-(--vf-danger-text)` e o nome do canal não.

- [ ] **Step 2: Rodar e ver falhar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/features/usage-logs/components/log-content` / Expected: FAIL nos testes novos.

- [ ] **Step 3: Implementar**, e a chave nova nos 8 arquivos.

- [ ] **Step 4: Rodar e ver passar** — mesmo comando / Expected: PASS em todo o diretório. Ajustar testes antigos que procuravam `getByText('1.2s')` para `toHaveTextContent` no `data-testid`. Tipos e lint.

- [ ] **Step 5: Commit** — `feat(logs): detail header badges, metric strip and tab counts`

### Task 5: Conversa — prosa, cor por papel, linha do tempo e tool call

**Files:**
- Modify: `web/src/features/usage-logs/components/log-content/content-format.ts`, `message-block.tsx`, `conversation-tab.tsx`
- Modify: os 8 arquivos de locale (`{{count}} lines`)
- Test: `web/src/features/usage-logs/components/log-content/__tests__/content-format.test.ts` (novo), `conversation-tab.test.tsx`

**Interfaces:**
- Consumes: `JsonView`, `PRETTY_MAX_CHARS`, `prettyJsonText`, tokens `--vf-role-*` (Tasks 1 e 3)
- Produces: `splitFences(text: string): { kind: 'prose' | 'code'; text: string }[]`, `looksLikeJson(text: string): boolean`; `LongText(props: { text: string; variant: 'prose' | 'mono'; className?: string })` (a prop `variant` passa a ser obrigatória e os chamadores de Tools e JSON bruto a recebem nas Tasks 5 e 6); `ROLE_STYLE: Record<'user' | 'assistant' | 'tool' | 'system' | 'error', string>` em `message-block.tsx`, com `roleStyle(role: string, hasError: boolean)` (erro vence; `developer`, vazio e desconhecido caem em `system`)

Decisões (M §3c, §3d):
- `MessageBlock`: `section` com `group`, raio `--vf-radius-inner`, `border p-3` e as classes de `ROLE_STYLE` (`bg-(--vf-role-user-bg) border-(--vf-role-user-border)` etc.). O rótulo do papel sai da caixa: linha acima com o nome 12px/600 `capitalize`, `t('{{count}} lines', { count })` em 11px muted (linhas de `messageText`), a pílula "Response" vira `TintBadge tone='primary'`, e a cópia vira ícone de 14px `opacity-0 group-hover:opacity-100 focus-visible:opacity-100` com `aria-label={t('Copy')}`.
- Linha do tempo: a lista de `ConversationTab` em `relative pl-5`, com um traço de 1px `bg-(--vf-divider)` à esquerda; cada mensagem tem um ponto de 6px `bg-(--vf-role-<papel>-dot)` `aria-hidden` alinhado ao rótulo.
- Os chips de filtro por papel continuam, com `rounded-(--vf-radius-badge)`.
- Exibição por bloco conforme a tabela de "Prosa e mono na conversa".

- [ ] **Step 1: Escrever os testes que falham**

`content-format.test.ts`:
- `splitFences separates fenced code from prose`: ``'a\n```js\nx=1\n```\nb'`` → `[prose 'a\n', code '```js\nx=1\n```\n', prose 'b']` (a junção dos textos é igual à entrada).
- `splitFences treats an unclosed fence as code to the end`.
- `looksLikeJson accepts objects and arrays only`: `'{"a":1}'` e `' [1] '` → `true`; `'hello'`, `'{bad'`, `'1'` → `false`; texto de `PRETTY_MAX_CHARS + 1` → `false`.

`conversation-tab.test.tsx` (fixture `view-openai.json`):
- `renders user and assistant text in the prose font`: o texto da fala do usuário fica num elemento com `data-text-style='prose'` e classe `font-sans`, sem `font-mono`.
- `renders a fenced code segment inside prose in mono`: mensagem com cerca → o trecho fica em `data-text-style='mono'` e o texto em volta em `prose`.
- `renders tool call arguments as highlighted JSON`: o bloco de tool call tem `[data-text-style=json][data-highlighted=true]` e o nome da função em mono no cabeçalho.
- `renders a JSON tool result as JSON and a plain one in mono`.
- `gives each role its own style and falls back to system`: `data-role='user'` tem classe `bg-(--vf-role-user-bg)`; uma mensagem `developer` e uma com papel `'weird'` têm `bg-(--vf-role-system-bg)`; mensagem com bloco `error` tem `bg-(--vf-role-error-bg)`.
- `shows the line count beside the role`: mensagem de 3 linhas → `3 lines`.
- `keeps HTML in message text literal`: texto `<b>x</b>` → nenhum elemento `b` e o texto visível.
- `wraps a long unbroken token inside the box`: o elemento `prose` tem a classe `[overflow-wrap:anywhere]`.

- [ ] **Step 2: Rodar e ver falhar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/features/usage-logs/components/log-content` / Expected: FAIL.

- [ ] **Step 3: Implementar**, e a chave nova nos 8 arquivos.

- [ ] **Step 4: Rodar e ver passar** — mesmo comando / Expected: PASS. Tipos e lint.

- [ ] **Step 5: Commit** — `feat(logs): prose conversation with role colors and highlighted tool calls`

### Task 6: Tools, Roteamento e JSON bruto

**Files:**
- Modify: `web/src/features/usage-logs/components/log-content/tools-tab.tsx`, `routing-tab.tsx`, `raw-json-tab.tsx`, `log-content-panel.tsx` (monta o `RetryBanner`)
- Modify: os 8 arquivos de locale (`Final attempt succeeded after {{count}} failures`, `All {{count}} attempts failed`)
- Test: `__tests__/tools-tab.test.tsx`, `routing-tab.test.tsx`, `raw-json-tab.test.tsx`

**Interfaces:**
- Consumes: `TintBadge`, `JsonView`, `LongText` com `variant`, `CopyButton` (Tasks 2, 3 e 5)
- Produces: `RetryBanner(props: { attempts: LogContentAttempt[] })` em `routing-tab.tsx`, usado também pelo painel acima das abas quando houver mais de uma tentativa (admin)

Decisões:
- **Tools (M §3e):** `grid gap-2 sm:grid-cols-2`. Cartão compacto `rounded-(--vf-radius-inner) border p-3`: tile de 28px `bg-(--vf-warning-fill)` com ícone `Wrench` `aria-hidden`, nome mono 13px/500, descrição `LongText variant='prose'` 12px muted. O esquema fica num `Collapsible` fechado por padrão, com o gatilho `aria-label` igual ao nome da tool e chevron; aberto, `JsonView` com `collapsedLines={24}` e a cópia. Chamadas: com `called === 0`, texto 11px muted com a chave atual `Called {{calls}} times`; com mais, `TintBadge tone='info'` com o mesmo texto.
- **Roteamento (M §3f, §4):** fatos com rótulo 11px/500 caixa alta. Tentativas numa `ol` com `divide-y` dentro de um cartão `rounded-(--vf-radius-inner) border`; número num círculo de 20px `bg-muted text-[11px]`; HTTP em `TintBadge strong` (`danger` se `>= 400` ou com erro, senão `success`); duração à direita em mono 12px `tabular-nums`. Motivo em 13px `text-muted-foreground`, sem vermelho no parágrafo, com rótulo `Reason`. Os `••••` atuais continuam.
- **Banner:** `RetryBanner` só com `attempts.length > 1`; texto `Final attempt succeeded after {{count}} failures` (count = tentativas antes da última) com `TintBadge tone='success'` quando a última tem status `< 400` e sem erro; senão `All {{count}} attempts failed` com `tone='danger'`.
- **JSON bruto (M §3g):** cada corpo num cartão `rounded-(--vf-radius-inner) border`, cabeçalho 13px/500 com tamanho e cópia; corpo com `JsonView collapsedLines={12}` quando o texto mostrado vem de `prettyJsonText` e passou no parse, ou quando é SSE/cortado (o lexer tolera); corpo `undecodable` continua sem texto. Sai o `break-all`. A cópia continua copiando `body.text` original.

- [ ] **Step 1: Escrever os testes que falham**

Tools:
- `starts with schemas folded and opens one on demand`: `queryByText(/"type"/)` é `null`; clicar no gatilho com nome `consultar_pedido` mostra o esquema em `[data-text-style=json]`.
- `shows an unused tool's call count as muted text`: `called: 0` → o texto `Called 0 times` fora de `[data-slot=tint-badge]`; `called: 2` → dentro de um com `data-tone='info'`.
- O teste existente `lists name, description, schema and call count, and copies the schema` passa a abrir o esquema antes de copiar.

Roteamento:
- `lists attempts with a tinted status and the duration aligned right`: tentativa 500 → `getByText('500').closest('[data-slot=tint-badge]')` com `data-tone='danger'`; `250 ms` em mono.
- `shows the failure reason in muted text`: o parágrafo do motivo não tem `text-destructive`.
- `summarizes a retry that ended well`: tentativas `[500, 200]` → `Final attempt succeeded after 1 failure` (forma `_one` em inglês) e `data-tone='success'`.
- `summarizes a call where every attempt failed`: `[500, 502]` → `All 2 attempts failed`, `data-tone='danger'`.
- `keeps routing details masked when sensitive data is hidden`: canal e motivo `••••`.

JSON bruto:
- `highlights a formatted body and folds it after 12 lines`: corpo de 30 linhas → `[data-highlighted=true]` e `Show more`.
- `does not highlight a body above 256 KiB`: corpo de `256 * 1024 + 1` caracteres → `data-highlighted='false'` (o teste existente `does not parse above 256 KiB` continua).
- `shows a cut SSE body without breaking words`: corpo SSE `head_only` → `pre` sem `break-all`, aviso de corte presente.

- [ ] **Step 2: Rodar e ver falhar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/features/usage-logs/components/log-content` / Expected: FAIL.

- [ ] **Step 3: Implementar**, e as duas chaves nos 8 arquivos.

- [ ] **Step 4: Rodar e ver passar** — mesmo comando / Expected: PASS. Tipos e lint.

- [ ] **Step 5: Commit** — `feat(logs): compact tools, routing list, retry banner and highlighted raw bodies`

### Task 7: Lista de Common Logs

**Files:**
- Modify: `web/src/features/usage-logs/index.tsx` (atributo), `components/usage-logs-table.tsx`, `components/columns/common-logs-columns.tsx` (coluna `status`), `components/common-logs-summary.tsx`, `components/common-logs-stats.tsx`, `components/common-logs-filter-bar.tsx` (sem `stats`), `components/logs-filter-toolbar.tsx` (`data-action='reset'` no Reset de desktop), `components/timing-metrics-cell.tsx` (`data-slot='stream-label'` no `span` do rótulo)
- Test: `components/__tests__/common-logs-summary.test.tsx`, `components/__tests__/log-status-column.test.tsx` (novo)

**Interfaces:**
- Consumes: `logsFinishScope`, `MetricStrip`, `StatusBar`, `toneFromStatusVariant`, regras de `visual-finish.css` (Tasks 1 e 2)

Decisões (M §2):
- Coluna `status`: `cell` vira `StatusBar` com `tone` de `toneFromStatusVariant(config.color)` e `label={t(config.label)}`; `header: () => <span className='sr-only'>{t('Status')}</span>`, `meta: { label: t('Status') }`, `size: 28`.
- `UsageLogsTable`, só quando `isCommon`: `getColumnClassName` devolve `'px-3 py-1.5'` (hoje `'py-2'`); `tableClassName` acrescenta `rounded-(--vf-radius-inner) [&_tbody>tr]:h-12`; `tableHeaderClassName='[&_th]:px-3'`. Drawing e Task continuam com `'py-3.5'` e as classes de hoje. O fundo rosado das linhas de erro continua.
- `CommonLogsSummary`: as 5 métricas viram `MetricStrip size='lg' columnsClassName='grid-cols-2 sm:grid-cols-5'`, com os `testId` atuais (`success-rate`, `summary-tokens`, `summary-cost`). Os chips de `CommonLogsStats` passam para logo abaixo da faixa, antes do gráfico, e saem do cartão de filtros (`stats` deixa de ser passado a `LogsFilterToolbar`, que continua aceitando a prop). Os chips trocam o traço `sky/rose/slate` por `bg-chart-1`, `bg-destructive`, `bg-muted-foreground` e raio `rounded-(--vf-radius-control)`. O cartão do resumo usa `rounded-(--vf-radius-inner)` e sai `bg-card/50`.
- Barra de filtros, paginação, modelo e token em texto simples, stream tingido e peso 400: pelas regras de `visual-finish.css` (Task 1), sem mais edição nos componentes.

- [ ] **Step 1: Escrever os testes que falham**

`log-status-column.test.tsx` (renderiza a `cell` da coluna `status` de `useCommonLogsColumns` com `renderHook` + `render`):
- `shows a status bar named OK for a consume log`: `type: 2` → `getByRole('img', { name: 'OK' })` com `data-tone='success'`, e `queryByText('OK')` é `null`.
- `shows a status bar named Error for an error log`: `type: 5` → `name: 'Error'`, `data-tone='danger'`.
- `keeps a visually hidden Status header`: o cabeçalho tem texto `Status` com classe `sr-only`.

`common-logs-summary.test.tsx` (com `getLogStats` mockado, como já é `getLogSummary`):
- `shows the totals as a metric strip`: `getByTestId('success-rate')` dentro de `[data-slot=metric-strip]`; 5 `[data-slot=metric-cell]`.
- `shows the usage chips next to the totals`: `RPM` e `TPM` dentro da `section` do resumo.
- Os testes atuais de valor passam a usar `toHaveTextContent` nos `testId`.

- [ ] **Step 2: Rodar e ver falhar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/features/usage-logs/components/__tests__` / Expected: FAIL.

- [ ] **Step 3: Implementar**

- [ ] **Step 4: Rodar e ver passar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/features/usage-logs` / Expected: PASS em todo o módulo, inclusive `mobile-card`, `log-details-sheet` e `row-interaction`. Tipos e lint.

- [ ] **Step 5: Commit** — `feat(logs): dense common logs table with status bars and metric strip`

### Task 8: Relatórios do dashboard

**Files:**
- Modify: `web/src/features/dashboard/components/reports/reports-section.tsx`, `report-primitives.tsx`, `report-totals-panel.tsx`, `report-chart-card.tsx`, `report-table.tsx`, `report-channel-panel.tsx`
- Test: `__tests__/reports-section.test.tsx`, `report-panels.test.tsx`, `report-table.test.tsx`

**Interfaces:**
- Consumes: `FINISH_ATTR`, `MetricStrip`, `TintBadge` (Tasks 1 e 2)

Decisões (M §1 aplicado a Relatórios, ver conflito 1):
- Raiz de `ReportsSection` com `data-visual-finish='reports'`. A barra de período e filtros e o aviso vazio passam de `rounded-2xl` para `rounded-lg` (o raio do tema).
- `PanelShell`: `rounded-lg`; título 14px/600 `leading-5`; descrição 12px muted; vazio com `rounded-(--vf-radius-inner)`. `EmptyOrLoading` e o alerta de `ReportChartCard`, idem.
- `ReportTotalsPanel`: os tiles viram `MetricStrip size='lg' columnsClassName='grid-cols-2 sm:grid-cols-4'`, com `sub` = `tile.sublabel`.
- `ReportChartCard`: figuras com `dt` 11px/500 caixa alta `tracking-[0.05em]` e `dd` sans 18px/600 `tabular-nums`.
- `ReportTable`: `th` e `td` com `px-3`, `td` `py-2`, texto 13px, cabeçalho 12px/500 muted; contêiner `rounded-(--vf-radius-inner) border`.
- `ReportChannelPanel`: taxa acima do limite vira `TintBadge tone='danger' dot` com o ícone e o `sr-only` atuais dentro.

- [ ] **Step 1: Escrever os testes que falham**

- `marks the reports section with its finish scope` (`reports-section.test.tsx`): `container.querySelector('[data-visual-finish=reports]')` existe.
- `shows the report totals as a metric strip` (`report-panels.test.tsx`): o painel de totais tem `[data-slot=metric-strip]` com uma célula por tile.
- `marks a degraded channel with a tinted badge`: canal com `error_rate` acima de `CHANNEL_ERROR_RATE_THRESHOLD` → a taxa dentro de `[data-slot=tint-badge][data-tone=danger]` e o texto `Degraded` presente; um canal saudável sem badge.

- [ ] **Step 2: Rodar e ver falhar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/features/dashboard/components/reports` / Expected: FAIL.

- [ ] **Step 3: Implementar**

- [ ] **Step 4: Rodar e ver passar** — mesmo comando, depois `bun run i18n:check-reports` / Expected: PASS e nenhuma chave faltando. Tipos e lint.

- [ ] **Step 5: Commit** — `feat(dashboard): report panels with metric strips and tighter data styling`

### Task 9: Verificação completa e em navegador

**Files:**
- Scripts só no scratchpad: `/private/tmp/claude-501/-Users-neimaravila-qwen3/890d5424-2c41-4f7f-a8d5-39c9cc4a1fac/scratchpad/compare/pw/vf-after.mjs`, `vf-oos.mjs`, `vf-contrast.mjs`

- [ ] **Step 1: Rodar todas as verificações do frontend**

Run (em `web/`): `bunx -p typescript@5.9 tsc -p tsconfig.app.json --noEmit && bunx oxlint -c .oxlintrc.json $(git diff --name-only --diff-filter=d production...HEAD -- 'src/**/*.ts' 'src/**/*.tsx' | sed 's#^web/##') && LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test && bun run i18n:check-reports`
Expected: tudo passa. Falha que já existia em `production` é registrada, não escondida.

- [ ] **Step 2: Conferir o diff do upstream**

Run: `git diff production...HEAD --stat -- web/src ':!web/src/features/usage-logs/components/log-content' ':!web/src/features/dashboard/components/reports' ':!web/src/components/finish' ':!web/src/i18n' ':!*__tests__*'`
Expected: só os arquivos listados em "Global Constraints", com poucas linhas cada; nada em `components/ui/`, `components/data-table/` nem `components/status-badge.tsx`.

- [ ] **Step 3: Recompilar e reiniciar a demo como antes**

Run:
```bash
cd /Users/neimaravila/Desenvolvimento/new-api/web && bun run build && cd .. \
 && go build -o /Users/neimaravila/qwen3/new-api-demo/new-api-next . \
 && cd /Users/neimaravila/qwen3/new-api-demo \
 && SECRET=$(ps eww -p "$(cat server.pid)" | tr ' ' '\n' | sed -n 's/^SESSION_SECRET=//p') \
 && kill "$(cat server.pid)" && mv new-api-next new-api \
 && PORT=13000 SQLITE_PATH=/Users/neimaravila/qwen3/new-api-demo/demo.db ERROR_LOG_ENABLED=true SESSION_SECRET="$SECRET" \
    nohup ./new-api --log-dir /Users/neimaravila/qwen3/new-api-demo/logs > server.log 2>&1 & echo $! > /Users/neimaravila/qwen3/new-api-demo/server.pid
```
Expected: `curl -s localhost:13000/api/status` responde `success: true`; `mock.pid` segue vivo (o mock não é reiniciado).

- [ ] **Step 4: Refazer as capturas da comparação**

Copiar `compare/shots` para `compare/shots-before-finish`. `vf-after.mjs` reusa `clib.mjs` e o roteiro de `cap.mjs` (mesmos IDs de chamada e mesmas telas), mais `/dashboard/reports`, nas variantes:
- 1440x900 claro e escuro (todas as telas de `cap.mjs` e Relatórios);
- 1440x900 claro com o preset `lake-view` (`localStorage['newapi:theme:v1:preset']`), logs e detalhe;
- 390x844 claro: lista, painel em largura total, Conversa e JSON bruto;
- pt-BR (`localStorage['i18nextLng'] = 'pt-BR'`), 1440x900 claro: lista, detalhe e Relatórios.

Expected, olhando cada `nx-*` ao lado do `bf-*` e da cópia antiga: os 10 itens da spec visíveis; sem rolagem horizontal da página no celular (`document.documentElement.scrollWidth <= innerWidth`); papéis distinguíveis no escuro e no preset; nada de verde do Bifrost nem Geist; textos em pt-BR sem corte nos rótulos em caixa alta e nos contadores.

- [ ] **Step 5: Medir contraste**

`vf-contrast.mjs` lê, no claro, no escuro e no preset, a cor de texto e o fundo efetivo (primeiro ancestral com fundo opaco, convertido por um canvas 1x1 para RGB) de: tokens de JSON, textos de `TintBadge` dos 6 tons, rótulos e unidades das faixas, aba inativa, metadado de linhas.
Expected: todos com contraste `>= 4.5`. Abaixo disso, ajustar só os percentuais em `visual-finish.css`, refazer a Task 1 Step 5 e este passo.

- [ ] **Step 6: Provar que telas fora do escopo não mudaram**

`vf-oos.mjs` sobe duas instâncias sobre cópias idênticas do banco: `new-api-production` (antes) na porta 13001 e o `new-api` novo na 13002, cada uma com `SQLITE_PATH` numa cópia própria de `demo.db` feita no mesmo instante e `--log-dir` no scratchpad. Em cada uma, com `page.clock.setFixedTime` igual, `reducedMotion: 'reduce'` e 1440x900 claro, captura Visão geral (`/dashboard/overview`), Canais (`/channels`) e Drawing Logs (`/usage-logs/drawing`), e repete a captura do "antes" uma segunda vez para medir o ruído.
Run: `compare -metric AE -fuzz 1% antes.png depois.png diff.png` para cada tela, e o mesmo entre as duas capturas do "antes".
Expected: AE do antes x depois menor ou igual ao AE do antes x antes, por tela (zero quando o ruído é zero); qualquer diferença é examinada em `diff.png` e precisa estar só em região que também muda entre as duas capturas do "antes". Depois, parar as duas instâncias e apagar as cópias do banco.

- [ ] **Step 7: Commit** — `chore(finish): verification notes` só se algum ajuste de token tiver sido feito; as capturas e scripts ficam no scratchpad, fora do repositório.

---

## Self-Review

- **Cobertura da spec:** itens 1 e 3 da spec (Task 5), 2 (Tasks 2, 4, 7 e 8), 4 (Tasks 3, 5 e 6), 5 e 9 (Task 7), 6 (tokens de raio na Task 1, aplicados nas Tasks 4 a 8), 7 (Tasks 2, 4 e 7), 8 (Task 4), 10 (Tasks 4 e 6). "Demais diferenças": barra de filtros, chips, paginação (Tasks 1 e 7); cabeçalho e abas do detalhe (Task 4); Roteamento, retry e JSON bruto (Task 6); hero, cards e saúde dos canais, levados a Relatórios (Task 8, conflito 1). Identidade e presets: tokens derivados (Task 1) e contraste medido (Task 9). Verificação: Task 9.
- **Consistência de tipos:** `FinishScope`, `TintTone`, `toneFromStatusVariant`, `TintBadge`, `MetricStrip`/`MetricItem`, `StatusBar`, `splitUnit`, `tokenizeJson`, `JSON_HIGHLIGHT_MAX_CHARS`, `JsonView`, `splitFences`, `looksLikeJson`, `LongText.variant`, `roleStyle` e `RetryBanner` têm o mesmo nome e forma em todas as tasks que os citam. Atributos fixos: `data-visual-finish`, `data-slot` (`tint-badge`, `metric-strip`, `metric-cell`, `metric-value`, `metric-unit`, `tab-count`, `stream-label`), `data-tone`, `data-token`, `data-text-style`, `data-highlighted`, `data-action='reset'`.
- **Review Focus:** token longo (Task 3 `JsonView scrolls long lines…`, Task 5 `wraps a long unbroken token…`); JSON cortado ou SSE (Task 3 `is lossless`, `unterminated string`); papel desconhecido (Task 5 `falls back to system`); modo sensível (Task 4 `keeps the cost masked…`, Task 6 `keeps routing details masked…`); Drawing e Task Logs (Task 1 `logsFinishScope…`, Task 9 Step 6).
- **Proporção:** decisões, assinaturas e asserts; valores de CSS por referência a "M §". O único bloco de código é o reinício da demo, que precisa ser exato para não trocar o segredo de sessão.
