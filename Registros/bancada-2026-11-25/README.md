# Bancada do nó físico — 25/11/2026

Três verificações que só a placa real responde, todas listadas como
limitação no cap. 6 até esta data:

1. o ciclo *registrar calibração pelo app → gravar o id no portal → leitura
   aceita*, nunca exercitado com o nó físico;
2. a fila de leituras pendentes do RNF04 (`696977c`), testada só no host
   (`Circuito/teste/teste_fila.c`);
3. as quatro raízes do `CA_RAIZ_PEM` validando cadeias do Let's Encrypt no
   mbedTLS do ESP32, verificadas até aqui só por `openssl` no notebook.

**Nada aqui é simulado**: é o `tensio-01`, o protótipo físico, contra a API e o
banco de desenvolvimento. **O tensiômetro estava fora do solo** (lendo ar, ~1
kPa): os valores de kPa não são medição de umidade, e nenhuma conclusão sobre o
método de medição sai daqui (ver `registro-2026-10-21.md`).

## Montagem

| | |
|---|---|
| Placa | ESP32 DevKit (`esp32doit-devkit-v1`), conversor USB 1a86:55d4 → `/dev/ttyACM0` |
| Firmware | `Circuito/sketch` em `696977c`, modo telemetria, `DEV_SEM_TLS` (HTTP na rede local) |
| Toolchain | arduino-cli 1.5.1, núcleo esp32 3.3.11, esptool 5.3.1 |
| Servidor | API compilada de `c58aa47` rodando no notebook (`:8080`), Postgres 16 do `tcc-pg` (Docker), migrations até a 0004 |
| Rede | Wi-Fi doméstico 2,4 GHz; notebook em 192.168.1.2; firewall `ufw` com a 8080 liberada só para 192.168.1.0/24 |
| Logs | `serial.log` (testes 1 e 2), `serial-tls.log` (teste 3), `api.log`; horários locais (UTC−3) |

Antes de começar, `make dump ENGINE=docker` (`Backend/dump-2026-11-25.sql`,
fora do git). O nome da rede doméstica foi trocado por `<rede>` nos logs.

**Estado de partida.** A NVS da placa guardava `seq` 540, e o banco tinha
leituras do `tensio-01` até o `seq` 1891, de setembro. Entre 535 e 1080 não
havia nenhuma, então a sessão cabia sem colisão; ver "Achados", item 5.

## T1 — calibração pelo app, gravada no portal

**Cadastro.** Pelo app ("Registrar novo ensaio", no sensor `tensio-01`), um
ensaio com os **mesmos coeficientes nominais** da `cal-2026-09-25-a`
(V₀ = 4,5 V, k = 0,04 V/kPa, divisor 1,5, 5000 mV), para a leitura não mudar
e o teste depender só do identificador. O servidor devolveu
`cal-2026-11-25-7c74`.

**Portal.** RESET, depois BOOT por ~3 s: `tensio-setup-A19C` no ar, 16 redes
na varredura, a página montada normalmente (o portal só tinha sido visto com
4). "Testar esta rede" associou. O formulário foi preenchido pelo notebook,
ligado ao AP da placa.

Na primeira volta a URL ficou com o endereço antigo (`…192.168.1.12…`): o
campo vem pré-preenchido com o valor gravado e passou despercebido. Na
segunda, com a URL nova, o token teve de ser colado de novo, como manda a
regra do portal (token em branco só vale com URL e nó iguais).

**Leitura aceita com o id novo.** O primeiro envio depois de cada volta falha
(o notebook ainda está fora da rede de casa), a leitura fica na fila e sai no
ciclo seguinte:

| seq | medida (UTC) | recebida (UTC) | kPa firmware | kPa servidor | calibração |
|---|---|---|---|---|---|
| 554 | 19:19:53 | 19:20:58 | 1,01 | 1,01 | `cal-2026-11-25-7c74` |
| 555 | 19:20:58 | 19:20:58 | 0,79 | 0,79 | `cal-2026-11-25-7c74` |
| 556 | 19:21:58 | 19:21:58 | 0,75 | 0,75 | `cal-2026-11-25-7c74` |

O kPa do servidor sai de `leituras_kpa` (migration 0003), calculado de
`raw_mv` e da calibração referenciada; coincide com o do firmware porque os
coeficientes são os mesmos.

**Caminho de erro.** Terceira volta, calibração `cal-nao-existe`, token em
branco (URL e nó iguais: a placa manteve o gravado e autenticou):

