// Teste das raizes TLS na placa real: o ESP32 valida as cadeias do Let's
// Encrypt com as mesmas quatro raizes que o firmware vai usar em producao?
//
// O QUE PROVA. O firmware valida o servidor com CA_RAIZ_PEM (config.h.example):
// ISRG Root X1, ISRG Root X2, Root YR e Root YE. Este sketch usa o mesmo
// cliente (WiFiClientSecure sobre mbedTLS) contra os sites de teste que o
// proprio Let's Encrypt mantem, um por raiz, e confere cada caso contra o
// resultado ESPERADO -- calculado antes, no notebook, com openssl e as mesmas
// raizes. Os casos de recusa sao a metade que importa: sem eles, um cliente
// que aceitasse qualquer certificado passaria em tudo.
//
// DUAS RODADAS. Com as quatro raizes juntas, um site que manda cadeia
// cruzada (valid.x1 manda folha > YR2 > Root YR assinada pela X1) valida pela
// YR, e a X1 nunca e exercitada. Por isso a segunda rodada carrega UMA raiz
// por vez, e um controle negativo (o site da YE so com a X1) mostra que a raiz
// certa e exigida.
//
// AS RAIZES VEM DO config.h.example, SEM COPIA. O arquivo e incluido, e a
// compilacao poe a pasta do firmware no caminho de include (a partir da raiz
// do repositorio):
//
//   arduino-cli compile --fqbn esp32:esp32:esp32doit-devkit-v1 \
//     --build-property "compiler.cpp.extra_flags=-I$PWD/Circuito/sketch" \
//     Circuito/teste_tls
//
// O WI-FI VEM DA NVS QUE O PORTAL GRAVOU, ABERTA SO PARA LEITURA. Nenhuma
// senha no codigo, e nada do que o firmware guarda (seq, token, URL,
// calibracao) e tocado: regravar o firmware depois devolve o no ao estado de
// antes. WiFi.persistent(false) pelo mesmo motivo -- sem ele, WiFi.begin
// regravaria a rede na NVS do proprio Wi-Fi.
//
// O que NAO prova: a cadeia que o NOSSO servidor vai entregar (isso e o
// docs/DEPLOY.md, passo 7, quando a VM existir), nem revogacao -- a placa nao
// consulta OCSP nem CRL, e o caso do certificado revogado esta aqui para
// deixar isso registrado, nao para passar.

#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <Preferences.h>
#include <time.h>
#include "mbedtls/x509_crt.h"

#include "config.h.example"

// Mesmo namespace e chaves de Circuito/sketch/prov_nvs.h.
#define PROV_NS     "tensio"
#define PROV_K_SSID "ssid"
#define PROV_K_SENHA "senha"

enum { X1, X2, YR, YE, N_RAIZES, TODAS = -1 };
static const char *NOME_RAIZ[N_RAIZES] = {"X1", "X2", "YR", "YE"};
// CN de cada raiz, para achar cada uma no pacote pelo conteudo, e nao pela
// ordem em que estao no arquivo.
static const char *CN_RAIZ[N_RAIZES] = {
    "CN=ISRG Root X1", "CN=ISRG Root X2", "CN=Root YR", "CN=Root YE"};

// Uma raiz em PEM tem ~2 KB; folga para a maior (X1 e YR, RSA 4096).
static char raiz[N_RAIZES][2600];

struct Caso {
  const char *host;
  int raizes;      // TODAS ou uma raiz so
  bool aceita;     // ESPERADO, do openssl no notebook
  const char *nota;
};

