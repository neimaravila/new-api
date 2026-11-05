# Conteúdo das chamadas nos logs — Design

Data: 2026-11-05
Estado: rascunho para aprovação

## Intenção

Esta é a etapa 2 do trabalho inspirado no Bifrost. A etapa 1 trouxe o painel e o resumo de logs com dados existentes. Agora o usuário quer ver, em cada chamada, o que o LLM Log do Bifrost mostra, da forma mais fiel possível: conversa, tools, custos, roteamento e JSON bruto. O uso é interno, e o valor buscado é entender e depurar qualquer chamada sem sair do new-api, mantendo a identidade dele.

## Decisões do usuário

- O mais fiel possível ao detalhe do Bifrost, incluindo tools, custos e roteamento.
- Admin vê o conteúdo de todos; cada usuário comum vê só o próprio.
- Retenção de 30 dias para o conteúdo. Métricas e custos continuam nos logs normais.
- Grava tudo: não há pessoa, token ou tipo de chamada excluídos.
- Painel largo, como no Bifrost: cerca de 60% da tela, por cima da lista.
- Usuário comum vê a própria conversa, tools, custos e o JSON que enviou e recebeu. Roteamento e o JSON enviado ao provedor são só para admins.

## Objetivos

1. Gravar, por tentativa, a requisição enviada ao provedor e a resposta recebida, e, por chamada, a requisição do cliente e a resposta entregue a ele.
2. Mostrar, ao clicar em uma linha de Common Logs, o painel com cabeçalho, indicadores, seção expansível de detalhes e as abas Conversa, Tools, Roteamento e JSON bruto.
3. Apagar automaticamente o conteúdo com mais de 30 dias.
4. Não afetar a resposta, o streaming, a latência percebida nem a cobrança das chamadas, mesmo se a captura falhar.

## Não objetivos

- Abas e recursos que o new-api não tem: sessões, plugins, guardrails, MCP, cache semântico, regras de roteamento e complexidade.
- Busca dentro do conteúdo ou filtros por tool na lista.
- Reprocessar chamadas antigas. Chamadas anteriores ao recurso mostram só os dados que já existem, com um aviso.
- Criptografia específica do conteúdo no banco.
- Exportação em massa de conteúdo.
- Excluir ou mascarar pessoas, tokens ou tipos de chamada.

## O que seria um fracasso mesmo funcionando

- Uma chamada falhar, atrasar ou ser cobrada de outro jeito por causa da captura.
- Um usuário comum ler conteúdo de outra pessoa, ou ver canais, chaves ou o JSON enviado ao provedor.
- Chaves de API, de cliente ou de provedor ficarem gravadas no conteúdo.
- A memória do servidor crescer com respostas grandes ou streams longos.
- O banco crescer sem limite, ou a limpeza travar os logs.
- A tela parecer outro produto.
- As próximas atualizações do upstream ficarem bem mais difíceis.

## Restrições do projeto

Valem as restrições da etapa 1 (`2026-11-05-dashboard-logs-visual-design.md`) e do `AGENTS.md`: identificadores protegidos, reutilização de componentes, i18n em todos os idiomas, formatação por `@/lib/format` e `@/lib/currency`, SQL portável em SQLite, MySQL, PostgreSQL e ClickHouse, e camada fina sobre o upstream.

- Rotas novas entram em `middleware/access_token_routes.go`.
- `patches/7033-sse-direct-forward.patch` é aplicado no build e altera `OaiStreamHandler`, `helper.go` e `response_compat.go`. Não colocar pontos de captura nesses arquivos.

## Situação atual

