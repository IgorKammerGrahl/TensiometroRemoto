# Nó sensor — firmware

Firmware do nó de tensiometria eletrônica: mede a tensão de água no solo com
um transdutor XGZP6847A100KPGN, converte para kPa e envia ao backend em Go
(`../Backend`).

Esta é a **etapa 1**: POST direto, uma leitura por ciclo, sem buffer. Falha de
rede perde a leitura. Buffer em LittleFS e deep sleep vêm na etapa 2.

## Hardware

| Item | Valor |
|---|---|
| Placa | ESP32 DOIT DevKit V1 (`esp32:esp32:esp32doit-devkit-v1`) |
| Sensor | XGZP6847A100KPGN, −100 a 0 kPa, alimentação 5 V, saída 0,5 a 4,5 V |
| Entrada analógica | GPIO34 (ADC1 — o ADC2 fica inutilizável com o Wi-Fi ativo) |
| Condicionamento | Divisor resistivo 10k / 20k, mais capacitor de 100 nF |

O divisor é necessário porque a saída do sensor chega a 4,5 V e a entrada do
ESP32 tolera 3,3 V:

```
OUT do sensor --[ R1 = 10k ]--+-- GPIO34
                              |
                           [ R2 = 10k ]
                              |
                           [ R3 = 10k ]
                              |
                             GND

Vpino = Vsensor × 20k/30k = Vsensor × (2/3)
Vsensor = Vpino × 1,5          ← FATOR_DIVISOR no sketch
```

Trocar esses resistores invalida a calibração de bancada. O `fator_divisor`
fica registrado na tabela `calibrations` do backend justamente por isso: um
divisor diferente é uma calibração nova, não um dispositivo novo.

## Arquivos

```
Circuito/
├── sketch/
│   ├── sketch.ino          medição, os dois modos, Wi-Fi → NTP → POST
│   ├── payload.h           montagem do JSON e leitura da resposta (C puro)
│   ├── prov.h              portal cativo de configuração (AP, HTML, rotas)
│   ├── prov_nvs.h          leitura e gravação da configuração na NVS
│   ├── prov_valida.h       validação dos campos do formulário (C puro)
│   ├── config.h.example    modelo versionado, com placeholders
│   └── config.h            física, coeficientes, intervalos, CA raiz e senha do AP — gitignored
└── teste/
    ├── teste_payload.c     teste do payload no host, sem hardware
    └── teste_prov.c        teste da validação e do escape de HTML, sem hardware
```

`config.h` está no `.gitignore` da raiz do repositório: é o arquivo que cada
instalação edita localmente (fica só `PROV_AP_SENHA` como valor sensível de
fato — o resto é física do circuito, coeficientes, intervalos e a CA raiz).
O `config.h.example` é o que se versiona.

`teste_payload.c` fica **fora** de `sketch/` de propósito: o arduino-cli
compila todo arquivo dentro da pasta do sketch, e um `main()` de host ali
dentro quebraria o build do firmware.

## Provisionamento

```bash
cp sketch/config.h.example sketch/config.h
```

Sem editar nada nesse arquivo já dá um firmware válido: rede, URL da API,
identidade, token e calibração não moram mais em `config.h` — são gravados na
NVS do próprio ESP32, pelo portal cativo, pelo celular de quem instala.
`config.h` guarda só o que não muda de instalação para instalação: física do
circuito, coeficientes, intervalos e a CA raiz.

### Fluxo em campo

1. No aplicativo, cadastre o nó. O token aparece **uma única vez** na tela —
   copie antes de sair dela. O banco guarda apenas o hash; não há como
   recuperá-lo depois.
2. Ligue o ESP32. Sem configuração gravada, ele sobe sozinho em modo
   provisionamento.
3. No celular, conecte-se à rede `tensio-setup-XXXX` (o sufixo vem dos
   últimos bytes do MAC do próprio nó) com a senha `tensiometro`. Android e
   iOS costumam abrir a página de configuração sozinhos, por ser um portal
   cativo de verdade — qualquer host resolve para o nó.
4. Preencha a rede Wi-Fi de destino, a URL da API, o identificador do nó, o
   token e a calibração.
