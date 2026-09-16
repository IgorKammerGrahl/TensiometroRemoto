# Desenho — autorização de usuário e faixas de atenção — 24/08/2026

Projeto: Protótipo de tensiometria eletrônica IoT (TCC — IFC Rio do Sul)
Autor: Igor Kammer Grahl

Etapa: camada de apresentação, item 1 de 4 (desenho; sem implementação).
Cobre: RF08, RF09, RF10, UC02, UC03, UC04, UC05, RNF05, RNF08.

---

## 1. Resumo

O backend autentica **dispositivos**, não pessoas. Um token de nó lê apenas a
própria série — decisão correta para ingestão e insuficiente para a interface,
porque UC02 pressupõe um usuário que enxerga vários nós.

Este documento especifica (a) o plano de autenticação de usuário e o modelo de
autorização, (b) o schema das faixas de atenção e a regra de classificação, e
(c) o contrato dos endpoints que o aplicativo consome.

Nada aqui altera o comportamento existente: a migration é aditiva e as três
rotas atuais permanecem intactas.

**Correção de premissa registrada:** o índice `readings_device_measured_idx
(device_id, measured_at DESC)` já existe e `ListarLeituras` já filtra e ordena
por `measured_at`. A confusão `measured_at` × `received_at` não é risco de
backend — é risco de front e de seed.

---

## 2. Decisões tomadas

| # | Decisão | Alternativa descartada |
|---|---------|------------------------|
| D1 | Sessão de usuário via token opaco em cookie | JWT |
| D2 | Concessão por talhão (`usuario_talhoes`) | Tabela `propriedades` |
| D3 | Autorização como cláusula `WHERE`, nunca `if` | Checagem por handler |
| D4 | Zero linhas → 404 | 403 |
| D5 | Faixas como colunas em `talhoes` | Tabela `faixas` separada |
| D6 | Dois limiares; três zonas derivadas | Três faixas com min/max |
| D7 | Classificação de zona no servidor | Derivar no cliente |
| D8 | PWA React + TS servida por `embed.FS` | React Native / Expo |
| D9 | Agregação por bucket no servidor | `LIMIT` cru |
| D10 | Renovação deslizante, no mesmo `UPDATE` que valida | Expiração fixa |
| D11 | `cmd/admin` com subcomandos | `cmd/usuario` + `cmd/talhao` separados |
| D12 | Proxy do Vite para `/api` em desenvolvimento | CORS + `SameSite=None` em dev |

---

## 3. Plano de autenticação de usuário

### 3.1 Dois planos, separados por namespace de rota

| Plano | Credencial | Alcance |
|-------|-----------|---------|
| Dispositivo | Bearer token | `POST /api/v1/readings` + os dois GET atuais |
| Usuário | Cookie de sessão | `/api/v1/app/*` |

A separação é estrutural, por prefixo de rota, e não uma checagem replicada
dentro de cada handler. "Token de dispositivo não abre a tela do produtor"
passa a ser propriedade do roteamento, não disciplina de quem escreve o
próximo handler.

### 3.2 Mecanismo de sessão — token opaco (D1)

Reusa `HashToken` (HMAC-SHA256 + índice único, lookup O(1)), já escrito e
testado para tokens de dispositivo.

Motivos, em ordem de peso:

1. **Revogação imediata.** Logout e desativação de usuário valem no mesmo
   instante. Um JWT só ganharia isso com blocklist — que é uma tabela de
   sessão com outro nome, somada à complexidade do JWT.
2. Sem biblioteca nova, sem rotação de chave de assinatura, sem tolerância a
   clock skew.
3. A única vantagem real do JWT é escala horizontal sem estado compartilhado.
   Não existe nesse cenário.

### Expiração e renovação (D10)

Janela de 30 dias com **renovação deslizante**: prazo fixo desloga o usuário no
meio do uso, sem aviso, e o lugar onde isso acontece é numa demonstração.

Validação e renovação são a mesma instrução:

```sql
UPDATE sessoes
   SET expira_em = now() + $2::interval
 WHERE token_hash = $1 AND expira_em > now()
RETURNING usuario_id;
```

Uma ida ao banco, atômica, e zero linhas cobre os dois casos de falha —
token inexistente e token expirado — de forma indistinguível para o cliente.
Não dá para descobrir que um token *já existiu*.

