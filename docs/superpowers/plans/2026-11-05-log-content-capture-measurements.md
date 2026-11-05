# Conteúdo das chamadas nos logs — Medições

Task 11. Captura ligada contra desligada, no mesmo processo e na mesma máquina, com o gravador real (fila, workers, gzip e gravação em SQLite).

- **Máquina:** Apple M5 Pro, 18 núcleos, 48 GiB, macOS 27.2
- **Go:** go1.26.10 darwin/arm64, `GOGC` e `GOMEMLIMIT` padrão
- **Data:** 2026-11-05
- **Commit:** `74742537c` (testes); árvore sem patch = worktree; árvore com patches = `git archive HEAD` + `scripts/apply-patches.sh` via `sh scripts/log-content-compare.sh --local --keep` (`directForward` encontrado)
- **Comando:** `go test -tags logcontentmeasure -count=1 -timeout 30m ./relay/channel/openai/ -run 'LogContentMeasure' -v`
- **Teste:** `relay/channel/openai/log_content_measure_test.go`. Sem a tag, roda uma versão curta (smoke, cerca de 8 s) que só verifica a montagem, a gravação e a devolução do orçamento, sem limites. A tag `logcontentmeasure` (`log_content_measure_full_test.go`) liga os tamanhos completos e os limites.

## Montagem

Cada chamada segue `controller.Relay`: `logcontent.Begin`, corpo do cliente em `BodyStorage`, `channel.DoApiRequest` contra um provedor `httptest`, `Adaptor.DoResponse` (OpenAI chat), `OnLogRow` de consumo e `Finish`. O cliente descarta os bytes e anota o tempo da primeira escrita. Os quadros de stream têm 4 KiB (512 B de texto e o resto em `obfuscation`), então a mensagem remontada atinge o teto de 1 MiB dentro de 10 MiB de stream. O stream termina com `usage` e `[DONE]`.

- **Memória:** `HeapInuse` amostrado por `runtime.ReadMemStats` a cada 5 ms; crescimento = pico − base (base depois de dois `GC`). Desligada e ligada alternadas, 3 execuções cada; compara-se a mediana. Pico do orçamento por `logcontent.Stats().BudgetInUse`.
- **Latência:** 20 rodadas de 50 streams simultâneos, alternando desligada e ligada (1000 amostras por modo).
- **Duração:** 1 stream de 100 MiB sem espera no provedor, 5 execuções alternadas; compara-se a mediana.
- **`Finish`:** chamadas não stream sequenciais com requisição e resposta de ~1 MiB e ~8 MiB; cada chamada começa com o orçamento livre.

## Resultados

| Medida | Limite | Sem patch | Com patches | Resultado |
|-|-|-|-|-|
| Primeiro byte, p50 ligada − desligada (50 streams) | ≤ 1 ms | 0,227 ms (2,790 → 3,016 ms) | 0,229 ms (2,602 → 2,831 ms) | Passa |
| Primeiro byte, p99 ligada − desligada | ≤ 5 ms | −0,930 ms (8,707 → 7,777 ms) | −1,619 ms (9,451 → 7,832 ms) | Passa |
| Duração de stream de 100 MiB | ≤ +5% | +4,52% (564 → 589 ms) | −0,48% (679 → 676 ms) | Passa (sem patch, perto do limite) |
| `Finish`, ~1 MiB, p99 | ≤ 2 ms | 0,455 ms (p50 0,102 ms, máx 0,552 ms) | 0,442 ms (p50 0,095 ms, máx 0,519 ms) | Passa |
| `Finish`, ~8 MiB, p99 | ≤ 10 ms | 1,627 ms (p50 0,864 ms, máx 1,784 ms) | 1,218 ms (p50 0,806 ms, máx 1,384 ms) | Passa |
| Pico de `HeapInuse`, ligada − desligada, 50 streams de 100 MiB | ≤ 207 MiB | 231,6 MiB | 270,3 MiB | **Falha** |
| Custo de pico, 10 MiB contra 100 MiB | diferença ≤ 10% | 17,4% (191,4 contra 231,6 MiB) | 38,8% (165,4 contra 270,3 MiB) | **Falha** |
| Pico, ligada − desligada, 20 respostas não stream de 50 MiB | ≤ 288 MiB | −232,8 MiB | −238,5 MiB | Passa (ver nota) |
| 5 s depois, com `GC`: `BudgetInUse` | = 0 | 0 em todas | 0 em todas | Passa |
| 5 s depois, com `GC`: `HeapInuse` | ≤ base + 16 MiB | máx +2,3 MiB | máx +2,3 MiB | Passa |

