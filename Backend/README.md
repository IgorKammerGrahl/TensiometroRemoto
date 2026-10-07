# Backend de telemetria — protótipo de tensiometria eletrônica

Ingestão e consulta de leituras de tensão de água no solo enviadas por nós
ESP32. TCC de Bacharelado em Ciência da Computação, IFC Campus Rio do Sul.

Go + `net/http` (roteamento do `ServeMux` 1.22) + Postgres via `pgx`.
Três dependências no total: `pgx`, `goose` e `x/crypto` (bcrypt).

## Executar

### Desenvolvimento local

```sh
cp .env.example .env     # preencha TELEMETRIA_TOKEN_SECRET
make dev                 # sobe o Postgres, migra e roda a API em :8080
```

`make dev` usa **Podman** por padrão (rootless: sem daemon e sem o seu
usuário num grupo equivalente a root). `make dev ENGINE=docker` roda
idêntico — o `compose.yaml` é a mesma especificação para os dois.

Sob Podman, o `compose` conversa com o socket do usuário. Uma vez por
máquina:

```sh
systemctl --user enable --now podman.socket
```

| Alvo | O que faz |
|---|---|
| `make dev` | banco + migrations + API em primeiro plano |
| `make up` / `make down` | só o Postgres (o volume **permanece**) |
| `make migrate` | migrations |
| `make seed` | imprime o banco alvo e **para**; `make seed ARGS=-confirmo` grava a série sintética |
| `make psql` | shell no banco |
| `make test` | suíte, com `TEST_DATABASE_URL` já apontado |
| `make dump` | backup datado — leituras de bancada só existem no volume |
| `make restore FILE=…` | restaura um dump |

Não há alvo que apague o volume. Apagar dados é decisão manual.

### Sem Makefile

```sh
export DATABASE_URL='postgres://user:senha@host:5432/telemetria?sslmode=disable'
export TELEMETRIA_TOKEN_SECRET='<32+ bytes aleatórios>'   # obrigatória

go run ./cmd/api -migrate    # aplica migrations e encerra
go run ./cmd/api             # sobe o servidor (PORT, padrão 8080)
```

Migrations **não** rodam no boot por padrão: com mais de uma réplica
subindo em paralelo, duas instâncias correriam o goose ao mesmo tempo.
`-migrate` é um passo separado do deploy. Para conveniência local,
`RUN_MIGRATIONS=true` aplica no boot (loga um aviso).

Trocar `TELEMETRIA_TOKEN_SECRET` invalida **todos** os tokens já emitidos:
os hashes gravados deixam de casar. Rotacionar exige reemitir os tokens.

`TELEMETRIA_TOKEN_SECRET` tem piso de **32 caracteres** e o processo **recusa
subir** abaixo disso — vale para `cmd/api`, `cmd/admin`, `cmd/devtoken` e
`cmd/seed`. O piso não mede entropia (não há como medir entropia de uma
string): ele pega o erro que de fato acontece, que é o `troque-me` ou o
`segredo123` digitado com pressa. `openssl rand -base64 32` sai com 44
caracteres e passa com folga.

### Cadastrar um nó

```sh
go run ./cmd/admin device -id=tensio-01 -descricao="nó do talhão norte" [-talhao=<uuid>]
```

Imprime o token em claro (uma única vez — grave no firmware) e grava o
device. O banco guarda só o HMAC.

`cmd/devtoken` faz o mesmo cálculo mas **não** conecta ao banco: imprime o
token e o `INSERT` correspondente, para quem gera o token de um nó sem ter
— e sem dever ter — acesso ao banco. Para subir um ambiente do zero, use
`admin device`.

### Bootstrap de um ambiente

Sequência completa, do banco vazio ao nó aparecendo na interface:

```sh
export DATABASE_URL='postgres://tcc:tcc@localhost:55432/telemetria?sslmode=disable'
export TELEMETRIA_TOKEN_SECRET='<32+ bytes aleatórios>'

go run ./cmd/api -migrate

go run ./cmd/admin usuario  -email=produtor@exemplo.com -nome="Produtor"
#   -> imprime a senha gerada, uma única vez

go run ./cmd/admin talhao   -nome="Talhão de Bancada"
#   -> imprime o uuid do talhão

go run ./cmd/admin conceder -email=produtor@exemplo.com -talhao=<uuid>
go run ./cmd/admin device   -id=tensio-01 -descricao="nó de bancada"
#   -> imprime o token; grave no firmware

go run ./cmd/admin associar -device=tensio-01 -talhao=<uuid>

go run ./cmd/admin calibracao -id=cal-2026-08-25-a -device=tensio-01 \
  -v-zero=<V> -k=<V/kPa> -divisor=<fator> -vdd=<mV> [-r2=] [-rmse=] [-nota=]
```

A senha e o token aparecem **uma vez cada**. O banco guarda bcrypt e HMAC, e
nenhum dos dois é reversível: perdeu, emita outro.

As faixas de atenção do talhão (`kpa_alerta`/`kpa_estresse`) não têm flag
aqui de propósito — são configuradas pela interface (`PATCH /app/talhoes/{id}`),
que é onde o produtor as ajusta por cultura.

**Sobre o nó que não aparece na lista:** o produtor faz login, a lista de nós
abre vazia, e parece falha de autenticação ou de autorização. Não é. A causa
é `devices.talhao_id` nulo: o acesso é concedido *por talhão*, e um nó que não
está em talhão nenhum não está no alcance de ninguém — nem de quem tem
concessão em todos os talhões. É *fail-closed* deliberado, para que um nó de
bancada não vaze para um produtor. Resolva com
`go run ./cmd/admin associar -device=tensio-01 -talhao=<uuid>`.

**Sobre toda leitura vir recusada:** o nó autentica, o `POST /readings`
responde `200`, e todas as leituras voltam em `rejections` com
`calibration_id ausente`. Parece erro do firmware. A causa é não existir
linha em `calibrations` para aquele `calibration_id`: o backend recusa
leitura cuja conversão em kPa não pode ser refeita depois. Resolva com
`go run ./cmd/admin calibracao -id=<id> -device=tensio-01 ...`.

### Testes

Rodam contra um Postgres real; o que está sob teste é exatamente
`ON CONFLICT`, chave estrangeira e `TIMESTAMPTZ`, que mock nenhum reproduz.
Sem `TEST_DATABASE_URL` a suíte é pulada.

```sh
make test
```

O `compose.yaml` já cria `telemetria_test` num banco separado, para que
`go test` possa truncar tabelas sem levar junto os dados de bancada.

## API de dispositivo — `/api/v1/*`

Todas as rotas exigem `Authorization: Bearer <token do dispositivo>`.
Um token autoriza apenas o próprio device, inclusive na leitura.

### `POST /api/v1/readings`

```json
{
  "device_id": "tensio-01",
  "readings": [
    { "seq": 1043, "measured_at": "2026-08-19T14:32:10Z", "raw_mv": 2659,
      "vdd_mv": 4980, "calibration_id": "cal-2026-08-20-a", "kpa": -12.4 }
  ]
}
```

Resposta `200`:

```json
{ "accepted": 1, "duplicates": 0, "rejected": 0,
  "rejections": [], "server_time": "2026-08-19T14:32:11Z" }
```

**Limite de lote: 500 leituras por requisição.** Faz parte do contrato — o
firmware precisa fatiar o buffer local em múltiplos POSTs ao respeitá-lo.
Lote maior recebe `413` com o limite na mensagem, para que o nó reaja em
vez de reenviar em loop. O corpo também é limitado a 1 MiB.

**Idempotência** por `(device_id, seq)`: reenvio após resposta perdida é
descartado em silêncio e contado em `duplicates` — não é erro.

**Rejeição é por leitura, não por lote.** Uma leitura inválida não derruba
as demais; ela sai em `rejections` com `index`, `seq` e motivo. Sem esse
retorno o nó retransmitiria para sempre algo que o servidor nunca aceitaria.
Motivos possíveis:

| Motivo | Regra |
|---|---|
| `raw_mv fora da faixa [0,3300]` | faixa física do divisor |
| `kpa fora da faixa [-100,10]` | faixa do XGZP6847A100KPGN + folga de offset |
| `measured_at fora da janela plausível` | 2020-01-01 até agora+2h |
| `calibration_id ausente` / `desconhecido` | precisa existir e ser do device |
| `<campo> ausente` | `seq`, `measured_at`, `raw_mv`, `kpa`, `calibration_id` |