Custo: grava uma tupla por requisição autenticada (MVCC; o `UPDATE` escreve
mesmo quando o valor não muda). Aceitável no protótipo. Se virar gargalo,
renovar só na metade final da janela, com `SELECT` seguido de `UPDATE`
condicional — duas idas ao banco no caso raro, uma no comum. O comentário no
código nomeia esse teto.

Limpeza de linhas expiradas: `DELETE FROM sessoes WHERE expira_em < now()` no
login, oportunista. Sem cron — o caminho de autenticação nunca mais toca numa
linha expirada, então elas só acumulam espaço.

### 3.3 Senha — bcrypt, pelo motivo inverso do token de dispositivo

`http.go:44-50` já documenta por que o token de dispositivo **não** usa KDF:
são 256 bits aleatórios, e um KDF lento obrigaria varrer a tabela a cada
requisição em vez de usar o índice.

Senha humana é o caso oposto — entropia baixa, lookup por e-mail — e exige
KDF lento. bcrypt, cost 12.

Duas credenciais, duas estratégias de verificação, cada uma justificada pela
razão que invalida a outra:

| | Token de dispositivo | Senha de usuário |
|---|---|---|
| Entropia | Alta (256 bits aleatórios) | Baixa (escolhida por humano) |
| Volume de verificações | Alto (toda ingestão) | Baixo (só no login) |
| Ataque relevante | Vazamento do banco + casamento offline | Força bruta sobre o hash |
| Consequência | HMAC-SHA256 com segredo do servidor | KDF lento (bcrypt, cost 12) |
| Por que não o outro | KDF obrigaria varrer a tabela por requisição | HMAC rápido é força-brutável |

**Requisito de implementação:** o comentário que acompanha `senha_hash` deve
desenvolver esse contraste em prosa completa, no mesmo registro do comentário
de `HashToken`, para ser reaproveitado como base do texto do trabalho.

Rate limit explícito no login fica de fora: bcrypt cost 12 (~250 ms) já limita
a ordem de 4 tentativas por segundo por conexão. Adicionar quando houver
exposição pública real.

### 3.4 Schema (migration `0002_usuarios.sql`, aditiva)

```sql
-- +goose Up

CREATE TABLE usuarios (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email      TEXT NOT NULL,
  senha_hash TEXT NOT NULL,              -- bcrypt, cost 12
  nome       TEXT NOT NULL,
  ativo      BOOLEAN NOT NULL DEFAULT true,
  criado_em  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- lower(email): o produtor nao deve descobrir que "Joao@" e "joao@" sao
-- contas diferentes.
CREATE UNIQUE INDEX usuarios_email_key ON usuarios (lower(email));

-- A concessao. O device herda o acesso do talhao onde esta instalado; nao
-- existe concessao direta usuario -> device.
CREATE TABLE usuario_talhoes (
  usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
  talhao_id  UUID NOT NULL REFERENCES talhoes(id)  ON DELETE CASCADE,
  PRIMARY KEY (usuario_id, talhao_id)
);

CREATE TABLE sessoes (
  token_hash TEXT PRIMARY KEY,           -- HMAC-SHA256(token, segredo)
  usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
  criada_em  TIMESTAMPTZ NOT NULL DEFAULT now(),
  expira_em  TIMESTAMPTZ NOT NULL
);

CREATE INDEX sessoes_usuario_idx ON sessoes (usuario_id);
```

**Talhão e não propriedade (D2).** `talhoes` já existe e `devices.talhao_id` já
aponta para lá. Uma tabela `propriedades` acrescentaria um nível de hierarquia
que nenhum requisito pede — o conjunto de concessões de um usuário já *é* a
propriedade dele, na forma degenerada. Um produtor com doze talhões tem doze
linhas. Se `propriedades` fizer falta, é migration, não redesenho.

### 3.5 Onde a autorização é aplicada — núcleo do desenho (D3, D4)

**Regra: autorização é cláusula `WHERE`, nunca `if`.**

O `autorizarDevice` atual (`http.go:402`) é um `if` e está correto, porque o
token *é* o device: uma comparação de igualdade, um caminho de execução. Com
usuário, um `if` por handler é precisamente onde nasce IDOR — o handler novo
não replica a checagem e devolve a série de outro produtor.

#### Leitura

```sql
FROM readings r
JOIN devices d          ON d.id = r.device_id
JOIN usuario_talhoes ut ON ut.talhao_id = d.talhao_id
                       AND ut.usuario_id = $1
WHERE r.device_id = $2 AND r.measured_at BETWEEN $3 AND $4
```

