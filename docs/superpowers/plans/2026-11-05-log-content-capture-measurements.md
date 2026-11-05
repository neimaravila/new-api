# Conteúdo das chamadas nos logs — Medições

Task 11. Captura ligada contra desligada, no mesmo processo e na mesma máquina, com o gravador real (fila, workers, gzip e gravação em SQLite).

- **Máquina:** Apple M5 Pro, 18 núcleos, 48 GiB, macOS 27.2
- **Go:** go1.26.10 darwin/arm64, `GOGC` e `GOMEMLIMIT` padrão
- **Data:** 2026-11-05
- **Commit:** `dd73abaa3`. Árvore sem patch = worktree. Árvore com patches = `git archive HEAD` + `scripts/apply-patches.sh`, via `sh scripts/log-content-compare.sh --local --keep` (`directForward` encontrado, testes do script passando).
- **Comando:** `go test -tags logcontentmeasure -count=1 -timeout 30m ./relay/channel/openai/ -run 'LogContentMeasure' -v`
- **Teste:** `relay/channel/openai/log_content_measure_test.go`. Sem a tag roda uma versão curta (cerca de 9 s) que verifica a montagem, a gravação e a devolução do orçamento, sem limites. A tag `logcontentmeasure` (`log_content_measure_full_test.go`) liga os tamanhos completos e os limites.

## Montagem

Cada chamada segue `controller.Relay`: `logcontent.Begin`, corpo do cliente em `BodyStorage`, `channel.DoApiRequest` contra um provedor `httptest`, `Adaptor.DoResponse` (OpenAI chat), `OnLogRow` de consumo e `Finish`. O cliente descarta os bytes e anota o tempo da primeira escrita. Os quadros de stream têm 4 KiB (1 KiB de texto e o resto em `obfuscation`), então a mensagem remontada atinge o teto de 1 MiB nos primeiros 5 MiB. O stream termina com `usage` e `[DONE]`.

- **Memória retida:** o provedor segura todos os streams na metade do tamanho. Quando os 50 chegaram e o cliente parou de receber por 100 ms, dois `GC` forçados e `HeapAlloc`, menos o `HeapAlloc` da base. Nada aloca nesse momento, então o valor é o que as chamadas mantêm vivo. Desligada e ligada alternadas, 3 execuções cada, mediana.
- **Orçamento:** pico de `logcontent.Stats().BudgetInUse`, amostrado a cada 5 ms.
- **Pico de `HeapInuse`:** só registrado, sem limite (ver "Critérios").
- **Latência:** 20 rodadas de 50 streams simultâneos, alternando desligada e ligada (1000 amostras por modo).
- **Duração:** 1 stream de 100 MiB sem espera no provedor, 5 execuções alternadas, mediana.
- **`Finish`:** chamadas não stream sequenciais com requisição e resposta de ~1 MiB e ~8 MiB; cada chamada começa com o orçamento livre.

## Critérios

Mudados na rodada de correção 1, por decisão do controlador, mantendo a exigência da spec de que a memória da captura não cresce com a duração:

- Removido o pico de `HeapInuse`, ligada − desligada (limites 207 e 288 MiB). Com `GOGC=100`, dados vivos a mais sobem a meta do heap na mesma proporção, e o pico varia entre execuções da mesma configuração mais que o limite: até 128 MiB nos streams e até 800 MiB no caso não stream, onde a linha de base passa de 2,4 GiB.
- No lugar: pico de `BudgetInUse` ≤ 175 MiB (50 × 3,5 MiB); memória retida, ligada − desligada, ≤ 175 + 32 MiB; 10 MiB contra 100 MiB comparados pela memória retida, diferença ≤ 10%; caso não stream com pico de `BudgetInUse` ≤ 256 MiB.

## Resultados

