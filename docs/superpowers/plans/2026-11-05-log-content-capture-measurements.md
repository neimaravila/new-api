# Conteúdo das chamadas nos logs — Medições

Task 11. Captura ligada contra desligada, no mesmo processo e na mesma máquina, com o gravador real (fila, workers, gzip e gravação em SQLite).

- **Máquina:** Apple M5 Pro, 18 núcleos, 48 GiB, macOS 27.2
- **Go:** go1.26.10 darwin/arm64, `GOGC` e `GOMEMLIMIT` padrão
- **Data:** 2026-11-05
- **Commit:** `c00eed921` (tabela principal, as duas árvores); `e6eab3db0` (rodada 3, só a worktree). Árvore sem patch = worktree. Árvore com patches = `git archive HEAD` + `scripts/apply-patches.sh`, via `sh scripts/log-content-compare.sh --local --keep` (`directForward` encontrado, testes do script passando).
- **Comando:** `go test -tags logcontentmeasure -count=1 -timeout 30m ./relay/channel/openai/ -run 'LogContentMeasure' -v`
- **Teste:** `relay/channel/openai/log_content_measure_test.go`. Sem a tag roda uma versão curta (cerca de 9 s) que verifica a montagem, a gravação e a devolução do orçamento, sem limites. A tag `logcontentmeasure` (`log_content_measure_full_test.go`) liga os tamanhos completos e os limites.

## Montagem

Cada chamada segue `controller.Relay`: `logcontent.Begin`, corpo do cliente em `BodyStorage`, `channel.DoApiRequest` contra um provedor `httptest`, `Adaptor.DoResponse` (OpenAI chat), `OnLogRow` de consumo e `Finish`. O cliente descarta os bytes e anota o tempo da primeira escrita. Os quadros de stream têm 4 KiB (1 KiB de texto e o resto em `obfuscation`), então a mensagem remontada atinge o teto de 1 MiB nos primeiros 5 MiB, muito antes do ponto de espera (50 e 150 MiB). O stream termina com `usage` e `[DONE]`.

- **Memória retida:** o provedor segura todos os streams na metade do tamanho. Quando os 50 chegaram e o cliente parou de receber por 100 ms, dois `GC` forçados e `HeapAlloc`, menos o `HeapAlloc` da base. Nada aloca nesse momento, então o valor é o que as chamadas mantêm vivo. Desligada e ligada alternadas, 3 execuções cada, mediana.
- **Orçamento:** pico de `logcontent.Stats().BudgetInUse`, amostrado a cada 5 ms.
- **Pico de `HeapInuse`:** só registrado, sem limite (ver "Critérios").
- **Latência:** 20 rodadas de 50 streams simultâneos, alternando desligada e ligada (1000 amostras por modo).
- **Duração:** 1 stream de 100 MiB sem espera no provedor, 5 execuções alternadas, mediana.
- **`Finish`:** chamadas não stream sequenciais com requisição e resposta de ~1 MiB e ~8 MiB; cada chamada começa com o orçamento livre.

## Critérios

Mudados nas rodadas de correção 1 e 2, por decisão do controlador, mantendo a exigência da spec de que a memória da captura não cresce com a duração:

- Removido o pico de `HeapInuse`, ligada − desligada (limites 207 e 288 MiB). Com `GOGC=100`, dados vivos a mais sobem a meta do heap na mesma proporção, e o pico varia entre execuções da mesma configuração mais que o limite: até 128 MiB nos streams e até 800 MiB no caso não stream, onde a linha de base passa de 2,4 GiB.
- Orçamento por stream = 3,5 MiB + reserva das cópias da requisição. Requisição com menos de 4 KiB ocupa a primeira capacidade do tap, 4 KiB (`tapBuffer.grow`), e há duas cópias: a enviada ao provedor e a do cliente em `Finish`. Limite = 50 × (3,5 MiB + 2 × 4 KiB) = 183910400 B (175,39 MiB), calculado no teste (`lcmStreamBudget`), não medido.
- Memória retida, ligada − desligada, ≤ 207 MiB em cada comprimento.
- Independência do comprimento: streams de 100 e 300 MiB, os dois com todos os tetos cheios antes da metade; memória retida, ligada − desligada, com diferença ≤ 10%. A comparação com 10 MiB saiu: um stream curto guarda menos porque os tetos não enchem até a metade (texto remontado de ~400 KiB por stream contra ~1,06 MiB), o que não é crescimento com a duração.
- Caso não stream (rodada 3): limite de projeto calculado no teste (`lcmLargeCallBudget`), como o dos streams. Por chamada com resposta de 50 MiB: a cópia do provedor sabe o tamanho por `Content-Length` e guarda só o começo desde o início, `maxStoredBody + tapMarginBytes` = 1 MiB + 64 KiB; a cópia do cliente não sabe e pode crescer até `maxParsedBody` = 8 MiB antes de passar ao começo; mais as cópias da requisição, 2 × 4 KiB. Limite = 20 × (1 MiB + 64 KiB + 8 MiB + 8 KiB) = 190218240 B (181,4 MiB). O orçamento é lido com as 20 chamadas seguradas depois de entregar a resposta e antes de `Finish`, quando todas as cópias estão vivas ao mesmo tempo. O limite anterior, 256 MiB, nunca falharia: `tryReserve` não deixa o orçamento passar disso.

## Resultados

| Medida | Limite | Sem patch | Com patches | Resultado |
|-|-|-|-|-|
| Primeiro byte, p50 ligada − desligada (50 streams) | ≤ 1 ms | 0,403 ms (2,984 → 3,387 ms) | 0,723 ms (3,016 → 3,739 ms) | Passa |
| Primeiro byte, p99 ligada − desligada | ≤ 5 ms | −0,201 ms (9,129 → 8,928 ms) | −0,048 ms (9,448 → 9,400 ms) | Passa |
| Duração de stream de 100 MiB | ≤ +5% | −2,26% (634 → 620 ms) | −0,58% (662 → 658 ms) | Passa |
| `Finish`, ~1 MiB, p99 | ≤ 2 ms | 0,595 ms (p50 0,108, máx 0,681) | 0,547 ms (p50 0,097, máx 0,856) | Passa |
| `Finish`, ~8 MiB, p99 | ≤ 10 ms | 1,391 ms (p50 0,693, máx 1,599) | 1,435 ms (p50 0,644, máx 1,539) | Passa |
| Pico de `BudgetInUse`, 50 × 100 MiB | ≤ 183910400 B | 183713792 B | 183709696 B | Passa |
| Pico de `BudgetInUse`, 50 × 300 MiB | ≤ 183910400 B | 183713792 B | 183717888 B | Passa |
| Memória retida, ligada − desligada, 50 × 100 MiB | ≤ 207 MiB | 152,8 MiB | 150,1 MiB | Passa |
| Memória retida, ligada − desligada, 50 × 300 MiB | ≤ 207 MiB | 154,9 MiB | 154,9 MiB | Passa |
| Memória retida, 100 contra 300 MiB | diferença ≤ 10% | 1,4% | 3,1% | Passa |
| Pico de `BudgetInUse`, 20 respostas não stream de 50 MiB (rodada 2, sem segurar) | ≤ 256 MiB (substituído) | 24,53 MiB | 22,39 MiB | — |
| 5 s depois, com `GC`: `BudgetInUse` | = 0 | 0 em todas | 0 em todas | Passa |
| 5 s depois, com `GC`: `HeapInuse` | ≤ base + 16 MiB | máx +6,2 MiB | máx +3,5 MiB | Passa |

Desligada, `Finish` é um receptor `nil`: p50 83–167 ns, p99 ≤ 500 ns. A execução completa leva cerca de 224 s em cada árvore.

### Execuções de memória

Memória retida na metade dos streams (`HeapAlloc` − base), em MiB, e pico do orçamento, por execução:

| Caso | Árvore | Desligada | Ligada | Pico do orçamento (B) |
|-|-|-|-|-|
| 50 × 100 MiB | sem patch | 635,0 / 634,8 / 634,8 | 785,4 / 789,6 / 787,6 | 183709696 / 183713792 / 183709696 |
| 50 × 100 MiB | com patches | 634,9 / 634,8 / 634,8 | 785,3 / 785,0 / 784,0 | 183709696 / 183709696 / 183709696 |
| 50 × 300 MiB | sem patch | 1929,8 / 1929,8 / 1929,7 | 2084,6 / 2084,7 / 2084,7 | 183709696 / 183709696 / 183713792 |
| 50 × 300 MiB | com patches | 1929,8 / 1929,7 / 1929,8 | 2084,7 / 2084,7 / 2084,7 | 183717888 / 183709696 / 183709696 |
| 20 × 50 MiB não stream | sem patch | — | — | 24600576 / 24600576 / 25718784 |
| 20 × 50 MiB não stream | com patches | — | — | 22364160 / 23482368 / 22364160 |

Pico de `HeapInuse` (pico − base, só registro), MiB: 50 × 100 MiB desligada 2214–2410, ligada 2435–2691; 50 × 300 MiB desligada 6521–7098, ligada 6851–7438; não stream desligada 2327–3127, ligada 2106–2348.

### Gravador

Todas as chamadas capturadas foram gravadas (2 linhas por chamada), sem descarte e sem erro de gravação, nas duas árvores, inclusive na rajada de 1000 chamadas da medição de latência.

## Rodada de correção 1

### Alocação da captura

Perfil `alloc_space` (`-memprofile`) de `TestLogContentMeasureLongStreamMemory` com o mesmo teste, antes e depois, só a parte de `service/logcontent`, dividida pelo volume transmitido com a captura ligada (3 × 50 × 100 MiB + 3 × 50 × 10 MiB = 16500 MiB):

| | Captura | Por byte transmitido |
|-|-|-|
| Antes | 22,47 GiB | 1,39 B/B |
| Depois | 5,14 GiB | 0,31 B/B |

Antes, 16,4 GiB vinham de `bytes.Clone` em `streamFold.feed`, uma cópia de cada bloco de 32 KiB mandado à remontagem, e 1,85 GiB de `StreamAssembler.line`, um buffer novo por quadro. Correções (`49fff90eb`): blocos vindos de um `sync.Pool`, devolvidos depois de escritos, e o buffer de quadro reaproveitado quando tem até 64 KiB. O montador já copiava tudo o que guarda (`gjson.ParseBytes` copia), então o comportamento não muda. O que resta: `gjson.ParseBytes` (1,4 GiB), o texto montado, `gzip` e a limpeza no worker. Para comparação, o relay sozinho aloca cerca de 6 B/B no mesmo teste.

### Descarte na rajada

Instrumentação temporária (não commitada) da fila: as 1000 chamadas capturadas da rajada chegam em cerca de 250 ms. Os 2 workers (limpeza e `gzip`) gastavam ~0,7 ms por registro, em 0,6 s de trabalho no total; o gravador SQLite gastou ~0,1 s em ~90 lotes e os workers esperaram 48 ms para entregar linhas. Gargalo: os workers. A fila chegou a 512 e descartou 113–137 chamadas por execução. Com 3 workers (`d481e4422`): 0 descartes em duas execuções e nas duas árvores. O desenho de nunca bloquear não mudou.

## Rodada de correção 3

Execução completa só na worktree, em `e6eab3db0`, 228 s, tudo passando:

| Medida | Limite | Valor |
|-|-|-|
| Primeiro byte, p50 / p99 ligada − desligada | ≤ 1 ms / ≤ 5 ms | 0,541 ms (3,016 → 3,558) / −1,028 ms (9,681 → 8,653) |
| Duração de stream de 100 MiB | ≤ +5% | +2,93% (565 → 582 ms) |
| `Finish` p99, ~1 MiB / ~8 MiB | ≤ 2 ms / ≤ 10 ms | 0,575 ms / 1,513 ms |
| Pico de `BudgetInUse`, 50 × 100 / 300 MiB | ≤ 183910400 B | 183713792 / 183717888 B |
| Memória retida, ligada − desligada, 100 / 300 MiB | ≤ 207 MiB | 152,3 / 155,0 MiB |
| Memória retida, 100 contra 300 MiB | ≤ 10% | 1,7% |
| `BudgetInUse` com 20 chamadas não stream de 50 MiB seguradas | ≤ 190218240 B | 44646400 B (42,58 MiB) nas 3 execuções |
| 5 s depois, com `GC` | orçamento 0, `HeapInuse` ≤ base + 16 MiB | 0, máx +7,2 MiB |
| Gravador | — | 0 descartes, 0 erros |