```
16:26:02 # HTTP 200  lote=2 accepted=0 duplicates=0 rejected=2
16:26:02 # ALERTA: leitura rejeitada pelo servidor -> {"accepted":0,"duplicates":0,"rejected":2,
         "rejections":[{"index":0,"seq":558,"reason":"calibration_id desconhecido para este device: cal-nao-existe"},
                       {"index":1,"seq":559,"reason":"calibration_id desconhecido para este device: cal-nao-existe"}], ...}
16:27:02 # HTTP 200  lote=1 accepted=0 duplicates=0 rejected=1
```

558 a 560 não estão no banco e não foram reenviadas: o firmware descarta o
que o servidor recusa por validação, porque reenviar travaria a fila. Quarta
volta, id certo de volta: 561 (da fila) e 562 aceitas com
`cal-2026-11-25-7c74`.

## T2 — fila do RNF04

A placa mede a cada 60 s; a última coluna da serial é o tamanho da fila.

**T2a — servidor fora.** API parada com `SIGTERM` às 16:29:52 (`encerrado sem
requisicoes pendentes`), religada às 16:34:40:

```
16:30:30 # ERRO no POST. Codigo: -1  (connection refused)
16:30:30 563,3026,4.539,0.98,1
   ...                                        (fila 2, 3, 4)
16:34:31 567,3024,4.536,0.90,5
16:35:31 # HTTP 200  lote=6 accepted=6 duplicates=0 rejected=0
```

| seq | 563 | 564 | 565 | 566 | 567 | 568 |
|---|---|---|---|---|---|---|
| espera (recebida − medida) | 5:01 | 4:01 | 3:00 | 2:00 | 1:00 | 0:00 |

**T2b — Wi-Fi fora.** O Wi-Fi do roteador foi desligado às ~16:40 e religado
às ~16:52. A cada ciclo, `# Wi-Fi caiu. Reconectando.` e `# ERRO: Wi-Fi nao
conectou dentro do timeout.`, com a fila crescendo de 1 a 9 (573 a 581). Com a
rede de volta, a placa reconectou sozinha:

```
16:52:37 # HTTP 200  lote=10 accepted=10 duplicates=0 rejected=0
```

| seq | 573 | 574 | 575 | 576 | 577 | 578 | 579 | 580 | 581 | 582 |
|---|---|---|---|---|---|---|---|---|---|---|
| espera | 12:05 | 10:45 | 9:24 | 8:04 | 6:43 | 5:22 | 4:02 | 2:41 | 1:21 | 0:00 |

Sem rede, o ciclo passa a ~80 s, e não 60: ver "Achados", item 2.

**Buracos na sequência** de 554 em diante, depois de T1 e T2:

```
 depois_de | faltam
-----------+--------
       557 |      3        <- 558-560, recusadas de proposito no T1
```

Nenhuma leitura se perdeu em T2a nem em T2b. A perda que a fila em RAM **não**
evita apareceu também: a 539, pendente às 15:57:59, sumiu com o RESET das
15:58:11; e de 540 a 553 (endereço antigo e as voltas pelo portal, cada uma
um reinício) nada chegou ao banco.

## T3 — raízes TLS na placa (`Circuito/teste_tls/`)

Sketch separado do firmware, com o mesmo cliente (`WiFiClientSecure` sobre
mbedTLS). O pacote de raízes é o `CA_RAIZ_PEM` do `config.h.example`,
incluído sem cópia. O Wi-Fi é lido da NVS gravada pelo portal, aberta só para
leitura. Doze casos contra os sites de teste do Let's Encrypt, cada um com o
resultado **esperado calculado antes**, no notebook, por `openssl s_client
-CAfile` com as mesmas raízes (`raizes.pem`, extraído do `config.h.example`).

Os `valid.*` mandam cadeias diferentes: `valid.x1` manda folha > YR2 > Root YR
assinada pela X1, e `valid.x2` manda folha > YE2 > Root YE assinada pela X2.
Com as quatro raízes juntas, a X1 e a X2 nunca precisam ser usadas. Por isso
a segunda rodada carrega **uma raiz por vez**, mais um controle negativo.

