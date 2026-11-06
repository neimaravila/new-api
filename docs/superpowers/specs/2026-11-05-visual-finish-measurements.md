# new-api x Bifrost: diferenças visuais medidas

Pasta: `/private/tmp/claude-501/-Users-neimaravila-qwen3/890d5424-2c41-4f7f-a8d5-39c9cc4a1fac/scratchpad/compare`

Todos os valores abaixo saíram de `getComputedStyle` e `getBoundingClientRect` (Playwright/Chromium, viewport 1440x900, tema claro). As cores foram convertidas para hex; `@NN%` indica alfa. Os dados brutos estão em `measure-nx.json`, `measure-bf.json`, `m2-*.json` (tokens de código) e `m3-*.json` (tema escuro), e a tabela lado a lado em `measure-compact.txt`.

- **new-api (nx)**: demo em `localhost:13000`, admin, idioma `en` (para os rótulos serem comparáveis). As telas seguem o commit `492200bce`.
- **Bifrost (bf)**: container novo `bifrost-compare` (imagem `maximhq/bifrost`), `127.0.0.1:8081`, dados em `compare/bifrost-data`. Configuração:
  - providers custom `mock-openai` (chave `chanA`, `max_retries=1`), `mock-openai-reserva` (chave `chanB`) e `mock-anthropic` (base anthropic), todos em `host.docker.internal:13190` com `allow_private_network` e `store_raw_request_response`;
  - 3 virtual keys.
  - O mesmo roteiro de chamadas da demo (`calls_bifrost.py`, derivado de `calls_demo.py` sem escrever em `replies.json`) foi enviado: tool call e resultado, multi-turn, código, resposta longa, stream e não-stream, retry com fallback `mock-openai` → `mock-openai-reserva`, formato Anthropic via `/anthropic/v1/messages` e uma falha sem reserva.
- Para parar o Bifrost: `docker stop bifrost-compare` (e `docker rm bifrost-compare` para remover). O container `bifrost` original e `/Users/neimaravila/qwen3/bifrost` não foram tocados.

---

## 0. Identidade x acabamento

**Identidade do new-api (manter):**
- logo e cabeçalho com navegação superior (Home, Console, Model Square…);
- sidebar agrupada (CHAT/GENERAL/PERSONAL/ADMIN) com item ativo em pílula azul;
- cor primária azul `#3ea4ec` (botão Search) e o azul dos links `#004c9f`;
- fonte de UI `--font-sans: var(--font-ui-sans)`, que resolve para `sans-serif` (Helvetica/Arial no macOS), e mono `ui-monospace, SFMono-Regular, Menlo…`;
- o "mood" arredondado dos presets (`--radius: 1rem`) no shell: sidebar, cartão principal e hero do Overview;
- avatar com inicial e chip de token.

**Acabamento (o que dá para adotar do Bifrost sem perder a identidade):**
- hierarquia tipográfica de rótulos e valores;
- quando usar mono e quando usar sans no conteúdo;
- raio *dentro* de painéis densos, densidade de tabela, badges tingidos;
- camadas de fundo por papel de mensagem;
- realce de sintaxe JSON;
- barra de status na linha e header de detalhe em grade.

O Bifrost usa Geist/Geist Mono, verde-esmeralda `#047857` como acento e raio de 4px. Isso é identidade *dele* e **não** deve ser copiado. O que se copia é a escala e a lógica.

---

## 1. Dashboard / Overview (`nx-dashboard.png` x `bf-dashboard.png`)

1. **Título da página.**
   - Bifrost: título no topbar, 15.2px/600, lh 24px, `#09090b`. O conteúdo começa logo abaixo, em abas segmentadas (Overview, Provider Usage…).
   - new-api: "Overview" com 18px/700, lh 28px e ls -0.45px dentro do cartão branco. Embaixo vem um hero "PLATFORM COMMAND CENTER" com título de ~30px e 141px de altura.
   - Mudar: mantenha o 18px/700 (é bom), mas reduza o hero: padding de 24px para 16px 20px e título para 22–24px/600. Hoje o hero e os 4 cartões ocupam ~330px antes de qualquer gráfico.