**O limite não stream pega a quebra da troca para o começo.** Mudança temporária, revertida com `git checkout`: `startHeadOnly` e `dropToHead` sem efeito, então as duas cópias guardam 8 MiB. Resultado: 260128768 B (248,08 MiB) nas 3 execuções, acima de 190218240 B, e o teste falha. Sem a mudança, 44646400 B (2,13 MiB por chamada: 1 MiB + 64 KiB de cada cópia de resposta, mais a requisição).

Outras mudanças da rodada 3:

- `lcmWaitWritten` lê enfileiradas, descartadas e gravadas na mesma leitura a cada volta, e espera até que todas as chamadas estejam enfileiradas ou descartadas e as enfileiradas gravadas. Antes, o número de descartes vinha de uma leitura feita uma vez, antes de a remontagem enfileirar os registros dela.
- Testes de unidade em `service/logcontent` (`9b9f60286`): `TestAssembleReusedFrameBufferKeepsFramesApart` (quadro acima de 64 KiB seguido de quadros menores, cada um mais curto que o anterior, em blocos de 1 MiB, 4096 e 7 bytes) e `TestStreamFoldReusesBlocksWithoutMixingThem` (fila cheia, bloco descartado volta ao pool, blocos seguintes reaproveitam buffers enquanto a remontagem lê, quadro atravessando dois blocos). Passam com `-race -count=3`. Mutações temporárias, revertidas: devolver o bloco ao pool antes de escrevê-lo faz o teste do fold falhar com 4 avisos de corrida; reaproveitar o buffer de quadro sem zerar faz o teste do montador falhar.

### Rodada 1, valores que motivaram a rodada 2

Com o limite de 175 MiB, o pico do orçamento passava 0,12% (183717888 B): a reserva das cópias da requisição não estava no projeto. Com 10 contra 100 MiB, a memória retida diferia 22% (118,0 contra 151,9 MiB sem patch; 117,6 contra 150,5 MiB com patches): perfil de heap no ponto de espera mostrou buffers dos taps iguais (~100 MiB) e texto remontado de 20 MiB contra 54 MiB, porque o stream de 10 MiB não enche o teto da remontagem até a metade.

## Defeitos em aberto

Nenhum. Todas as medidas passam nas duas árvores.

## Task 12: PostgreSQL, MySQL e ClickHouse reais

- **Data:** 2026-11-05, base `9a95468c2` mais as mudanças da Task 12. Docker Desktop em macOS (Apple M5 Pro), contêineres temporários `newapi-lc-pg` (postgres:16), `newapi-lc-my` (mysql:8.0, 8.0.46) e `newapi-lc-ch` (clickhouse-server:24.8), só em 127.0.0.1, portas livres, removidos no fim. PostgreSQL com a configuração padrão da imagem.
- **Armazenamento, rotas e retenção:** `go test -count=1 -v ./model/ ./controller/ ./service/ -run 'LogContent|SelfRoute|AdminRoute|AdminRead|LogRowReasons|Capture'` com os três DSNs: PASS, nenhum SKIP. `TestLogContentStorageOnDatabases` passou em sqlite, mysql, postgres e clickhouse (corpo binário de 2 MiB com NUL, `GetLogContents` por usuário e usuário 0). A DDL do ClickHouse foi executada no servidor por `MigrateLogContents` (duas vezes, idempotente); `TestLogContentCreateTableClickHouseSQL` em si só compara o texto da DDL.
- **Defeito de fixture achado no ClickHouse (corrigido no teste):** a suíte gravava `CreatedAt: 1000` (1970). Com o TTL de 30 dias da tabela, o ClickHouse apaga a linha na fusão logo após a inserção (`system.part_log`: `NewPart` seguido de `MergeParts` com 0 linhas no mesmo segundo), e a leitura com usuário 0 voltava vazia, de forma determinística (3 de 3). A fixture agora usa `time.Now().Unix()`. Código de produção e limites não mudaram.
- **Carga no PostgreSQL compartilhado** (`service/logcontent/load_test.go`, tag `logcontentload`): um só banco para `users`, `logs` e `log_contents` (1000 usuários, 100 mil linhas em `logs`, 200 mil linhas expiradas e 100 linhas-alvo em `created_at = corte + 0..99` em `log_contents`). Sondas a cada 5 ms (INSERT em `logs`, SELECT em `users` por id), 60 s sem carga e 60 s com 200 chamadas/s (2 linhas cada, requisição 4 KiB e resposta 16 KiB) na fila real e a retenção (`DeleteOldLogContentBatch`, lote de 200 como em produção, sem a pausa de 1 s: pior caso) apagando as 200 mil linhas. Comando: `AUDIT_POSTGRES_DSN=... go test -tags logcontentload -count=1 -timeout 20m ./service/logcontent/ -run 'LogContentPostgres' -v`.

