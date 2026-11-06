# Acabamento visual no estilo do Bifrost — Design

Data: 2026-11-05
Estado: rascunho para aprovação

## Intenção

Nas etapas 1 e 2, o conteúdo do painel de Relatórios, da tela de Logs de uso e do painel de detalhe da chamada ficou certo, mas o usuário comparou as mesmas telas com o Bifrost, com os mesmos dados, e concluiu: "sem dúvida o lado Bifrost é mais bem trabalhado". O que incomoda é "a parte de CSS, exibição mesmo". Esta etapa traz o acabamento do Bifrost para essas telas, sem mudar o que elas mostram nem a identidade do new-api.

## Decisões do usuário

- Adotar o acabamento do Bifrost, todos os itens levantados.
- Aplicar no painel de detalhe da chamada e também na lista de logs e no dashboard (Relatórios).
- Aplicar só nas telas que fizemos. O resto do new-api fica como está.

## Base

As medidas vêm de estilos computados nos dois sistemas: `docs/superpowers/specs/2026-11-05-visual-finish-measurements.md`. Capturas lado a lado ficam em `/private/tmp/claude-501/-Users-neimaravila-qwen3/890d5424-2c41-4f7f-a8d5-39c9cc4a1fac/scratchpad/compare/shots/` (`nx-*` é o new-api, `bf-*` o Bifrost). As instâncias de comparação são o new-api em `http://localhost:13000` e o Bifrost em `http://127.0.0.1:8081`.

O documento de medidas separa identidade de acabamento. A seção 0 dele vale como regra.

## Escopo

**Telas:**
- seção Relatórios do dashboard, com abas, grade de gráficos, rankings e os painéis que ficaram lá;
- seção Common Logs de Logs de uso: resumo, gráfico de volume, barra de filtros da tela, tabela e paginação;
- painel de detalhe da chamada, com cabeçalho, indicadores, detalhes expansíveis e as abas Conversa, Tools, Roteamento e JSON bruto.

**Fora:**
- demais seções do dashboard, Drawing e Task Logs;
- menu lateral, cabeçalho, logo e qualquer outra tela;
- componentes compartilhados de `web/src/components/` na forma como outras telas os usam. Mudanças valem só onde as telas acima os usam: por classes ou props passadas a partir delas, ou por componentes próprios dessas features.

## O que muda (os 10 itens de maior impacto, na ordem)

1. **Conversa em fonte comum.** Prosa em sans 13px, lh 1.6. Mono 12.5px só para JSON, argumentos e resultados de tools e trechos de código.
2. **Hierarquia dos números.** Valores maiores com `tabular-nums` (18px/600 no detalhe, 22-24px/600 nas faixas de métricas da lista e dos relatórios). Rótulos em 11-12px/500, caixa alta, `letter-spacing` 0.05em. Unidades menores e esmaecidas.
3. **Cor por papel na conversa.** Sistema, usuário, assistente e tool com fundo e borda próprios, tingidos de leve a partir das cores do tema, mais uma linha do tempo à esquerda com um ponto por mensagem. Vale no tema escuro também.
4. **JSON colorido.** Cores de sintaxe para chaves, strings, números e literais nas chamadas de tool, nos resultados e no JSON bruto. Altura de linha 1.5. Sem `break-all` cortando palavras: quebra em pontos naturais e rolagem horizontal quando preciso.
5. **Tabela mais compacta.** Linhas de cerca de 48px, células com `6px 12px`, peso 400 no texto principal e 11-12px esmaecido no secundário.
6. **Cantos padronizados dentro das telas.** 8-10px em tabelas, blocos de mensagem, campos e cards internos, 6px em badges. Sem raios de 13 a 42px misturados na mesma tela.
7. **Status como badge.** Selos com fundo tingido e texto da mesma cor para status, tipo de chamada e stream. Modelo, token e custo ficam como texto simples.
8. **Topo do detalhe em faixa única.** Os indicadores viram uma faixa com divisórias de 1px, e não cinco cards separados.
9. **Barra de status na linha.** Uma barra de 3-4px por 24px, com cantos de 4px, no começo de cada linha da tabela, substituindo o ponto mais a palavra. O fundo rosado das linhas com erro continua.
10. **Abas com contagem.** Abas do detalhe mostram contadores. Na aba Tools os esquemas começam recolhidos.

As demais diferenças do documento de medidas (hero e cards do dashboard, lista de saúde dos canais como linhas, títulos de seção, barra de filtros, chips Usage/RPM/TPM fora dos filtros, paginação, cabeçalho e abas do detalhe, Roteamento e JSON bruto) também entram, aplicando os valores de lá.

## Identidade que não muda

- Cores primárias e de link do new-api, fonte principal (`--font-ui-sans`) e mono do tema. Nada da fonte Geist nem do verde-esmeralda do Bifrost.
- Presets de tema e tema escuro continuam funcionando. As cores novas saem dos tokens existentes (`--chart-*`, `--success`, `--destructive`, `--muted`, `--border`, `--primary`) por mistura ou opacidade, sem cores fixas.
- O raio de 16px do tema continua no contorno das páginas e dos cards de seção. O raio menor vale para elementos internos de dados.
- Textos e conteúdo das telas não mudam, só a forma de exibir.

## Restrições

- Valem as do `AGENTS.md` e `web/AGENTS.md`: reutilizar componentes, Tailwind e tokens, i18n e formatação por `@/lib/format`.
- Nenhuma dependência nova. O realce de JSON é implementado sem biblioteca externa nova. Se já houver algo no projeto, como o Shiki, usar só até um limite de tamanho; acima dele, mostrar texto sem cores.
- Acessibilidade: contraste de texto 4.5:1 e indicadores de status que não dependem só de cor (badge com texto, barra com rótulo acessível).
- Desempenho: o realce de JSON e a renderização da conversa continuam limitados como hoje (256 KiB para formatar, blocos longos recolhidos).

## O que seria um fracasso mesmo funcionando

- Outras telas do new-api mudarem de aparência.
- A tela parecer o Bifrost "com outra logo", ou perder a cor e a fonte do new-api.
- O tema escuro ou um preset ficarem ilegíveis.
- Mudar algum dado ou comportamento das telas, como filtros, navegação, permissões e dados sensíveis.

## A cargo de quem implementar

- Como organizar as classes, variantes e componentes de cada tela.
- Valores exatos dentro das faixas indicadas, buscando ficar próximo das medidas do Bifrost.

## Verificação

- Testes existentes passando, mais testes que fixem o que mudou de comportamento visual testável, como badges com texto, contadores nas abas e JSON sem cores acima do limite.
- Checagem em navegador real, refazendo as mesmas capturas da comparação no new-api, nos temas claro e escuro, em um preset, no celular e em pt-BR, ao lado das capturas do Bifrost.
- Conferência de que telas fora do escopo não mudaram, com capturas antes e depois de pelo menos três delas: Visão geral do dashboard, Canais e Drawing Logs.