2. **Cartão de métrica.**
   - Bifrost: cartão com `border 1px #e4e4e7`, raio 4px, padding 8px, sem sombra. Título 13.2px/500 na cor de acento e valor "Total 45" em 13.2px/600 com acento.
   - new-api: cartão 292x166, raio 28.8px (`rounded-2xl`), borda 1px `#e8e8e8`, sombra `0 1px 2px #0000000d`, padding 12px. Rótulo 12px/500 `#606060`, valor 24px/600 ui-monospace com ls -0.6px, subtítulo 12px `#616161@60%`, ícone em tile 28x28 com raio 12.8px e fundo `#4ea7ff@10%`.
   - Mudar:
     - raio do cartão para `rounded-lg` (16px) ou `rounded-md` (12.8px);
     - altura natural, sem `min-h`: os cartões têm ~60px vazios embaixo;
     - subtítulo com contraste maior, `#616161` sem alfa (o @60% fica perto de 3:1);
     - valor numérico com `font-variant-numeric: tabular-nums` em sans 24px/600, em vez de mono (ver item 2 da seção 2).
3. **Lista "Channel health".**
   - Bifrost: listas e rankings vão em tabela, com linhas de 1px.
   - new-api: cada canal é um cartão de 54px com raio 22.4px, borda 1px `#e8e8e8`, fundo `#ffffff@60%` e padding 8px 12px. O "Status 1" é uma pílula de raio 41.6px com borda.
   - Mudar: troque cartões empilhados por linhas `border-bottom: 1px solid var(--border)` com `py-2 px-3` e raio 0. O status vira um ponto de 6px com a cor do status mais texto (`Ativo`), e não "Status 1" cru.
4. **Seções.**
   - Bifrost: título de seção 13.2px/500 na cor de acento.
   - new-api: "Smart insights" e "Channel health" com 14px/600 e ícone à esquerda. Está bom.
   - Mudar: só alinhe todos os títulos de seção em 14px/600, lh 20px, e o ícone em 16px `text-muted-foreground`.

## 2. Lista de logs (`nx-logs.png` x `bf-logs.png`)