| # | Site | Raízes | Esperado | Placa | |
|---|---|---|---|---|---|
| 1 | valid.x1 | todas | aceita | aceita | ok |
| 2 | valid.x2 | todas | aceita | aceita | ok |
| 3 | valid.yr | todas | aceita | aceita | ok |
| 4 | valid.ye | todas | aceita | aceita | ok |
| 5 | **expired.x1** | todas | **recusa** | **aceita** | **DIVERGE** |
| 6 | www.google.com | todas | recusa | recusa | ok |
| 7 | revoked.x1 | todas | aceita¹ | aceita | ok |
| 8 | valid.x1 | só X1 | aceita | aceita | ok |
| 9 | valid.x2 | só X2 | aceita | aceita | ok |
| 10 | valid.yr | só YR | aceita | aceita | ok |
| 11 | valid.ye | só YE | aceita | aceita | ok |
| 12 | valid.ye | só X1 | recusa | recusa | ok |

¹ Nem o `openssl` sem CRL nem o mbedTLS consultam revogação; o caso está aí
para deixar isso registrado.

As recusas (6 e 12) vêm com `mbedTLS -0x2700 X509 - Certificate verification
failed`. As aceitas respondem `HTTP/1.1 404` ao `HEAD /`, porque os sites de
teste não têm página na raiz: a conexão era real. A placa tinha o relógio
sincronizado (`Relogio (UTC): 2026-11-25T20:16:25Z`).

**A divergência do caso 5 é do núcleo, não do sketch.** O
`include/sdkconfig.h` do núcleo esp32 3.3.11 tem `CONFIG_MBEDTLS_HAVE_TIME`
ligado e **não** tem `CONFIG_MBEDTLS_HAVE_TIME_DATE`: o mbedTLS dessa
compilação não compara as datas de validade do certificado com o relógio. O
nó aceita um certificado vencido de uma cadeia que, fora isso, é válida. O
`openssl`, com as mesmas raízes, recusa (`Verify return code: 10 (certificate
has expired)`).

**Volta ao firmware.** Firmware de telemetria regravado às 17:18. A primeira
leitura chegou com `seq` 583, logo depois da 582, com a mesma calibração e o
mesmo token: o sketch de teste não tocou na NVS.

## Achados

1. **O nó não confere a validade do certificado do servidor** (T3, caso 5).
   Somado à ausência de consulta de revogação, um certificado antigo do
   domínio, com a chave vazada, continuaria aceito pelo nó. O comentário do
   `config.h.example`, que afirmava o contrário, foi corrigido. Correção
   possível, não feita: comparar no firmware, depois do aperto de mão, a
   validade do certificado do servidor com o relógio do NTP, ou usar um
   núcleo compilado com `CONFIG_MBEDTLS_HAVE_TIME_DATE`.
2. **Sem rede, o ciclo de medição estica para ~80 s** (T2b). A reconexão no
   `loop` bloqueia até `WIFI_TIMEOUT_MS` (20 s) e chama `WiFi.begin` com o
   driver já reconectando sozinho (`wifi:sta is connecting, cannot set
   config`). Nada se perdeu, mas o intervalo entre medições não é o
   configurado durante a queda. Com 720 posições, a fila cobre ~16 h nesse
   ritmo, em vez de 12 h.
3. **O campo de URL do portal vem pré-preenchido** com o valor gravado, e a
   troca de servidor passou despercebida na primeira volta (T1). A placa
   avisa no boot (`# Destino: ...`), mas só na serial.
4. **A placa fica muda ao abrir a porta serial**: duas vezes hoje, e o botão
   EN destravou nas duas. Não afeta a operação, que não usa a serial.
5. **A NVS estava atrás do banco** (`seq` 540 contra 1891). A partir do `seq`
   1081, que já existe no banco, as leituras novas seriam descartadas como
   duplicatas. Com 1 leitura por minuto, isso aconteceria ~8 h depois do fim
   da sessão; a placa não deve ficar ligada até lá sem resolver isso.

## O que este registro não cobre

- **HTTPS contra o nosso servidor.** T1 e T2 rodaram com `DEV_SEM_TLS`; o T3
  valida as raízes contra sites de teste, não a cadeia que o nosso domínio
  vai entregar (`docs/DEPLOY.md`, passo 7, quando houver servidor).
- **Fila cheia na placa.** A regra "sai a mais antiga" está coberta pelo
  `teste_fila.c` no host, não foi provocada na placa.
- **Quedas longas e falta de energia.** A maior queda foi de 12 min. A fila em
  RAM se perde num reinício (visto na 539).
- **Medição de umidade.** O tensiômetro estava fora do solo.
