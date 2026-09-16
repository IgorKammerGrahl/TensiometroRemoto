# Aplicativo — PWA de monitoramento

React + TypeScript, servido em produção pelo próprio binário Go via `embed.FS`.
Autenticação por cookie de sessão emitido pelo backend (`/api/v1/app/login`).

## Construir

**Um comando. É este:**

```sh
npm run build      # em App/ — escreve em Backend/internal/webapp/dist/
```

Não há etapa de cópia depois. O `outDir` do Vite aponta direto para dentro do
módulo Go porque `//go:embed` **não aceita caminho com `..`**: o destino tem que
morar sob o módulo ou não existe forma de embarcá-lo.

`dist/` é versionado, para que `go build` funcione em máquina sem Node. O preço é
que o artefato **envelhece calado**: alterar um `.tsx` e commitar sem reconstruir
produz um binário que serve a versão anterior da interface, sem erro nenhum.

> **Rode `npm run build` antes de qualquer commit que altere `App/`.**

Para conferir:

```sh
npm run dist:check   # sai 1 se dist/ está ausente ou mais velho que a fonte
```

Compara a data do `dist/index.html` com a de `src/`, `index.html`, `public/` e
`vite.config.ts`. Não é gancho de commit nem etapa de CI de propósito — é uma
linha que se roda de graça quando bate a dúvida.

## Desenvolver

```sh
npm install
npm run dev        # http://localhost:5173, com proxy de /api para :8080
```

O proxy do `vite.config.ts` não é conveniência: **é o que faz a sessão
funcionar em desenvolvimento.** Sem ele o front (`:5173`) e a API (`:8080`) são
origens diferentes, o cookie `SameSite=Lax` não acompanha a requisição, o login
responde 204, o cookie é descartado em silêncio e a requisição seguinte volta
401 — sem nada na tela que aponte a causa.

Para testar no celular pela rede local, suba o backend com
`SESSAO_COOKIE_INSEGURO=true` (ver `Backend/README.md`) e acesse
`http://192.168.x.x:5173`.

```sh
npm run typecheck  # tsc --noEmit
npm test           # vitest (funções puras: zona, tempo, série, transporte)
```

O destino do proxy pode ser trocado por variável de ambiente, para apontar o
front a um backend dublê sem editar o `vite.config.ts`:

```sh
API=http://localhost:8081 npm run dev
```

Serve para exercitar estados que o banco de desenvolvimento não tem no momento
— nó nunca reportado, nó desativado, talhão sem faixa. **Não substitui a
verificação contra o backend real:** um dublê gera a série com o mesmo espaçamento
que o servidor usa para agregar, e foi exatamente essa coincidência que escondeu
o defeito descrito abaixo.

## Limitação conhecida — a API não expõe o período de amostragem do nó

Duas regras da interface precisam saber de quanto em quanto tempo o nó reporta,
e **esse dado não está no contrato**. As duas contornam a ausência de jeitos
diferentes, e vale saber por quê.

**`bucket_s` não é o período.** É a resolução com que o servidor agrega — hoje
60 s para qualquer janela. O período é propriedade do dispositivo. Quando o nó
reporta mais devagar que o balde, os dois divergem: `tensio-demo` amostra a cada
15 min, então 14 de cada 15 baldes saem vazios e todo par de pontos consecutivos
fica a 900 s de distância. A regra original de lacuna — `Δt > 1,5 × bucket_s`,
tirada do contrato do backend — declarava lacuna em **todos** os 76 intervalos e
a legenda anunciava "76 trechos sem dados" sobre uma série contínua. É o erro
que a segmentação existe para impedir, invertido: em vez de inventar dado onde
faltou, inventa falta onde o dado está inteiro.

O dublê de desenvolvimento nunca reproduziu isso porque emitia um ponto por
balde. Só apareceu contra o backend real.

`src/serie.ts` passou a derivar o período **da própria série** — mediana dos
intervalos observados, com `bucket_s` de piso. Mediana e não média porque a
média seria puxada pela lacuna que se quer detectar; piso porque série curta
demais para ter ritmo deve cair no limite menor, que separa, em vez de num
limite grande que ligaria.

`LIMIAR_LEITURA_VELHA_S` em `src/tempo.ts` continua **fixo em 30 minutos**, e um
valor único não serve para todos os nós: um nó em *deep sleep* de 30 minutos e um
nó de bancada amostrando a cada 60 segundos não podem compartilhar o mesmo
limiar. O `tensio-demo`, a 15 min, está a dois reportes perdidos de parecer
velho. Aqui a mediana não ajuda — a pergunta é sobre a leitura mais recente, não
sobre o ritmo do histórico.