5. Opcionalmente, toque em "Testar esta rede" antes de salvar (ver aviso de
   troca de canal abaixo).
6. Toque em "Salvar e reiniciar". O nó grava na NVS e reinicia direto em modo
   telemetria; a rede `tensio-setup-XXXX` desaparece.

### Pegadinhas que a bancada de 16/09/2026 produziu

Nenhuma delas é hipótese: todas custaram tempo no primeiro provisionamento
real. Estão aqui porque as três falham de um jeito que não se parece com a
causa.

**A URL é o endereço inteiro, não só o esquema.** `http://192.168.0.10:8080`
sem o `/api/v1/readings` não dá erro no portal — o firmware valida apenas o
prefixo, e o resto vai literal para o `http.begin()`. O sintoma aparece
depois, como `404` em toda leitura.

**Num build `DEV_SEM_TLS`, o esquema é `http://`.** Esse build usa
`WiFiClient` puro; ele não tem como falar TLS. Uma URL `https://` é aceita
pela validação e falha em campo como `-11 (read Timeout)`, que parece
problema de rede. O placeholder do campo acompanha o build justamente para
não induzir ao erro, mas quem digita por cima ainda consegue errar.

**O gesto do BOOT é sequencial, nunca simultâneo.** Solte o RESET e só
*depois* segure o BOOT por ~3 s. GPIO0 é pino de strapping: com ele
pressionado no instante do reset, o chip entra em modo de gravação serial e o
firmware nem roda — não há portal, não há telemetria, não há mensagem
nenhuma. A janela de amostragem dura 1,5 s e trava no primeiro nível baixo,
então segurar demais é inofensivo e soltar cedo não estraga.

**Servidor com duas interfaces na mesma sub-rede derruba tudo em
intermitência.** Se a máquina que roda a API tem cabo e Wi-Fi ativos no mesmo
`/24`, ela tem dois IPs. O `rp_filter` do Linux descarta em silêncio os
pacotes que chegam pela interface que não é a da rota de volta — e como
qualquer uma das duas pode responder o ARP, o nó acerta ou erra conforme o
cache, alternando sem padrão. Diagnóstico: `nstat -az | grep ReversePath`
crescendo. Correção: desligar uma das interfaces antes do ensaio.

### Quando o portal sobe

Só em dois casos, e não há um terceiro:

- a NVS não tem uma configuração **completa** — algum campo obrigatório ficou
  vazio, ou a última gravação não chegou ao fim. A NVS não tem transação, e
  cada campo é uma escrita independente; por isso o firmware grava uma marca
  de conclusão por último e só confia na configuração quando ela está lá. Não
  existe "meio configurado": uma gravação interrompida manda o nó de volta ao
  portal, em vez de subir com o token de uma identidade e o `device_id` de
  outra — que não falha no boot, falha com 401 em campo; ou
- alguém aperta e solta o **RESET** e, logo em seguida, aperta e segura o
  **BOOT** por cerca de dois segundos, até o portal subir.

Segurar o BOOT **junto** com o RESET não serve — e é justamente o gesto que a
mão faz por hábito. O GPIO0 é pino de *strapping* do ESP32: com ele
pressionado no instante do reset, o chip entra em modo de gravação serial e o
firmware nem chega a rodar. O AP não aparece, o serial não imprime nada, e
não há como diagnosticar isso de fora. Primeiro solte o RESET, só depois
aperte o BOOT.

O firmware amostra o GPIO0 a cada 20 ms durante o primeiro 1,5 s de boot, e
uma única leitura em nível baixo já basta: a janela cobre o gesto inteiro, não
um instante. Soltar o botão cedo demais não cancela a abertura do portal.

O portal nunca coexiste com a telemetria: salvar reinicia o dispositivo. Não
há fallback automático para este modo quando o Wi-Fi cai — seria tentador,
mas quem consegue derrubar o Wi-Fi do nó passaria a conseguir levantar o AP
de configuração sozinho.

### Placas que já rodaram o firmware antigo

O firmware anterior guardava o contador `seq` neste mesmo namespace da NVS,
mas nunca guardava a identidade do nó. Num `seq` órfão desses não há como
saber de qual `device_id` ele era.

