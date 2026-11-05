# Conteúdo das chamadas nos logs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Gravar, por chamada e por tentativa, o conteúdo enviado e recebido, com retenção de 30 dias, e mostrá-lo em um painel de 60% da tela com Conversa, Tools, Roteamento e JSON bruto, sem afetar resposta, streaming, latência ou cobrança.

**Architecture:** Um pacote próprio, `service/logcontent`, copia os dados de forma limitada na memória durante a chamada e faz todo o trabalho caro (descompressão, limpeza, remontagem, gzip, gravação em lote) fora do caminho da chamada. O upstream recebe só 4 pontos de gancho de uma ou duas linhas: `controller.Relay`, `DoApiRequest`, `model/log.go` e o encerramento em `main.go`. O servidor normaliza os quatro formatos em um único modelo de visão JSON, e o frontend só o renderiza.

**Tech Stack:** Go, Gin, GORM, gjson; React 19, TypeScript, TanStack Query/Table, Tailwind, i18next, Vitest, Bun.

**Spec:** `docs/superpowers/specs/2026-11-05-log-content-capture-design.md`

## Global Constraints

- Branch de execução: `feat/log-content-capture`, a partir de `production`.
- Valem as restrições da etapa 1 e do `AGENTS.md`: identificadores protegidos, cabeçalho AGPL nos arquivos novos do frontend, JSON por `common.Marshal`/`common.Unmarshal*`, `t('English key')`, `@/lib/format`, `@/lib/currency`, sem cores fixas.
- Limites exatos da spec: 1 MB por corpo gravado; interpretação de até 8 MB fora do caminho da chamada; acima de 8 MB, só o começo em texto; 1 MB para a mensagem remontada; retenção padrão de 30 dias. Em código: `1 << 20` e `8 << 20`.
- Cabeçalhos, URLs e chaves não são gravados. Do provedor ficam só método, caminho sem query e status.
- Código novo em arquivos próprios. Nunca tocar `relay/channel/openai/helper.go`, `relay/channel/openai/relay-openai.go`, `relaykit/relayconvert/response_compat.go` nem `service/quota.go` (alterado pelo patch 6496). Em arquivos do upstream, adicionar linhas sem realinhar vizinhos com `gofmt`.
- SQL portável em SQLite, MySQL, PostgreSQL e ClickHouse. A tabela vive em `LOG_DB`: usar `common.UsingLogDatabase`, nunca `UsingMainDatabase`, e nunca fazer join com tabelas do banco principal. `/` entre inteiros não arredonda em MySQL nem ClickHouse; este plano não usa divisão em SQL.
- `use_time` é em segundos inteiros. Duração de tentativa é outro campo, em ms, e os dois nunca se convertem entre si.
- `logGroupCol` não é lido na inicialização do pacote. As consultas deste plano não usam a coluna `group`.
- Valores não podem vir enviesados por limites: corpo reduzido a só o começo gera `null` nas contagens de tools, nunca `0`. A retenção apaga sempre os mais antigos primeiro.
- Comandos Go na raiz; `bun` em `web/`. `bun run build` antes de `go test ./...`.
- Frontend: tipos com `bunx -p typescript@5.9 tsc -p tsconfig.app.json --noEmit`; lint com `bunx oxlint -c .oxlintrc.json <arquivos alterados>`; testes com `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test`. `bun run typecheck` está quebrado, `bun run lint` tem 156 erros antigos.
- i18n: adicionar as chaves direto nos 8 arquivos de idioma e rodar `bun run i18n:check-reports`. Não usar `i18n:sync`.
- Commits terminam com as linhas de atribuição da sessão.

## Review Focus

- Chave de canal curta ou composta (`ollama`, `ak|sk|região`, JSON de conta de serviço): as partes reais somem do conteúdo, mas uma palavra comum de menos de 8 caracteres não corrompe prompts e respostas (Task 3).
- Abrir o painel logo que a chamada termina, antes de o lote ser gravado: mostra carregando e atualiza sozinho, em vez de "sem conteúdo" (Tasks 9 e 13).
- Prompt ou resposta com HTML, `<script>` ou Markdown: aparece como texto literal, nunca é interpretado (Task 14).
- Cliente que desconecta ou provedor que corta no meio do stream: a chamada termina como hoje, o conteúdo fica como incompleto, e escritas tardias do scanner não causam corrida (Task 7).
- Corpo com NUL, UTF-8 inválido ou binário disfarçado de JSON: não derruba o lote no PostgreSQL e o painel continua abrindo, com o JSON bruto em texto (Tasks 1 e 3).

---

## Decisões fixadas

### Pontos de captura

| Dado | Onde | Como |
|-|-|-|
| Resposta entregue | `logcontent.Begin` troca `c.Writer` por `clientWriter` | Cópia limitada depois de cada `Write`/`WriteString`, só dos bytes aceitos |
| Requisição do cliente | `Capture.Finish` | `c.Get(common.KeyBodyStorage)` + `storage.NewReader()`. Nunca `GetBodyStorage`, que consumiria o corpo se não houvesse armazenamento |
| Enviada ao provedor | `DoApiRequest`, antes de `doRequest` | `req.GetBody()` (leitor independente, preenchido por `http.NewRequest` e `ApplyUpstreamBodyMetadata`). Sem `GetBody`: não copia e marca `request_unavailable`. Não usar tee: quebraria `ContentLength` e a repetição do transporte |
| Recebida do provedor | `DoApiRequest`, depois de `doRequest` | `resp.Body = bodyTap`, sem envolver `http.NoBody` |

- **Gancho no `controller.Relay`, não em middleware.** Uma única linha, `defer logcontent.Begin(c, relayFormat).Finish()`, entra como primeiro comando depois de `requestId := ...`. Ela cobre `/v1`, `/v1beta`, `/pg` (`Playground` chama `Relay`) e `POST /v1/responses`, registrada em `router/task-plugin-protocol-router.go`. Por ser o primeiro `defer`, `Finish` roda depois do `defer` que escreve o erro ao cliente, e `BodyStorageCleanup`, mais externo, só fecha o corpo depois. `RelayFormatOpenAIRealtime` retorna `nil`. Todos os métodos aceitam receptor `nil`. O WebSocket de Responses usa `relay.ResponsesWebSocketHelper` e não passa por aqui.
- **Só `DoApiRequest`, não `doRequest`.** `doRequest` também atende `DoFormRequest`, `DoRequest` e tarefas, que a spec deixa de fora. AWS por SDK e `aws` Mantle também ficam fora, com o aviso de tipo não capturado.
- **Elegibilidade:** `RelayMode` em `ChatCompletions`, `Completions`, `Responses`, `ResponsesCompact`, `Embeddings`, `Rerank`, `Gemini`, com corpo que não seja `multipart/*`. O resto não gera tentativa.
- **Cadeia de escritores:** `streamKeepAliveWriter` → `streamResponseWriter` → `clientWriter` → gin. `Begin` instala o nosso antes de `StartStreamKeepAlive`, então a verificação `c.Writer.(*streamKeepAliveWriter)` (`relay/helper/stream_scanner.go:128`) continua vendo o tipo certo, e `previous.ResponseWriter` restaura para o nosso. `clientWriter` embute `gin.ResponseWriter` (`Flush`, `Hijack`, `Status` etc. passam direto) e precisa definir `Unwrap() http.ResponseWriter`: sem ele, `WriteDeadline` e `FlushError` param silenciosamente, porque a interface do gin não promove o `Unwrap`. O `Write` tem mutex próprio, porque pings rodam em outra goroutine.
- **Patch 7033 verificado:** `scripts/apply-patches.sh` aplicado sobre `git archive HEAD` termina sem `.rej` e `go build ./relay/...` passa. Os patches não tocam `api_request.go`, `stream_scanner.go`, `relay/helper/common.go`, `controller/relay.go`, `model/log.go`, `model/main.go` nem `main.go`. O 7033 só muda quando `HandleStreamFormat` é chamado: os bytes continuam passando por `c.Writer` e `resp.Body`, que são os únicos pontos que tocamos. A Task 10 prova isso.
- **Numeração:** `attempt = 0` é o nível da chamada (cliente). `attempt >= 1` é um contador por chamada, incrementado a cada `DoApiRequest`. Não usar `RetryIndex`, que repetiria em adaptadores com duas chamadas na mesma tentativa.

### Memória

Constantes em `service/logcontent/limits.go`: `maxStoredBody = 1<<20`, `maxParsedBody = 8<<20`, `maxAssembled = 1<<20`, `sseHeadBytes = sseTailBytes = 512<<10`, `foldChunkBytes = 32<<10`, `foldQueueChunks = 16`.

- **Corpo não SSE:** cresce até 8 MiB. Ao passar disso, reduz para os primeiros 1 MiB, devolve o orçamento e marca `head_only`. Se `Size()`/`ContentLength` já indicar mais de 8 MiB, copia só 1 MiB desde o início.
- **SSE** (`Content-Type: text/event-stream`, lido na criação do tap ou na primeira escrita): cabeça de 512 KiB + anel de cauda de 512 KiB, os dois cortados em fim de linha ao selar. Guarda o começo e o fim com `usage` e `finish_reason`, com custo constante.
- **SSE do cliente, quando elegível:** também alimenta uma goroutine de remontagem (`gopool.Go`) por um canal de 16 blocos de 32 KiB, com envio sem bloqueio. Bloco perdido marca `truncated`.
- **Pior caso por stream:** 1 MiB (provedor) + 1 MiB (cliente) + 512 KiB (blocos) + 1 MiB (remontada) = 3,5 MiB, qualquer que seja a duração.
- **Orçamento global:** contador atômico de `LOG_CONTENT_CAPTURE_MEMORY_MB` (padrão 256), adicionado ao `.env.example`. Toda cópia reserva antes de crescer, e a reserva vai até o gzip terminar. Sem orçamento, o tap para de copiar e marca `truncated`/`head_only`. A chamada nunca espera.

