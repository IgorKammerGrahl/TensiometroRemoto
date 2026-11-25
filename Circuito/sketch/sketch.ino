// No sensor de tensiometria eletronica
// TCC - Igor Kammer Grahl - IFC Campus Rio do Sul
//
// Sensor:  XGZP6847A100KPGN (-100 a 0 kPa, alimentacao 5 V, saida 0,5 a 4,5 V)
// Entrada: GPIO34 (ADC1 - unico ADC utilizavel com Wi-Fi ativo)
//
// Condicionamento de sinal (divisor resistivo):
//
//   OUT do sensor --[ R1 = 10k ]--+-- GPIO34
//                                 |
//                              [ R2 = 10k ]
//                                 |
//                              [ R3 = 10k ]
//                                 |
//                                GND
//
//   Vpino = Vsensor * 20k / 30k = Vsensor * (2/3)
//   Logo, para recuperar Vsensor: Vsensor = Vpino * 1,5
//
// DOIS MODOS, selecionados em config.h:
//
//   MODO_CALIBRACAO  ensaio de bancada. So CSV no serial: sem Wi-Fi, sem
//                    NTP, sem POST e sem nenhuma escrita na NVS.
//   (padrao)         telemetria: mede, carimba em UTC e envia ao backend.
//
// RESILIENCIA (RNF04): a leitura entra numa fila em RAM e sai dela quando o
// servidor responde. Rede fora, a fila cresce; rede de volta, ela drena em
// lotes, da mais antiga para a mais nova. Ver fila.h para as decisoes. Ficam
// para depois: fila em LittleFS (sobreviver a falta de energia) e deep sleep.

#include "config.h"
#include "payload.h"
#include "fila.h"

const int PINO_SENSOR = 34;

// ---- Condicionamento ----
const float FATOR_DIVISOR = 1.5;

// ---- Alimentacao do sensor ----
// O XGZP6847A e RATIOMETRICO: a saida e proporcional a tensao de alimentacao.
// A curva do datasheet pressupoe 5,00 V exatos. Se o VIN real for diferente
// (USB tipicamente entrega 4,6 a 4,9 V), a curva desloca proporcionalmente.
//
// DECISAO: mantido em 5,00 sem medicao direta do VIN. A bancada em atmosfera
// leu 4,524 V contra 4,50 V teoricos -- cerca de 0,6 kPa, dentro do erro do
// proprio sensor (+/-2% FSS = +/-2 kPa, datasheet). Um VIN de 4,7 V produziria
// desvio bem maior que isso. O residuo que sobrar sera absorvido pelos
// coeficientes da calibracao experimental contra o vacuometro mecanico, que
// captura qualquer vies de alimentacao junto com o resto.
//
// ponytail: constante em vez de medicao. Refinamento possivel, nao necessario
// agora: medir o VIN com multimetro, ou um segundo divisor no GPIO35 para
// compensacao ratiometrica dinamica (o campo vddMV do payload ja existe para
// receber esse valor -- hoje vai o nominal de config.h).
const float VDD_SENSOR = 5.00;

// ---- Curva de conversao ----
// Datasheet XGZP6847A v2.7, p. 5, "Vacuum Pressure", modelo 100KPGN @ 5 V:
//   0,5 V -> -100 kPa
//   4,5 V ->    0 kPa
//   pressao = (Vsaida - 4,5) / 0,04
//
// ATENCAO: valores NOMINAIS de catalogo. Devem ser substituidos pelos
// coeficientes da calibracao experimental contra o vacuometro mecanico
// (objetivo especifico 5 do TCC).
//
// O backend guarda raw_mv e calibration_id junto com o kpa justamente para
// que este historico possa ser reprocessado quando isso acontecer.
const float V_ZERO_KPA_NOMINAL = 4.5;
const float K_VOLTS_POR_KPA_NOMINAL = 0.04;