| Medida | Limite | Base | Com carga | Resultado |
|-|-|-|-|-|
| INSERT em `logs`, p99 (60 s) | 2 x base + 5 ms | 1,82 ms | 5,50 ms (limite 8,65 ms) | PASS |
| SELECT em `users`, p99 (60 s) | 2 x base + 5 ms | 0,70 ms | 4,69 ms (limite 6,41 ms) | PASS |
| INSERT em `logs`, p99 só durante a retenção (2,5 s, n=499) | 2 x base + 5 ms | 1,82 ms | 5,59 ms | PASS |
| SELECT em `users`, p99 só durante a retenção (n=499) | 2 x base + 5 ms | 0,70 ms | 4,67 ms | PASS |
| p50 INSERT / SELECT | sem limite | 1,17 / 0,39 ms | 1,26 / 0,42 ms | |
| máximo INSERT / SELECT | sem limite | 46,4 / 8,4 ms | 47,5 / 6,7 ms | |
| `wait_event_type = 'Lock'` (amostra a cada 100 ms) | nenhum em `logs`/`users` | | 0 esperas | PASS |
| Retenção | 200 mil apagadas em 2,5 s; 100 linhas-alvo intactas; 0 expiradas restantes | | | PASS |
| Gravador (200/s, 60 s) | | | enqueued 12000, dropped 0, written 24000, write errors 0 | PASS |

O p99 sob carga sobe de 1,8 ms para 5,5 ms (INSERT) e de 0,7 ms para 4,7 ms (SELECT) e fica a 3 ms e 1,7 ms do limite. As amostras da retenção são 4% do total, o bastante para formar o p99 dos 60 s, e o p99 só da retenção (5,59 e 4,67 ms) é igual ao dos 60 s; não medi a parte da janela sem retenção separadamente. A retenção sem pausa levou 2,5 s (80 mil linhas/s), 400 vezes mais rápida que a produção (200 linhas por segundo, cerca de 17 min para 200 mil), então a janela medida da retenção é pior que a real.

- **Rajada da Task 11 no PostgreSQL** (1000 chamadas sem pausa, 2 linhas cada, requisição 4 KiB e resposta 64 KiB; `TestLogContentPostgresBurst`): chegando em 250 ms, 838 enfileiradas e 162 descartadas (16,2%; 798/202 numa execução anterior), 1676 linhas gravadas, 0 erros de gravação, esvaziou em 1,7 s. Todas de uma vez (92 us): 515 enfileiradas, 485 descartadas (48,5%), 1030 linhas, 0 erros, 1,5 s. A fila tem 512 vagas; a taxa de chegada acima da vazão dos trabalhadores descarta, por projeto, sem bloquear a chamada. As respostas de 64 KiB são mais pesadas que a medição da Task 11 em SQLite, então os percentuais não são comparáveis. O gargalo (gzip nos trabalhadores ou INSERT) não foi isolado.
