# Tensiometria eletrônica — monitoramento de tensão de água no solo

Protótipo de irrigação assistida por sensor: um nó ESP32 lê um tensiômetro
no solo, envia leituras autenticadas para um servidor Go, e um PWA mostra ao
produtor quando irrigar.

TCC de Bacharelado em Ciência da Computação, IFC Campus Rio do Sul.

```
   solo                                                    produtor
    │                                                          │
 [tensiômetro]                                              [celular]
    │  0–5 V                                                   │
 [ESP32] ──── Wi-Fi ────► [API Go :8080] ────► [Postgres]      │
    │        HTTP/Bearer        │                              │
    │                           └── serve o PWA em / ──────────┘
    │                                (embed.FS)
    └─ portal cativo para configuração em campo (SoftAP + NVS)
```

O nó não guarda configuração no código: rede, URL, identidade, token e
calibração são gravados na NVS pelo celular de quem instala, através de um
portal cativo. Uma placa pré-gravada é configurada no talhão sem notebook e
sem recompilar.

## Os três subsistemas

| Pasta | O que é | Leia |
|---|---|---|
| `Circuito/` | Firmware do nó ESP32 (Arduino/C++), portal cativo, calibração | [`Circuito/README.md`](Circuito/README.md) |
| `Backend/` | API de ingestão e consulta, Go + Postgres, serve o PWA | [`Backend/README.md`](Backend/README.md) |
| `App/` | PWA de monitoramento, React + TypeScript | [`App/README.md`](App/README.md) |

E o que não é código:

| Pasta | O que é |
|---|---|
| `Escrita/` | O texto do TCC (`.docx` e PDF). É o artefato avaliado. |
| `Registros/` | Registros de decisão datados — por que as coisas são como são |

## Começar

Ordem obrigatória: o banco precede a API, e a API precede o nó — um nó sem
destino perde as leituras, porque **não há buffer nem reenvio**.

### 1. Backend

```sh
cd Backend
cp .env.example .env     # preencha TELEMETRIA_TOKEN_SECRET
make dev                 # Postgres + migrations + API em :8080
```

Sobe o Postgres via Podman (`ENGINE=docker` roda idêntico). Sob Podman, uma
vez por máquina: `systemctl --user enable --now podman.socket`.

### 2. Cadastrar o nó e a calibração

```sh
cd Backend
go run ./cmd/admin talhao      -nome="Talhão Norte"
go run ./cmd/admin device      -id=tensio-01 -descricao="nó do talhão norte"
go run ./cmd/admin associar    -device=tensio-01 -talhao=<uuid>
go run ./cmd/admin calibracao  -id=cal-2026-09-25-a -device=tensio-01 ...
```

O token do dispositivo aparece **uma única vez**. O banco guarda só o hash;
não há como recuperá-lo. E **sem uma linha em `calibrations` toda leitura
volta rejeitada** — o POST é aceito e o dado não entra.

### 3. Gravar e provisionar o nó

```sh
cd Circuito
cp sketch/config.h.example sketch/config.h    # não precisa editar
arduino-cli compile --fqbn esp32:esp32:esp32doit-devkit-v1 sketch
arduino-cli upload   --fqbn esp32:esp32:esp32doit-devkit-v1 -p <porta> sketch
```

A porta varia com o conversor USB da placa: `/dev/ttyUSB0` (CP210x, CH340)
ou `/dev/ttyACM0` (CH9102, que enumera como CDC).

Ligue a placa: sem configuração na NVS ela abre o portal sozinha. No celular,
conecte-se a `tensio-setup-XXXX` (senha `tensiometro`), preencha e salve.

Três coisas que fazem perder tempo na primeira vez, detalhadas em
[`Circuito/README.md`](Circuito/README.md): a URL é o endereço **inteiro**
com `/api/v1/readings`; em build `DEV_SEM_TLS` o esquema é `http://`; e o
gesto do BOOT é **sequencial** — solte o RESET e só depois segure o BOOT.

### 4. App

```sh
cd App && npm install && npm run build
```

O build vai para `Backend/internal/webapp/dist/`, de onde o binário Go o
serve. Em produção não há servidor de front separado.

## Estado

O provisionamento por portal cativo foi validado em hardware real em
16/09/2026, ponta a ponta e sem recompilar: portal → NVS → telemetria →
Wi-Fi → NTP → POST autenticado → linha no Postgres.

O ensaio final com tensiômetro físico está **pendente** — o instrumento
quebrou e o substituto não chegou. As leituras gravadas até aqui são de
bancada, com o tubo aberto para a atmosfera.

Limitações conhecidas e declaradas como tal no texto, não omissões: sem
buffer de leituras (20% de perda medidos em sinal fraco), sem deep sleep,
coeficientes de catálogo à espera da calibração experimental, e a API não
expõe o período de amostragem do nó. Cada README detalha as suas.