A janela de `measured_at` existe porque o ESP32 não tem RTC: sem NTP ele
boota em 1970 e envenenaria a série temporal.

Status: `400` JSON inválido ou `device_id` ausente · `401` token ausente ou
desconhecido · `403` device inativo, ou `device_id`/rota que não é do token ·
`413` lote ou corpo acima do limite.

### `GET /api/v1/devices/{id}/readings?from=&to=&limit=`

`from`/`to` em RFC3339 (padrão: últimas 24 h). `limit` padrão 100, teto 1000.
Ordem: `measured_at` decrescente.

### `GET /api/v1/devices/{id}/latest`

Leitura mais recente, ou `404` se o device não tem nenhuma.

## API do app — `/api/v1/app/*`

Plano separado do de dispositivo, e a separação é estrutural: as rotas
`/app/*` nascem embrulhadas no middleware de sessão, que nunca lê
`Authorization`. Token de dispositivo em `/app/*` é `401`; cookie de sessão
em `POST /readings` é `401`. Nenhum dos dois depende de alguém lembrar de
checar.

Autenticação por cookie `sessao` opaco (`HttpOnly`, `SameSite=Lax`,
`Secure`), 32 bytes aleatórios, guardado como HMAC igual ao token de
dispositivo. O `expira_em` é estendido a cada requisição autenticada — sem
isso o produtor seria deslogado no meio do uso.

**Autorização é cláusula `WHERE`, não `if`.** Toda consulta recebe o
`usuario_id` como primeiro parâmetro e resolve o acesso no próprio `JOIN`:
`device → talhão → concessão → usuário`. Não existe variante da assinatura
que dispense o `usuario_id`, então esquecer a autorização é erro de
compilação, não vazamento silencioso.

**Recurso fora do alcance responde `404`, nunca `403`.** `403` confirmaria
que o recurso existe; `404` não distingue "não existe" de "não é seu".

| Rota | O que faz |
|---|---|
| `POST /app/login` | `{email, senha}` → `204` + cookie. `401` credencial inválida (mesma resposta para senha errada e conta inexistente), `403` conta desativada, `429` tentativas demais |
| `POST /app/logout` | Apaga a sessão e expira o cookie |
| `GET /app/me` | `{usuario, talhoes}` — só os talhões concedidos |
| `GET /app/devices` | `{count, devices}`, **pior primeiro** |
| `POST /app/devices` | `{id, descricao, talhao_id}` → `201` `{device, token, aviso}`. `404` talhão sem concessão, `409` id repetido |
| `GET /app/devices/{id}` | Um nó, com talhão e última leitura |
| `PATCH /app/devices/{id}` | `{descricao?, talhao_id?, ativo?}` |
| `GET /app/devices/{id}/series?from=&to=&bucket=` | Série agregada no servidor |
| `GET /app/devices/{id}/calibracoes` | Ensaios do nó, do mais recente para o mais antigo |
| `POST /app/devices/{id}/calibracoes` | `{v_zero_kpa, k_v_por_kpa, fator_divisor, vdd_ensaio_mv, r2?, rmse_kpa?, nota?}` → `201` `{calibracao, aviso}`. `400` coeficiente fora da faixa física, `404` nó fora do alcance |
| `GET /app/talhoes` | Talhões concedidos, com os limiares |
| `PATCH /app/talhoes/{id}` | `{cultura?, kpa_alerta?, kpa_estresse?}` |

### Calibração pelo app

Sem uma linha em `calibrations` o nó autentica, o `POST /readings` responde
`200` e **toda** leitura volta em `rejected`. O sintoma parece do firmware e
o que falta é cadastro. `POST /app/devices/{id}/calibracoes` existe para que
fechar esse ciclo não exija um terminal — é o mesmo registro que
`admin calibracao` faz pela linha de comando.