Ao provisionar uma dessas placas pelo portal, o firmware **zera o `seq` por
precaução** — o serial imprime `# NVS: identidade nova ou desconhecida -- seq
zerado.`. A consequência visível é que o primeiro lote depois da migração
recomeça a numeração, que é o comportamento correto para uma identidade nova.
Herdar a contagem é que seria o erro: o backend passaria a contabilizar as
leituras novas como `duplicates`, em silêncio.

### A troca de canal durante o teste

O ESP32 tem um rádio só. Ao testar uma rede, o SoftAP é obrigado a operar no
mesmo canal da conexão que está sendo testada — e a conexão do celular com o
próprio nó **pode piscar ou cair** nesse instante. Isso é esperado, não é
travamento: reconecte no AP `tensio-setup-XXXX` e o resultado do teste
continua lá, esperando. O mesmo acontece ao usar "Procurar redes de novo".

### O token

`base64url` de 43 caracteres (é o que `cmd/devtoken` imprime no backend). O
campo do token nunca é reexibido pela página — deixá-lo em branco ao salvar
mantém o valor que já está gravado, o que permite trocar só a rede Wi-Fi em
campo sem redigitar 43 caracteres numa tela de celular. Se o nó ainda não tem
nenhum token gravado, o campo em branco é rejeitado: um nó sem token nunca
autentica.

**Deixar em branco só vale enquanto o destino não muda.** Mudar a URL da API
ou o identificador do nó passa a exigir o token colado por inteiro — a página
responde *"Mudar a URL ou o identificador do nó exige colar o token de novo."*
Trocar SSID, senha do Wi-Fi ou calibração não exige nada disso.

O motivo: sem essa regra, quem alcançasse o portal apontaria o nó para o
próprio servidor, deixaria o token em branco, e no primeiro POST o nó
entregaria o token **real** no cabeçalho `Authorization`. O segredo sairia sem
nunca ter aparecido na tela, e não haveria sintoma nenhum do lado do dono. A
comparação é literal, sem normalizar: `https://A.B/x` e `https://a.b/x` são o
mesmo lugar e ainda assim pedem o token — falhar fechado custa uma
redigitação; normalizar URL na mão custa uma brecha que ninguém vê.

### Timeout do portal

Sem salvar, o AP `tensio-setup-XXXX` cai sozinho depois de **10 minutos**. Se
ele já tiver caído, aperte e solte o RESET e, logo em seguida, segure o BOOT
por cerca de três segundos para abri-lo de novo — nessa ordem, nunca os dois
juntos. BOOT já segurado no instante em que o RESET é solto põe a placa em modo
de gravação e o firmware nem chega a rodar (ver "Quando o portal sobe").

### O token fica em claro na NVS

A NVS não é criptografada por padrão. É a mesma exposição que o token já
tinha dentro do binário compilado a partir do `config.h` — mas agora
registrada aqui, e não mais uma decorrência silenciosa da forma antiga de
configurar. Quem tem acesso físico ao ESP32 tem acesso ao token.

A senha do AP de configuração (`PROV_AP_SENHA`, em `config.h`) segue a mesma
lógica: está documentada neste README, logo é pública de fato. Ela **não
autentica ninguém**. O que ela ganha é criptografia de link — o vizinho
passivo não lê o token no ar sem antes se associar ao AP.

Derivar a senha do MAC não resolveria: o MAC vai em todo *beacon*, e o próprio
SSID (`tensio-setup-A19C`) já entrega dois dos seus bytes. Seria teatro. O que
de fato limita o estrago são três camadas independentes:

1. O AP **não fica no ar**. Ele só sobe com a NVS incompleta ou depois do gesto
   físico do BOOT, e morre em 10 minutos.
2. O token gravado **nunca é reemitido** para o HTML. Quem abre o portal não lê
   o que está lá dentro.
3. Token em branco **só vale com URL e identificador inalterados** (ver
   "O token"). Sem isso, o portal seria um canal de exfiltração silencioso.