#### Escrita — onde a regra costuma ser esquecida

O mesmo vale para RF10. O `talhao_id` de destino chega no corpo da requisição:
é dado não confiável e não pode ser gravado por confiança.

```sql
-- POST /devices: so insere se o talhao de destino for concedido.
INSERT INTO devices (id, descricao, talhao_id, token_hash)
SELECT $1, $2, $3, $4
  FROM usuario_talhoes
 WHERE usuario_id = $5 AND talhao_id = $3;
```

`PATCH /devices/{id}` que muda `talhao_id` exige **duas** concessões: o talhão
atual (para poder mexer no nó) e o talhão de destino (para poder colocá-lo lá).
Verificar só uma das duas permite mover um nó para fora ou para dentro do
alcance indevidamente.

Zero linhas afetadas → 404, em leitura e em escrita.

#### Consequências deliberadas

- **`ListarLeituras(ctx, usuarioID, deviceID, ...)`** — parâmetro obrigatório,
  sem variante sem usuário. Esquecer vira erro de compilação, não vazamento
  silencioso. É o motivo de mudar a assinatura em vez de adicionar uma função
  paralela.
- **Zero linhas → 404, não 403 (D4).** 403 confirma que o device existe. 404
  não distingue "não existe" de "não é seu", e varrer IDs não rende informação.
- **`devices.talhao_id IS NULL` → invisível para todos.** Fail-closed: o nó de
  bancada não vaza para nenhum produtor. A coluna permanece nullable (nó ainda
  não instalado é estado legítimo); quem exige talhão é o formulário do RF10.

### 3.6 Ordem de verificação no middleware

Explícita, e nenhum dos passos 1–3 é influenciado por dado que o cliente
controla (path, query, corpo):

1. Cookie presente → senão **401**
2. `UPDATE ... WHERE token_hash = $1 AND expira_em > now() RETURNING usuario_id`
   → zero linhas: **401**
3. `usuarios.ativo` → senão **403**
4. Só então o handler executa, com `usuario_id` no contexto

O passo 2 valida e renova de uma vez (D10). Existência e validade do token
colapsam num único resultado, então "token nunca existiu" e "token expirou"
não são distinguíveis pela resposta.

O 403 do passo 3 segue a mesma lógica já usada para device inativo: a
credencial é válida, a conta é que foi desativada.

### 3.7 Sessão no cliente (D8)

A PWA compilada é servida pelo próprio binário Go via `embed.FS`. Mesma
origem, e portanto:

```
Set-Cookie: sessao=<token>; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=2592000
```

- `HttpOnly` — XSS não lê o token.
- `SameSite=Lax` — bloqueia CSRF de POST cross-site sem tabela nem token de
  CSRF.
- Mesma origem — sem CORS, sem preflight, sem `SameSite=None`.

Bearer em `localStorage` trocaria CSRF por XSS permanente **e** adicionaria
configuração de CORS: pior nos dois eixos.

### 3.8 Provisionamento — `cmd/admin` (D11)

Um binário com subcomandos, em vez de três binários:

```
admin usuario  -email= -nome=            # cria usuario, exibe a senha uma vez
admin talhao   -nome=                    # cria talhao, imprime o UUID
admin conceder -email= -talhao=          # linha em usuario_talhoes
admin associar -device= -talhao=         # adota um no orfao
```

`cmd/devtoken` permanece separado e inalterado: já está documentado no README
do firmware, e quebrar um fluxo documentado custa mais do que a assimetria.

#### O nó órfão só pode ser adotado fora do aplicativo

Consequência direta do fail-closed de 3.5, e não é acidental:

Um device com `talhao_id IS NULL` é invisível para todo usuário. Logo, nenhum
usuário pode executar `PATCH /devices/{id}` sobre ele — a query devolve zero
linhas e o handler responde 404. **O aplicativo não consegue adotar um nó
órfão, por construção.**

A alternativa — permitir `PATCH` quando o talhão atual é `NULL`, desde que o
destino seja concedido — deixaria qualquer usuário reivindicar qualquer nó não
associado. Rejeitada.

Por isso `admin associar` existe: a adoção é operação fora de banda,
deliberadamente. É o caminho para `tensio-01`, o nó de bancada da etapa 1,
entrar no aplicativo.