// Escala os coeficientes para o VDD real (correcao ratiometrica)
const float FATOR_VDD = VDD_SENSOR / 5.00;
const float V_ZERO_KPA = V_ZERO_KPA_NOMINAL * FATOR_VDD;
const float K_VOLTS_POR_KPA = K_VOLTS_POR_KPA_NOMINAL * FATOR_VDD;

// ---- Filtragem ----
// Mediana e mais robusta que media contra picos espurios do ADC.
const int N_AMOSTRAS = 31;

struct Leitura {
  float mvPino;
  float vSensor;
  float kPa;
};

float lerMedianaMilivolts() {
  uint32_t amostras[N_AMOSTRAS];

  for (int i = 0; i < N_AMOSTRAS; i++) {
    amostras[i] = analogReadMilliVolts(PINO_SENSOR);
    delay(2);
  }

  // Ordenacao por insercao (suficiente para N pequeno)
  for (int i = 1; i < N_AMOSTRAS; i++) {
    uint32_t chave = amostras[i];
    int j = i - 1;
    while (j >= 0 && amostras[j] > chave) {
      amostras[j + 1] = amostras[j];
      j--;
    }
    amostras[j + 1] = chave;
  }

  return (float) amostras[N_AMOSTRAS / 2];
}

Leitura medir() {
  Leitura l;
  l.mvPino = lerMedianaMilivolts();

  // Desfaz o divisor para recuperar a tensao real na saida do sensor
  l.vSensor = (l.mvPino / 1000.0) * FATOR_DIVISOR;

  // Converte para succao. Valores negativos sao esperados e corretos.
  l.kPa = (l.vSensor - V_ZERO_KPA) / K_VOLTS_POR_KPA;
  return l;
}

bool foraDaFaixaFisica(const Leitura &l) {
  return l.vSensor < 0.40 || l.vSensor > 4.60;
}

// ===========================================================================
#ifdef MODO_CALIBRACAO
// ===========================================================================
// Ensaio de bancada. Formato de CSV identico ao firmware de bancada
// original, para importacao direta em planilha.

void setup() {
  Serial.begin(115200);

  // Atenuacao maxima: faixa util de leitura ate ~3,1 V
  analogSetPinAttenuation(PINO_SENSOR, ADC_11db);

  delay(1500);

  Serial.println();
  Serial.println("# No sensor de tensiometria - MODO CALIBRACAO");
  Serial.println("# Sem Wi-Fi, sem NTP, sem POST, sem escrita na NVS.");
  Serial.print("# VDD assumido: ");
  Serial.print(VDD_SENSOR, 2);
  Serial.println(" V");
  Serial.print("# Fator do divisor: ");
  Serial.println(FATOR_DIVISOR, 2);
  Serial.println("#");
  Serial.println("# Faixa esperada de v_sensor: 0,50 a 4,50 V");
  Serial.println("# Fora dessa faixa indica erro de montagem ou de fator.");
  Serial.println("#");
  Serial.println("mv_pino,v_sensor,kpa");
}

void loop() {
  Leitura l = medir();

  Serial.print(l.mvPino, 0);
  Serial.print(",");
  Serial.print(l.vSensor, 3);
  Serial.print(",");
  Serial.print(l.kPa, 2);

  // Alerta de sanidade: fora da faixa fisica do sensor
  if (foraDaFaixaFisica(l)) {
    Serial.print("   <-- FORA DA FAIXA");
  }

  Serial.println();

  delay(INTERVALO_CALIBRACAO_MS);
}

// ===========================================================================
#else  // telemetria
// ===========================================================================

#include <WiFi.h>
#include <HTTPClient.h>
#include <Preferences.h>
#include <time.h>
#include "prov.h"