**`v_zero_kpa` e `k_v_por_kpa` estão em VOLTS e V/kPa, não em milivolts.**
O nome da coluna atrapalha: `v_zero_kpa` é a *tensão* no ponto de 0 kPa. Para
o XGZP6847A os valores são `4.5` e `0.04`. Informar `4500` e `40` passava por
todos os `CHECK` do banco — o nó de bancada foi cadastrado assim — e produz
uma série inteira de kPa plausíveis e mil vezes errados, que é pior que série
nenhuma, porque ninguém desconfia dela. Desde a migration 0003 o próprio banco
recusa (ver "Conversão de kPa a partir de `raw_mv`"). O endpoint barra isso de
duas formas: pela faixa de cada coeficiente e, mais
importante, verificando que o ponto de 0 kPa (`v_zero_kpa * 1000 /
fator_divisor`) cai dentro dos `[0, 3300]` mV que o ADC do nó consegue ler.

A verificação olha só o ponto de 0 kPa, e não a faixa inteira: a calibração
nominal do sensor alcança −112 kPa na borda de 0 mV e seria recusada por um
teste mais estrito.

O `id` é gerado no servidor no formato `cal-AAAA-MM-DD-xxxx`, com sufixo
sorteado. Curto porque é **digitado no celular**, no campo `CALIBRATION_ID`
do portal cativo do nó; sorteado e não sequencial porque `calibrations.id` é
um namespace global — um contador diria a cada usuário quantos ensaios os
outros registraram naquele dia.

**Não há `PATCH` nem `DELETE`: ensaio novo é id novo.** Os coeficientes dão
significado às leituras já gravadas que apontam para eles; editar no lugar
reescreveria o passado em silêncio, sem nada no dado registrando a mudança.

### Limite de tentativas no login

`POST /app/login` é o único endpoint que roda bcrypt custo 12 — ~250 ms de CPU
por requisição, **inclusive quando o e-mail não existe**, porque a defesa
contra enumeração por timing exige o mesmo custo nos dois caminhos. Isso o
torna caro para o servidor e barato para quem ataca.

Dois *token buckets* em memória, por chaves diferentes:

| Chave | Rajada | Regime |
|---|---|---|
| IP de origem | 10 | 1 tentativa / 6 s |
| e-mail (minúsculo) | 5 | 1 tentativa / 30 s |

O limite por e-mail existe para o ataque distribuído: muitos IPs, cada um
folgadamente abaixo do limite de IP, todos contra a **mesma** conta. Ele vale
também para a senha correta — se cedesse ao acerto, bastaria tentar até
acertar.

Estourou, `429` com `Retry-After` e **a mesma mensagem nos dois casos**:
distinguir "seu IP estourou" de "esta conta estourou" responderia "essa conta
existe" a quem contasse respostas.

O IP vem de `RemoteAddr`, nunca de `X-Forwarded-For` — o cabeçalho é do
cliente e confiar nele daria um balde novo por tentativa. Atrás de um proxy
reverso, todos compartilham um balde; quem puser um proxy na frente precisa
passar a ler o cabeçalho *dele*, com o número de saltos confiáveis fixado.

`POST /api/v1/readings` fica **fora do limite**, de propósito: o nó não tem
buffer, então uma leitura recusada com `429` está perdida para sempre, e o que
se protegeria custa um `SELECT` por índice único, não um bcrypt.

`PATCH` que altera `talhao_id` exige **dupla concessão**: na origem e no
destino. Checar só uma permitiria mover um nó para dentro do próprio alcance
ou para fora do alcance de quem o vigiava. Como a concessão de origem lê o
`talhao_id` antigo, um nó órfão nunca é adotável pela API.

### Zona de atenção

O servidor calcula a zona; a resposta traz também os limiares brutos, porque
o gráfico precisa deles para as linhas de referência e o front **não** deve
re-derivar a regra de sinal.

kPa é negativo e mais negativo é pior, então a comparação é `>=` — o inverso
da intuição "maior é pior", e é onde o erro nasce:

```
kpa >= kpa_alerta    -> conforto
kpa >= kpa_estresse  -> alerta
senão                -> estresse
```

As fronteiras caem na zona *menos* severa: com `-30`/`-60`, exatamente
`-30 kPa` é conforto e exatamente `-60 kPa` é alerta.