**Nota obrigatória no README**, no formato sintoma-antes-da-causa já usado na
nota de DHCP do firmware:

> *O aplicativo abre, autentica, e a lista de nós vem vazia. Nenhum erro.*
> Um device sem `talhao_id` é invisível para todos os usuários, por desenho: o
> acesso é concedido por talhão e um nó sem talhão não está no alcance de
> ninguém. Não é falha de autorização — é a autorização funcionando. Resolva
> com `admin associar -device=tensio-01 -talhao=<uuid>`.

Fora do escopo, porque nenhum RF pede: tela de cadastro, verificação de e-mail,
recuperação de senha, papéis e permissões (toda concessão é leitura + escrita).

### 3.9 Cookie em desenvolvimento (D12)

Em produção o binário serve a PWA e tudo é mesma origem. Em desenvolvimento
são duas portas — Vite e servidor Go — logo duas origens, e o cookie
`SameSite=Lax` não acompanha a requisição.

```ts
// vite.config.ts
server: { proxy: { '/api': 'http://localhost:8080' } }
```

O navegador enxerga uma origem só nos dois ambientes. Sem isso a autenticação
parece quebrada sem sintoma que aponte a causa — e a correção é uma linha.

---

## 4. Faixas de atenção (RF08, RF09, UC04, UC05)

### 4.1 O sinal, resolvido por construção (D6)

Tensão de água no solo é negativa e mais negativa significa mais seco: −60 kPa
é mais crítico que −20 kPa. É o inverso da intuição "valor maior é pior", e é
onde a comparação vai ser escrita errada na primeira tentativa.

Modelar três faixas com `min`/`max` abriria buraco e sobreposição e exigiria
código de validação. Em vez disso, **dois limiares**; as três zonas são
derivadas:

```
kpa >= kpa_alerta    → conforto
kpa >= kpa_estresse  → alerta
senão                → estresse
```

A comparação é `>=`. Buraco e sobreposição tornam-se impossíveis por
construção, e não por checagem.

### 4.2 Schema (D5)

```sql
ALTER TABLE talhoes
  ADD COLUMN cultura      TEXT,
  ADD COLUMN kpa_alerta   REAL,
  ADD COLUMN kpa_estresse REAL,
  -- Ambos nulos (nao configurado) ou ambos definidos. kpa_estresse e o mais
  -- negativo. O teto de -80 e o mesmo limite de cavitacao do tensiometro:
  -- limiar alem disso e limiar que nunca dispara com leitura valida.
  ADD CONSTRAINT talhoes_faixas_ck CHECK (
    (kpa_alerta IS NULL AND kpa_estresse IS NULL)
    OR (kpa_alerta IS NOT NULL AND kpa_estresse IS NOT NULL
        AND kpa_estresse < kpa_alerta
        AND kpa_alerta   BETWEEN -80 AND 0
        AND kpa_estresse BETWEEN -80 AND 0)
  );
```

O `CHECK` segue a justificativa já registrada em `0001`: a aplicação valida
para devolver o motivo ao usuário; o banco valida para que nenhum outro
caminho de escrita (carga manual, seed, CSV) contorne a regra física.

**Faixa é 1:1 com talhão**, então uma tabela separada teria exatamente uma
linha por talhão. UC04 vira `PATCH /api/v1/app/talhoes/{id}` — sem join e sem
CRUD de coleção. Se histórico por fase fenológica passar a ser requisito, a
tabela separada com `vigente_de` entra por migration.

`cultura` é rótulo de texto no talhão, não entidade: nenhum requisito tem
ciclo de vida de cultura.

### 4.3 Quem classifica: o servidor (D7)

A API devolve `zona` já calculada **e** os limiares brutos.

- Limiares brutos: o gráfico precisa deles para desenhar as linhas de
  referência (RF08).
- `zona` pronta: o cliente não re-deriva a regra de sinal.

O critério não é "lógica pertence ao servidor" — é que **esta** regra tem uma
inversão de sinal que se escreve errado com facilidade, e duplicá-la em
TypeScript duplica a chance de inverter o `>=`. Por contraste, o limiar de
"leitura velha" (4.5) fica no cliente: é um limiar de apresentação, sem
armadilha de sinal.

```sql
CASE
  WHEN t.kpa_alerta IS NULL    THEN NULL
  WHEN r.kpa >= t.kpa_alerta   THEN 'conforto'
  WHEN r.kpa >= t.kpa_estresse THEN 'alerta'
  ELSE                              'estresse'
END AS zona
```