Quando a API expuser o período, ele substitui a mediana em `serie.ts` e vira a
base do limiar em `tempo.ts` (algo como `3 × intervalo_s`). A restrição vira
concreta quando a etapa 2 do firmware entregar o deep sleep.

Duas ressalvas ficam registradas em `src/serie.ts`, com os números medidos: a
mediana descreve o histórico visível, e uma janela que cruze a transição para
deep sleep vai misturar dois regimes; e a grade do balde é fixa enquanto a
amostragem deriva, então um nó de 61 s contra balde de 60 s deixa um balde vazio
de tempos em tempos sem ter perdido reporte nenhum. As duas erram para o lado de
quebrar a linha, que é o lado seguro — nunca para o de ligá-la.

**Expor o período de amostragem resolve as duas de uma vez**, e é trabalho de
backend: o limiar passaria a ser função do período do nó em vez da grade do
balde, o artefato de alinhamento deixaria de existir e o regime não precisaria
mais ser inferido por mediana.

Enquanto isso, o gráfico **não conta quantas paradas houve**. Artefato de grade e
reporte perdido são indistinguíveis na série agregada, então a legenda diz que a
linha se interrompe onde leituras consecutivas ficaram distantes, e para por aí.
Contar seria afirmar uma precisão que o dado não tem.

## Limitação conhecida — o PATCH de talhão não sabe apagar a faixa

Configurar os limiares funciona; **voltar um talhão para não-configurado, não**.
Descoberto do mesmo jeito que o anterior: contra o servidor de verdade, com um
botão que devolvia `200 OK` e não mudava nada.

A causa está no Go. Os campos são `*float32`, e `json.Unmarshal` deixa o
ponteiro `nil` tanto para `null` explícito quanto para chave ausente — os dois
casos ficam indistinguíveis. `AtualizarTalhao` então faz
`COALESCE($4, t.kpa_alerta)`, que lê `nil` como *mantenha o que está lá*. Enviar
`{"kpa_alerta": null, "kpa_estresse": null}` é, para aquele endpoint,
exatamente o mesmo que enviar `{}`.

O botão saiu do formulário. Um controle que reporta sucesso e não faz nada é
pior que controle nenhum: a interface passa a afirmar um fato falso, que é o
erro que este projeto persegue no gráfico e no indicador. Hoje se limpa a faixa
pelo banco:

```sql
UPDATE talhoes SET kpa_alerta = NULL, kpa_estresse = NULL WHERE id = '<uuid>';
```

O conserto é de backend e fica para depois da PWA. Ele é pequeno — distinguir
ausente de nulo e trocar o `COALESCE` —, mas a escolha entre as três formas de
distinguir (ponteiro duplo, `json.RawMessage` ou um sentinela) tem efeito
observável no contrato da API e merece um parágrafo no capítulo 5, não uma nota
de rodapé.

**Status do `RF07`: cumprido apenas na direção de configurar.** Não é *atendido*.
O texto do TCC precisa declarar isso como limitação conhecida; afirmar o
requisito como entregue seria a mesma classe de erro que o botão removido —
relatar como feito o que não acontece.

## `RF10` — verificado ponta a ponta contra o servidor

O caminho inteiro do RF10 foi exercido contra o backend em execução, pela
própria interface, em 09/09/2026 — não contra dublê e não só por chamada de
API. Vale para o capítulo de resultados.

| passo | como foi feito | resultado |
|---|---|---|
| Cadastrar nó | formulário de `/nos/novo` | `201`, token exibido **uma vez** |
| Editar descrição | painel do detalhe | `200`, talhão preservado |
| Mover para talhão sem concessão | `PATCH talhao_id` | `404 device nao encontrado` |
| Enviar diff vazio | painel do detalhe | não sai requisição nenhuma |
| Desativar | painel do detalhe | `200`, `ativo=false` |
| **Ingerir com o nó desativado** | `POST /readings` com o token real emitido no cadastro | **`403 device inativo`** |

A última linha é a que fecha o requisito. A confirmação de desativar promete,
em português, que *"o servidor passa a recusar as leituras deste nó"*; o 403
com o token verdadeiro transforma essa frase em fato verificado, em vez de
intenção de quem escreveu o texto. E o token só respondeu 403 — e não `401
token invalido` — porque era o token certo: o mesmo ensaio prova que o valor
mostrado uma única vez na tela é o valor que autentica.

O `404` da terceira linha é deliberadamente ambíguo: ele funde nó inexistente,
nó sem concessão e talhão sem concessão. A interface **não tenta adivinhar
qual das duas concessões faltou** — e a garantia não está na disciplina de
quem escreve a mensagem, está na assinatura de `explicarFalhaDeNo`, que recebe
o erro e a operação e nada mais. Não há parâmetro por onde o palpite entrasse.