#ifdef DEV_SEM_TLS
// Os DOIS sao necessarios. O nivel de avisos padrao do arduino-cli e da IDE
// compila com -w, que engole o #warning; o #pragma message e uma nota e
// sobrevive ao -w. Sem o pragma, uma compilacao insegura passaria calada,
// que e exatamente o que nao pode acontecer.
#warning "DEV_SEM_TLS ativo: trafego em HTTP puro, sem TLS. APENAS PARA DESENVOLVIMENTO."
#pragma message("DEV_SEM_TLS ativo: trafego em HTTP puro, sem TLS. APENAS PARA DESENVOLVIMENTO.")
#else
#include <WiFiClientSecure.h>
#endif

Preferences prefs;

// Configuracao vinda da NVS. Substitui WIFI_SSID, WIFI_SENHA, API_URL,
// DEVICE_ID, DEVICE_TOKEN e CALIBRATION_ID, que sairam do config.h.
ProvConfig cfg;

// GPIO0 e o botao BOOT da DevKit V1, o unico livre, e ja tem pull-up
// externo. Apertar BOOT logo DEPOIS de soltar o RESET, dentro da janela de
// amostragem do setup(), e a acao fisica explicita que reabre o portal num
// no ja configurado. Segurar BOOT junto com o RESET nao serve: GPIO0 e pino
// de strapping e o chip entra em modo de gravacao serial, sem rodar isto
// aqui.
const int PINO_BOTAO_PORTAL = 0;

// Proximo seq a usar. Persistido na NVS a cada leitura para sobreviver a
// reset e (na etapa 2) a deep sleep.
//
// ponytail: a NVS faz wear leveling, entao no intervalo de campo (dezenas
// de minutos) sao ~20 mil escritas/ano -- irrelevante. Mas em bancada num
// intervalo de segundos isso vira dezenas de milhares por dia; para esse
// regime existe o MODO_CALIBRACAO, que nao escreve nada.
long long proximoSeq = 0;

// Qualquer instante posterior a esta data prova que o NTP respondeu: o
// ESP32 sem sincronizacao boota em 1970. O backend rejeita measured_at
// anterior a 2020, mas aqui a exigencia e mais dura de proposito -- ou o
// relogio e real, ou nao ha o que enviar.
const time_t EPOCH_MINIMO_VALIDO = 1735689600;  // 2025-01-01T00:00:00Z

bool conectarWiFi() {
  Serial.print("# Wi-Fi: conectando a ");
  Serial.print(cfg.ssid);

  WiFi.mode(WIFI_STA);
  WiFi.begin(cfg.ssid.c_str(), cfg.senha.c_str());

  unsigned long inicio = millis();
  while (WiFi.status() != WL_CONNECTED) {
    if (millis() - inicio > WIFI_TIMEOUT_MS) {
      Serial.println();
      Serial.println("# ERRO: Wi-Fi nao conectou dentro do timeout.");
      return false;
    }
    Serial.print(".");
    delay(500);
  }

  Serial.println();
  Serial.print("# Wi-Fi conectado. IP: ");
  Serial.println(WiFi.localIP());
  return true;
}

// Sincroniza o relogio em UTC e bloqueia ate ter hora valida.
//
// Offsets zerados = UTC puro. O backend armazena tudo em UTC; converter
// para America/Sao_Paulo e responsabilidade da camada de apresentacao.
//
// Isto roda ANTES do primeiro POST porque o carimbo de cada leitura vem
// deste relogio (ver loop). NAO e pela validacao TLS: o mbedTLS deste
// nucleo nao compara as datas do certificado com o relogio (sem
// CONFIG_MBEDTLS_HAVE_TIME_DATE; Registros/bancada-2026-11-25, T3).
bool sincronizarNTP() {
  Serial.print("# NTP: sincronizando");
  configTime(0, 0, NTP_SERVIDOR_1, NTP_SERVIDOR_2);

  unsigned long inicio = millis();
  while (time(nullptr) < EPOCH_MINIMO_VALIDO) {
    if (millis() - inicio > NTP_TIMEOUT_MS) {
      Serial.println();
      Serial.println("# ERRO: NTP nao respondeu dentro do timeout.");
      Serial.println("#       Sem hora valida o measured_at sairia em 1970");
      Serial.println("#       e o backend rejeitaria a leitura por faixa.");
      return false;
    }
    Serial.print(".");
    delay(250);
  }

  char agora[32];
  formatarInstanteUTC(time(nullptr), agora, sizeof agora);
  Serial.println();
  Serial.print("# NTP sincronizado. Agora (UTC): ");
  Serial.println(agora);
  return true;
}