### Compressão vinda do provedor

Sem `Accept-Encoding` explícito, o transporte do Go já descomprime gzip. Conteúdo comprimido só chega se um header override pedir. O tap copia os bytes como vieram e guarda o `Content-Encoding` (`resp.Header` ou `c.Writer.Header()`). O worker decodifica `gzip`, `deflate`, `br` e `zstd` com dependências que já existem (`andybalholm/brotli`, `klauspost/compress`), com saída limitada a 8 MiB. Falha, codificação desconhecida ou excesso: texto vazio e `undecodable: true`, sem gravar binário. SSE comprimido é tratado como não SSE no caminho e remontado no worker depois de decodificado.

### Linha de log, junção e tempo

- `model/log_content_observer.go` define `LogRowObserver { OnLogRow(log *Log) }`, `SetLogRowObserver(c, obs)` e `notifyLogRowWritten(c, log, err)`. `RecordConsumeLog` e `RecordErrorLog` chamam `notifyLogRowWritten` uma vez, logo depois de `createLog`, e só avisam com `err == nil`. Assim, `LOG_CONSUME_ENABLED`, `ERROR_LOG_ENABLED`, `IsRecordErrorLog` e falha de gravação valem sem repetir nenhuma regra.
- `Capture.OnLogRow` marca a tentativa atual como gravável e guarda `log.UserId`; em linha de erro, guarda também `log.Content` como `error_message`. Qualquer linha habilita `attempt = 0`.
- **Regra por tentativa:** tentativa sem linha própria não é gravada. Com `ERROR_LOG_ENABLED=false`, uma chamada que acerta na segunda tentativa grava `attempt` 0 e 2, e a linha do tempo mostra só o que tem conteúdo.
- **Tempo:** toda linha de log é gravada no fluxo do handler, inclusive depois de o stream terminar, então `Finish` já sabe o resultado. Ele não copia nem espera nada além do corpo do cliente. O Go só envia respostas pequenas quando o handler retorna, então a duração de `Finish` conta como latência percebida: a Task 11 a mede.
- **Marcador de não capturado:** se houve linha mas nenhuma tentativa elegível, grava `attempt = 0` com `capture_status = 'not_captured'`, sem corpos.
- **Junção:** a leitura busca as linhas de `logs` por `request_id` (`type in (consume, error)`, e `user_id` na rota própria) e só depois `log_contents`. Sem linha de log, é não encontrado, mesmo com conteúdo gravado.

### Tabela `log_contents`

| Coluna | Tipo GORM | Conteúdo |
|-|-|-|
| `id` | `int64`, primary key | ClickHouse: `Int64 DEFAULT 0` |
| `request_id` | `varchar(64)`, not null | Unique `idx_log_contents_request_attempt` (priority 1) |
| `attempt` | `int`, not null | Mesmo unique (priority 2) |
| `user_id` | `int`, not null | Filtro da rota própria, sem índice: o unique já limita as linhas |
| `created_at` | `bigint`, not null | Índice `idx_log_contents_created_at`, para a retenção |
| `capture_status` | `varchar(16)`, not null | `captured` ou `not_captured` |
| `meta` | `text` | JSON de `model.LogContentMeta` |
| `request_body` | `[]byte`, `size:16777215` | gzip do texto limpo |
| `response_body` | `[]byte`, `size:16777215` | gzip do texto limpo |
| `assembled_body` | `[]byte`, `size:16777215` | gzip do JSON `[]ViewMessage`, só SSE no `attempt = 0` |

`LogContentMeta` (JSON em `meta`, sem colunas que exigiriam migração ao evoluir): `relay_format`, `http_status`, `is_stream`, `incomplete`, `channel_id`, `multi_key_index` (-1 sem multi-chave), `method`, `path`, `started_at_ms`, `duration_ms`, `error_message`, `request_unavailable`, e os campos de `BodyMeta` em `request` e `response`: `original_bytes`, `truncated`, `head_only`, `undecodable`, `media_omitted`.

Tudo o que é texto no banco é `meta`, que passa por `common.Marshal` depois de `strings.ToValidUTF8`, e o NUL sai como `\u0000`. Os corpos são binários, então conteúdo arbitrário não derruba o lote no PostgreSQL.

**Migração.**

- SQLite, MySQL, PostgreSQL: `LOG_DB.AutoMigrate(&LogContent{})`. O `size` vira `mediumblob` no MySQL, e `bytea`/`blob` nos demais.
- `model.MigrateLogContents()` entra nos dois caminhos existentes de `model/main.go`: `InitLogDB` (linha ~241, banco de logs igual ao principal, o caso da produção) e `migrateLOGDB` (linha ~408). Só no nó master. A tabela é nova, então não há bloqueio em tabelas do sistema.
- ClickHouse: `CREATE TABLE IF NOT EXISTS`, `ENGINE = MergeTree()`, `PARTITION BY toYYYYMM(toDateTime(created_at))`, `ORDER BY (created_at, request_id, attempt)`, `INDEX idx_request_id request_id TYPE bloom_filter GRANULARITY 1`, `TTL toDateTime(created_at) + INTERVAL 30 DAY DELETE`. Colunas: `String`, `Int32`/`Int64`.
- Sem particionamento no PostgreSQL. A limpeza diária apaga cerca de 3% por dia e o autovacuum padrão limita o inchaço em 20%, o que basta sem SQL específico.

### Fila e gravação

- Fluxo: `Finish` → `queue` (canal com 512 registros) → 2 workers (limpeza + gzip) → 1 gravador. Enfileirar nunca bloqueia: fila cheia descarta, devolve o orçamento e incrementa `Dropped`.
- Registros com remontagem são enfileirados pela goroutine de remontagem quando ela termina, para `Finish` não esperar.
- Lote: 20 registros ou 2 MiB (abaixo do `max_allowed_packet` de 4 MiB do MySQL 5.7) ou 1 s, o que vier primeiro. Timeout de 5 s por lote. Um único gravador usa no máximo uma conexão do pool compartilhado.
- Erro no lote: tenta um a um; o que falhar incrementa `WriteErrors` e é descartado, sem nova tentativa. Conflito de chave é `clause.OnConflict{DoNothing: true}` (sem cláusula no ClickHouse).
- `logcontent.Stats()` devolve `{Enqueued, Written, Dropped, WriteErrors, BudgetInUse}`, contadores por nó. Um `SysLog` por minuto, só quando `Dropped` ou `WriteErrors` mudar.
- `logcontent.Shutdown(ctx)` entra em `main.go` depois de `srv.Shutdown`: recusa novos registros e drena por até 10 s. Compressão: `compress/gzip` com `BestSpeed`.

### Limpeza do conteúdo

Ordem fixa, por corpo, no worker:

1. **Decodificar** o `Content-Encoding`.
2. **Substituir valores exatos** no buffer inteiro, antes de qualquer corte, por `[REDACTED]`. Conjunto: todos os segredos da chamada aplicados a todos os corpos, ou seja, `token_key` e o `info.ApiKey` de cada tentativa. Variantes: valor inteiro; forma com escape JSON; partes de `|`; folhas string com 16+ caracteres quando a chave for um objeto JSON. Valores com menos de 8 caracteres são ignorados. Texto com `sk-` + token vira `sk-[REDACTED]`.
3. **Mais de 8 MiB ou `head_only`:** `stripBase64Runs` no começo, cortar em 1 MiB sem quebrar runa, `truncated` e `head_only`. Sem Conversa.
4. **JSON de até 8 MiB:** percorrer com `gjson` (`ForEach`, `Result.Index`), juntar edições de trecho e aplicar de trás para frente. O texto original é preservado byte a byte fora dos trechos editados: ordem de chaves e números não mudam.
   - **Base64:** string `data:<mime>;base64,` ou campo conhecido (`b64_json`, `image_url.url`, `input_audio.data`, `source.data` com `type: base64`, `inlineData.data`, `inline_data.data`, `audio.data`, `image_generation_call.result`) com 32+ caracteres; ou qualquer string de 1024+ caracteres só do alfabeto base64. Marcador exato: `[base64 <mime> <bytes> bytes omitted]`, com `bytes = len*3/4` e mime do data URL, de `media_type`/`mimeType`/`mime_type`/`format` vizinho, ou `unknown`.
   - **Se ainda passar de 1 MiB:** achar por busca binária o maior `L` tal que encurtar todas as strings maiores que `L` caiba em 1 MiB. Cada uma vira início (`L/2`) + `…[<n> chars omitted]…` + fim (`L/2`), cortando em limite de runa. Com `L` mínimo de 1024 ainda sem caber (por exemplo, vetores de embeddings), cai no passo 3.
5. **Não JSON, ou SSE:** `stripBase64Runs` e, se preciso, corte em 1 MiB. SSE já chega com no máximo 1 MiB.

`stripBase64Runs` varre bytes e troca sequências de 1024+ caracteres do alfabeto (incluindo `-_`) por `[base64 unknown <bytes> bytes omitted]`. A remontada passa só pelos passos 2 e 5.

### Visão normalizada: no servidor

Decisão: **Go**. A remontagem precisa rodar no servidor de qualquer forma, porque o SSE guardado é só cabeça e cauda. Assim, o limite de 1 MiB vale antes de gravar, o papel é filtrado antes de sair do backend, os quatro formatos têm testes de tabela em um só lugar, e o navegador nunca interpreta JSON de 1 MB para cada aba. A leitura usa `gjson`, não os DTOs de `relaykit/dto`: corpos encurtados, de erro ou de provedores irregulares não podem falhar por tipo.

Contrato de `GET .../content/:request_id` (`data`, nos tipos de `service/logcontent/view.go`):

