# Conteúdo das chamadas nos logs — Medições

Task 11. Captura ligada contra desligada, no mesmo processo e na mesma máquina, com o gravador real (fila, workers, gzip e gravação em SQLite).

- **Máquina:** Apple M5 Pro, 18 núcleos, 48 GiB, macOS 27.2
- **Go:** go1.26.10 darwin/arm64, `GOGC` e `GOMEMLIMIT` padrão
- **Data:** 2026-11-05
- **Commit:** `c00eed921`. Árvore sem patch = worktree. Árvore com patches = `git archive HEAD` + `scripts/apply-patches.sh`, via `sh scripts/log-content-compare.sh --local --keep` (`directForward` encontrado, testes do script passando).
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
- Caso não stream: pico de `BudgetInUse` ≤ 256 MiB.

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
| Pico de `BudgetInUse`, 20 respostas não stream de 50 MiB | ≤ 256 MiB | 24,53 MiB | 22,39 MiB | Passa |
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

### Rodada 1, valores que motivaram a rodada 2

Com o limite de 175 MiB, o pico do orçamento passava 0,12% (183717888 B): a reserva das cópias da requisição não estava no projeto. Com 10 contra 100 MiB, a memória retida diferia 22% (118,0 contra 151,9 MiB sem patch; 117,6 contra 150,5 MiB com patches): perfil de heap no ponto de espera mostrou buffers dos taps iguais (~100 MiB) e texto remontado de 20 MiB contra 54 MiB, porque o stream de 10 MiB não enche o teto da remontagem até a metade.

## Defeitos em aberto

Nenhum. Todas as medidas passam nas duas árvores.