static const Caso CASOS[] = {
    // Rodada 1: o pacote de producao inteiro.
    {"valid.x1.test-certs.letsencrypt.org", TODAS, true, "cadeia da X1"},
    {"valid.x2.test-certs.letsencrypt.org", TODAS, true, "cadeia da X2"},
    {"valid.yr.test-certs.letsencrypt.org", TODAS, true, "cadeia da YR"},
    {"valid.ye.test-certs.letsencrypt.org", TODAS, true, "cadeia da YE"},
    {"expired.x1.test-certs.letsencrypt.org", TODAS, false, "vencido: depende do relogio"},
    {"www.google.com", TODAS, false, "outra autoridade"},
    {"revoked.x1.test-certs.letsencrypt.org", TODAS, true,
     "revogado: a placa NAO consulta revogacao"},
    // Rodada 2: uma raiz por vez.
    {"valid.x1.test-certs.letsencrypt.org", X1, true, "so a X1 (via YR cruzada)"},
    {"valid.x2.test-certs.letsencrypt.org", X2, true, "so a X2 (via YE cruzada)"},
    {"valid.yr.test-certs.letsencrypt.org", YR, true, "so a YR"},
    {"valid.ye.test-certs.letsencrypt.org", YE, true, "so a YE"},
    {"valid.ye.test-certs.letsencrypt.org", X1, false, "controle: raiz errada"},
};
static const int N_CASOS = sizeof CASOS / sizeof CASOS[0];

static int conferem = 0;
static bool preparado = false;

// Separa CA_RAIZ_PEM em uma raiz por buffer e identifica cada uma pelo CN,
// lido pelo proprio mbedTLS. Falha se nao achar exatamente as quatro.
static bool separarRaizes() {
  bool achada[N_RAIZES] = {false, false, false, false};
  const char *fim = "-----END CERTIFICATE-----\n";
  const char *p = CA_RAIZ_PEM;
  int lidas = 0;

  while (const char *q = strstr(p, "-----BEGIN CERTIFICATE-----")) {
    const char *e = strstr(q, fim);
    if (e == nullptr) break;
    e += strlen(fim);
    size_t n = e - q;
    lidas++;

    static char pem[sizeof raiz[0]];
    if (n >= sizeof pem) {
      Serial.printf("# ERRO: certificado %d com %u bytes nao cabe no buffer.\n", lidas, (unsigned) n);
      return false;
    }
    memcpy(pem, q, n);
    pem[n] = '\0';

    mbedtls_x509_crt crt;
    mbedtls_x509_crt_init(&crt);
    int r = mbedtls_x509_crt_parse(&crt, (const unsigned char *) pem, n + 1);
    char sujeito[160] = "";
    if (r == 0) mbedtls_x509_dn_gets(sujeito, sizeof sujeito, &crt.subject);
    mbedtls_x509_crt_free(&crt);
    if (r != 0) {
      Serial.printf("# ERRO: certificado %d nao e PEM valido (mbedTLS -0x%04x).\n", lidas, -r);
      return false;
    }

    int qual = -1;
    for (int i = 0; i < N_RAIZES; i++) {
      if (strstr(sujeito, CN_RAIZ[i]) != nullptr) qual = i;
    }
    if (qual < 0 || achada[qual]) {
      Serial.printf("# ERRO: certificado %d inesperado ou repetido: %s\n", lidas, sujeito);
      return false;
    }
    memcpy(raiz[qual], pem, n + 1);
    achada[qual] = true;
    Serial.printf("# raiz %s: %s\n", NOME_RAIZ[qual], sujeito);
    p = e;
  }

  for (int i = 0; i < N_RAIZES; i++) {
    if (!achada[i]) {
      Serial.printf("# ERRO: a raiz %s nao esta em CA_RAIZ_PEM (%d certificados lidos).\n",
                    NOME_RAIZ[i], lidas);
      return false;
    }
  }
  if (lidas != N_RAIZES) {
    Serial.printf("# ERRO: CA_RAIZ_PEM tem %d certificados, esperados %d.\n", lidas, N_RAIZES);
    return false;
  }
  return true;
}

static bool conectarWiFi() {
  Preferences nvs;
  if (!nvs.begin(PROV_NS, true)) {  // true = so leitura
    Serial.println("# ERRO: NVS sem o namespace do portal. Provisione o no antes.");
    return false;
  }
  String ssid = nvs.getString(PROV_K_SSID, "");
  String senha = nvs.getString(PROV_K_SENHA, "");
  nvs.end();
  if (ssid.length() == 0) {
    Serial.println("# ERRO: nenhuma rede gravada pelo portal.");
    return false;
  }

  WiFi.persistent(false);
  WiFi.mode(WIFI_STA);
  WiFi.begin(ssid.c_str(), senha.c_str());
  Serial.printf("# Wi-Fi: conectando a %s", ssid.c_str());
  unsigned long t0 = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - t0 < WIFI_TIMEOUT_MS) {
    delay(500);
    Serial.print(".");
  }
  Serial.println();
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("# ERRO: Wi-Fi nao conectou.");
    return false;
  }
  Serial.printf("# Wi-Fi conectado. IP: %s\n", WiFi.localIP().toString().c_str());
  return true;
}