O que sobra é negação de serviço com presença física: quem estiver ao alcance
durante os 10 minutos pode apontar o nó para um destino inválido e ele para de
enviar. Isso é visível — os dados somem do painel. Quem quiser fechar também
esse caso troca `PROV_AP_SENHA` no próprio `config.h`, que não é versionado.

### Ajustes de operação

| Campo | Padrão | Observação |
|---|---|---|
| `PROV_AP_SENHA` | `tensiometro` | Senha do AP `tensio-setup-XXXX`. Documentada aqui, logo pública — ver acima |
| `VDD_MV_NOMINAL` | `5000` | Vai no campo `vdd_mv` do payload. O firmware ainda não mede a alimentação real |
| `INTERVALO_ENVIO_MS` | `60000` | Intervalo entre leituras no modo telemetria |
| `INTERVALO_CALIBRACAO_MS` | `2000` | Intervalo no modo calibração |
| `WIFI_TIMEOUT_MS` | `20000` | Estourou: log de erro e reinício |
| `NTP_TIMEOUT_MS` | `30000` | Idem |
| `HTTP_TIMEOUT_MS` | `10000` | Timeout de uma requisição |
| `NTP_SERVIDOR_1/2` | `a.st1.ntp.br`, `pool.ntp.org` | O primeiro é do NTP.br |

## As três configurações

Selecionadas por dois `#define` no topo do `config.h`. As duas linhas vêm
comentadas, o que dá o modo telemetria com TLS.

### 1. Telemetria com TLS — configuração final

Ambos os `#define` comentados. O firmware valida o certificado do servidor
contra a raiz embutida em `CA_RAIZ_PEM`.

```c
// #define MODO_CALIBRACAO
// #define DEV_SEM_TLS
```