// ---- Fila de pendentes (RNF04) ----
//
// Padroes aqui, e nao no config.h, para que um config.h antigo continue
// compilando: quem quiser outro valor define no config.h e ganha.
//
// 720 leituras de 24 bytes = ~17 KB de RAM estatica. Cobrem 12 h com o
// intervalo de bancada (60 s) e dias com o de campo (dezenas de minutos).
#ifndef FILA_CAPACIDADE
#define FILA_CAPACIDADE 720
#endif
// Leituras por POST. O servidor aceita 500; 20 mantem o corpo em ~4 KB, que
// cabe num buffer estatico sem apertar a RAM que o TLS precisa.
#ifndef LOTE_MAX
#define LOTE_MAX 20
#endif
// Lotes por ciclo: com a fila cheia, drena 200 leituras por ciclo, sem
// segurar a proxima medicao por mais que alguns segundos de TLS.
#ifndef LOTES_POR_CICLO
#define LOTES_POR_CICLO 10
#endif

LeituraPendente pendentes[FILA_CAPACIDADE];
Fila fila;

// Envia ate `n` leituras num POST. Em `enviadas`, quantas de fato entraram
// no corpo (o resto, se o buffer acabou, fica para o proximo lote).
Resultado enviarLote(const LeituraPendente *ls, size_t n, size_t *enviadas) {
  // Estaticos: ~8 KB na pilha da tarefa do loop (8 KB ao todo) a estourariam.
  static char leituras[3584];
  static char corpo[4096];

  *enviadas = montarLeituras(leituras, sizeof leituras, ls, n, VDD_MV_NOMINAL,
                             cfg.calibracao.c_str());
  if (*enviadas == 0 ||
      !montarLote(corpo, sizeof corpo, cfg.deviceId.c_str(), leituras)) {
    // So acontece com um campo absurdo (id de calibracao gigante). Essa
    // leitura nunca vai caber; descarta-la impede que trave a fila inteira.
    Serial.println("# ERRO: leitura nao coube no lote; descartada.");
    *enviadas = 1;
    return RECUSADO;
  }

#ifdef DEV_SEM_TLS
  WiFiClient cliente;
#else
  WiFiClientSecure cliente;
  cliente.setCACert(CA_RAIZ_PEM);
#endif

  HTTPClient http;
  if (!http.begin(cliente, cfg.url)) {
    Serial.println("# ERRO: URL invalida na configuracao gravada pelo "
                   "portal. Reabra o portal e corrija a URL da API.");
    return FALHOU;
  }
  http.setTimeout(HTTP_TIMEOUT_MS);
  http.addHeader("Content-Type", "application/json");
  http.addHeader("Authorization", String("Bearer ") + cfg.token);

  int codigo = http.POST((uint8_t *) corpo, strlen(corpo));
  String resposta = (codigo > 0) ? http.getString() : String();
  http.end();

  Resultado r = resultadoDoStatus(codigo);
  if (r != ENTREGUE) {
    Serial.print("# ERRO no POST. Codigo: ");
    Serial.print(codigo);
    if (codigo > 0) {
      Serial.print("  resposta: ");
      Serial.print(resposta);
    } else {
      // Codigo negativo e erro do proprio HTTPClient (conexao, timeout, TLS)
      Serial.print("  (");
      Serial.print(HTTPClient::errorToString(codigo));
      Serial.print(")");
    }
    Serial.println();
    if (r == RECUSADO) {
      Serial.printf("# %u leitura(s) descartada(s): o servidor recusou o lote.\n",
                    (unsigned) *enviadas);
    }
    return r;
  }

  int aceitas = lerCampoInteiro(resposta.c_str(), "accepted", -1);
  int duplicadas = lerCampoInteiro(resposta.c_str(), "duplicates", -1);
  int rejeitadas = lerCampoInteiro(resposta.c_str(), "rejected", -1);

  Serial.printf("# HTTP 200  lote=%u accepted=%d duplicates=%d rejected=%d\n",
                (unsigned) *enviadas, aceitas, duplicadas, rejeitadas);

  if (rejeitadas > 0) {
    // O servidor recusou por validacao. Reenviar seria loop infinito.
    Serial.print("# ALERTA: leitura rejeitada pelo servidor -> ");
    Serial.println(resposta);
  }
  return ENTREGUE;
}