Talhão sem faixa configurada devolve `"zona": null`, que é distinto de
conforto — verde com limiar ausente é pior que indicador nenhum.

A regra existe em um único lugar (`Zona()`, em `store_app.go`) e é aplicada
pela camada de dados, não pelo handler: assim um handler novo não tem como
esquecer de classificar.

### Série

Agregada no servidor em janelas de `bucket_s` segundos. `bucket_s` volta na
resposta para o cliente detectar lacuna sem adivinhar o período de
amostragem — bucket sem leitura simplesmente não gera ponto, então o período
offline continua sendo ausência de dado e não linha reta.

A zona do bucket sai de `min(kpa)`, não da média: uma hora que tocou
estresse não pode aparecer como conforto.

Sem `bucket`, o servidor escolhe o menor valor redondo que mantenha a série
abaixo de ~1500 pontos.

### Ordenação

`GET /app/devices` ordena por `kpa ASC` — mais negativo primeiro. "Pior
primeiro" com kPa negativo é `ASC`, não `DESC`; quem lê a frase escreve
`DESC`. Nó sem leitura nenhuma vai para o fim: ausência de dado não é a pior
situação, é uma situação diferente.

### Cookie em demonstração na rede local

`SESSAO_COOKIE_INSEGURO=true` omite o atributo `Secure` do cookie.

Existe para um caso só: abrir a PWA num celular apontando para
`http://192.168.x.x`. Origem de rede local não é *trustworthy* para o
navegador, então o cookie `Secure` é descartado em silêncio e o login parece
simplesmente não funcionar — sem mensagem que aponte a causa.

Sem `Secure` o token da sessão viaja legível em HTTP. Por isso o boot
imprime um banner no stderr **fora do filtro de nível do log**: um aviso que
`slog` com `LevelError` engoliria não é aviso. É a mesma armadilha do
`#warning` do firmware, que o `-w` padrão do arduino-cli descartava — o
build inseguro passava calado.

## Aplicativo (PWA)

O front vive em `App/` e é construído **para dentro deste módulo**:

```sh
cd App && npm run build   # escreve em Backend/internal/webapp/dist/
```

`//go:embed` não aceita caminho com `..`, então o destino do Vite tem que estar
sob o módulo Go — não há etapa de cópia entre `npm run build` e `go build`.
`dist/` é versionado para que compilar não exija Node; em troca, **todo commit
que altere `App/` precisa rodar o build antes**, senão o binário serve a versão
anterior da interface sem acusar erro. `cd App && npm run dist:check` detecta o
caso.

Ver `App/README.md` — inclusive a limitação conhecida do limiar de leitura
velha, hoje fixo em 30 minutos para todos os nós.

### Como o binário serve as duas coisas

`webapp.Servir` monta o handler raiz com dois padrões, e a precedência do
`ServeMux` faz o resto:

| padrão | quem responde |
|---|---|
| `/api/` | a API (`telemetria.Router`) |
| `/` | a PWA embarcada |

**Rota desconhecida sob `/api/` devolve 404 em JSON — nunca `index.html`.**
Não é uma convenção a lembrar: `/api/` é padrão mais específico que `/`, então
a requisição sequer alcança o pacote `webapp`. Ela cai no fundo de poço
registrado no fim do `Router`, que responde `{"error":"rota nao encontrada"}`.

O motivo de tanto cuidado com um 404: o modo de falha oposto é silencioso. Um
endpoint digitado errado que caísse no fallback da SPA responderia **200 com
HTML**, o cliente tentaria fazer parse disso como JSON, e o sintoma — erro de
sintaxe — não apontaria para a causa. Pior, um 200 é sucesso para qualquer
camada acima. O teste está dos dois lados: `internal/webapp/webapp_test.go`
prova que a rota chega na API, `http_test.go` prova o formato da resposta, e
`App/src/api.ts` recusa 200 que não seja JSON mesmo assim.

Rota que não é arquivo (`/no/tensio-01`, `/nos/novo`) recebe `index.html` com
200, porque o roteamento é do lado do cliente. Método diferente de GET/HEAD na
PWA recebe 405: escrita é `/api/`.

