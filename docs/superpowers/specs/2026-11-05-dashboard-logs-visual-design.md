# Dashboard e logs de uso com painel inspirado no Bifrost — Design

Data: 2026-11-05
Estado: rascunho para aprovação

## Intenção

O usuário testou o Bifrost e gostou muito das telas de dashboard e de LLM log, mas quer continuar usando o new-api e não quer perder a identidade dele. Esta é a etapa 1: trazer essa experiência usando apenas os dados que o new-api já registra. O uso é interno, e o valor buscado é ver melhor o consumo, o custo, os erros e o desempenho.

## Decisões do usuário

- Fazer a etapa 1 primeiro, sem captura do conteúdo das chamadas.
- Manter a identidade do new-api, citada como "fontes, disposição, etc".
- Dashboard na direção B: menu e cards do new-api, com abas, grade de gráficos com troca de tipo e rankings dentro da página.
- Logs na direção B: resumo, gráfico de volume com erros destacados, coluna de status e detalhe em painel lateral sem sair da lista.

## Objetivos

1. Entregar um painel com abas Visão geral, Canais, Modelos, Usuários, Tokens e Grupos, filtros de modelo, canal e período, e exportação.
2. Na Visão geral, mostrar uma grade com volume de requisições, tokens, custo por modelo e latência p50/p95. Cada gráfico traz seus números de resumo, e os gráficos de série temporal permitem alternar entre barra e linha.
3. Nas demais abas, mostrar um ranking da dimensão por requisições, tokens, custo, taxa de erro e latência.
4. Na seção Common Logs, acrescentar faixa de resumo, gráfico de volume com erros, coluna de status e painel lateral de detalhe.
5. Manter tudo correto nos temas claro e escuro, nos presets de tema, em todos os idiomas e em telas de celular.

## Não objetivos

- Capturar ou exibir prompts, respostas, tools ou request/response bruto. Isso é a etapa 2.
- Alterar relay, cobrança ou a forma como os logs são gravados.
- Alterar as seções Drawing Logs e Task Logs.
- Trazer abas de recursos que o new-api não tem, como MCP.
- Criar tabelas novas ou migrações de banco.

## O que seria um fracasso mesmo funcionando

- A tela parecer outro produto, ou o menu, a fonte e os cards mudarem.
- Um usuário comum ver dados de outra pessoa, ou dados de canal e IP que hoje são restritos.
- O relay ficar mais lento, ou as consultas novas pesarem no banco de logs.
- Os totais do painel e da tela de logs divergirem entre si para o mesmo período e filtro.
- As próximas atualizações do upstream ficarem bem mais difíceis.

## Restrições do projeto

- Referências ao new-api e à QuantumNous, licença e cabeçalhos são protegidos pelo `AGENTS.md` e não podem mudar.
- O frontend deve reutilizar primeiro os componentes de `web/src/components/` e das features, conforme `AGENTS.md` e `web/AGENTS.md`.
- Gráficos usam VChart, o padrão de `web/AGENTS.md`, com os tokens de tema existentes (`--chart-*`, `--radius`, `--font-*`). Sem cores ou fontes fixas.
- Textos passam por i18next, com chave em inglês, nos arquivos de `web/src/i18n/locales/`.
- Números usam `@/lib/format`, e valores monetários usam `@/lib/currency`.
- SQL deve funcionar nos bancos suportados. Os relatórios atuais calculam buckets e percentis de forma portável, sem funções específicas de banco, e o código novo segue o mesmo padrão.
- Este repositório é o upstream mais uma camada fina de alterações próprias (`patches/README.md`). O código novo deve ficar em arquivos próprios sempre que possível, com alterações mínimas em arquivos do upstream.

## Situação atual

- A seção `reports` do dashboard é própria deste repositório, não do upstream. Ela usa `service/report.go`, `controller/report.go` e `web/src/features/dashboard/components/reports/`, e já calcula tendência, modelos, erros, latência média e p95, throughput, tokens, stream e canais diretamente da tabela `logs`, com escopo por papel.
- A seção `logs` não armazena conteúdo das chamadas. Os campos usados são: `type`, `created_at`, `username`, `token_name`, `model_name`, `quota`, tokens, `use_time`, `is_stream`, `channel_id`, `group`, `ip`, `request_id`, `upstream_request_id` e `other`.
- A tela de logs já tem filtros, exportação, `CommonLogsStats` e `details-dialog.tsx`. `/api/log/stat` retorna apenas cota, RPM e TPM, então não cobre o resumo novo.

## Comportamento

### Dashboard