```jsonc
{
  "request_id": "…",
  "available": true,
  "reason": "",              // pending | expired | not_captured | unavailable
  "format": "openai",        // openai | openai_responses | claude | gemini | embedding | rerank
  "is_stream": true,
  "incomplete": false,
  "http_status": 200,
  "conversation": {
    "available": true,       // false: requisição só com o começo
    "response_available": true,
    "messages": [{
      "role": "user",        // system | developer | user | assistant | tool
      "source": "request",   // request | response
      "blocks": [{
        "type": "text",      // text | thinking | tool_call | tool_result | media | error
        "text": "…",
        "id": "…", "name": "…", "arguments": "…",  // tool_call; id em tool_result
        "media_type": "image/png", "size_bytes": 123, // media
        "is_error": false    // tool_result
      }]
    }]
  },
  "tools": [{ "name": "…", "description": "…", "schema": {}, "called": 1 }],
  "tools_available": 3,      // null quando a requisição não pôde ser lida
  "tool_calls": 1,           // null quando a resposta não pôde ser lida
  "client": {
    "request":  { "text": "…", "original_bytes": 1, "truncated": false, "head_only": false, "undecodable": false, "media_omitted": 0 },
    "response": { "…": "mesma forma" }
  },
  "attempts": [{             // só na rota de admin; a rota própria omite a chave
    "attempt": 1, "channel_id": 1, "channel_name": "…", "multi_key_index": -1,
    "status_code": 429, "error_message": "…", "started_at_ms": 1, "duration_ms": 1,
    "method": "POST", "path": "/v1/chat/completions", "request_unavailable": false,
    "request": {}, "response": {}
  }]
}
```

O marcador de base64 vira bloco `media`. A visão não devolve `logs.other`: grupo, afinidade e modelo mapeado vêm da linha que a lista já entrega filtrada por papel, o que mantém a regra de `admin_info`/`root_info` em um só lugar. Embeddings e rerank têm só o lado da requisição em Conversa (`response_available: false`).

### Rotas e papéis

| Rota | Auth | Escopo do token | Efeito |
|-|-|-|-|
| `GET /api/log/content/:request_id` | `AdminAuth` | `log:read` | Inclui `attempts`; registra auditoria |
| `GET /api/log/self/content/:request_id` | `UserAuth` | `usage:read` | Filtra `user_id = c.GetInt("id")`; nunca inclui `attempts` |
| `GET /api/log/content_capture/stats` | `AdminAuth` | `log:read` | `logcontent.Stats()` |

- **Não encontrado:** id vazio, com mais de 64 caracteres, inexistente ou de outra pessoa dão a mesma resposta, `common.ApiErrorT(c, "Log content not found")`.
- **`reason`** (só para linha existente sem conteúdo capturado): linha com menos de 15 s e captura ligada → `pending`; `not_captured` gravado, ou `other.request_path` não elegível → `not_captured`; `created_at` anterior à retenção → `expired`; demais → `unavailable`.
- **Auditoria:** toda leitura bem-sucedida pela rota de admin, inclusive de si mesmo, chama `model.RecordOperationAuditLog` com ação `log_content.view`, categoria `operation`, `AuditAdminInfo` de `controller/audit.go:121` e parâmetros `request_id` e `owner_user_id`, nunca corpos.
- **Nomes de canal:** consulta separada em `model.DB` (`id, name`), porque os bancos podem ser diferentes.

### Configurações

`common.LogContentCaptureEnabled` (padrão `true`) e `common.LogContentRetentionDays` (padrão 30), em `common/log_content.go`. Opções homônimas em `model/option.go`. `controller/option.go` rejeita retenção fora de 1 a 3650. `Begin` lê o interruptor uma vez por chamada: desligado, não há nenhum wrapper, e o caminho fica igual ao upstream. Desligar não apaga o que já existe, e a retenção continua rodando.

### Retenção

`logContentRetentionHandler` em `service/log_content_retention.go` implementa `ScheduledSystemTaskHandler` e se registra no próprio `init()`, sem tocar em `system_task.go`. `Type() = "log_content_retention"`, `Interval() = 24h`, `Enabled() = true`. Com `days < 1` usa 30; `cutoff = now - days*86400`.

Loop: `SELECT id ... WHERE created_at < ? ORDER BY created_at LIMIT 200` → `DELETE WHERE id IN (...)` → pausa de 500 ms, respeitando `ctx`. É a forma portável, já que `LIMIT` em `DELETE` e em subconsulta `IN` varia por banco. Em ClickHouse, nenhum `DELETE`: roda `ALTER TABLE log_contents MODIFY TTL ...` com o valor atual, então uma mudança de configuração vale na execução seguinte.

### Painel

- `LogDetailsSheet` passa a renderizar `LogContentPanel`, em `components/log-content/`: `Sheet modal={false}`, `showOverlay={false}`, por cima da lista, com `LOG_PANEL_WIDTH_CLASS = 'w-full sm:max-w-full lg:w-[60vw] lg:max-w-[60vw]'`.
- **Removidos:** `LOG_PANEL_RESERVE_CLASS`, o `className` de `SectionPageLayout` em `usage-logs/index.tsx`, `cn` e `selectedLog` lá, e a prop `className` de `section-page-layout.tsx` junto com `<Main className>`. Foi a etapa 1 (`971e4e010`) que a criou, e nenhum outro consumidor a usa.
- **Detalhes:** `DetailsDialog` ganha `presentation='embedded'`, que retorna só o corpo, no lugar de `'sheet'`. O corpo atual fica em um `Collapsible` "Details".
- **Rota:** escolhida pelo mesmo `isAdmin` passado a `LogDetailsSheet`, que segue o escopo da visão.
- **Consulta:** `useQuery(['log-content', isAdmin, requestId])`, com `useDebounce(requestId, 150)`, `retry: false`, `staleTime` de 5 min, `refetchInterval` de 3 s enquanto `reason === 'pending'`. Só para `type` consume ou error com `request_id`. Outros tipos mostram só os detalhes.
- **Validação:** esquema zod. Falha no parse leva ao estado de erro.
- **Teclado:** `ArrowUp`/`ArrowDown` no `window`, sem modificadores, ignorando `input, textarea, select, [contenteditable], [role=tablist], [role=menu], [role=listbox]`. Sem mudar de página: nas pontas, os botões ficam desabilitados, porque mudar de página fecha o painel via `resetKey`. A lista vem de `table.getRowModel().rows`.
- **Foco ao fechar:** `getLogRowInteractionProps` passa a incluir `data-log-row-id`, e `close()` procura essa linha, porque o `opener` antigo pode ser outra linha.
- **Texto:** sempre `whitespace-pre-wrap`, sem Markdown e sem HTML. JSON bruto em `<pre>` simples, sem destaque de sintaxe, porque 1 MB em Shiki trava a aba. Tentar `JSON.parse` + `stringify` só até 256 KiB; acima disso, ou em JSON inválido, texto como veio.

---

### Task 1: Tabela e operações de armazenamento

**Files:**
- Create: `model/log_content.go`, `model/log_content_test.go`
- Modify: `model/main.go` (duas chamadas)

**Interfaces:**
- Produces: `model.LogContent`, `model.LogContentMeta`, `model.BodyMeta` (colunas e campos acima)
- Produces: `model.MigrateLogContents() error`
- Produces: `model.InsertLogContents(ctx context.Context, rows []*LogContent) error`
- Produces: `model.GetLogContents(ctx context.Context, requestID string) ([]*LogContent, error)`, por `attempt`, com `LIMIT 32`
- Produces: `model.GetLogRowsForContent(ctx context.Context, requestID string, userID int) ([]*Log, error)`, com `userID == 0` para todos e `type in (consume, error)`
- Produces: `model.DeleteOldLogContentBatch(ctx context.Context, cutoff int64, limit int) (int64, error)`
- Produces: `model.SyncLogContentClickHouseTTL(days int) error`, `model.LogContentCreateTableClickHouseSQL(days int) string`

- [ ] **Step 1: Criar a branch `feat/log-content-capture` a partir de `production` e commitar a spec e este plano**

- [ ] **Step 2: Escrever os testes que falham**

`TestLogContentStorageOnDatabases`, tabela `sqlite`, `mysql`, `postgres`, `clickhouse` (DSNs `AUDIT_MYSQL_DSN`, `AUDIT_POSTGRES_DSN`, `AUDIT_CLICKHOUSE_DSN`; pula sem a variável, como `controller/log_export_test.go`, usando `common.SetLogDatabaseType`): migra duas vezes sem erro; insere corpo binário de 2 MiB com bytes `0x00` e `0xFF` e lê igual; `meta` com `"a\x00b\xff"` vindo de `NewLogContentMeta` persiste.
`TestInsertLogContentsIgnoresDuplicateKey`: mesmo `(request_id, attempt)` duas vezes não dá erro e mantém 1 linha (sem ClickHouse).
`TestDeleteOldLogContentBatchRemovesOldestFirst`: linhas com `created_at` em `cutoff-2`, `cutoff-1`, `cutoff`, `cutoff+1`; `limit = 1` apaga só `cutoff-2`; uma segunda chamada apaga `cutoff-1`; `cutoff` e `cutoff+1` ficam.
`TestGetLogRowsForContentFiltersUserAndType`: linhas de consume e error do usuário 1, de consume do usuário 2 com o mesmo `request_id`, e de topup: `userID=1` traz 2, `userID=0` traz 3, e topup nunca aparece.
`TestLogContentCreateTableClickHouseSQL`: contém `ORDER BY (created_at, request_id, attempt)`, `bloom_filter`, `INTERVAL 30 DAY`.

- [ ] **Step 3: Rodar e ver falhar** — Run: `go test ./model/ -run 'LogContent' -v` / Expected: FAIL de compilação.

- [ ] **Step 4: Implementar `model/log_content.go` e as duas chamadas de `MigrateLogContents` em `model/main.go`**

Em `InitLogDB`, `return MigrateAuditLogs()` vira chamar as duas em sequência. Em `DeleteOldLogContentBatch`, com ClickHouse, retornar `0, nil`.

- [ ] **Step 5: Rodar e ver passar** — Run: `go test ./model/ -run 'LogContent' -v` / Expected: PASS em SQLite; os demais rodam na Task 12.