Cache: `assets/` tem hash de conteúdo no nome e sai com
`max-age=31536000, immutable`; todo o resto — `index.html`, `sw.js`,
`manifest.webmanifest`, ícones — tem nome fixo e sai com `no-cache`. Cachear
esses seria servir interface velha depois do deploy, e um `sw.js` cacheado se
perpetua sozinho: o service worker desatualizado é quem decide o que buscar
depois, inclusive a própria atualização.

## Conversão de kPa a partir de `raw_mv`

O kPa que o app mostra **é calculado aqui**, pela view `leituras_kpa`
(migration 0003), a partir de `raw_mv` e dos coeficientes da calibração que a
leitura referencia:

```
Vsensor = raw_mv * fator_divisor / 1000
kPa     = (Vsensor - v_zero_kpa) / k_v_por_kpa     -- arredondado a 0,01 kPa
```

`readings.kpa` é o valor que o firmware calculou com as constantes compiladas
nele. Continua gravado, como registro do que o nó reportou, e é o que a API de
dispositivo (`GET /api/v1/devices/{id}/…`) devolve; a lista e a série do app
não o usam. Consequência prática: registrar a calibração pelo app e gravar o id
no portal do nó basta para o kPa exibido refletir o ensaio — sem regravar o
firmware.

**Sem correção ratiométrica.** O XGZP6847A é ratiométrico, e a
`0001_init.sql` descreve uma correção por `vdd_mv / vdd_ensaio_mv`. Ela não é
aplicada: o nó não mede a alimentação, e `vdd_mv` é a constante
`VDD_MV_NOMINAL` do `config.h`. Dividir o VDD medido no ensaio por uma
constante escalaria a série inteira (2,7% com um ensaio a 4870 mV) sem nada no
dado acusar. A calibração vale, portanto, para a alimentação em que o ensaio
foi feito. Quando o nó medir o VDD (segundo divisor no GPIO35), a correção
entra na view, e só nela.

Cada leitura fica presa à calibração que referencia, e calibrações não são
editadas: um ensaio novo vale para as leituras que chegarem com o id novo.
Reprocessar o histórico antigo pelo ensaio novo exigiria reapontar leituras —
operação que não existe, de propósito.

`fator_divisor` mora em `calibrations` e não em `devices`: trocar os
resistores do divisor invalida o ensaio — é calibração nova, não device novo.

**Erro de unidade corrigido na migration 0003.** O nó de bancada tinha sido
cadastrado pelo `admin calibracao` antigo com `4500` e `40` (mV) em vez de
`4.5` e `0.04`. Com o kPa vindo do firmware o erro era invisível; calculado
aqui, a série iria a −112 kPa. A migration corrige as linhas em que os dois
coeficientes estão em mV e acrescenta `CHECK` de faixa
(`0 < v_zero_kpa ≤ 10`, `|k_v_por_kpa| ≤ 1`), para que nenhum caminho de
escrita grave mV de novo. Verificado contra uma cópia do banco de bancada: nas
2401 leituras, o kPa da view difere do reportado pelo nó em no máximo
0,01 kPa — o histórico aparece igual na tela.

## Autenticação

`token_hash = HMAC-SHA256(token, TELEMETRIA_TOKEN_SECRET)`, em hex, com
índice único. O token em claro nunca é persistido.

Senha de usuário é bcrypt com custo 12, não HMAC — a inversão é
deliberada. Senha humana tem entropia baixa e *há* dicionário a atacar, e o
custo lento é aceitável porque só o login a verifica. Token de dispositivo é
o oposto: 256 bits aleatórios, verificados a cada requisição de ingestão. O
argumento que justifica cada um invalida o outro.

O custo fica gravado no próprio hash, então subir o custo não invalida os
hashes antigos — eles continuam autenticando com o custo com que nasceram.

HMAC e não SHA-256 puro: o segredo do servidor impede casar hashes offline
caso o banco vaze e a entropia real dos tokens seja menor que a nominal.
E não bcrypt/argon2: o token é 256 bits aleatórios, não senha humana — não
há dicionário a atacar, e um KDF lento obrigaria varrer a tabela a cada
requisição, inviabilizando o lookup indexado do endpoint de ingestão.