Desligada, `Finish` é um receptor `nil`: p50 125–208 ns, p99 ≤ 458 ns.

### Execuções de memória

Crescimento de `HeapInuse` (pico − base), em MiB, por execução:

| Caso | Árvore | Desligada | Ligada | Pico do orçamento |
|-|-|-|-|-|
| 50 × 100 MiB | sem patch | 1193,3 / 1208,9 / 1172,1 | 1378,4 / 1506,0 / 1424,9 | 175,2 |
| 50 × 100 MiB | com patches | 1164,1 / 1163,6 / 1176,1 | 1434,4 / 1410,7 / 1434,4 | 175,2 |
| 50 × 10 MiB | sem patch | 184,6 / 173,9 / 163,0 | 366,3 / 357,4 / 365,3 | 175,2 |
| 50 × 10 MiB | com patches | 179,5 / 179,2 / 156,8 | 344,6 / 339,3 / 398,5 | 175,2 |
| 20 × 50 MiB não stream | sem patch | 2414,6 / 2464,1 / 2551,8 | 2411,3 / 2192,8 / 2231,3 | 25,6 / 23,5 / 22,4 |
| 20 × 50 MiB não stream | com patches | 2376,4 / 2626,7 / 2439,3 | 2148,0 / 2523,0 / 2200,7 | 22,4 / 22,4 / 23,5 |

Duração das execuções de 100 MiB: 2,6–3,0 s (50 streams); 10 MiB: 0,36–0,44 s; não stream: 0,58–0,76 s.

### Gravador

Todas as chamadas capturadas foram gravadas (2 linhas por chamada) sem erro de gravação, exceto na rajada de latência: 20 rodadas de 50 streams seguidas, sem pausa, descartaram 84 de 1000 chamadas (sem patch) e 124 de 1000 (com patches) por fila cheia. A chamada nunca esperou nem falhou; só o conteúdo foi perdido.

## Defeitos encontrados

1. **Pico de memória, 50 streams de 100 MiB, acima de 207 MiB** (231,6 e 270,3 MiB). O orçamento em uso chega a 175,2 MiB, exatamente os 50 × 3,5 MiB calculados, então o que a captura retém está no projeto. A diferença provavelmente vem do `GC`: com `GOGC=100`, 175 MiB a mais de dados vivos sobem a meta do heap na mesma proporção, e a remontagem gera lixo por quadro entre as coletas (hipótese, não perfilada). O pico de `HeapInuse` pode crescer até perto de 2 × 175 MiB, e 32 MiB de folga não cobrem isso. Limite mantido; correção pendente de decisão.
2. **Custo de pico depende da duração** (17,4% e 38,8%, limite 10%). Com 10 MiB tudo termina em ~0,4 s, com poucas coletas; com 100 MiB há ~2,9 s de remontagem gerando lixo. Além disso, com a linha de base desligada em ~1,2 GiB (o relay acumula o texto da resposta para contar tokens), o ruído entre execuções da mesma configuração chega a 128 MiB (ligada, sem patch), da ordem do próprio limite.
3. **Descarte sob rajada:** 8,4% e 12,4% das chamadas descartadas por fila cheia, com rodadas de 50 streams de ~70 KiB seguidas sem pausa (2000 chamadas, metade capturada; a taxa não foi medida) e gravação em SQLite. Gargalo (limpeza ou gravação) não identificado. Não há limite para isso na Task 11; fica registrado.

Nota sobre o caso não stream: o pico desligado já passa de 2,4 GiB (cada chamada lê os 50 MiB e os decodifica), e a variação entre execuções (até 250 MiB) é maior que o custo da captura. O valor negativo mostra que a medida não resolve esse custo; o custo direto é o pico do orçamento, 22–26 MiB, bem abaixo de 288 MiB.

## Correções possíveis (não aplicadas)

- Medir o que a captura retém, não o pico do heap: pico de `BudgetInUse` ≤ 175 MiB, ou `HeapAlloc` depois de `GC` forçado no meio dos streams, ligada − desligada, com 32 MiB de folga. Para o pico de `HeapInuse`, um limite coerente com `GOGC=100` seria 2 × 175 + 32 = 382 MiB.
- Reduzir o lixo por quadro da remontagem (reuso de buffers no `streamFold`), o que baixa o pico sem mudar o limite.
- Para a comparação 10 contra 100 MiB, comparar o pico do orçamento ou a memória retida, que é constante (175,2 MiB nos dois), em vez do pico do heap.
- Descarte: medir a vazão dos workers e do gravador separadamente; se o gargalo for a limpeza, mais workers; se for o banco, lotes maiores.