- O corpo do cliente fica em `common/gin.go` (`GetRequestBody`, com armazenamento em disco para corpos grandes), e o limite padrão é `MAX_REQUEST_BODY_MB` = 128.
- O corpo enviado ao provedor passa por `DoApiRequest` em `relay/channel/api_request.go`, e toda resposta HTTP passa por `relayClient.Do` no mesmo arquivo. `DoFormRequest`, `DoWssRequest` e os adaptadores siliconflow, jimeng e xunfei ficam fora desse caminho.
- As tentativas ficam no laço de `controller/relay.go`, com `RetryIndex` e `AppendUsedChannel`. Cada tentativa com falha gera um registro de erro com o mesmo `request_id`.
- `service/log_info_generate.go` já grava em `other`: canais usados, modelo mapeado, conversões, afinidade, tempo até o primeiro token e fatores de preço.
- Os DTOs de `relaykit/dto` permitem interpretar mensagens, tools, chamadas de tools e uso nos formatos OpenAI, Responses, Claude e Gemini.
- `createLog` é síncrono, não existe gravador em lote, e a limpeza de logs é sob demanda, com `ScheduledSystemTaskHandler` disponível para tarefas agendadas.
- `FormatAdminLogs`, `FormatRootLogs` e `formatUserLogs` já separam o que cada papel vê.

## Comportamento

### Captura

- Vale para chamadas de texto dos formatos Chat, Responses, Claude e Gemini, e para embeddings e rerank com corpo JSON. Outros tipos, e chamadas pelos caminhos fora de `DoApiRequest`, ficam com o aviso "Conteúdo não capturado para este tipo de chamada", sem quebrar nada.
- A captura copia os dados enquanto eles passam, sem atrasar o envio ao cliente. A gravação é assíncrona e em lote, em uma fila limitada. Com a fila cheia, descarta o conteúdo e registra um contador, sem bloquear a chamada.
- Cada corpo gravado tem um limite de tamanho, com aviso de "conteúdo truncado" no painel e na API. Imagens e áudio em base64 são substituídos por um marcador com tipo e tamanho.
- Cabeçalhos, URLs e chaves não são gravados. Para o provedor, ficam só método, caminho sem query e status.
- Resposta em streaming: guarda o SSE como veio e remonta a mensagem final para a aba Conversa. Stream interrompido fica marcado como incompleto, com o que chegou até ali.
- Chamada que falha em todas as tentativas também tem o conteúdo gravado.

### Armazenamento

- Tabela nova, separada de `logs`, no banco de logs, comprimida, com chave por `request_id` e tentativa. A tabela `logs` não muda.
- Limpeza diária de conteúdo com mais de 30 dias, em lotes, com `ScheduledSystemTaskHandler`. No ClickHouse, com TTL.
- A produção usa PostgreSQL, com o banco de logs junto do principal (decisão do usuário: o espaço está ok). Não há limite total de tamanho; a retenção é o único limite. Gravação e limpeza não podem disputar com as tabelas do sistema: lotes pequenos, com pausas e índice adequado, e PostgreSQL é o alvo principal de testes de carga.

### Painel

- Ocupa cerca de 60% da tela em telas grandes, por cima da lista, e largura total no celular. A reserva de espaço da etapa 1 deixa de existir. O conteúdo atual do painel é mantido na seção expansível.
- O conteúdo só é buscado ao abrir uma chamada.
- Topo: faixa de tentativas quando houver mais de uma; badges de tipo, status, HTTP e stream; canal, só para admin.
- Indicadores: latência e primeiro token, modelo e grupo, tokens de entrada e saída e cache, custo, quantidade de tools disponíveis e chamadas.
- Seção expansível: tempos, tokens, detalhamento do custo, origem e IDs, respeitando as regras atuais de papel e dados sensíveis.
- Conversa: mensagens por papel, com filtro por papel e histórico antigo recolhido. Cada bloco tem copiar e "mostrar mais". Chamadas de tools e seus resultados aparecem em blocos próprios, imagens e áudio como marcadores, e erros em um bloco destacado.
- Tools: nome, descrição e esquema de cada tool, com copiar.
- Roteamento, só para admin: linha do tempo das tentativas, com canal, chave, status, motivo e duração, além de grupo, afinidade e modelo mapeado.
- JSON bruto: para todos, requisição do cliente e resposta entregue. Para admin, também a requisição enviada e a resposta recebida de cada tentativa.
- Setas e teclado passam para a chamada anterior ou seguinte da lista. Menu com copiar o corpo da requisição e exportar a chamada em JSON.
- Estados: carregando, sem conteúdo (chamada antiga, tipo não capturado, conteúdo expirado), truncado e erro de carregamento.

