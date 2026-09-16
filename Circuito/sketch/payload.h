// Montagem do JSON de telemetria e leitura da resposta do servidor.
//
// C puro, sem nenhuma dependencia do Arduino, de proposito: o mesmo codigo
// que roda no ESP32 compila no host (ver Circuito/teste/teste_payload.c).
// Assim erro de formato de timestamp, nome de campo ou formatacao de float
// e detectado contra o backend real antes de subir para o hardware.
//
// Contrato em Backend/README.md. Campos obrigatorios pelo servidor:
// seq, measured_at, raw_mv, kpa, calibration_id.

#ifndef PAYLOAD_H
#define PAYLOAD_H

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

// Formata epoch em ISO 8601 UTC (2026-08-19T14:32:10Z), que e o formato
// que o backend espera em measured_at. Devolve 0 em caso de falha.
static int formatarInstanteUTC(time_t quando, char *saida, size_t tamanho) {
  struct tm decomposto;
  if (gmtime_r(&quando, &decomposto) == NULL) {
    return 0;
  }
  return strftime(saida, tamanho, "%Y-%m-%dT%H:%M:%SZ", &decomposto) > 0;
}

// Monta o objeto JSON de uma leitura. Devolve o numero de bytes escritos,
// ou 0 se nao coube (snprintf trunca em silencio; aqui isso vira erro).
static int montarLeitura(char *saida, size_t tamanho,
                         long long seq, time_t medidoEm,
                         int rawMV, int vddMV, float kPa,
                         const char *calibrationId) {
  char instante[32];
  if (!formatarInstanteUTC(medidoEm, instante, sizeof instante)) {
    return 0;
  }
  int n = snprintf(saida, tamanho,
                   "{\"seq\":%lld,\"measured_at\":\"%s\",\"raw_mv\":%d,"
                   "\"vdd_mv\":%d,\"kpa\":%.2f,\"calibration_id\":\"%s\"}",
                   seq, instante, rawMV, vddMV, (double) kPa, calibrationId);
  return (n > 0 && (size_t) n < tamanho) ? n : 0;
}

// Envolve um ou mais objetos ja montados (separados por virgula) no lote.
static int montarLote(char *saida, size_t tamanho,
                      const char *deviceId, const char *leituras) {
  int n = snprintf(saida, tamanho, "{\"device_id\":\"%s\",\"readings\":[%s]}",
                   deviceId, leituras);
  return (n > 0 && (size_t) n < tamanho) ? n : 0;
}

// Le um campo inteiro da resposta do servidor. Devolve padrao se ausente.
//
// ponytail: strstr no lugar de um parser JSON. A resposta e gerada pelo
// nosso proprio backend e tem tres inteiros de topo (accepted, duplicates,
// rejected); nenhum deles aparece aninhado. Se a resposta ganhar estrutura,
// trocar por um parser de verdade.
static int lerCampoInteiro(const char *json, const char *campo, int padrao) {
  char alvo[32];
  if (snprintf(alvo, sizeof alvo, "\"%s\":", campo) <= 0) {
    return padrao;
  }
  const char *p = strstr(json, alvo);
  if (p == NULL) {
    return padrao;
  }
  return atoi(p + strlen(alvo));
}

#endif  // PAYLOAD_H