// Sem relogio certo, todo certificado valido parece "ainda nao emitido" e o
// caso do vencido nao prova nada. Exige uma hora de 2026 em diante.
static bool sincronizarNTP() {
  configTime(0, 0, NTP_SERVIDOR_1, NTP_SERVIDOR_2);
  const time_t piso = 1767225600;  // 2026-01-01T00:00:00Z
  unsigned long t0 = millis();
  while (time(nullptr) < piso && millis() - t0 < NTP_TIMEOUT_MS) delay(200);
  time_t agora = time(nullptr);
  if (agora < piso) {
    Serial.println("# ERRO: NTP nao sincronizou.");
    return false;
  }
  char iso[32];
  strftime(iso, sizeof iso, "%Y-%m-%dT%H:%M:%SZ", gmtime(&agora));
  Serial.printf("# Relogio (UTC): %s\n", iso);
  return true;
}

// true se o handshake fechou: com setCACert o mbedTLS roda em
// VERIFY_REQUIRED, entao handshake fechado = cadeia validada. A linha de
// status HTTP vai junto so como evidencia de que a conexao era real.
static bool tentar(const Caso &c, String &detalhe) {
  WiFiClientSecure cliente;
  if (c.raizes == TODAS) {
    cliente.setCACert(CA_RAIZ_PEM);
  } else {
    cliente.setCACert(raiz[c.raizes]);
  }
  cliente.setHandshakeTimeout(30);

  if (!cliente.connect(c.host, 443)) {
    char erro[128] = "";
    int cod = cliente.lastError(erro, sizeof erro);
    detalhe = String("mbedTLS -0x") + String(-cod, HEX) + " " + erro;
    return false;
  }
  cliente.print(String("HEAD / HTTP/1.1\r\nHost: ") + c.host +
                "\r\nConnection: close\r\n\r\n");
  unsigned long t0 = millis();
  while (!cliente.available() && cliente.connected() && millis() - t0 < 10000) delay(10);
  detalhe = cliente.readStringUntil('\n');
  detalhe.trim();
  cliente.stop();
  return true;
}

void setup() {
  Serial.begin(115200);
  delay(1500);
  Serial.println();
  Serial.println("# TESTE TLS - raizes de CA_RAIZ_PEM (config.h.example)");

  if (!separarRaizes() || !conectarWiFi() || !sincronizarNTP()) {
    Serial.println("# Teste NAO executado.");
    return;
  }
  preparado = true;

  Serial.println("# caso | host | raizes | esperado | obtido | confere | detalhe");
  for (int i = 0; i < N_CASOS; i++) {
    const Caso &c = CASOS[i];
    String detalhe;
    bool aceitou = tentar(c, detalhe);
    bool ok = aceitou == c.aceita;
    if (ok) conferem++;
    Serial.printf("TLS %2d | %s | %s | %s | %s | %s | %s (%s)\n", i + 1, c.host,
                  c.raizes == TODAS ? "todas" : NOME_RAIZ[c.raizes],
                  c.aceita ? "ACEITA" : "RECUSA", aceitou ? "ACEITA" : "RECUSA",
                  ok ? "ok" : "DIVERGE", detalhe.c_str(), c.nota);
  }
}

void loop() {
  // O resumo se repete para quem abrir a serial depois do fim.
  if (preparado) {
    Serial.printf("# RESUMO: %d de %d casos conferem com o esperado.\n", conferem, N_CASOS);
  } else {
    Serial.println("# RESUMO: teste NAO executado (ver o motivo acima).");
  }
  delay(30000);
}