- [ ] **Step 6: Commit** — `feat(logs): add log_contents storage`

### Task 2: Opções de captura e retenção no backend

**Files:**
- Create: `common/log_content.go`, `controller/log_content_option_test.go`
- Modify: `model/option.go`, `controller/option.go`

**Interfaces:**
- Produces: `common.LogContentCaptureEnabled bool` (padrão `true`), `common.LogContentRetentionDays int` (padrão `30`)
- Produces: chaves de opção `LogContentCaptureEnabled`, `LogContentRetentionDays`

- [ ] **Step 1: Escrever os testes que falham**

`TestLogContentOptionsDefaultsAndUpdate`: depois de `InitOptionMap`, `OptionMap` tem `"true"` e `"30"`; `UpdateOption("LogContentRetentionDays", "7")` deixa `common.LogContentRetentionDays == 7`; `UpdateOption("LogContentCaptureEnabled", "false")` deixa a variável `false`.
`TestUpdateOptionRejectsInvalidRetention`, no controller: `0`, `-1`, `3651`, `abc` retornam `success: false` e não mudam a variável; `1` e `3650` passam.

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./model/ ./controller/ -run 'LogContentOption|InvalidRetention' -v` / Expected: FAIL.

- [ ] **Step 3: Implementar** seguindo `LogConsumeEnabled` (`model/option.go:53` e `:434`) e o padrão de `case` de validação de `controller/option.go`.

- [ ] **Step 4: Rodar e ver passar** — Run: mesmo comando / Expected: PASS.

- [ ] **Step 5: Commit** — `feat(logs): add content capture and retention options`

### Task 3: Limpeza do conteúdo

Função pura, sem gin nem banco.

**Files:**
- Create: `service/logcontent/limits.go`, `service/logcontent/sanitize.go`, `service/logcontent/sanitize_test.go`

**Interfaces:**
- Produces: `type Secrets []string`; `func NewSecrets(values ...string) Secrets` (variantes e mínimo de 8)
- Produces: `func SanitizeBody(raw []byte, encoding string, headOnly bool, secrets Secrets) SanitizedBody`
- Produces: `type SanitizedBody struct { Text string; Meta model.BodyMeta }`
- Produces: `func SanitizeAssembled(text string, secrets Secrets) string`

- [ ] **Step 1: Escrever os testes que falham**

`TestSanitizeReplacesExactSecrets`: token e chave em corpo JSON, em erro `{"error":{"message":"bad key sk-abc12345"}}` e com escape JSON (chave com `/`): nenhum `Contains` do valor original.
`TestNewSecretsCompositeAndShortKeys`: `ak12345678|sk12345678|us-east-1` substitui as 3 partes; JSON de conta de serviço substitui a folha `private_key`; `ollama` não é substituído em "use ollama locally"; string vazia não altera nada.
`TestSanitizeSecretNotSplitByCut`: chave nas posições em volta de 1 MiB em corpo de 9 MiB: resultado nem contém a chave inteira, nem os primeiros 8 caracteres dela no fim.
`TestSanitizeBase64Markers`, tabela: `data:image/png;base64,` → `[base64 image/png <n> bytes omitted]`; Claude `source.data` com `media_type: image/jpeg`; Gemini `inlineData` com `mimeType: audio/wav`; `input_audio` com `format: mp3` → `audio/mp3`; `b64_json`; string base64 solta de 2000 caracteres → `unknown`; texto de 2000 caracteres com espaços fica intacto; `Meta.MediaOmitted` conta certo.
`TestSanitizePreservesJSONOutsideEdits`: `{"z":1.50,"a":12345678901234567890,"img":"data:…"}` mantém ordem e números no texto final.
`TestSanitizeShortensLongMessagesKeepingStartAndEnd`: mensagem de 3 MiB com marcas em início e fim: `len(Text) <= 1<<20`, `gjson.Valid`, marcas presentes, `…[`, `Truncated == true`, `HeadOnly == false`; uma mensagem curta vizinha fica intacta.
`TestSanitizeBodyAtExactLimits`: 1 MiB exato sem corte e `Truncated == false`; 1 MiB + 1 encurtado; 8 MiB exato ainda JSON; `headOnly=true` guarda 1 MiB, `HeadOnly == true`, runa não quebrada no fim.
`TestSanitizeEmbeddingVectorsFallBackToHead`: 2 MiB de números: `HeadOnly == true`.
`TestSanitizeHeadOnlyStripsBase64`: começo de JSON cortado no meio de um base64 longo: o trecho vira marcador.
`TestSanitizeDecodesAndLimitsEncodings`: `gzip`, `deflate`, `br`, `zstd` decodificam; bomba gzip de 1 KiB para 1 GiB para em 8 MiB, com `Undecodable == true` e `Text == ""`; `snappy` → `Undecodable`; lixo com `gzip` → `Undecodable`.
`TestSanitizeInvalidUTF8AndNul`: corpo com `\xff\x00`: não entra em pânico, `utf8.ValidString(Text)`, tratado como não JSON.

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./service/logcontent/ -run 'Sanitize|NewSecrets' -v` / Expected: FAIL de compilação.

- [ ] **Step 3: Implementar a ordem fixada em "Limpeza do conteúdo"**

- [ ] **Step 4: Rodar e ver passar** — Run: mesmo comando / Expected: PASS.

- [ ] **Step 5: Commit** — `feat(logs): sanitize captured bodies`

### Task 4: Visão normalizada dos quatro formatos

**Files:**
- Create: `service/logcontent/view.go`, `view_openai.go`, `view_responses.go`, `view_claude.go`, `view_gemini.go`, `view_embedding.go`, `view_test.go`
- Create (golden compartilhado com o frontend): `web/src/features/usage-logs/components/log-content/__tests__/fixtures/view-{openai,responses,claude,gemini}.json`
- Test fixtures: `service/logcontent/testdata/{openai,responses,claude,gemini}_{request,response}.json`

**Interfaces:**
- Produces: `ViewMessage`, `ViewBlock`, `ViewTool`, `BodyView`, `AttemptView`, `ContentView`, tags JSON exatamente como no contrato
- Produces: `func ParseRequestView(format string, body string) (msgs []ViewMessage, tools []ViewTool)`
- Produces: `func ParseResponseView(format string, body string) []ViewMessage`
- Produces: `func BuildContentView(rows []*model.LogContent, includeAttempts bool, channelNames map[int]string) ContentView`
- Produces: formatos válidos em constantes: `FormatOpenAI`, `FormatResponses`, `FormatClaude`, `FormatGemini`, `FormatEmbedding`, `FormatRerank`

Mapeamento por formato:

- **OpenAI:** `messages[]`, conteúdo string ou partes, `tool_calls`, `tool_call_id`, `prompt` legado, `tools[].function`, `functions`, `reasoning_content`.
- **Responses:** `instructions`, `input` string ou itens (`message`, `function_call`, `function_call_output`, `reasoning`), `tools[]` (tool sem `name` usa `type`), `output[]`.
- **Claude:** `system`, `messages[].content[]` (`text`, `image`, `document`, `tool_use`, `tool_result`, `thinking`), `tools[].input_schema`, `content[]`.
- **Gemini:** `systemInstruction`/`system_instruction`, `contents[]` (`model` → `assistant`), `parts[]` (`text`, `thought`, `functionCall`, `functionResponse`, `inlineData`), `tools[].functionDeclarations[]` com `parameters` ou `parametersJsonSchema`, `candidates[].content`.
- **Todos:** `error` na resposta vira bloco `error`.

- [ ] **Step 1: Escrever os testes que falham**

`TestBuildViewGolden`, para cada formato: fixture com system, usuário com texto e imagem marcada, chamada de tool, resultado de tool e resposta; `BuildContentView` igual ao JSON em `web/.../fixtures/view-<formato>.json`.
`TestViewCountsTools`: `tools_available == 3`, `tool_calls == 2`, `tools[0].called == 2`.
`TestViewHeadOnlyGivesNullCounts`: requisição com `HeadOnly`: `conversation.available == false`, `tools_available == null` no JSON serializado (nunca `0`), `client.request.text` presente.
`TestViewResponseHeadOnlyKeepsRequest`: `response_available == false` e mensagens da requisição presentes.
`TestViewErrorBodies`, para cada formato: corpo de erro do formato vira bloco `error`, `is_error` correto em `tool_result` Claude.
`TestViewToleratesGarbage`: `""`, `"not json"`, `"[]"`, `{"messages":"x"}`, `{"messages":[1,null]}`: sem pânico e `messages` vazio.
`TestViewEmbeddingsAndRerank`: `input` string e lista viram blocos do usuário; `query` do rerank idem; `response_available == false`.
`TestViewOmitsAttemptsForSelf`: com `includeAttempts=false`, o JSON serializado não contém `attempts`, `channel`, `multi_key` nem o sentinela gravado só no corpo do provedor.
`TestViewCorruptGzipIsUndecodable`: blob inválido em `response_body`: `client.response.undecodable == true`, sem erro geral.

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./service/logcontent/ -run 'View' -v` / Expected: FAIL.

- [ ] **Step 3: Implementar** com `gjson`, tolerando tipos inesperados em cada campo. O marcador de base64 (`^\[base64 (\S+) (\d+) bytes omitted\]$`) vira bloco `media`.

- [ ] **Step 4: Rodar e ver passar** — Run: mesmo comando / Expected: PASS.

- [ ] **Step 5: Commit** — `feat(logs): normalize captured content into a view model`

### Task 5: Remontagem de streams

**Files:**
- Create: `service/logcontent/assemble.go`, `assemble_test.go`
- Test fixtures: `service/logcontent/testdata/{openai,responses,claude,gemini}_stream.sse`

**Interfaces:**
- Consumes: `ViewMessage`, `ViewBlock`, `ParseResponseView`
- Produces: `func NewStreamAssembler(format string) *StreamAssembler`; `(*StreamAssembler).Write(chunk []byte)`; `(*StreamAssembler).Result() AssembledStream`
- Produces: `type AssembledStream struct { Messages []ViewMessage; Terminal bool; Truncated bool }`

Regras por formato:

- **OpenAI:** `choices[].delta.{content,reasoning_content,reasoning,tool_calls[index]}`, `choices[].text`, terminal em `[DONE]` ou `finish_reason`; até 8 `choices`.
- **Responses:** `response.output_text.delta`, `response.reasoning_summary_text.delta`, `response.output_item.added`, `response.function_call_arguments.delta`; terminal `response.completed|failed|incomplete`; em `completed` com `response.output` não vazio, troca o acumulado por `ParseResponseView`.
- **Claude:** `content_block_start`, `content_block_delta` (`text_delta`, `thinking_delta`, `input_json_delta`), `message_stop`, `error`.
- **Gemini:** `candidates[0].content.parts[]`, `finishReason`, `error`; corpo começando com `[` é tratado como lista.
- **Todos:** quadro com mais de 1 MiB é ignorado e marca `Truncated`.

- [ ] **Step 1: Escrever os testes que falham**

`TestAssembleGolden`, para cada formato: resultado igual a `ParseResponseView` da resposta não stream equivalente, e `Terminal == true`.
`TestAssembleAnyChunkBoundary`: a mesma fixture alimentada em blocos de 1, 7 e 4096 bytes dá resultado idêntico, incluindo `\r\n` e um caractere multibyte cortado entre blocos.
`TestAssembleIgnoresKeepAlive`: `: PING`, `event: ping` e `data: {"type":"ping"}` não criam blocos.
`TestAssembleInterruptedStream`: fixture cortada no meio: `Terminal == false` e o texto até ali presente.
`TestAssembleMessageLimit`: 3 MiB de deltas: bytes de texto `<= 1<<20`, `Truncated == true`.
`TestAssembleToolCallAcrossFrames`: argumentos em 5 quadros viram um bloco `tool_call` com o JSON inteiro.
`TestAssembleMalformedFrame`: `data: {nope` no meio é ignorado e o resto continua.

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./service/logcontent/ -run 'Assemble' -v` / Expected: FAIL.

- [ ] **Step 3: Implementar**

- [ ] **Step 4: Rodar e ver passar** — Run: mesmo comando / Expected: PASS.

- [ ] **Step 5: Commit** — `feat(logs): reassemble streams for the four formats`

### Task 6: Orçamento, fila, gravação e encerramento

**Files:**
- Create: `service/logcontent/budget.go`, `queue.go`, `queue_test.go`
- Modify: `main.go` (uma linha depois de `srv.Shutdown`), `.env.example`

**Interfaces:**
- Consumes: `model.InsertLogContents`, `SanitizeBody`, `SanitizeAssembled`
- Produces: `type captureRecord` (dados crus de uma chamada, com a reserva de orçamento)
- Produces: `func enqueue(rec *captureRecord) bool`
- Produces: `type StatsSnapshot struct { Enqueued, Written, Dropped, WriteErrors, BudgetInUse int64 }`; `func Stats() StatsSnapshot`
- Produces: `func Shutdown(ctx context.Context)`
- Produces: `func tryReserve(n int64) bool`, `func release(n int64)`

- [ ] **Step 1: Escrever os testes que falham**

`TestEnqueueNeverBlocksWhenFull`: gravador parado, 513 registros: chamadas em menos de 100 ms no total, `Dropped == 1`, orçamento do descartado devolvido.
`TestWriteErrorIsCountedAndDropped`: `LOG_DB` sem a tabela: `WriteErrors` sobe e não há pânico nem bloqueio.
`TestBatchFallsBackToSingleInserts`: lote com um registro em conflito de tipo simulado por gancho de teste: os outros são gravados.
`TestBatchFlushesOnCountSizeAndTime`: 20 registros gravam já; 2 MiB gravam já; 1 registro grava em até 1,5 s.
`TestShutdownFlushesQueue`: 10 registros e `Shutdown`: `Written == 10`; `enqueue` depois retorna `false`. `TestShutdownRespectsDeadline`: gravador travado e `ctx` de 50 ms retorna em menos de 200 ms.
`TestBudgetExhaustedAndReleased`: com limite pequeno, `tryReserve` falha, e depois da gravação `BudgetInUse == 0`.
`TestStoredBodiesAreGzip`: colunas começam com `1f 8b` e descomprimem no texto limpo.

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./service/logcontent/ -run 'Enqueue|Write|Batch|Shutdown|Budget|Stored' -race -v` / Expected: FAIL.

- [ ] **Step 3: Implementar**, com workers iniciados por `sync.Once` no primeiro `enqueue` e proteção contra `model.LOG_DB == nil` (conta `WriteErrors`). Adicionar `logcontent.Shutdown(ctx)` em `main.go` e `# LOG_CONTENT_CAPTURE_MEMORY_MB=256` no `.env.example`.

- [ ] **Step 4: Rodar e ver passar** — Run: mesmo comando / Expected: PASS.

- [ ] **Step 5: Commit** — `feat(logs): add bounded queue and batch writer for content`

### Task 7: Captura no caminho da chamada

**Files:**
- Create: `service/logcontent/capture.go`, `tap.go`, `tap_test.go`, `model/log_content_observer.go`, `relay/channel/log_content.go`, `relay/helper/log_content_keepalive_test.go`, `controller/log_content_relay_test.go`
- Modify: `controller/relay.go` (1 linha), `relay/channel/api_request.go` (2 linhas em `DoApiRequest`), `model/log.go` (1 linha em `RecordConsumeLog`, 1 em `RecordErrorLog`)

**Interfaces:**
- Consumes: `enqueue`, `tryReserve`, `release`, `NewStreamAssembler`, `model.LogRowObserver`
- Produces: `func Begin(c *gin.Context, relayFormat types.RelayFormat) *Capture`; `(*Capture).Finish()`; `(*Capture).OnLogRow(log *model.Log)`
- Produces: `type AttemptInfo struct { Eligible bool; ChannelID, MultiKeyIndex int; ChannelKey, Format string }`
- Produces: `func BeginAttempt(c *gin.Context, info AttemptInfo, req *http.Request) *Attempt`; `(*Attempt).ObserveResponse(resp *http.Response, err error) *http.Response`
- Produces em `relay/channel/log_content.go`: `func beginLogContentAttempt(c *gin.Context, info *common.RelayInfo, req *http.Request) *logcontent.Attempt`

- [ ] **Step 1: Escrever os testes que falham**

Em `tap_test.go`:
`TestClientWriterPassesThroughExactly`: mesmo retorno e bytes de `Write` e `WriteString`, inclusive em escrita parcial com erro; só os bytes aceitos entram na cópia.
`TestClientWriterUnwrapReachesFlushError`: a mesma caminhada por `Unwrap` de `relay/helper/common.go` chega ao `FlushError` do escritor de teste.
`TestClientWriterPreservesWriteDeadline`: `http.NewResponseController(w).SetWriteDeadline` atinge o escritor de base.
`TestBodyTapPassesThroughReadErrors`: `n`, `err` e `Close` iguais ao corpo original.
`TestTapSSEKeepsHeadAndTailOnly`: 100 MiB em frames: memória retida `<= sseHeadBytes+sseTailBytes`; começo, fim com `[DONE]` e cortes em fim de linha.
`TestTapNonSSEDropsToHeadOver8MiB`: 8 MiB exato mantém tudo; 8 MiB+1 reduz para 1 MiB e devolve o orçamento.
`TestLateWriteAfterFinishIsIgnored`: `Write`/`Read` concorrentes com `Finish`, com `-race`.
`TestFinishDoesNotSwallowRelayPanic`: `defer Begin(...).Finish()` em função que entra em pânico: o pânico chega a quem chamou.
`TestCaptureInternalPanicDoesNotAffectCall`: gancho de teste faz a cópia entrar em pânico: `Write` retorna normal e a captura é desativada.
`TestBeginDisabledInstallsNothing`: com `LogContentCaptureEnabled=false`, `c.Writer` é o mesmo objeto e `Finish` em `nil` não falha.

Em `relay/helper/log_content_keepalive_test.go`:
`TestKeepAliveOverCaptureWriter`: `Begin` e depois `StartStreamKeepAlive`, ping recebido; `StreamScannerHandler` para o keepalive pela verificação de tipo; os dados chegam em seguida, e a cópia contém ping e dados na ordem da saída.

Em `controller/log_content_relay_test.go`, com `controller.Relay` real, SQLite e provedor `httptest`, seguindo `controller/plugin_native_e2e_test.go`:
`TestCaptureNonStreamStoresCallAndAttempt`: linhas `attempt` 0 e 1, corpos certos, `method`, `path` sem query, `status_code`, e chave do canal e token ausentes de todos os blobs descomprimidos. O provedor recebe uma query com segredo: ela não aparece.
`TestCaptureStreamStoresRawAndAssembled`: `assembled_body` com a mensagem inteira; `incomplete == false`.
`TestCaptureRetryStoresEachAttempt`: 429 depois 200. Com `ERROR_LOG_ENABLED=true`, tentativas 0, 1, 2; com `false`, 0 e 2.
`TestCaptureAllAttemptsFailed`: com logs de erro ligados, `attempt = 0` traz na resposta o JSON de erro escrito pelo `defer` de `Relay`, com `http_status` certo; sem logs de erro, nenhuma linha.
`TestCaptureGatedByConsumeLogOff`: `LogConsumeEnabled=false`, zero linhas.
`TestCaptureSkipsRefusedBeforeAttempt`: sem cota, zero linhas.
`TestCaptureIneligibleWritesMarker`: `/v1/moderations`: uma linha `not_captured`, sem corpos.
`TestCaptureClientDisconnectMarksIncomplete`: cancelar no meio: `incomplete == true`, com o que chegou até ali, e a linha de `logs` igual à de hoje.
`TestCaptureUpstreamCutMarksIncomplete`: provedor fecha sem terminal.
`TestUpstreamGzipBodyDecodedOffPath`: header override `Accept-Encoding: gzip`: cliente recebe os mesmos bytes de hoje; o conteúdo gravado é o texto descomprimido.
`TestUpstreamRequestWithoutGetBodyIsMarked`: leitor sem `GetBody`: `request_unavailable == true` e o provedor recebe o corpo inteiro.
`TestBillingUnchangedWithCapture`: mesma chamada com captura ligada e desligada: `quota`, tokens e cota do usuário iguais, com `cache_tokens` diferente de zero na fixture.

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./service/logcontent/ ./relay/helper/ ./controller/ -run 'ClientWriter|BodyTap|Tap|LateWrite|Finish|CaptureInternal|BeginDisabled|KeepAliveOverCapture|Capture|UpstreamGzip|UpstreamRequest|BillingUnchanged' -race -v` / Expected: FAIL.

- [ ] **Step 3: Implementar**

Em `controller/relay.go`, primeira linha depois de `requestId := ...`: `defer logcontent.Begin(c, relayFormat).Finish()`.

Em `DoApiRequest`, troca mínima em volta de `doRequest`:

```go
logContent := beginLogContentAttempt(c, info, req)
resp, err := doRequest(c, req, info)
resp = logContent.ObserveResponse(resp, err)
```

Todos os ganchos usam `defer` com `recover` interno, sem reexecutar nada na chamada. `Finish` lê o corpo do cliente só se houver linha gravável.

- [ ] **Step 4: Rodar e ver passar** — Run: mesmo comando / Expected: PASS, incluindo `go test ./controller/ -run 'Responses' ` sem regressão com a captura ligada por padrão.

- [ ] **Step 5: Commit** — `feat(logs): capture call content on the relay path`

### Task 8: Retenção diária

**Files:**
- Create: `service/log_content_retention.go`, `service/log_content_retention_test.go`

**Interfaces:**
- Consumes: `model.DeleteOldLogContentBatch`, `model.SyncLogContentClickHouseTTL`, `model.FinishSystemTask`, `NewSystemTaskProgressReporter`, `common.LogContentRetentionDays`
- Produces: `logContentRetentionHandler`, `func logContentRetentionCutoff(now int64, days int) int64`

- [ ] **Step 1: Escrever os testes que falham**

`TestLogContentRetentionDeletesOnlyExpired`: linhas 1 s antes do corte, no corte e 1 s depois: só a primeira some.
`TestLogContentRetentionUsesConfiguredDays`: `days=7` usa `now-604800`; `days=0` e `-1` usam 30.
`TestLogContentRetentionPausesBetweenBatches`: 450 linhas: 3 lotes e 2 pausas, com pausa injetada no teste.
`TestLogContentRetentionHonorsCancel`: `ctx` cancelado entre lotes termina com a tarefa finalizada, sem novas exclusões.
`TestLogContentRetentionRunsWithCaptureOff`: com o interruptor desligado, `Enabled() == true` e apaga.
`TestLogContentRetentionIsScheduled`: o handler está no registro, implementa `ScheduledSystemTaskHandler`, `Interval() == 24h`.
`TestLogContentRetentionClickHouseUsesTTL`: com banco de logs ClickHouse (`SetLogDatabaseType`), chama o TTL, não `DELETE`.
`TestLogContentRetentionLeavesLogsAlone`: a tabela `logs` mantém todas as linhas.

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./service/ -run 'LogContentRetention' -v` / Expected: FAIL.

- [ ] **Step 3: Implementar**

- [ ] **Step 4: Rodar e ver passar** — Run: mesmo comando / Expected: PASS.

- [ ] **Step 5: Commit** — `feat(logs): expire captured content daily`

### Task 9: Rotas, papéis e auditoria

**Files:**
- Create: `controller/log_content.go`, `controller/log_content_test.go`
- Modify: `router/api-router.go` (3 linhas), `middleware/access_token_routes.go` (3 linhas)

**Interfaces:**
- Consumes: `model.GetLogRowsForContent`, `model.GetLogContents`, `logcontent.BuildContentView`, `logcontent.Stats`
- Produces: `controller.GetLogContent`, `controller.GetSelfLogContent`, `controller.GetLogContentCaptureStats`
- Produces: as rotas e o contrato em "Rotas e papéis" e "Visão normalizada"

- [ ] **Step 1: Escrever os testes que falham**

`TestSelfRouteNotFoundParity`: `request_id` de outra pessoa, inexistente e com 65 caracteres: status e corpo idênticos byte a byte.
`TestSelfRouteOmitsProviderData`: resposta não contém a chave `attempts`, `channel`, nem o sentinela gravado nos corpos do provedor, mesmo para root usando a rota própria.
`TestSelfRouteReturnsOwnContent`: conversa e `client` presentes.
`TestAdminRouteReadsAnyUserIncludingRoot`: admin lê conteúdo de root; `attempts` com `channel_name` do banco principal e `multi_key_index`, nunca o valor da chave.
`TestAdminRouteRejectsCommonUser`: usuário comum recebe a negação de `AdminAuth`.
`TestAdminReadWritesAuditLog`: uma linha em `audit_logs` com ação `log_content.view`, `actor_role`, `request_id` e `owner_user_id`, e sem o texto da conversa. Não encontrado não gera auditoria. A rota própria não gera auditoria.
`TestContentNeedsLogRow`: conteúdo gravado sem linha em `logs`: não encontrado.
`TestLogRowReasons`, tabela: linha com 5 s e sem conteúdo → `pending`; 16 s → `unavailable`; mais antiga que a retenção por 1 s → `expired`; no limite exato da retenção → `unavailable`; marcador `not_captured` → `not_captured`; `other.request_path` de imagem sem conteúdo → `not_captured`; captura desligada com 5 s → `unavailable`.
`TestAccessTokenRouteRules`: as 3 rotas com os escopos da tabela; um token só com `usage:read` não acessa a de admin; o teste existente de cobertura de rotas passa.
`TestGetLogContentCaptureStatsAdminOnly`: formato e negação para usuário comum.

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./controller/ ./middleware/ ./router/ -run 'SelfRoute|AdminRoute|AdminRead|ContentNeeds|LogRowReasons|AccessToken|LogContentCapture' -v` / Expected: FAIL.

- [ ] **Step 3: Implementar**, registrando as rotas ao lado de `/summary` em `router/api-router.go:326`.

- [ ] **Step 4: Rodar e ver passar** — Run: mesmo comando / Expected: PASS.

- [ ] **Step 5: Commit** — `feat(logs): add content routes with role filtering and audit`

### Task 10: Comparação da saída com captura ligada e desligada, com o patch 7033

**Files:**
- Create: `relay/channel/openai/log_content_compare_test.go`, `scripts/log-content-compare.sh`

**Interfaces:**
- Consumes: `logcontent.Begin`, `channel.DoApiRequest`, `openai.Adaptor.DoResponse`

- [ ] **Step 1: Escrever o teste**

`TestLogContentCaptureDoesNotChangeClientBytes`, com provedor `httptest` de fixture fixa (`request_id` e `created` fixos), roda cada caso com a captura desligada e ligada e compara corpo byte a byte, status e cabeçalhos (menos `X-Oneapi-Request-Id`). Casos:

- SSE OpenAI normal (caminho direto do 7033)
- SSE com quadro só de `usage` no fim, e com quadro não padrão depois do `usage`
- SSE convertido para Claude (`RelayFormatClaude`, caminho atrasado em um quadro)
- `/v1/responses` em stream
- Resposta não stream
- Não stream gzip por header override
- Erro 429
- Provedor que corta no meio do stream
- Quadros com caractere multibyte cortado entre leituras

`TestLogContentCaptureKeepsPingDelivery`, separado por depender de tempo: com ping de 20 ms e provedor lento, os dois modos entregam pings antes do primeiro dado e os mesmos dados depois, comparando sem as linhas de ping.

Assert adicional: com a captura ligada, sobre os casos acima, houve cópia (senão a comparação não prova nada).

- [ ] **Step 2: Escrever `scripts/log-content-compare.sh`**

Passos, com `set -eu`: `git archive HEAD -- . ':!web'` em um diretório temporário; `sh scripts/apply-patches.sh` nele, como no `Dockerfile`, usando Alpine com GNU `patch` (`golang:1.26.10-alpine`) para replicar o build, ou `patch` local quando `--local`, e mantendo o diretório quando `--keep`; `grep -q directForward relay/channel/openai/relay-openai.go`, falhando se o 7033 não estiver aplicado; `go test -count=1 ./relay/channel/openai/ ./relay/helper/ -run 'LogContent|KeepAliveOverCapture'` (sem `-race`, que a imagem Alpine não suporta sem cgo; a Task 7 já cobre corridas); e os testes de aceitação do `patches/README.md` (`go test -count=1 ./relay/channel/openai/` e `cd relaykit && GOWORK=off go test -count=1 ./relayconvert/internal/oai_chat/`).

- [ ] **Step 3: Rodar sem patch** — Run: `go test -count=1 ./relay/channel/openai/ -run 'LogContent' -v` / Expected: PASS.

- [ ] **Step 4: Commitar e rodar com os patches** — Run: `sh scripts/log-content-compare.sh` / Expected: `directForward` encontrado e tudo PASS. Qualquer diferença de byte é defeito na captura: corrigir a Task 7, nunca normalizar a comparação.

- [ ] **Step 5: Commit** — `test(logs): prove client output is identical with capture on and off`

### Task 11: Medição de latência e memória

**Files:**
- Create: `relay/channel/openai/log_content_measure_test.go` (build tag `logcontentmeasure`), `docs/superpowers/plans/2026-11-05-log-content-capture-measurements.md`

**Interfaces:**
- Consumes: mesma montagem da Task 10; `logcontent.Stats`, `runtime.ReadMemStats`

Limites e motivo:

| Medida | Limite | Por quê |
|-|-|-|
| Tempo até o primeiro byte do stream, ligada menos desligada, com 50 streams simultâneos | p50 ≤ 1 ms, p99 ≤ 5 ms | O primeiro token de provedores leva ao menos centenas de ms; 1 ms fica abaixo de 0,5% e abaixo do ganho que motivou o 7033 |
| Duração total de um stream de 100 MiB sem espera no provedor | ≤ +5% | É o pior caso: nada além da cópia dilui o custo |
| `Finish` com corpos de 1 MiB / 8 MiB | p99 ≤ 2 ms / ≤ 10 ms | O Go só envia respostas pequenas quando o handler retorna, então isso soma à latência percebida |
| Pico de `HeapInuse`, ligada menos desligada, 50 streams de 100 MiB | ≤ 175 MiB | 50 × 3,5 MiB, o pior caso calculado |
| Pico com stream de 10 MiB contra 100 MiB | diferença ≤ 10% | A memória não pode depender da duração, como exige a spec |
| Pico com 20 respostas não stream de 50 MiB simultâneas | ≤ 288 MiB | Orçamento de 256 MiB + 32 MiB de folga para o runtime |
| 5 s depois do fim, com `GC` | `BudgetInUse == 0` e `HeapInuse` ≤ base + 16 MiB | Sem vazamento |

- [ ] **Step 1: Escrever `TestLogContentMeasureStreamLatency`, `TestLogContentMeasureLongStreamMemory`, `TestLogContentMeasureLargeResponseMemory`, `TestLogContentMeasureFinishCost`** com esses asserts, cada uma imprimindo os números das duas configurações. Comparar sempre as duas no mesmo processo e na mesma máquina, com o gravador real.

- [ ] **Step 2: Rodar em SQLite** — Run: `go test -tags logcontentmeasure -count=1 -timeout 30m ./relay/channel/openai/ -run 'LogContentMeasure' -v` / Expected: PASS.

- [ ] **Step 3: Rodar com a árvore com patches** — Run: o mesmo comando dentro do diretório criado por `scripts/log-content-compare.sh --keep` / Expected: PASS.

- [ ] **Step 4: Registrar os números no arquivo de medições**, com máquina, Go e data. Passar de um limite é defeito: corrigir antes de seguir, sem mudar o limite sem aprovação.

- [ ] **Step 5: Commit** — `test(logs): measure capture latency and memory`

### Task 12: Execução em PostgreSQL, MySQL e ClickHouse

**Files:**
- Create: `service/logcontent/load_test.go` (build tag `logcontentload`)
- Modify: arquivo de medições da Task 11

**Interfaces:**
- Consumes: `enqueue`, `model.DeleteOldLogContentBatch`

- [ ] **Step 1: Subir os bancos só em loopback**

Run: `docker run -d --rm --name lc-pg -p 127.0.0.1:55432:5432 -e POSTGRES_PASSWORD=test postgres:16` / `docker run -d --rm --name lc-my -p 127.0.0.1:53306:3306 -e MYSQL_ROOT_PASSWORD=test mysql:8` / `docker run -d --rm --name lc-ch -p 127.0.0.1:59000:9000 -e CLICKHOUSE_PASSWORD=test clickhouse/clickhouse-server:24.8`
Expected: os três aceitam conexão.

- [ ] **Step 2: Rodar os testes com os DSNs**

Run: `AUDIT_POSTGRES_DSN=... AUDIT_MYSQL_DSN=... AUDIT_CLICKHOUSE_DSN=... go test -count=1 ./model/ ./controller/ ./service/ -run 'LogContent|SelfRoute|AdminRoute|AdminRead|LogRowReasons|Capture'`
Expected: PASS, sem `SKIP` em PostgreSQL e MySQL, incluindo binário de 2 MiB, marcas NUL e `TestLogContentCreateTableClickHouseSQL` executada no servidor. Sem ClickHouse disponível, registrar no arquivo de medições.

- [ ] **Step 3: Escrever `TestLogContentPostgresDoesNotDisturbLogs`**

Com o banco de logs igual ao principal, como na produção: mede o p99 de `INSERT` em `logs` e de `SELECT` em `users` por 60 s sem carga, depois com 200 registros/s na fila e a retenção apagando 200 mil linhas expiradas. Asserts: p99 com carga ≤ 2 × base + 5 ms; `pg_stat_activity` sem `wait_event_type = 'Lock'` em `logs`/`users`; `Dropped` registrado. Rodar também com 100 linhas-alvo logo depois do corte, para confirmar que não são apagadas.

- [ ] **Step 4: Rodar** — Run: `AUDIT_POSTGRES_DSN=... go test -tags logcontentload -count=1 -timeout 20m ./service/logcontent/ -run 'LogContentPostgres' -v` / Expected: PASS; registrar os números.

- [ ] **Step 5: Commit** — `test(logs): run content storage on PostgreSQL, MySQL and ClickHouse`

### Task 13: Painel: invólucro, cabeçalho, indicadores e navegação

**Files:**
- Create em `web/src/features/usage-logs/components/log-content/`: `log-content-panel.tsx`, `log-content-header.tsx`, `log-content-stats.tsx`, `log-content-notice.tsx`, `log-content-menu.tsx`, `use-log-content.ts`, `__tests__/log-content-panel.test.tsx`
- Create: `web/src/features/usage-logs/data/log-content-schema.ts`
- Modify: `components/log-details-sheet.tsx`, `components/dialogs/details-dialog.tsx`, `usage-logs/index.tsx`, `components/usage-logs-table.tsx`, `lib/row-interaction.ts`, `lib/export.ts`, `api.ts`, `web/src/components/layout/components/section-page-layout.tsx`, `components/__tests__/log-details-sheet.test.tsx`

**Interfaces:**
- Consumes: contrato JSON da visão
- Produces: `logContentSchema`, `type LogContentView`
- Produces: `getLogContent(requestId: string, isAdmin: boolean): Promise<LogContentView>`, tratando `success: false` como erro de não encontrado
- Produces: `useLogContent(log: UsageLog, isAdmin: boolean)`, retornando o resultado da query
- Produces: `LogContentPanel(props: { log, logs: UsageLog[], isAdmin, isRoot, onClose })`, com slots para as abas das Tasks 14 e 15
- Produces: `downloadText(filename: string, mime: string, content: string): void` em `lib/export.ts`, usado também por `downloadCsv`, que mantém o BOM

- [ ] **Step 1: Escrever os testes que falham**

Largura: o painel tem `lg:w-[60vw]`, e nem a página nem `<Main>` têm classe `lg:pr-`; `LOG_PANEL_RESERVE_CLASS` deixou de ser exportada.
Estados: carregando; `pending` faz nova busca e depois mostra o conteúdo; `expired`, `not_captured` ("Content not captured for this type of call") e `unavailable` têm textos distintos, com Details disponível; erro de rede mostra erro com "Retry"; truncado mostra aviso; `incomplete` mostra aviso.
Cabeçalho: badges de tipo, status, HTTP e stream; canal só para admin; faixa de tentativas só com mais de uma, e clicar nela troca a tentativa selecionada.
Indicadores, com fixture com `cache_tokens` e `cache_creation_tokens` diferentes de zero: latência `use_time` em segundos, primeiro token de `other.frt`, modelo, grupo, tokens de entrada, saída e cache, custo por `formatLogQuota`, tools disponíveis e chamadas. `tools_available: null` mostra `-`, nunca `0`.
Details: fechado por padrão; abre com o corpo atual e mantém as regras de papel e de `sensitiveVisible`.
Navegação: setas do painel e `ArrowDown`/`ArrowUp` trocam de log; nas pontas, botão desabilitado e a tecla não faz nada; digitar em `input` do filtro, ou tecla com `Ctrl`, não navega; `Tab` e Escape continuam; fechar devolve o foco à linha selecionada atual via `data-log-row-id`; trocar de página fecha o painel.
Menu: "Copy request body" copia `client.request.text`; "Export call as JSON" salva `log-<request_id>.json` só com o que a rota retornou; os dois ficam desabilitados sem conteúdo e com `sensitiveVisible=false`.
Modo sensível: Conversa, Tools e JSON bruto mostram aviso, e o canal do cabeçalho vira `••••`.
Busca: só uma chamada ao abrir; nenhuma para linha de topup; nenhuma antes de abrir; rota certa por `isAdmin`.
`downloadCsv` mantém o BOM e o tipo `text/csv`.
Regra existente: clicar em linha com o painel aberto troca o log.

- [ ] **Step 2: Rodar e ver falhar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/features/usage-logs` / Expected: FAIL.

- [ ] **Step 3: Implementar**, reaproveitando `Sheet`, `Tabs`, `Collapsible`, `DropdownMenu`, `StatusBadge`, `loading-state`, `error-state`, `empty-state`, `copy-button`, `useDebounce`. As abas Tools e Roteamento só aparecem quando fazem sentido: Roteamento só com `isAdmin`.

- [ ] **Step 4: Rodar e ver passar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/features/usage-logs && bunx -p typescript@5.9 tsc -p tsconfig.app.json --noEmit && bunx oxlint -c .oxlintrc.json <arquivos alterados>` / Expected: PASS.

- [ ] **Step 5: Commit** — `feat(logs): wide content panel with header, stats and navigation`

### Task 14: Abas Conversa e Tools

**Files:**
- Create em `components/log-content/`: `conversation-tab.tsx`, `message-block.tsx`, `tools-tab.tsx`, `__tests__/conversation-tab.test.tsx`, `__tests__/tools-tab.test.tsx`

**Interfaces:**
- Consumes: `LogContentView`; fixtures `__tests__/fixtures/view-{openai,responses,claude,gemini}.json` da Task 4
- Produces: `ConversationTab(props: { view: LogContentView })`, `ToolsTab(props: { view: LogContentView })`

- [ ] **Step 1: Escrever os testes que falham**

`renderiza cada formato`, nas 4 fixtures: papéis, texto, bloco de chamada de tool com nome e argumentos, bloco de resultado, marcador de imagem/áudio com tipo e tamanho formatado, nunca o base64.
Filtro por papel: mostra só o escolhido, e limpar volta tudo; lista vazia não quebra.
Histórico: com 12 mensagens da requisição, 6 mais recentes visíveis e botão "Show N earlier messages"; com 10 ou menos, nada recolhido; a resposta nunca é recolhida.
Bloco longo: mais de 4000 caracteres mostra o início e "Show more"; "Copy" copia o texto inteiro.
Erro: bloco `error` destacado com token de tema; `is_error` em resultado de tool também.
**Texto literal:** `<script>alert(1)</script>` e `**negrito**` aparecem como texto; nenhum elemento `script`, nenhum `strong`, e o HTML não é interpretado.
Sem conversa: `conversation.available=false` mostra "Only raw JSON is available for this call"; `response_available=false` mostra as mensagens da requisição e um aviso sobre a resposta; embeddings sem o aviso de erro.
Tools: nome, descrição e esquema formatado de cada tool, "Copy" no esquema, contagem de chamadas; lista vazia mostra estado vazio; `tools_available=null` mostra o mesmo aviso de JSON bruto.
Fixture com 200 mensagens renderiza sem exceder 1 s no teste.

- [ ] **Step 2: Rodar e ver falhar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/features/usage-logs/components/log-content` / Expected: FAIL.

- [ ] **Step 3: Implementar**, com tamanhos por `@/lib/format` e `whitespace-pre-wrap break-words`.

- [ ] **Step 4: Rodar e ver passar** — Run: comando da Task 13, Step 4 / Expected: PASS.

- [ ] **Step 5: Commit** — `feat(logs): conversation and tools tabs`

### Task 15: Abas Roteamento e JSON bruto

**Files:**
- Create em `components/log-content/`: `routing-tab.tsx`, `raw-json-tab.tsx`, `__tests__/routing-tab.test.tsx`, `__tests__/raw-json-tab.test.tsx`

**Interfaces:**
- Consumes: `LogContentView`, `parseLogOther`
- Produces: `RoutingTab(props: { view: LogContentView; log: UsageLog; sensitiveVisible: boolean })`, `RawJsonTab(props: { view: LogContentView; selectedAttempt: number | null })`

- [ ] **Step 1: Escrever os testes que falham**

Roteamento: linha do tempo com canal, índice da multi-chave (só quando `>= 0`, nunca valor), status, motivo e duração em ms; grupo do `log.group`; afinidade de `other.admin_info.channel_affinity`; modelo mapeado de `other.upstream_model_name`; com `sensitiveVisible=false`, nome do canal `••••`; aba ausente quando `isAdmin=false`.
JSON bruto, usuário: só "Client request" e "Delivered response"; sem seletor de tentativa, porque `attempts` não existe na resposta.
JSON bruto, admin: também requisição enviada e resposta recebida da tentativa selecionada na faixa; `request_unavailable` mostra aviso.
Formatação: JSON válido é formatado; SSE, texto cortado e inválido aparecem como vieram; acima de 256 KiB não chama `JSON.parse`; aviso de truncado por `truncated`/`head_only`; `undecodable` mostra aviso sem texto.
"Copy" copia o texto original, não o formatado.

- [ ] **Step 2: Rodar e ver falhar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/features/usage-logs/components/log-content` / Expected: FAIL.

- [ ] **Step 3: Implementar**

- [ ] **Step 4: Rodar e ver passar** — Run: comando da Task 13, Step 4 / Expected: PASS.

- [ ] **Step 5: Commit** — `feat(logs): routing and raw JSON tabs`

### Task 16: Configurações do sistema

**Files:**
- Modify: `web/src/features/system-settings/maintenance/log-settings-section.tsx`, `system-settings/types.ts`, `operations/index.tsx`, `operations/section-registry.tsx`, `maintenance/__tests__/log-settings.test.tsx`

**Interfaces:**
- Consumes: opções `LogContentCaptureEnabled`, `LogContentRetentionDays`; `GET /api/log/content_capture/stats`

- [ ] **Step 1: Escrever os testes que falham**

O formulário mostra o interruptor e o campo de dias com os valores recebidos; salvar envia só o que mudou, por `useUpdateOption`; dias `0` e `3651` não enviam e mostram erro de validação; `1` e `3650` enviam. Mostra descartados e erros de gravação vindos de `stats`, e falha nessa consulta não impede salvar. Sem valor vindo do servidor, o padrão de `operations/index.tsx` é ligado e 30.

- [ ] **Step 2: Rodar e ver falhar** — Run: `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test src/features/system-settings/maintenance` / Expected: FAIL.

- [ ] **Step 3: Implementar**, com `z.number().int().min(1).max(3650)` e `safeNumberFieldProps`.

- [ ] **Step 4: Rodar e ver passar** — Run: mesmo comando, seguido de tipos e lint / Expected: PASS.

- [ ] **Step 5: Commit** — `feat(settings): content capture switch and retention`

### Task 17: Traduções e verificação final

**Files:**
- Modify: `web/src/i18n/locales/{en,fr,ja,pt-BR,ru,vi,zh,zh-TW}.json`, e o que os relatórios exigirem em `web/src/i18n/locales/_reports/`

**Interfaces:**
- Consumes: todas as chaves `t('…')` novas das Tasks 13 a 16.

- [ ] **Step 1: Adicionar as chaves novas diretamente nos 8 arquivos**, com pt-BR revisado. Não rodar `i18n:sync`.

Run: `bun run i18n:check-reports`
Expected: sem chaves faltando, e `git diff` só com chaves novas.

- [ ] **Step 2: Rodar todas as verificações**

Run: `(cd web && bun run build && bunx -p typescript@5.9 tsc -p tsconfig.app.json --noEmit && bunx oxlint -c .oxlintrc.json <arquivos alterados> && LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bun run test) && go test ./... && sh scripts/log-content-compare.sh`
Expected: tudo passa. Registrar qualquer falha já existente antes desta branch, sem escondê-la.

- [ ] **Step 3: Conferir o tamanho do diff no upstream**

Run: `git diff production...HEAD --stat -- . ':!web' ':!docs' ':!service/logcontent' ':!*_test.go'`
Expected: nos arquivos do upstream, só `controller/relay.go`, `relay/channel/api_request.go`, `model/log.go`, `model/main.go`, `model/option.go`, `controller/option.go`, `router/api-router.go`, `middleware/access_token_routes.go`, `main.go`, `.env.example`, com poucas linhas cada, e nenhum realinhamento.

- [ ] **Step 4: Verificar em navegador real, com a imagem construída com os patches**

Run: `docker compose -f docker-compose.yml -f docker-compose.prod-local.yml up -d --build new-api`, com PostgreSQL, `ERROR_LOG_ENABLED=true` e provedor de teste.

Fazer chamadas de cada um dos 4 formatos, com e sem stream, com tools, com imagem base64, com retentativa, com falha em todas as tentativas, e um stream cancelado. Conferir, em pt-BR, tema claro e escuro, largura de desktop e de celular:
- painel a 60% por cima da lista, sem reserva de espaço, e largura total no celular;
- faixa de tentativas, badges, indicadores, Details;
- as 4 abas; texto com HTML literal; marcador de imagem; aviso de truncado com um corpo de 3 MB e de 10 MB;
- setas e teclado, menu de copiar e exportar;
- modo sensível;
- como admin: Roteamento e JSON por tentativa, e a linha nos Audit Logs. Como root: `root_info` só para ele. Como usuário comum: sem Roteamento, sem provedor, e `request_id` de outra pessoa pela URL dá não encontrado;
- estados: chamada antiga, tipo não capturado (imagem), abertura imediata (`pending`), erro de carregamento;
- interruptor desligado: a chamada funciona e nada é gravado. Com `ERROR_LOG_ENABLED=false`: sem conteúdo para falhas, como a spec define;
- no banco, `SELECT` em `log_contents`: blobs comprimidos e chaves ausentes.

- [ ] **Step 5: Commit** — `i18n: add log content panel strings`

---

## Self-Review

- **Cobertura da spec:** captura (Tasks 3, 5, 6, 7), armazenamento e migração (1, 12), retenção (8), painel e estados (13 a 15), acesso e auditoria (9), interruptor (2, 16), comparação (10), medição (11), bancos (12), navegador (17). Criptografia, busca e exportação em massa ficam de fora, como não objetivos.
- **Consistência de tipos:** `Begin`, `Finish`, `OnLogRow`, `BeginAttempt`, `ObserveResponse`, `SanitizeBody`, `BuildContentView`, `NewStreamAssembler`, `Stats`, `Shutdown`, `getLogContent`, `useLogContent` e `LogContentView` usam o mesmo nome e a mesma forma em todas as tasks que os citam.
- **Review Focus:** cada linha tem teste: `TestNewSecretsCompositeAndShortKeys` (3); `TestLogRowReasons` e o estado `pending` (9, 13); "Texto literal" (14); `TestCaptureClientDisconnectMarksIncomplete`, `TestCaptureUpstreamCutMarksIncomplete`, `TestLateWriteAfterFinishIsIgnored` (7); `TestLogContentStorageOnDatabases`, `TestSanitizeInvalidUTF8AndNul` (1, 3).
- **Lições da etapa 1 aplicadas:** `logGroupCol` e `/` não usados; `UsingLogDatabase`; limites não enviesam contagens (`null`, não `0`); nenhum parâmetro de tempo vindo do cliente, então não há timestamp negativo a rejeitar; `downloadCsv` reaproveitado por `downloadText`; fixtures com tokens de cache e linhas logo fora do limite; ganchos fora dos arquivos alterados por patches.
- **Limitação aceita:** uma chave partida entre quadros SSE não casa com a substituição exata no SSE bruto. Só o texto remontado a pega. Exigiria interpretar o stream inteiro no bruto, por um caso improvável.
- **Proporção:** o texto traz decisões, assinaturas e asserts. Só há bloco de código para o contrato JSON e os três pontos de gancho.
