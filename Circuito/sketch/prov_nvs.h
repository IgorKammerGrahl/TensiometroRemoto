// Configuracao do no persistida na NVS.
//
// Estes sao os valores que deixaram de morar em config.h: segredo e
// instalacao ficam na NVS, definidos pelo portal; fisica e engenharia
// (pinos, coeficientes, intervalos, CA raiz) seguem em config.h.
//
// Namespace "tensio" e o mesmo que ja guarda o seq -- de proposito: e a
// gravacao do device_id, feita aqui, que decide se aquele seq continua
// valendo.

#ifndef PROV_NVS_H
#define PROV_NVS_H

#include <Arduino.h>
#include <Preferences.h>

#include "prov_valida.h"

// Chaves da NVS tem no maximo 15 caracteres.
#define PROV_NS        "tensio"
#define PROV_K_SSID    "ssid"
#define PROV_K_SENHA   "senha"
#define PROV_K_URL     "url"
#define PROV_K_DEVICE  "dev_id"
#define PROV_K_TOKEN   "token"
#define PROV_K_CALIB   "calib"
#define PROV_K_SEQ     "seq"

// Marca de configuracao completa. A NVS nao tem transacao: cada putString e
// uma escrita independente, e uma falha no meio deixaria campos de duas
// identidades misturados. Esta chave e o commit -- sai antes das escritas e
// so volta quando todas passaram.
#define PROV_K_OK      "ok"

struct ProvConfig {
  String ssid;
  String senha;
  String url;
  String deviceId;
  String token;
  String calibracao;
};

// Carrega a configuracao. Devolve true so se ela esta COMPLETA.
//
// Nao existe "meio configurado": faltando qualquer campo obrigatorio o
// boot vai para o portal, e nao para uma telemetria que falharia de um
// jeito dificil de ler no serial -- ou, pior, em campo, sem serial.
static bool provCarregar(ProvConfig &c) {
  Preferences nvs;
  if (!nvs.begin(PROV_NS, true)) {
    return false;
  }
  c.ssid = nvs.getString(PROV_K_SSID, "");
  c.senha = nvs.getString(PROV_K_SENHA, "");
  c.url = nvs.getString(PROV_K_URL, "");
  c.deviceId = nvs.getString(PROV_K_DEVICE, "");
  c.token = nvs.getString(PROV_K_TOKEN, "");
  c.calibracao = nvs.getString(PROV_K_CALIB, "");
  bool completa = nvs.getBool(PROV_K_OK, false);
  nvs.end();

  // A marca sozinha nao basta: ela diz que a ultima gravacao terminou, e os
  // comprimentos dizem que ha o que usar. Campo vazio com a marca presente
  // nao deveria existir -- se existir, o portal e o lugar certo de parar.
  return completa && c.ssid.length() > 0 && c.url.length() > 0 &&
         c.deviceId.length() > 0 && c.token.length() > 0 &&
         c.calibracao.length() > 0;
}

// Grava a configuracao. Se a identidade gravada mudou -- ou se nao da para
// saber qual era --, remove o seq: ver a justificativa em provDeveZerarSeq.
static bool provGravar(const ProvConfig &c) {
  Preferences nvs;
  if (!nvs.begin(PROV_NS, false)) {
    return false;
  }

  // A identidade anterior tem de ser lida ANTES das escritas: depois de
  // gravar o dev_id novo nao existe mais com o que comparar.
  String anterior = nvs.getString(PROV_K_DEVICE, "");

  // Anterior vazia e conhecimento desta camada, nao de provDeveZerarSeq: o
  // firmware antigo gravava seq neste mesmo namespace sem nunca gravar
  // dev_id, entao um seq orfao pode pertencer a outra identidade. Sem saber
  // de quem ele e, zerar e a escolha que falha fechado -- herdar a contagem
  // faria o backend contabilizar o lote novo como duplicates, em silencio.
  // Em placa virgem nao ha chave seq e o remove nao faz nada.
  bool zerarSeq = anterior.length() == 0 ||
                  provDeveZerarSeq(anterior.c_str(), c.deviceId.c_str());

  // A marca cai ANTES de qualquer escrita. Deste ponto ate o fim a NVS esta
  // declaradamente incompleta: se faltar energia no meio, o boot seguinte
  // encontra a marca ausente e vai para o portal, em vez de subir em
  // telemetria com token de uma identidade e dev_id de outra -- que nao
  // falha no boot, falha com 401 em campo, para sempre e sem serial.
  nvs.remove(PROV_K_OK);

  // O dev_id NAO entra aqui: ele e o ultimo a ser gravado. Enquanto nao for,
  // `anterior` continua verdadeira, e uma tentativa que falhe no meio deixa
  // a identidade antiga intacta para o retry comparar.
  bool ok = nvs.putString(PROV_K_SSID, c.ssid) > 0 &&
            nvs.putString(PROV_K_URL, c.url) > 0 &&
            nvs.putString(PROV_K_TOKEN, c.token) > 0 &&
            nvs.putString(PROV_K_CALIB, c.calibracao) > 0;

  // Rede aberta tem senha vazia, e putString de string vazia devolve 0.
  // Tratar isso como falha recusaria justamente a rede sem senha.
  nvs.putString(PROV_K_SENHA, c.senha);

  // O seq so sai DEPOIS de as escritas terem passado. A cadeia && acima
  // curto-circuita: se uma falhou, a configuracao ANTIGA continua valendo, e
  // o seq dela precisa continuar intacto.
  if (ok && zerarSeq) {
    nvs.remove(PROV_K_SEQ);
    Serial.println("# NVS: identidade nova ou desconhecida -- seq zerado.");
  }

  // A identidade por ultimo, e a marca depois dela: nesta ordem, nenhum boot
  // ve um dev_id novo com o resto pela metade.
  ok = ok && nvs.putString(PROV_K_DEVICE, c.deviceId) > 0;
  if (ok) {
    nvs.putBool(PROV_K_OK, true);
  }

  nvs.end();
  return ok;
}

#endif  // PROV_NVS_H