### Acesso

- Rotas separadas, uma para admin e uma para o próprio usuário, seguindo `AdminAuth` e `UserAuth`. A rota do usuário comum filtra pelo `user_id` no backend e não retorna Roteamento nem dados enviados ao provedor.
- Um `request_id` de outra pessoa na rota do usuário comum retorna o mesmo erro de "não encontrado".
- Consultas de admin ao conteúdo de outra pessoa ficam registradas nos Audit Logs.

## Referência ao Bifrost

Apenas visual e de estrutura. As telas são reimplementadas sobre componentes do new-api, sem copiar código.

## Decisões do autor para revisão

- Limite de 1 MB por corpo gravado. Corpos de até 8 MB são interpretados fora do caminho da chamada: base64 vira marcador e, se ainda passar do limite, o texto de cada mensagem longa é encurtado mantendo começo e fim, para Conversa e Tools continuarem funcionando, sempre com aviso de truncado. Corpos acima de 8 MB guardam só o começo, em texto, e mostram só o JSON bruto. A mensagem remontada do stream tem limite próprio de 1 MB.
- O conteúdo só é alcançável a partir de uma linha de `logs`, e nada muda na lista nem nos totais. Só há captura quando a linha correspondente é gravada: sem `LOG_CONSUME_ENABLED` ou `ERROR_LOG_ENABLED`, aquele conteúdo não é guardado. Chamadas recusadas antes de qualquer tentativa, como falta de canal ou cota, seguem sem conteúdo.
- "Chaves não gravadas" significa: antes de gravar, trocar por marcador os valores exatos do token do cliente e da chave do canal usada, onde aparecerem nos corpos, inclusive em erros devolvidos pelo provedor. Não há busca por padrões, então uma chave de terceiros colada em um prompt fica gravada, conforme "grava tudo".
- Admin e root seguem a separação atual: admin vê `admin_info`, só root vê `root_info`. Admin pode ler o conteúdo de todos, inclusive de root. Em Roteamento, "chave" é o índice da multi-chave, nunca o valor.
- O modo de dados sensíveis também oculta Conversa, Tools e JSON bruto, com um aviso, para proteger telas compartilhadas.
- Interruptor global nas configurações do sistema para desligar a captura, ligado por padrão, conforme o pedido de gravar tudo. A retenção de 30 dias é configurável no mesmo lugar.
- Imagens e áudio em base64 não são gravados, só marcados.
- Consultas de admin ao conteúdo ficam nos Audit Logs.
- Quem usa o gateway deve ser informado de que o conteúdo é gravado. Isso cabe à empresa e não é tratado por este recurso.

## A cargo de quem implementar

- Nome da tabela, compressão e divisão de arquivos.
- Tamanho da fila, tamanho dos lotes e frequência de gravação.
- Formato exato das rotas e respostas.
- Pontos exatos de captura, respeitando as restrições acima.

## Verificação

- Testes em Go: captura em chamadas normais, em streaming e com tentativas; fila cheia e erro de gravação não afetam a resposta; truncamento; chaves ausentes do conteúdo gravado; retenção; acesso por papel, incluindo `request_id` de outra pessoa.
- Teste de comparação: corpo e SSE entregues ao cliente são idênticos com a captura ligada e desligada, com o patch 7033 aplicado como no build.
- Medição: latência adicional e memória com stream longo e resposta grande.
- Execução em SQLite, MySQL e PostgreSQL. ClickHouse pelo menos na criação da tabela, se houver ambiente.
- Frontend: as verificações da etapa 1, mais testes de renderização da conversa nos quatro formatos.
- Checagem em navegador real: painel, abas, papéis, celular, tema claro e escuro, pt-BR.
