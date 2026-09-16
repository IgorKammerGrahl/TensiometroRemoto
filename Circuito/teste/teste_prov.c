// Verificacao do provisionamento no host, sem hardware.
//
// Compila o MESMO prov_valida.h que o ESP32 usa. O que esta sob teste e a
// fronteira de confianca: um SSID de terceiro que chega ao HTML da pagina,
// um token colado pela metade que nao pode ser gravado, e a regra que
// impede o token gravado de ser reaproveitado para um destino novo.
//
//   cd Circuito/teste
//   gcc -Wall -Wextra -o teste_prov teste_prov.c && ./teste_prov

#include <assert.h>
#include <stdio.h>
#include <string.h>

#include "../sketch/prov_valida.h"

static const char *TOKEN_OK = "0123456789012345678901234567890123456789012";

// Destino ja gravado na NVS, usado como "atual" nos casos que simulam um no
// que ja estava em producao.
static const char *URL_GRAVADA = "https://a.b/api/v1/readings";
static const char *DEV_GRAVADO = "tensio-01";

int main(void) {
  char buf[256];
  int manter;

  // ---- Escape de HTML: o SSID vem de terceiro ----
  assert(provEscaparHtml(buf, sizeof buf, "rede normal") == 11);
  assert(strcmp(buf, "rede normal") == 0);

  assert(provEscaparHtml(buf, sizeof buf, "<script>") > 0);
  assert(strcmp(buf, "&lt;script&gt;") == 0);

  assert(provEscaparHtml(buf, sizeof buf, "a&b") > 0);
  assert(strcmp(buf, "a&amp;b") == 0);

  assert(provEscaparHtml(buf, sizeof buf, "asp\"as") > 0);
  assert(strcmp(buf, "asp&quot;as") == 0);

  assert(provEscaparHtml(buf, sizeof buf, "o'brien") > 0);
  assert(strcmp(buf, "o&#39;brien") == 0);

  // Ordem importa: o & ja escapado nao pode ser reescapado.
  assert(provEscaparHtml(buf, sizeof buf, "&lt;") > 0);
  assert(strcmp(buf, "&amp;lt;") == 0);

  // Buffer pequeno e erro, nunca truncagem silenciosa.
  char curto[4];
  assert(provEscaparHtml(curto, sizeof curto, "<script>") == -1);
  assert(curto[0] == '\0');

  // ---- Token ----
  assert(strlen(TOKEN_OK) == PROV_TOKEN_LEN);
  assert(provValidar("rede", URL_GRAVADA, DEV_GRAVADO, TOKEN_OK, "cal-1",
                     NULL, NULL, 0, &manter) == PROV_OK);
  assert(manter == 0);

  // 42 e 44 caracteres: colagem truncada ou com sujeira.
  char curto42[PROV_TOKEN_LEN];
  memcpy(curto42, TOKEN_OK, PROV_TOKEN_LEN - 1);
  curto42[PROV_TOKEN_LEN - 1] = '\0';
  assert(provValidar("rede", "https://a.b/x", "d", curto42, "c",
                     NULL, NULL, 0, &manter) == PROV_TOKEN_TAMANHO);

  char longo44[PROV_TOKEN_LEN + 2];
  memcpy(longo44, TOKEN_OK, PROV_TOKEN_LEN);
  longo44[PROV_TOKEN_LEN] = 'A';
  longo44[PROV_TOKEN_LEN + 1] = '\0';
  assert(provValidar("rede", "https://a.b/x", "d", longo44, "c",
                     NULL, NULL, 0, &manter) == PROV_TOKEN_TAMANHO);

  // '+', '/' e '=' sao base64 padrao, nao base64url: token errado.
  char sujo[PROV_TOKEN_LEN + 1];
  memcpy(sujo, TOKEN_OK, PROV_TOKEN_LEN + 1);
  sujo[10] = '+';
  assert(provValidar("rede", "https://a.b/x", "d", sujo, "c",
                     NULL, NULL, 0, &manter) == PROV_TOKEN_ALFABETO);
  sujo[10] = '/';
  assert(provValidar("rede", "https://a.b/x", "d", sujo, "c",
                     NULL, NULL, 0, &manter) == PROV_TOKEN_ALFABETO);
  sujo[10] = '=';
  assert(provValidar("rede", "https://a.b/x", "d", sujo, "c",
                     NULL, NULL, 0, &manter) == PROV_TOKEN_ALFABETO);

  // ---- Token em branco: "manter o que esta na NVS" ----
  //
  // O caso que a regra existe para servir: trocar so o Wi-Fi, mesmo destino.
  // Ninguem redigita 43 caracteres numa tela de celular no meio do talhao.
  assert(provValidar("outra-rede", URL_GRAVADA, DEV_GRAVADO, "", "cal-1",
                     URL_GRAVADA, DEV_GRAVADO, 0, &manter) == PROV_OK);
  assert(manter == 1);

  // A calibracao tambem pode mudar sem redigitar: ela nao muda a quem o
  // token e entregue.
  assert(provValidar("rede", URL_GRAVADA, DEV_GRAVADO, "", "cal-nova",
                     URL_GRAVADA, DEV_GRAVADO, 0, &manter) == PROV_OK);
  assert(manter == 1);

  // ---- Token em branco: o ataque que a regra fecha ----
  //
  // Apontar o no para outro servidor mantendo o token gravado faria o no
  // entregar o token REAL no primeiro POST, sem nunca exibi-lo.
  assert(provValidar("rede", "https://atacante.example/x", DEV_GRAVADO, "",
                     "cal-1", URL_GRAVADA, DEV_GRAVADO, 0, &manter)
         == PROV_TOKEN_EXIGIDO);
  assert(manter == 0);

  // Mesma ideia trocando a identidade em vez do endereco.
  assert(provValidar("rede", URL_GRAVADA, "tensio-99", "", "cal-1",
                     URL_GRAVADA, DEV_GRAVADO, 0, &manter)
         == PROV_TOKEN_EXIGIDO);

  // Basta o caminho mudar: a comparacao e da URL inteira, nao do host.
  assert(provValidar("rede", "https://a.b/api/v1/outra", DEV_GRAVADO, "",
                     "cal-1", URL_GRAVADA, DEV_GRAVADO, 0, &manter)
         == PROV_TOKEN_EXIGIDO);

  // No virgem: nao ha destino anterior, entao nao ha token a manter.
  assert(provValidar("rede", URL_GRAVADA, DEV_GRAVADO, "", "cal-1",
                     NULL, NULL, 0, &manter) == PROV_TOKEN_EXIGIDO);
  assert(provValidar("rede", URL_GRAVADA, DEV_GRAVADO, "", "cal-1",
                     "", "", 0, &manter) == PROV_TOKEN_EXIGIDO);

  // Quem tem o token digita e muda o destino que quiser. A regra pede a
  // credencial, nao proibe a operacao.
  assert(provValidar("rede", "https://atacante.example/x", "tensio-99",
                     TOKEN_OK, "cal-1", URL_GRAVADA, DEV_GRAVADO, 0, &manter)
         == PROV_OK);
  assert(manter == 0);

  // ---- Esquema da URL ----
  assert(provValidar("rede", "http://192.168.0.10:8080/x", "d", TOKEN_OK,
                     "c", NULL, NULL, 0, &manter) == PROV_URL_ESQUEMA);
  assert(provValidar("rede", "http://192.168.0.10:8080/x", "d", TOKEN_OK,
                     "c", NULL, NULL, 1, &manter) == PROV_OK);
  // https continua valendo mesmo com DEV_SEM_TLS compilado.
  assert(provValidar("rede", "https://a.b/x", "d", TOKEN_OK, "c",
                     NULL, NULL, 1, &manter) == PROV_OK);
  assert(provValidar("rede", "ftp://a.b/x", "d", TOKEN_OK, "c",
                     NULL, NULL, 1, &manter) == PROV_URL_ESQUEMA);
  // "https" sem :// nao passa por prefixo parcial.
  assert(provValidar("rede", "https:/a.b", "d", TOKEN_OK, "c",
                     NULL, NULL, 0, &manter) == PROV_URL_ESQUEMA);

  // O esquema e conferido ANTES do destino: URL invalida nao pode sair como
  // "cole o token de novo", que mandaria consertar a coisa errada.
  assert(provValidar("rede", "ftp://a.b/x", DEV_GRAVADO, "", "c",
                     URL_GRAVADA, DEV_GRAVADO, 0, &manter)
         == PROV_URL_ESQUEMA);

  // ---- device_id ----
  char id65[66];
  memset(id65, 'a', 65);
  id65[65] = '\0';
  assert(provValidar("rede", "https://a.b/x", id65, TOKEN_OK, "c",
                     NULL, NULL, 0, &manter) == PROV_DEVICE_ID_LONGO);
  id65[64] = '\0';  // exatamente 64 e o limite, e passa
  assert(provValidar("rede", "https://a.b/x", id65, TOKEN_OK, "c",
                     NULL, NULL, 0, &manter) == PROV_OK);

  // ---- Obrigatorios ----
  assert(provValidar("", "https://a.b/x", "d", TOKEN_OK, "c",
                     NULL, NULL, 0, &manter) == PROV_SSID_VAZIO);
  assert(provValidar("rede", "", "d", TOKEN_OK, "c",
                     NULL, NULL, 0, &manter) == PROV_URL_VAZIA);
  assert(provValidar("rede", "https://a.b/x", "", TOKEN_OK, "c",
                     NULL, NULL, 0, &manter) == PROV_DEVICE_ID_VAZIO);
  assert(provValidar("rede", "https://a.b/x", "d", TOKEN_OK, "",
                     NULL, NULL, 0, &manter) == PROV_CALIBRACAO_VAZIA);
  // NULL nao pode derrubar o portal.
  assert(provValidar(NULL, "https://a.b/x", "d", TOKEN_OK, "c",
                     NULL, NULL, 0, &manter) == PROV_SSID_VAZIO);

  // Toda mensagem de erro existe; nenhuma cai no generico por descuido.
  for (int e = PROV_OK; e <= PROV_CALIBRACAO_VAZIA; e++) {
    assert(provErroTexto((ProvErro) e) != NULL);
    assert(provErroTexto((ProvErro) e)[0] != '\0');
  }

  // ---- seq x device_id ----
  // Hardware que nunca teve id gravado: nada a zerar.
  assert(provDeveZerarSeq("", "tensio-01") == 0);
  assert(provDeveZerarSeq(NULL, "tensio-01") == 0);
  // Mesmo no, so trocando o Wi-Fi: o seq continua valendo.
  assert(provDeveZerarSeq("tensio-01", "tensio-01") == 0);
  // Hardware reaproveitado como outro no: o seq do antigo nao vale mais.
  assert(provDeveZerarSeq("tensio-01", "tensio-02") == 1);

  printf("teste_prov: todos os casos passaram\n");
  return 0;
}