Uma expressão no SQL, uma função em Go para o resto. Fonte única.

### 4.4 Ordenação e eixo

- **Lista (UC05), pior primeiro:** `ORDER BY kpa ASC` — mais negativo primeiro.
  Coincide com o natural, mas merece comentário no código: quem lê "pior
  primeiro" escreve `DESC`.
- **Gráfico:** kPa negativo em eixo normal. Linha descendo = secando =
  piorando. Lê certo sem treinamento, que é o que o RNF05 pede.

### 4.5 Os dois estados que a interface separa de "conforto"

- **Talhão sem faixa configurada** → `zona: null` → "não configurado". Não é
  verde.
- **Última leitura antiga (nó offline)** → o payload carrega `medido_ha_s`; a
  interface mostra "sem dados há 6 h", não indicador verde.

Indicador verde com dado de ontem é pior que indicador nenhum: o produtor
decide irrigação com base num número morto. É o fail-closed da camada de
apresentação e conecta com as lacunas do seed sintético (etapa 3).

---

## 5. Contrato dos endpoints novos

Todos sob `/api/v1/app/`, todos exigindo cookie de sessão.

```
POST   /login                     {email, senha}        → 204 + Set-Cookie
POST   /logout                                          → 204 + cookie expirado
GET    /me                                              → usuário + talhões concedidos

GET    /devices                                         → UC02: lista + última leitura + zona
GET    /devices/{id}                                    → UC02: detalhe + faixa do talhão
GET    /devices/{id}/series?from=&to=&bucket=           → UC03: série agregada
POST   /devices                   {id, descricao, talhao_id}          → RF10: cadastro
PATCH  /devices/{id}              {descricao?, talhao_id?, ativo?}    → RF10: editar/desativar

GET    /talhoes                                         → talhões concedidos
PATCH  /talhoes/{id}              {cultura?, kpa_alerta?, kpa_estresse?}  → UC04
```

### 5.1 `POST /devices` e o token do nó

O cadastro gera o token do dispositivo e o devolve **em claro uma única vez**,
como já faz `cmd/devtoken`. Não há endpoint que recupere um token existente:
`devices.token_hash` é irreversível por construção. Perdeu, rotaciona.

### 5.2 `GET /devices/{id}/series` — agregação por bucket (D9)

`ORDER BY measured_at DESC LIMIT 100` sobre um intervalo de meses devolve as
100 leituras mais recentes, não uma amostra do intervalo. A tela de Histórico
plotaria um dia rotulado como seis meses: gráfico mentiroso, não gráfico
incompleto.

```sql
SELECT to_timestamp(floor(extract(epoch FROM r.measured_at) / $bucket) * $bucket) AS t,
       avg(r.kpa) AS kpa_med,
       min(r.kpa) AS kpa_min,      -- pior caso: o mais negativo
       max(r.kpa) AS kpa_max,
       count(*)   AS n
  FROM readings r
  JOIN devices d          ON d.id = r.device_id
  JOIN usuario_talhoes ut ON ut.talhao_id = d.talhao_id AND ut.usuario_id = $1
 WHERE r.device_id = $2 AND r.measured_at BETWEEN $3 AND $4
 GROUP BY 1
 ORDER BY 1;
```

Propriedades que essa forma garante:

- **Lacunas sobrevivem à agregação.** Bucket sem leitura simplesmente não gera
  linha. O período offline continua sendo ausência de dado, não interpolação.
- **`bucket_s` volta na resposta.** A interface detecta lacuna por
  `Δt > 1.5 × bucket_s` — uma regra, um número, vindo do servidor. Sem isso o
  cliente teria de adivinhar o período de amostragem.
- **Zona de um bucket usa `kpa_min`**, o pior caso. Uma hora que tocou estresse
  não deve aparecer como conforto por causa da média.
- **`bucket` automático** quando o parâmetro é omitido: escolhido para manter a
  contagem de pontos na casa de 500–1500 e arredondado para valor redondo
  (60, 300, 900, 3600, 10800, 21600, 86400 s), para que os rótulos do eixo
  caiam em horários redondos.

---

## 6. Impacto no que já existe