1. **Altura de linha.**
   - Bifrost: linha de **45px**, padding da célula `6px 16px`, texto 13.2px/400, lh 20px.
   - new-api: linha de **60px**, padding `8px`, texto 13px/**500**, lh 18.57px. Quase toda célula tem duas linhas (data + "Consume", canal + nome, token + grupo).
   - Mudar:
     - `td { padding: 6px 12px }`;
     - texto primário com peso 400 (hoje é 500 em tudo, então nada se destaca);
     - linha secundária em 11–12px `text-muted-foreground`;
     - alvo de 48px por linha. Opcional: modo "compacto" de linha única com 40px.
2. **Status da linha.**
   - Bifrost: barra vertical de **4x24px**, raio 4px, na 1ª coluna, em `#ce5249` (erro) ou verde-azulado (sucesso). A linha em si não muda de fundo.
   - new-api: ponto de 6px mais o texto "OK" `#009767` / "Error" `#e40014` 13px/500. A linha de erro ganha fundo `#fff0f2@40%`.
   - Mudar: mantenha o fundo rosado (é um bom sinal), mas troque o ponto e a palavra por uma barra de 3–4px x 24px, `border-radius: 4px`, na mesma coluna, liberando ~60px de largura. Se o texto ficar, use 12px/500.
3. **Badges na linha.**
   - Bifrost: badge de tipo em **mono 11px/500, uppercase**, `padding 2px 6px`, raio 4px, fundo tingido por categoria (`#dbeafe` com texto `#193cb8` para CHAT, verde para RESPONSES, roxo para STREAM) e borda 1px `#e4e4e7`.
   - new-api: badges de modelo/token/custo com fundo `#f5f5f5@30%`, borda `#e8e8e8@60%`, raio 12.8px, 13px/500 e altura 24px. São todas iguais e neutras, então nada se diferencia.
   - Mudar:
     - raio 6px (`rounded-sm`);
     - 12px/500;
     - `padding: 1px 6px`;
     - altura 20–22px;
     - tingir **só** o que é categórico (tipo de chamada, stream x não-stream) com fundo 10–12% da cor e texto 700 da mesma matiz. Modelo, token e custo ficam como texto simples (o modelo em mono 12px).
4. **Cabeçalho da tabela.**
   - Bifrost: `th` com 40px, 13.2px/500, `padding 0 16px`, `border-bottom 1px #e4e4e7`.
   - new-api: `th` com 40px, 13px/500, `padding 0 8px`, `border-bottom 1px #e8e8e8`. Equivalente.
   - Mudar: só o padding horizontal para 12px, alinhado ao `td`.
5. **Contêiner da tabela.**
   - Bifrost: borda 1px, raio 4px.
   - new-api: borda 1px `#e8e8e8`, raio 16px, dentro de um cartão de raio 22.4px.
   - Mudar: raio de 8–10px na tabela. Raios grandes num contêiner de dados cortam o canto do header e da 1ª coluna.
6. **Faixa de métricas (stats).**
   - Bifrost: grade 3x2 de células separadas por `gap:1px` sobre fundo `#e4e4e7` (o fundo vira a linha divisória), padding `16px 18px`. Rótulo **11.5px/400 uppercase, ls 0.69px**, `#686872`. Valor **Geist Mono 24px/500, ls -0.48px**, com unidade menor ("ms", "%"). Embaixo, sparkline e delta.
   - new-api: rótulo 12px/400 `#606060` e valor **ui-monospace 14px/600** (`#0a0a0a@90%`). O valor tem praticamente o tamanho do rótulo, então não há hierarquia.
   - Mudar:
     - valor para 22–24px/600, lh 32px, ls -0.02em, `tabular-nums` (sans ou mono; mono 24px já existe no Overview);
     - rótulo para 11–12px/500 uppercase com `letter-spacing: .05em`;
     - unidade (`s`, `%`) em 0.6em `text-muted-foreground`;
     - separar as métricas com divisórias de 1px (`divide-x`) em vez de espaço vazio.
7. **Barra de filtros.**
   - Bifrost: toolbar com botões de **30px de altura, raio 4px, 13.2px**. Busca única de largura total (787px, 28px de altura, fundo `#f8fafc`) e painel lateral de filtros (256px, borda 1px, raio 6px, grupos colapsáveis 13px).
   - new-api: cartão de filtros (borda `#e8e8e8`, raio 16px, padding 12px, fundo `#fdffff@50%`), inputs de 32px com **raio 16px (pílula)** e botões de 32px também em pílula (`padding 0 10px`, 14px/500). Chips Usage/RPM/TPM com 28px, raio 12.8px e barra colorida à esquerda.
   - Mudar:
     - inputs e botões com `border-radius: 8px` (pílula total em input de texto parece tag);
     - altura 32px mantida;
     - texto dos controles 13px;
     - "Search" primário pode continuar azul (identidade);
     - "Reset" como ghost sem borda.
     - Os chips Usage/RPM/TPM ficam melhor *fora* do cartão de filtro, junto da faixa de stats (é métrica, não filtro).
8. **Paginação.**
   - Bifrost: seletor de 28px, 12px, raio 4px, e "Page 1 of 2" em texto.
   - new-api: botões numéricos de 32px em pílula e "Rows per page 100" de 32px. Funciona.
   - Mudar: só o raio (8px) e o tamanho do texto (13px).

## 3. Detalhe da chamada com tool call

### 3a. Header do painel (`nx-crop-stats.png` x `bf-crop-stats.png`)

1. **Título e status.**
   - Bifrost: título do sheet 13.2px/500. Abaixo, cartão com badges: "Chat" (`#dbeafe`/`#193cb8`, 12px/500, padding 2px 8px, raio 4px) e "SUCCESS" (11px/**600 uppercase**, texto `#006659`, fundo `#149d89@10%`, borda `#119f8e@30%`, ponto de 6px). Provider em badge outline 12px/500.
   - new-api: título "Log Details" 16px/600. O status "OK" e o "HTTP 200" são **texto solto** 14px/500 `#009767`, sem badge. "OpenAI Chat" e "#6 Mock OpenAI" também são texto solto 14px.
   - Mudar: transformar status e tipo em badges:
     - `font-size:11px; font-weight:600; text-transform:uppercase; letter-spacing:.04em; padding:2px 8px; border-radius:6px;`
     - `background: color-mix(in oklab, var(--success) 10%, transparent); border:1px solid color-mix(... 30%)`;
     - ponto de 6px antes.
     - "HTTP 200" pode virar parte do mesmo badge ("OK · 200").
2. **Request ID.**
   - Bifrost: rótulo "REQUEST" 10.5px/600 uppercase e valor Geist Mono **13px**, `#09090b`, com ícone de copiar.
   - new-api: ID em ui-monospace **12px `#606060`**, sem rótulo e sem botão de copiar.
   - Mudar: mono 12.5–13px na cor de texto principal, rótulo "Request" 11px/600 uppercase muted, e botão de copiar de 16px.
3. **Cartões de métrica do header.**
   - Bifrost: **uma** faixa com divisórias de 1px (`border-top 1px #e4e4e7` entre linhas e colunas). Rótulo **10.5px/600 uppercase, ls .525px**, `#686872`. Valor **18px/600** (latência na cor de sucesso `#047857`), modelo em Geist Mono 15px/600, subtítulo 11px `#686872`.
   - new-api: 5 cartões separados de 160x104, raio 12.8px, borda `#e8e8e8`, fundo `#f5f5f5@30%`, padding `8px 12px`. Rótulo 12px/400 `#606060`, valor **ui-monospace 14px/500**, subtítulos 12px empilhados ("Output Tokens", "Cache Read", "Cache Write").
   - Mudar:
     - um contêiner com `border:1px solid var(--border); border-radius:10px` e células `divide-x`, sem fundo próprio;
     - rótulo 11px/600 uppercase ls .05em;
     - **valor 18px/600** (sans com tabular-nums; o modelo em mono 14–15px/600);
     - subtítulo 11–12px muted em **uma** linha ("12 / 9 · total 21"; cache só se > 0).
     - Isso reduz a faixa de 104px para ~80px e dá hierarquia.
4. **"Details" colapsável.**
   - Bifrost: "More details" é um cartão de 42px com borda 1px, raio 4px e resumo à direita ("timings, request meta, tokens…" em 12px muted).
   - new-api: um chevron com o texto "Details" 14px, sem contêiner.
   - Mudar: dar contêiner (borda 1px e raio 8px, padding `10px 16px`) e um resumo em 12px muted à direita.

### 3b. Abas (`nx-detail-*.png` x `bf-detail-*.png`)

- Bifrost: abas segmentadas com tablist de fundo `#f3f3f5@60%`, raio 8px, padding 3px e altura 40px. Aba ativa com fundo branco, sombra `0 1px 3px #0000001a, 0 1px 2px -1px #0000001a`, raio 4px e padding `4px 12px` em 13.2px/500. Cada aba tem um **contador** (badge 10px/500, fundo `#f4f4f5`, borda 1px, raio 4px): Messages 5, Tools 2, Routing 1.
- new-api: abas "underline", tablist sem fundo e aba ativa com sublinhado preto de 2px. Texto 14px/500; inativas em `#0a0a0a@60%`. Sem contadores.
- Mudar: o estilo underline pode ficar (é coerente com o resto do new-api), mas:
  - adicione contadores (`Conversation 5`, `Tools 2`, `Routing 2`) como badge 11px/500, padding `0 6px`, raio 6px, fundo `var(--muted)`;
  - suba o contraste da aba inativa para `text-muted-foreground` sólido (hoje é preto com alfa .6).
  - Se quiser o segmented do Bifrost, reaproveite o `TabsList` padrão do shadcn com `bg-muted p-[3px] rounded-lg`.

### 3c. Mensagens / Conversa (`nx-crop-message.png` x `bf-crop-message.png`, `*-detail-messages.png`)

1. **Fonte do conteúdo — a maior diferença.**
   - Bifrost: prosa (User e Assistant) em **sans 13px/400, lh 21.1px (1.625)**. Só o system prompt e o tool result vão em **mono 12.5px, lh 20px**.
   - new-api: **todo** conteúdo em `font-mono text-sm`, ui-monospace **14px**, lh 20px (`message-block.tsx:73`), inclusive a resposta do assistente e a fala do usuário. O texto natural em mono 14px fica largo, pesado e cansativo.
   - Mudar:
     - prosa em `font-sans text-[13px] leading-[1.6]`;
     - `font-mono text-[12.5px] leading-5` só para JSON, código e tool result (detectar por `content-format.ts`, que já classifica o conteúdo).
2. **Bloco de mensagem.**
   - Bifrost: estrutura em *timeline*. Rótulo do papel fora da caixa (11.5px/600) com metadado "1 line · ~23 tokens" (11px `#686872`) e ponto de 6px colorido à esquerda. A caixa tem padding 12px, raio 4px, borda 1px e **fundo por papel**:
     - System: `#fafafa`, borda `#e4e4e7`;
     - User: `#eef7ff@60%`, borda `#bedbff`;
     - Tool result: `#fffceb@70%`, borda `#fee685`;
     - Assistant: branco, borda `#e4e4e7`.
   - new-api: todos os papéis no **mesmo** cartão (borda 1px `#e8e8e8`, raio 16px, padding 12px, sem fundo), com o rótulo dentro, em 14px/500, e "Copy" (12px/500 `#606060`, 62x24) em todos. Só a resposta final ganha a pílula "Response" (fundo `#eeeeee`, texto `#004c9f`, raio 41.6px).
   - Mudar:
     - raio 10px;
     - fundo e borda por papel, mantendo a paleta do new-api: user `color-mix(var(--primary) 6%, transparent)` com borda 25%; tool `color-mix(amber 8%)` com borda `amber 40%`; system `var(--muted)/50`; assistant transparente;
     - rótulo 12px/600 com metadado 11px muted ("1 linha · ~23 tokens");
     - "Copy" só ícone de 14px, visível no hover (`opacity-0 group-hover:opacity-100`).
3. **Histórico colapsado.**
   - Bifrost: "Show 2 earlier messages" esconde system e user por padrão e destaca o turno atual.
   - new-api: mostra tudo e tem chips de filtro por papel (System/User/Assistant/Tool, 24px, raio 12.8px).
   - Mudar: opcional. Colapse system e mensagens antigas quando houver mais de 4 e mantenha os chips.
4. **Tema escuro** (`nx-detail-messages-dark.png` x `bf-detail-messages-dark.png`).
   - Bifrost: sheet `#18181b` sobre página `#0a0a0a`. User com fundo `#172456@30%` e borda `#1c398e`; tool result com fundo `#461a00@30%` e borda `#7b3306`. Os papéis continuam distinguíveis.
   - new-api: sheet `#1e1e1e` sobre `#1c1c1c` (quase sem contraste de camada). Cartões só com borda `#ffffff@10%` e sem fundo, então todos os blocos parecem iguais.
   - Mudar: sheet uma camada acima (`#202020` sobre `#141414`), e os mesmos tints por papel do item 2 com alfa 15–20% no escuro.

### 3d. Tool call (`nx-crop-toolcall.png` x `bf-crop-toolcall.png`)

- Bifrost: caixa própria com header "Tool Call: consultar_pedido" (13.2px/500, padding ~12px 24px, separador 1px) e botão de copiar. Corpo em Geist Mono **12.5px, lh 19px**, com **realce de sintaxe**: chaves `#a31515`, strings `#0451a5`, números `#098658`. Tem guia vertical de indentação, "show more" em faixa cinza no rodapé e mostra o objeto inteiro (`id`, `type`, `function`).
- new-api: caixa interna com raio 12.8px, borda `#e8e8e8`, fundo `#f5f5f5@30%` e padding `8px 12px`. Título "Tool call: consultar_pedido" **12px/500**, corpo mono 14px **sem realce**, só com os argumentos.
- Mudar:
  - header da caixa com nome da função em mono 13px/600 e o id (`call_PED-48213`) em mono 11px muted, separados do corpo por `border-bottom:1px`;
  - corpo mono 12.5px, lh 1.5;
  - realce simples de JSON com 3 cores de token derivadas do tema (`--chart-1..3` ou equivalentes);
  - raio da caixa 8px.

### 3e. Tools (`nx-detail-tools.png` x `bf-detail-tools.png`)

- Bifrost: "2 tools exposed to the model" (12px muted). Grade de 2 colunas com cartões compactos (66px): ícone em tile âmbar de 28px, nome em mono 13px/500, descrição 12px muted e chevron para expandir o schema.
- new-api: um cartão por tool, com o schema JSON **sempre aberto** em mono 14px. Uma tool ocupa ~340px e a segunda já sai da tela. A pílula "Called 0 times" tem fundo azul e raio de pílula.
- Mudar: lista compacta em grade de 2 colunas (`grid-cols-2 gap-2`), com schema colapsado por padrão (`<details>`) e o schema em mono 12px quando aberto. "Called 0 times" vira texto muted 11px quando for 0 e só fica badge tingido quando for maior que 0.

### 3f. Routing (`nx-detail-routing.png` x `bf-detail-routing.png`)

- Bifrost: "Routing Decision Logs" com filtro de nível (DEBUG/INFO/WARN/ERROR) em segmented 10px mono uppercase. Cada linha tem timestamp mono 12.5px, badge de origem "GOVERNANCE" (verde 10px mono uppercase) e badge de nível "INFO" (azul).
- new-api: "Group vip" e a lista de tentativas, cada uma em cartão com raio 16px, texto 14px, "Duration: 0 ms" em 12px.
- Mudar: tentativas em lista com `divide-y` dentro de um cartão (raio 10px), número da tentativa num círculo de 20px, status HTTP em badge tingido 11px/600 e duração alinhada à direita em mono 12px `tabular-nums`.

### 3g. Raw (`nx-detail-raw.png` x `bf-detail-raw.png`)

- Bifrost: título de contexto "Raw Request sent to Mock-Openai" (12px), cartão com header 13.2px/500 e botão de copiar, corpo Geist Mono 12.5px/19px **com realce**, colapsado em ~3 linhas com "show more".
- new-api: "Client request" 14px/500 com "Original size 1.11 KB" e Copy. `pre` em ui-monospace **12px, lh 16px**, fundo `#f5f5f5@30%`, raio 12.8px, padding 8px, `break-all` (quebra palavras no meio: "cordialidad / e."), sem realce, `max-h-96`.
- Mudar:
  - lh 16px para **18–19px** (1.5);
  - `word-break: normal; overflow-wrap: anywhere` no lugar de `break-all`;
  - padding 12px;
  - raio 8px;
  - realce de sintaxe (o mesmo da seção 3d);
  - opção de colapsar em 12 linhas com "mostrar mais" em faixa de 28px no rodapé (`bg-muted/50`, 12px).

## 4. Detalhe do retry (`nx-retry-routing.png` x `bf-retry-routing.png`, `*-retry.png`)

- Bifrost: banner no topo do painel com badge "REQUEST SUCCEEDED" (11px/600 uppercase tingido de verde), frase "Attempt 1 of 2. This attempt failed and the request fell back to…" (13px) e link "Open final attempt". Logo abaixo, a **trilha visual** `mock-openai 500 → mock-openai-reserva ✓` em chips outline de 28px com o código 500 em vermelho. Na aba Routing: "Fallback chain" com tentativas numeradas em círculo de 20px, a tentativa vista com fundo `#fafafa` e badge "VIEWING", e o motivo "mock upstream exploded" em 13px muted. A "Attempt Trail" fica em tabela.
- new-api: chips mono 12px "#1 Mock OpenAI 500" e "#2 Mock OpenAI Reserva 200" acima dos cartões (bom e equivalente). Na aba Routing, cartões por tentativa com "Reason:" em vermelho `#e40014` 14px.
- Mudar:
  - incluir a seta `→` entre os chips;
  - colorir só o código HTTP (não o chip inteiro);
  - adicionar um banner de 1 linha "Tentativa final OK após 1 falha" com badge tingido;
  - na aba Routing, motivo em 13px `text-muted-foreground` com o código em badge vermelho (o parágrafo inteiro em vermelho grita demais).

## 5. Close-ups

| Recorte | nx | bf | Principal diferença medida |
|-|-|-|-|
| mensagem | `nx-crop-message.png` | `bf-crop-message.png` | mono 14px sem fundo e raio 16px x sans 13px/1.625, fundo azul `#eef7ff@60%` e raio 4px |
| tool call | `nx-crop-toolcall.png` | `bf-crop-toolcall.png` | sem realce, título 12px x realce em 3 cores, header com separador |
| header stats | `nx-crop-stats.png` | `bf-crop-stats.png` | 5 cartões, valor 14px x faixa única, valor 18px/600, rótulo 10.5px uppercase |
| stats da lista | `nx-crop-logstats.png` | `bf-crop-logstats.png` | valor 14px mono x 24px mono, grade com divisórias de 1px |
| linha de tabela | `nx-crop-row.png` | `bf-crop-row.png` | 60px, pad 8px, 500 x 45px, pad 6/16px, 400 e barra de status de 4px |
| barra de filtros | `nx-crop-filter.png` | `bf-crop-filter.png` | controles em pílula (raio 16px) x 30px com raio 4px |

---

## Top 10 por impacto visual

1. **Prosa em sans, mono só para dados** (seção 3c.1). Em `message-block.tsx:73`, troque `font-mono text-sm` por sans 13px/1.6 em User/Assistant; mono 12.5px só em JSON, tool result e código.
2. **Hierarquia dos números** (seções 2.6 e 3a.3). Valores de métrica de 14px para 18px/600 (detalhe) e 24px/600 (lista), com `tabular-nums`; rótulos 11px/600 uppercase com ls .05em.
3. **Fundo e borda por papel da mensagem** (3c.2): user com tint da primária a 6%, tool em âmbar a 8%, system em `muted/50`, assistant neutro. No escuro, alfa de 15–20% (3c.4).
4. **Realce de sintaxe JSON** em tool call e Raw (3d, 3g), com 3 cores de token, mono 12.5px e lh 1.5; trocar `break-all` por `overflow-wrap:anywhere`.
5. **Densidade da tabela** (2.1): linha de 60px para ~48px, `td` com padding `6px 12px`, peso 400 no texto primário e linha secundária em 11–12px muted.
6. **Raio interno menor** (2.3, 2.5, 2.7, 3c.2): manter o raio grande do shell (identidade), mas usar 8–10px em tabela, cartões de mensagem, inputs e botões de filtro, e 6px em badges. Hoje há 12.8, 16, 22.4, 28.8 e 41.6px na mesma tela.
7. **Status e tipo como badges tingidos** (3a.1, 2.3): 11px/600 uppercase, fundo a 10% e borda a 30% da cor, raio 6px, em vez de texto verde/vermelho solto; badges neutras só para categorias.
8. **Header de métricas em faixa única com divisórias** (3a.3): um contêiner `border` com células `divide-x`, sem 5 cartões com fundo; subtítulos em uma linha.
9. **Barra de status de 4px na linha** (2.2), no lugar de ponto e palavra, mantendo o fundo rosado do erro.
10. **Contadores nas abas e Tools compacto** (3b, 3e): badges de contagem em Conversation/Tools/Routing e schema das tools colapsado em grade de 2 colunas.

## Arquivos

- Telas: `shots/nx-<tela>.png` e `shots/bf-<tela>.png`, com as telas `dashboard`, `logs`, `detail-messages`, `detail-tools`, `detail-routing`, `detail-raw`, `retry`, `retry-routing`, `crop-message`, `crop-toolcall`, `crop-stats`, `crop-logstats`, `crop-row`, `crop-filter`, mais `detail-messages-dark`. Algumas telas também têm a variante `-full`.
- Scripts: `pw/cap.mjs` (capturas), `pw/measure.mjs`, `pw/m2.mjs`, `pw/m3.mjs` (medição), `bfsetup.py`, `bfkeys.py`, `bfvk.py`, `bfraw.py` (configuração do Bifrost) e `calls_bifrost.py` (chamadas).
- Credenciais do Bifrost: `bifrost-admin-password.txt`, `bifrost-setup-token.txt`, `bf-vkeys.json` (modo 600).