// Drena a fila em lotes, da mais antiga para a mais nova, ate esvaziar, ate
// LOTES_POR_CICLO ou ate a primeira falha. Na falha, para: se a rede caiu no
// meio, insistir so atrasaria a proxima medicao.
void drenar() {
  static LeituraPendente lote[LOTE_MAX];
  for (int i = 0; i < LOTES_POR_CICLO && fila.tamanho > 0; i++) {
    size_t n = filaEspiar(&fila, lote, LOTE_MAX);
    size_t enviadas = 0;
    if (enviarLote(lote, n, &enviadas) == FALHOU) return;
    filaDescartar(&fila, enviadas);
  }
}

void setup() {
  Serial.begin(115200);

  // Atenuacao maxima: faixa util de leitura ate ~3,1 V
  analogSetPinAttenuation(PINO_SENSOR, ADC_11db);

  // ---- Maquina de estados do boot ----
  //
  // A ordem importa e nao e arbitraria: o botao e lido ANTES de qualquer
  // decisao sobre a NVS, para que um no configurado errado -- URL de um
  // backend que sumiu, rede que nao existe mais -- sempre tenha um caminho
  // de volta ao portal que nao dependa de nada gravado estar correto.
  //
  // E a leitura e uma JANELA, nao um instante. GPIO0 e pino de strapping:
  // segurar BOOT no momento do reset poe o chip em modo de gravacao serial
  // e o firmware nem roda. O gesto que funciona e apertar BOOT DEPOIS de o
  // reset ter sido solto, entao a janela precisa cobrir esse gesto. Ela
  // ocupa o mesmo 1,5 s de estabilizacao que ja existia aqui -- o boot nao
  // ficou mais lento.
  pinMode(PINO_BOTAO_PORTAL, INPUT_PULLUP);

  bool botaoPressionado = false;
  for (int i = 0; i < 75; i++) {  // 75 x 20 ms = 1500 ms
    // O delay vem antes da primeira leitura para dar tempo ao pull-up.
    delay(20);
    if (digitalRead(PINO_BOTAO_PORTAL) == LOW) {
      // Trava em true: quem soltou o botao cedo demais ainda entra no
      // portal. Uma janela que so valesse no instante final seria a mesma
      // loteria da leitura unica que isto substitui.
      botaoPressionado = true;
    }
  }

  bool temConfiguracao = provCarregar(cfg);

  if (botaoPressionado || !temConfiguracao) {
    if (botaoPressionado) {
      Serial.println("# BOOT pressionado: abrindo o portal.");
    } else {
      Serial.println("# NVS sem configuracao completa: abrindo o portal.");
    }
    provRodarPortal(cfg);  // salvar reinicia; so retorna no timeout
    provOcioso();          // nao retorna
  }

  Serial.println();
  Serial.println("# No sensor de tensiometria - MODO TELEMETRIA");
  Serial.print("# Device: ");
  Serial.println(cfg.deviceId);
  Serial.print("# Destino: ");
  Serial.println(cfg.url);
#ifdef DEV_SEM_TLS
  Serial.println("# ATENCAO: DEV_SEM_TLS ativo. Trafego SEM CRIPTOGRAFIA.");
#endif

  prefs.begin("tensio", false);
  proximoSeq = prefs.getLong64("seq", 0);
  Serial.print("# seq recuperado da NVS: ");
  Serial.println((long) proximoSeq);

  if (proximoSeq == 0) {
    // Se este no ja enviou dados antes, a NVS foi apagada (reflash com
    // "erase all", troca de particao). O backend vai receber seq que ja
    // existem e descartar as leituras novas como duplicatas -- e o alarme
    // de colisao do servidor dispara justamente nesse caso.
    Serial.println("# ATENCAO: seq zerado. Se este device ja enviou leituras,");
    Serial.println("#          o backend vai descarta-las como duplicatas.");
  }

  filaIniciar(&fila, pendentes, FILA_CAPACIDADE);

  if (!conectarWiFi() || !sincronizarNTP()) {
    // A fila guarda leituras CARIMBADAS, e o carimbo depende do NTP: o ESP32
    // nao tem relogio de bateria e boota em 1970. Sem a primeira hora valida
    // nao ha o que guardar, entao o boot continua exigindo rede. Depois dela,
    // o relogio interno segue contando, e uma queda de rede passa a encher a
    // fila em vez de perder a leitura.
    Serial.println("# Reiniciando em 10 s...");
    delay(10000);
    ESP.restart();
  }

  Serial.printf("# Fila de pendentes: ate %d leituras em RAM.\n", FILA_CAPACIDADE);
  Serial.println("#");
  Serial.println("# seq,mv_pino,v_sensor,kpa,pendentes");
}

