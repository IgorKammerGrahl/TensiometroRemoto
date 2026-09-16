// Verificacao do payload no host, sem hardware.
//
// Compila o MESMO payload.h que o ESP32 usa. Se o formato do timestamp, o
// nome de um campo ou a formatacao do float quebrarem, quebra aqui, em
// segundos, e nao numa gravacao de firmware.
//
//   cd Circuito/teste
//   gcc -Wall -Wextra -o teste_payload teste_payload.c && ./teste_payload
//
// A ultima linha da saida e um lote JSON pronto para conferir contra o
// backend real:
//
//   LOTE=$(./teste_payload | tail -1)
//   curl -sS -X POST http://localhost:8080/api/v1/readings
//        -H "Authorization: Bearer $TOKEN"
//        -H "Content-Type: application/json" -d "$LOTE"

#include <assert.h>
#include <stdio.h>
#include <string.h>

#include "../sketch/payload.h"

// Reproduz a cadeia de conversao do sketch para conferir que o kpa que vai
// no JSON e o mesmo que sai da bancada.
static float paraKPa(int mvPino) {
  const float fatorDivisor = 1.5f;
  const float vZero = 4.5f;
  const float k = 0.04f;
  float vSensor = (mvPino / 1000.0f) * fatorDivisor;
  return (vSensor - vZero) / k;
}

int main(void) {
  char leitura[256];
  char lote[384];

  // 2026-08-19T14:32:10Z
  const time_t QUANDO = 1787149930;

  // ---- Timestamp em UTC ----
  char instante[32];
  assert(formatarInstanteUTC(QUANDO, instante, sizeof instante));
  assert(strcmp(instante, "2026-08-19T14:32:10Z") == 0);

  // ---- Leitura real de bancada, tubo aberto para a atmosfera ----
  // 3016 mV no pino -> 4,524 V no sensor -> +0,60 kPa
  float kPa = paraKPa(3016);
  assert(kPa > 0.59f && kPa < 0.61f);

  int n = montarLeitura(leitura, sizeof leitura, 42, QUANDO, 3016, 5000, kPa,
                        "cal-2026-08-20-a");
  assert(n > 0);
  assert(strcmp(leitura,
                "{\"seq\":42,\"measured_at\":\"2026-08-19T14:32:10Z\","
                "\"raw_mv\":3016,\"vdd_mv\":5000,\"kpa\":0.60,"
                "\"calibration_id\":\"cal-2026-08-20-a\"}") == 0);

  // ---- Succao: kpa negativo nao pode virar notacao cientifica nem perder sinal
  int nNeg = montarLeitura(leitura, sizeof leitura, 43, QUANDO, 2940, 5000,
                           paraKPa(2940), "cal-2026-08-20-a");
  assert(nNeg > 0);
  assert(strstr(leitura, "\"kpa\":-2.25") != NULL);

  // ---- Buffer curto demais e erro, nao truncamento silencioso ----
  char apertado[16];
  assert(montarLeitura(apertado, sizeof apertado, 42, QUANDO, 3016, 5000, kPa,
                       "cal-2026-08-20-a") == 0);

  // ---- Lote ----
  assert(montarLote(lote, sizeof lote, "tensio-01", leitura) > 0);
  assert(strncmp(lote, "{\"device_id\":\"tensio-01\",\"readings\":[{", 37) == 0);
  assert(lote[strlen(lote) - 2] == ']');

  // ---- Leitura da resposta do servidor ----
  const char *ok =
      "{\"accepted\":1,\"duplicates\":0,\"rejected\":0,\"rejections\":[],"
      "\"server_time\":\"2026-08-19T14:32:11Z\"}";
  assert(lerCampoInteiro(ok, "accepted", -1) == 1);
  assert(lerCampoInteiro(ok, "duplicates", -1) == 0);
  assert(lerCampoInteiro(ok, "rejected", -1) == 0);
  assert(lerCampoInteiro(ok, "inexistente", -1) == -1);

  const char *dup = "{\"accepted\":0,\"duplicates\":1,\"rejected\":0}";
  assert(lerCampoInteiro(dup, "duplicates", -1) == 1);

  printf("todos os testes passaram\n");

  // Lote com a leitura real de bancada, para conferir contra o backend.
  montarLeitura(leitura, sizeof leitura, 42, QUANDO, 3016, 5000, paraKPa(3016),
                "cal-2026-08-20-a");
  montarLote(lote, sizeof lote, "tensio-01", leitura);
  printf("%s\n", lote);
  return 0;
}