| Medida | Limite | Sem patch | Com patches | Resultado |
|-|-|-|-|-|
| Primeiro byte, p50 ligada − desligada (50 streams) | ≤ 1 ms | 0,519 ms (2,869 → 3,387 ms) | 0,337 ms (3,004 → 3,341 ms) | Passa |
| Primeiro byte, p99 ligada − desligada | ≤ 5 ms | 0,858 ms (8,131 → 8,988 ms) | −0,510 ms (9,207 → 8,697 ms) | Passa |
| Duração de stream de 100 MiB | ≤ +5% | +2,85% (558 → 574 ms) | +2,81% (576 → 592 ms) | Passa |
| `Finish`, ~1 MiB, p99 | ≤ 2 ms | 0,529 ms (p50 0,083, máx 0,773) | 0,445 ms (p50 0,100, máx 0,487) | Passa |
| `Finish`, ~8 MiB, p99 | ≤ 10 ms | 1,427 ms (p50 0,680, máx 1,435) | 1,594 ms (p50 0,829, máx 1,625) | Passa |
| Pico de `BudgetInUse`, 50 streams de 100 MiB | ≤ 175 MiB (183500800 B) | 183717888 B (175,21 MiB) | 183713792 B (175,20 MiB) | **Falha por 0,12%** |
| Memória retida, ligada − desligada, 50 × 100 MiB | ≤ 207 MiB | 151,9 MiB | 150,5 MiB | Passa |
| Memória retida, 10 MiB contra 100 MiB | diferença ≤ 10% | 22,3% (118,0 contra 151,9 MiB) | 21,9% (117,6 contra 150,5 MiB) | **Falha** |
| Pico de `BudgetInUse`, 20 respostas não stream de 50 MiB | ≤ 256 MiB | 22,39 MiB | 22,39 MiB | Passa |
| 5 s depois, com `GC`: `BudgetInUse` | = 0 | 0 em todas | 0 em todas | Passa |
| 5 s depois, com `GC`: `HeapInuse` | ≤ base + 16 MiB | máx +2,0 MiB | máx +4,3 MiB | Passa |

Desligada, `Finish` é um receptor `nil`: p50 125–209 ns, p99 ≤ 417 ns.

### Execuções de memória

Memória retida na metade dos streams (`HeapAlloc` − base), em MiB, e pico do orçamento, por execução:

| Caso | Árvore | Desligada | Ligada | Pico do orçamento (B) |
|-|-|-|-|-|
| 50 × 100 MiB | sem patch | 634,9 / 634,8 / 634,8 | 786,7 / 787,6 / 786,2 | 183717888 / 183709696 / 183709696 |
| 50 × 100 MiB | com patches | 634,9 / 634,8 / 634,8 | 788,1 / 785,3 / 785,3 | 183713792 / 183709696 / 183709696 |
| 50 × 10 MiB | sem patch | 72,0 / 71,9 / 71,9 | 190,5 / 189,6 / 190,0 | 183726080 / 183713792 / 183721984 |
| 50 × 10 MiB | com patches | 72,0 / 71,9 / 71,9 | 190,7 / 188,7 / 189,5 | 183709696 / 183721984 / 183721984 |
| 20 × 50 MiB não stream | sem patch | — | — | 23482368 / 23482368 / 22364160 |
| 20 × 50 MiB não stream | com patches | — | — | 22364160 / 22364160 / 23482368 |

Pico de `HeapInuse` (pico − base, só registro), MiB: 50 × 100 MiB desligada 2223–2371, ligada 2379–2588; 50 × 10 MiB desligada 247–304, ligada 506–526; não stream desligada 2327–3127, ligada 2103–2611.

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

## Defeitos em aberto

1. **Pico de `BudgetInUse` 0,12% acima de 175 MiB.** O excesso é de 4,1–4,4 KiB por stream: a cópia da requisição ao provedor reserva no mínimo 4 KiB (`tapBuffer.grow`), e os 3,5 MiB do projeto contam só o lado da resposta. Correção possível: limite de 50 × (3,5 MiB + reserva da requisição), ou o projeto passar a contar a requisição. Limite mantido.
2. **Memória retida, 10 contra 100 MiB, diferença de 22%.** Não é crescimento com a duração: as duas ficam abaixo do orçamento e o pico do orçamento é o mesmo (175,2 MiB). É o stream mais curto que guarda menos. Perfil de heap no ponto de espera, captura ligada: buffers dos taps iguais nos dois casos (~100 MiB), mas texto montado de 20 MiB com 10 MiB (cerca de 400 KiB por stream) contra 54 MiB com 100 MiB (cerca de 1,06 MiB por stream). Explicação provável (não confirmada): com o relay passando ~1,7 GiB/s, a remontagem fica para trás e perde blocos (canal sem bloqueio), e nos primeiros 5 MiB não chega ao teto de 1 MiB. Correção possível: comparar com um critério só de um lado (o mais longo não passa do orçamento), ou medir no ponto em que os tetos já foram atingidos. Limite mantido.