void loop() {
  Leitura l = medir();
  time_t medidoEm = time(nullptr);

  // Carimba no momento da medicao, nao no do envio: e isso que faz a leitura
  // que esperou na fila chegar ao banco com a hora em que foi medida.
  long long seq = proximoSeq;

  // Avanca e persiste ANTES de enfileirar. O seq nunca e reaproveitado: uma
  // leitura que sai da fila sem ser enviada (fila cheia, reinicio) vira um
  // buraco na sequencia, que e diagnostico. Reaproveitar produziria duas
  // leituras diferentes com o mesmo seq, e a segunda seria descartada como
  // duplicata.
  proximoSeq++;
  prefs.putLong64("seq", proximoSeq);

  LeituraPendente p = {seq, medidoEm, (int) lroundf(l.mvPino), l.kPa};
  if (filaEmpurrar(&fila, p)) {
    Serial.printf("# ALERTA: fila cheia. A leitura mais antiga saiu sem ser "
                  "enviada (%lu desde o boot).\n", fila.descartadas);
  }

  // O Wi-Fi nao volta sozinho depois de uma queda: conectarWiFi() so roda
  // no setup. Sem isto o no fica amostrando e enfileirando ate alguem apertar
  // RESET -- e ninguem esta no talhao para apertar. Falhar aqui nao e fatal:
  // a leitura fica na fila, e a proxima volta tenta de novo.
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("# Wi-Fi caiu. Reconectando.");
    conectarWiFi();
  }
  if (WiFi.status() == WL_CONNECTED) {
    drenar();
  }

  Serial.print((long) seq);
  Serial.print(",");
  Serial.print(l.mvPino, 0);
  Serial.print(",");
  Serial.print(l.vSensor, 3);
  Serial.print(",");
  Serial.print(l.kPa, 2);
  Serial.print(",");
  Serial.print((unsigned) fila.tamanho);
  if (foraDaFaixaFisica(l)) {
    Serial.print("   <-- FORA DA FAIXA");
  }
  Serial.println();

  delay(INTERVALO_ENVIO_MS);
}

#endif  // MODO_CALIBRACAO