- Implementado como evolução da seção `reports`. Decisão do autor: é código próprio do repositório, o que reduz conflitos com o upstream e reaproveita o backend existente.
- As seções atuais do dashboard continuam existindo, e a Visão geral atual continua sendo a tela de entrada (decisão do usuário). O painel novo fica em Relatórios.
- Nenhum painel atual de Relatórios é removido (decisão do usuário). Erros, composição de tokens e comparação de streaming ficam abaixo da grade na aba Visão geral do painel. Saúde e latência dos canais vão para a aba Canais.
- Usuário comum vê apenas os próprios dados e não vê as abas Canais e Usuários. O escopo é aplicado no backend, por `reportRoleScope`.
- Os filtros de modelo e canal valem para todas as abas. Filtro de canal é só para admin.
- O período usa as opções atuais (`ResolveReportWindow`). A exportação continua em CSV e passa a respeitar os filtros.
- Cada gráfico tem estados de carregamento, vazio e erro. A falha de um não derruba os outros, se os dados vierem de consultas separadas.

### Logs

- A faixa de resumo mostra requisições, taxa de sucesso, tokens, custo e latência média. Tudo respeita os filtros ativos.
- Somente registros `consume` e `error` entram no resumo. Sucesso é `consume / (consume + error)`.
- O gráfico mostra requisições por intervalo de tempo, com os erros destacados na cor de erro do tema. O tamanho do intervalo se ajusta ao período. Clicar nele não filtra nesta etapa.
- A coluna de status mostra OK para `consume`, Erro para `error` e o rótulo do tipo atual nos demais registros.
- Todos os filtros atuais continuam disponíveis, inclusive a escolha por tipo de log.
- Clicar em uma linha abre um painel lateral (`components/ui/sheet.tsx`) com: status e IDs da requisição, stream, origem (usuário, token, grupo, canal), consumo (tokens de prompt, resposta e cache, custo), tempos (total e primeira resposta, quando houver) e mensagem de erro.
- O painel lateral substitui o diálogo de detalhes na seção Common Logs e reaproveita o conteúdo dele. Tudo que hoje é exclusivo de admin ou oculto pelo modo de dados sensíveis continua igual.
- No celular, os cards atuais permanecem, e o painel abre em largura total.

### Backend

- Aditivo: nenhuma rota existente muda de contrato.
- `GET /api/dashboard/report` ganha filtros opcionais de modelo e canal, p50 na latência e uma forma de obter o ranking por dimensão. O p50 sai da mesma amostra limitada do p95.
- Novas rotas de resumo de logs para admin e para o próprio usuário, com os mesmos filtros da listagem, retornando totais e buckets. Escopo igual ao de `/api/log` e `/api/log/self`.
- Consultas de ranking devem ter limite de linhas, como as atuais. A primeira resposta (`frt`) fica em JSON no campo `other` e não entra em agregados, apenas no detalhe da linha.

## Referência ao Bifrost

O Bifrost é apenas referência visual e de estrutura. As telas são reimplementadas sobre os componentes do new-api, sem copiar código. Decisão do autor: os frameworks são diferentes, e isso evita qualquer obrigação extra de licenciamento.

## Decisões do autor para revisão

- Evoluir a seção `reports` em vez de criar uma seção nova.
- Não copiar código do Bifrost.
- Substituir o diálogo de detalhes pelo painel lateral na seção Common Logs.
- Latência dos rankings com a mesma amostragem limitada dos relatórios atuais, aceitando que o valor é aproximado em grandes volumes.
- Definição única de totais em todas as telas: requisições são `consume + error`; tokens, custo e latência vêm só de `consume`; reembolsos não são abatidos do custo. `queryReportTrendBuckets` hoje soma todos os tipos de log, então os relatórios são corrigidos para esta definição e seus números podem mudar.
- Custo por modelo é uma série temporal com os 5 modelos de maior custo mais "Outros". Latência p50/p95 é uma série temporal calculada em Go a partir de uma única amostra limitada do período. Nos dois casos, a troca barra/linha vale como nos demais gráficos de série.
- Rankings mostram até 50 linhas, escolhidas por volume de requisições, com todas as métricas como colunas ordenáveis no navegador. Um item fora dessas 50 não aparece, mesmo com taxa de erro alta.
- O painel lateral mantém todo o conteúdo do diálogo atual, por tipo de log e papel. Os campos listados aqui vêm primeiro, e cobrança, recarga e auditoria aparecem depois.
- O CSV continua sendo o relatório completo, agora com os filtros aplicados. Nas abas de ranking, a exportação baixa o ranking da aba.

## A cargo de quem implementar

- Componentes e divisão de arquivos.
- Formato exato dos parâmetros e das respostas das rotas.
- Tamanho dos intervalos do gráfico e limites de consulta.

## Verificação

- Testes em Go para filtros, escopo por papel, totais, buckets e p50, seguindo `controller/report_test.go` e `service/report_test.go`.
- Na pasta `web/`: `bun run typecheck`, `bun run lint`, `bun run test`, `bun run build`, `bun run i18n:check-reports` e testes dos componentes novos.
- Checar visualmente com o app rodando: temas claro e escuro, pelo menos um preset, celular, admin e usuário comum.
- Conferir se os totais do painel e da tela de logs batem entre si no mesmo período e filtro, já com a definição única de totais.