> O token de bancada emitido nesse ensaio **não é versionado** e não aparece
> em lugar nenhum do repositório. Se um nó real for gravado com ele, reemita
> antes da entrega: cadastre outro nó e desative este.

## Escopo

Instalação como PWA (manifest, ícones, service worker) está no escopo.
**Funcionamento offline não está** — e não por falta de tempo: um cache serviria
uma leitura de três horas atrás com a mesma aparência de uma leitura de agora, o
que destrói justamente a distinção que a interface inteira existe para fazer. Ver
o comentário em `public/sw.js`.

## Organização

| Arquivo | Papel |
|---|---|
| `src/api.ts` | Tipos do contrato e transporte. Nenhuma regra de negócio. |
| `src/zona.ts` | Zona → aparência. **Não recebe limiar**: a comparação é inexpressável aqui. |
| `src/tempo.ts` | Idade da leitura ancorada em `medido_ha_s`, e formatação de duração. |
| `src/serie.ts` | Quebra a série em trechos contíguos para o gráfico não ligar através de lacuna. |
| `src/leitura.ts` | A **ordem** desativado → nunca reportou → velha → atual. A zona só é alcançável no último caso. |
| `src/escala.ts` | Geometria da régua: escala 0…−80 kPa (medição) separada das bandas (classificação). |
| `src/janela.ts` | Janelas do histórico. **O cliente escolhe a duração; o servidor decide quando é "agora"** — `to` nunca é enviado. |
| `src/relogio.ts` | `useAgora`: relógio que avança sozinho, porque *atual → velha* muda sem resposta nova do servidor. |
| `src/sessao.tsx` | Sessão, login/logout e o porteiro das rotas (401 ≠ 403 ≠ sem rede). |
| `src/componentes/` | Régua, gráfico e as marcas de estado. Sem regra de negócio: recebem o que decidir já decidiu. |
| `src/telas/` | Lista de nós e detalhe do nó. |
| `src/erros.ts` | Erro de nó → frase. Não tem por onde receber *qual* concessão faltou. |
| `src/componentes/Faixa.tsx` | Configuração dos limiares. A inversão é **inexprimível**, não detectada. |
| `src/telas/NovoNo.tsx` | Cadastro. É sobre a **ordem das operações**: o token aparece uma vez. |
| `src/componentes/Gerenciar.tsx` | Editar, mover e desativar. Dois controles porque são dois riscos. |
| `src/estilo/` | Tokens e base. Cor é reservada para estado; interação é tinta e forma. |

As seis primeiras são funções puras com teste no Vitest. É onde o erro seria
silencioso — inverter uma comparação de kPa negativo, ligar dois pontos através
de uma lacuna, envelhecer uma leitura pelo relógio errado, mostrar verde sobre
leitura morta — então é onde o teste está.

Duas decisões de desenho registradas no próprio código, porque são do tipo que
alguém "otimiza" depois sem que nada quebre visivelmente:

- **A lista nunca é reordenada no cliente** (`src/api.ts`). O servidor já devolve
  `ORDER BY kpa ASC`, que é o mais seco primeiro. Ordenar por kPa *decrescente*
  pareceria certo em qualquer revisão rápida e poria o talhão mais seco no fim.
- **Talhão sem faixa configurada mantém a escala e o entalhe, e perde só as
  bandas** (`src/escala.ts`). Posição é medição e não depende de limiar; banda é
  classificação e depende. Suprimir a régua inteira seria o único lugar da
  interface onde falta de configuração remove um componente em vez de marcá-lo.
- **O formulário de faixas não compara os dois limiares** (`src/componentes/Faixa.tsx`).
  Não existe `estresse < alerta` escrito em TypeScript: o campo de "seco" tem
  como teto o valor corrente de "secando" e o de "secando" tem como piso o de
  "seco", via `min`/`max` nativos. A inversão não é detectada — é inexprimível,
  e o navegador ainda dá a mensagem e a acessibilidade de graça. Validar
  continua sendo do servidor, cuja mensagem vai para a tela como veio.
- **A troca de janela se apoia no relógio do servidor** (`src/janela.ts`). A
  interface manda só `from`, calculado a partir do `to` que a resposta anterior
  trouxe, e nunca manda `to`. Recalcular pelo relógio do celular pareceria
  equivalente e não é: o ensaio de 17 h de 27/08/2026 mediu latência **mínima de
  −0,123 s** — `received_at` anterior a `measured_at` —, ou seja, os dois
  relógios já divergiram na prática.

`d3-shape` foi avaliado e descartado. Suas curvas inventam suavidade entre
amostras — o mesmo pecado da lacuna ligada, em escala menor — e a interpolação
linear honesta é uma string montada em uma linha. Ficou `d3-scale`, pelos ticks
de tempo.