| Arquivo | Mudança |
|---------|---------|
| `migrations/0002_usuarios.sql` | Novo, aditivo. `0001` não muda. |
| `internal/telemetria/http.go` | Inalterado. Já tem 13 KB. |
| `internal/telemetria/http_app.go` | Novo. Separação de arquivo espelha a separação dos dois planos de auth. |
| `internal/telemetria/store.go` | Assinaturas de leitura ganham `usuarioID`. É o que torna o esquecimento erro de compilação. |
| `cmd/api/main.go` | Monta `embed.FS` da PWA e registra o router de app. |
| `cmd/admin/main.go` | Novo. Subcomandos `usuario`/`talhao`/`conceder`/`associar`. |
| `cmd/devtoken/main.go` | Inalterado. Fluxo já documentado no README do firmware. |
| `App/` | Novo: PWA React + TypeScript. `vite.config.ts` proxia `/api` (3.9). |
| `Backend/README.md` | Nota do nó órfão, formato sintoma-antes-da-causa (3.8). |

---

## 7. Stack da interface (D8)

**PWA responsiva, React + TypeScript.**

1. RNF08 pede Android, iOS e navegadores. Expo só entrega navegador via
   `react-native-web` — mais infraestrutura, não menos. A PWA cobre os três
   alvos com um build.
2. Distribuir aplicativo nativo em iOS exige conta paga Apple e um Mac.
   Bloqueio de cronograma num semestre final, sem contrapartida técnica.
3. Banca: abrir no celular do avaliador na hora vale mais que instalar um APK.
4. Nenhuma tela precisa de API nativa — sem BLE, sem câmera, sem localização em
   segundo plano. É gráfico e formulário. O único ganho real do Expo seria push
   para o RF09, fora do escopo, e web push cobre Android e iOS 16.4+.
5. Custo de errar é baixo: componentes React portam para RN com esforço, e a
   API não muda.

Consequência prática: com `embed.FS`, o trabalho entrega **um binário**.
Resolve o cookie de mesma origem (3.7) e o deploy da banca de uma vez.

---

## 8. Verificações que a implementação precisa deixar para trás

Uma checagem executável por risco, não uma suíte por função.

| Risco | Verificação |
|-------|-------------|
| Planos de auth se misturam | Token de dispositivo em rota `/app/*` → 401. Cookie de sessão em `POST /readings` → 401. |
| IDOR em leitura | Usuário A pede device do talhão de B → 404 (não 403, não 200). |
| IDOR em escrita | `POST /devices` com `talhao_id` não concedido → 404, e nada inserido. |
| Nó órfão vaza | Device com `talhao_id IS NULL` não aparece para nenhum usuário. |
| Sinal invertido | Limiares (−30, −60): −10 → conforto; −40 → alerta; −70 → estresse. Casos de fronteira exatos: −30 → conforto, −60 → alerta. |
| Lacuna virou linha reta | Série com buraco de 12 h retorna ausência de bucket, não bucket interpolado. |
| Histórico truncado | Intervalo de 6 meses devolve pontos distribuídos no intervalo, não só o último dia. |
| Sessão morre no meio do uso | Requisição autenticada estende `expira_em`; sessão usada além da janela original continua válida. |
| Sessão expirada revive | Requisição com `expira_em` no passado → 401, e o `UPDATE` não renova. |
| Nó órfão adotável pelo app | `PATCH /devices/{id}` sobre device com `talhao_id IS NULL` → 404, para qualquer usuário. |

As fronteiras exatas (−30 e −60 caindo na zona *menos* severa) são
consequência do `>=` e precisam estar no teste, porque é o caractere que muda
se alguém "corrigir" a comparação.

---

## 9. Deliberadamente fora de escopo

Registrado para que a ausência seja lida como decisão, não como esquecimento:

- Tabela `propriedades` — a concessão já cumpre o papel (3.4).
- Papéis e permissões — toda concessão é leitura + escrita.
- Cadastro self-service, verificação de e-mail, recuperação de senha.
- Refresh token — a renovação deslizante (3.2) cobre o caso.
- Rate limit explícito de login — bcrypt já limita (3.3).
- Histórico de faixas por fase fenológica (4.2).
- Notificação push para o RF09 — a sinalização é visual, na interface.

---

## 10. Estado ao final

Desenho concluído e aguardando revisão. Nenhuma linha de código escrita.

Próximas etapas, na ordem acordada: (2) seed de dados sintéticos, (3) endpoints
faltantes, (4) interface.