O `config.h.example` já traz a **ISRG Root X1** (Let's Encrypt), que cobre
a maioria dos hosts com certificado gratuito. Para outra CA:

```bash
openssl s_client -showcerts -connect SEU_HOST:443 </dev/null
```

O último certificado da cadeia é a raiz. Cole o PEM no formato que já está
no arquivo — uma string C por linha, cada uma terminada em `\n`.

O `setup()` sincroniza o NTP **antes** de qualquer conexão TLS, e isso não é
ordem arbitrária: a validação do certificado compara a validade contra o
relógio, e com o relógio em 1970 um certificado perfeitamente válido é
recusado como ainda não emitido.

### 2. Telemetria sem TLS — apenas desenvolvimento

```c
#define DEV_SEM_TLS
```

HTTP puro, para testar contra um backend local que não tem certificado válido.
**Não vai para a versão final.** A compilação avisa:

```
note: '#pragma message: DEV_SEM_TLS ativo: trafego em HTTP puro, sem TLS.
APENAS PARA DESENVOLVIMENTO.'
```

O código tem `#warning` **e** `#pragma message`, e os dois são necessários: o
nível de avisos padrão do arduino-cli e da IDE compila com `-w`, que silencia
`#warning` mas não `#pragma message` — que é uma *note*, não um *warning*.
Sem o pragma, uma compilação insegura passaria calada. Não remova nenhum dos
dois.

Além disso o firmware imprime o alerta no serial a cada boot.

### 3. Modo calibração — ensaio de bancada

```c
#define MODO_CALIBRACAO
```

Sem Wi-Fi, sem NTP, sem POST e **sem nenhuma escrita na NVS**. Só CSV no
serial, no intervalo de `INTERVALO_CALIBRACAO_MS`:

```
mv_pino,v_sensor,kpa
3016,4.524,0.60
3019,4.529,0.71
```

Copie do monitor serial direto para a planilha. A ausência de escrita na NVS
é o motivo de existir este modo: o ensaio de calibração lê a cada poucos
segundos por horas, o que geraria dezenas de milhares de escritas por dia num
`seq` que nem está sendo usado.

Uma linha marcada `<-- FORA DA FAIXA` significa `v_sensor` fora de 0,40–4,60 V:
erro de montagem, sensor sem alimentação, ou `FATOR_DIVISOR` errado.

## Compilando e gravando

```bash
arduino-cli core install esp32:esp32          # uma vez

cd Circuito
arduino-cli compile --fqbn esp32:esp32:esp32doit-devkit-v1 sketch
arduino-cli upload  --fqbn esp32:esp32:esp32doit-devkit-v1 -p /dev/ttyUSB0 sketch
arduino-cli monitor -p /dev/ttyUSB0 -c baudrate=115200
```

Tamanhos de referência das três configurações:

| Configuração | Flash | % de 1.310.720 B |
|---|---:|---:|
| Telemetria com TLS e portal cativo | 1.095.116 B | 83% |
| Telemetria `DEV_SEM_TLS` | 1.004.076 B | 76% |
| `MODO_CALIBRACAO` | ~279.000 B | 21% |

Os dois primeiros foram medidos no mesmo estado de código em 17/09/2026,
compilando a partir do `config.h.example` — que já é a configuração de
produção, com a ISRG Root X1 e `DEV_SEM_TLS` comentado.

Os 91.040 B a mais do modo TLS são o mbedTLS mais o certificado embutido.
RAM em 49.800 B (15%), 215.604 B de flash livres. Não há aperto: o limite é
a partição de 1.310.720 B, e o build de produção usa 83% dela.

O build de produção também é onde se verifica que a guarda de build inseguro
funciona. Com `DEV_SEM_TLS` o compilador emite
`note: '#pragma message: DEV_SEM_TLS ativo...'`; sem ele, silêncio. O
`#warning` da mesma dupla é de fato engolido pelo `-w` do `arduino-cli` —
sobrevive só o `#pragma message`, que é o motivo de os dois existirem.

## Teste de payload no host

O `payload.h` é C puro, sem nenhuma dependência do Arduino. O mesmo arquivo
que roda no ESP32 compila na máquina de desenvolvimento, então erro de formato
de timestamp, de nome de campo ou de formatação de float aparece em segundos,
sem gravar nada:

```bash
cd teste
gcc -Wall -Wextra -o teste_payload teste_payload.c && ./teste_payload
```

A última linha da saída é um lote JSON pronto para conferir contra o backend:

```bash
LOTE=$(./teste_payload | tail -1)
curl -sS -X POST http://localhost:8080/api/v1/readings \
  -H "Authorization: Bearer $DEVICE_TOKEN" \
  -H 'Content-Type: application/json' -d "$LOTE"
```

Resposta esperada no primeiro envio, e no segundo:

```json
{"accepted":1,"duplicates":0,"rejected":0,"rejections":[],"server_time":"..."}
{"accepted":0,"duplicates":1,"rejected":0,"rejections":[],"server_time":"..."}
```

O segundo confirma a idempotência por `(device_id, seq)`.

## Teste do provisionamento no host

`prov_valida.h` também é C puro: valida os campos do formulário do portal e
escapa HTML, sem depender do Arduino. Mesma lógica de `payload.h` — a
fronteira de confiança do portal (SSID vindo do scan de rádio, campos
digitados no celular) compila e testa na máquina de desenvolvimento:

```bash
cd teste
gcc -Wall -Wextra -o teste_prov teste_prov.c && ./teste_prov
```

## O que o serial mostra no boot

```
# No sensor de tensiometria - MODO TELEMETRIA
# Device: tensio-01
# Destino: http://192.168.0.10:8080/api/v1/readings
# ATENCAO: DEV_SEM_TLS ativo. Trafego SEM CRIPTOGRAFIA.
# seq recuperado da NVS: 137
# Wi-Fi: conectando a MinhaRede....
# Wi-Fi conectado. IP: 192.168.0.42
# NTP: sincronizando...
# NTP sincronizado. Agora (UTC): 2026-08-19T20:41:59Z
#
# seq,mv_pino,v_sensor,kpa,enviado
137,3016,4.524,0.60,sim
# HTTP 200  accepted=1 duplicates=0 rejected=0
```

## Diagnóstico

| Sintoma | Causa provável |
|---|---|
| `ERRO: Wi-Fi nao conectou dentro do timeout` | SSID/senha errados, ou rede só em 5 GHz |
| `ERRO: NTP nao respondeu dentro do timeout` | Rede sem saída para a internet, ou UDP/123 bloqueado |
| Código HTTP `401` | Token gravado pelo portal errado, ou `TELEMETRIA_TOKEN_SECRET` do servidor diferente do usado para gerar o token |
| Código HTTP `403` | Identificador do nó gravado pelo portal não corresponde ao dono do token, ou device com `ativo = false` |
| `rejected=1` na resposta | Calibração gravada pelo portal inexistente ou de outro device, ou leitura fora da faixa física |
| Código negativo (`-1`, `-11`) | Erro do próprio HTTPClient: conexão recusada, timeout, falha de TLS. O IP do backend mudou? |
| Códigos negativos **variados** (`-1`, `-5`, `-11` alternando) | Sinal fraco. Porta fechada dá `-1` constante; a variedade é o rádio oscilando |
| Falha constante com servidor comprovadamente no ar | Servidor com duas interfaces no mesmo `/24` — ver `rp_filter` nas pegadinhas de bancada |
| `-11 (read Timeout)` logo após provisionar | URL `https://` num firmware `DEV_SEM_TLS`, que não fala TLS |
| HTTP `404` em toda leitura | URL gravada sem o caminho `/api/v1/readings` |
| `duplicates` em toda leitura | `seq` regrediu — NVS apagada num device que já enviou. O alerta no boot cobre esse caso |
| `ATENCAO: seq zerado` no boot | NVS apagada por reflash com *erase all* ou troca de partição |
| Todas as leituras em `0.60` parado | Tubo aberto para a atmosfera — é o valor correto de repouso |
| O nó abre o portal sozinho toda vez | NVS sem configuração completa — algum campo ficou vazio ao salvar, ou a gravação não chegou ao fim e a marca de conclusão não foi escrita |
| O AP `tensio-setup` não aparece | Já se passaram 10 min, ou o nó está em telemetria. Solte o RESET e, logo depois, segure o BOOT por ~2 s — nunca os dois juntos |
| "Testar esta rede" parece travar | Troca de canal derrubou o celular. Reconecte no AP: o resultado continua lá |
| `duplicates` em toda leitura depois de reconfigurar | O `device_id` foi mantido e o `seq` também, mas o backend já tinha aqueles seq |

## Limitações conhecidas desta etapa

- **Sem buffer.** POST que falha perde a leitura. O `seq` avança mesmo assim,
  de propósito: o buraco na sequência é diagnosticável, enquanto reaproveitar
  o `seq` produziria duas leituras diferentes com o mesmo identificador.
  Medido em 16/09/2026, em cômodo com sinal fraco: de 154 amostras a cada
  60 s, 123 chegaram — **20% de perda**, quase toda em falhas isoladas, com
  um trecho de 36 envios consecutivos sem erro no meio. Foi o próprio buraco
  no `seq` que permitiu medir isso sem instrumentação nenhuma, o que é o
  argumento a favor da decisão; a perda em si continua sendo o custo dela.
- **Sem reenvio de configuração pelo ar.** Reconfigurar exige presença
  física: o portal só sobe com a NVS incompleta ou com o gesto do BOOT. É
  requisito de segurança, não omissão — um AP de configuração permanente
  seria superfície de ataque no talhão.
- **Sem deep sleep.** O ciclo usa `delay()`, o que não serve para operação a
  bateria (RNF02).
- **Coeficientes nominais de catálogo.** `V_ZERO_KPA_NOMINAL` e
  `K_VOLTS_POR_KPA_NOMINAL` saem do datasheet e serão substituídos pelos
  valores da calibração experimental contra o vacuômetro mecânico. O backend
  guarda `raw_mv` e `calibration_id` junto com o `kpa` para permitir
  reprocessar o histórico quando isso acontecer.
- **`VDD_SENSOR` fixo em 5,00 V.** O sensor é ratiométrico e a alimentação real
  pode ser um pouco menor. O desvio observado em bancada (4,524 V contra 4,50 V
  teóricos, cerca de 0,6 kPa) está dentro do erro do próprio sensor
  (±2% FSS = ±2 kPa) e será absorvido pela calibração experimental. Refinamento
  possível: segundo divisor no GPIO35 para compensação dinâmica — o campo
  `vdd_mv` do payload já existe para receber esse valor.
