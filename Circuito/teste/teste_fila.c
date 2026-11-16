// Verificacao da fila de leituras pendentes (RNF04) no host, sem hardware.
//
// Compila o MESMO fila.h que o ESP32 usa:
//
//   cd Circuito/teste
//   gcc -Wall -Wextra -o teste_fila teste_fila.c && ./teste_fila
#include <assert.h>
#include <stdio.h>
#include <string.h>
#include "../sketch/fila.h"

static LeituraPendente lp(long long seq) {
  LeituraPendente l = {seq, (time_t) 1794853000 + seq * 60, 3000 - (int) seq, -10.5f - (float) seq};
  return l;
}

static void teste_ordem_e_descarte(void) {
  LeituraPendente mem[4];
  Fila f;
  filaIniciar(&f, mem, 4);
  assert(f.tamanho == 0);

  for (long long s = 1; s <= 3; s++) assert(filaEmpurrar(&f, lp(s)) == 0);
  assert(f.tamanho == 3);

  // Espiar nao remove, e devolve da mais antiga para a mais nova.
  LeituraPendente lote[8];
  assert(filaEspiar(&f, lote, 2) == 2);
  assert(lote[0].seq == 1 && lote[1].seq == 2);
  assert(f.tamanho == 3);

  // So o servidor confirmar tira da fila.
  filaDescartar(&f, 2);
  assert(f.tamanho == 1);
  assert(filaEspiar(&f, lote, 8) == 1 && lote[0].seq == 3);

  // Descartar mais do que ha nao estoura.
  filaDescartar(&f, 99);
  assert(f.tamanho == 0);
  assert(filaEspiar(&f, lote, 8) == 0);
  puts("ok  ordem e descarte");
}

// Cheia, sai a MAIS ANTIGA: a leitura de agora vale mais para decidir a
// irrigacao. A que saiu vira buraco na sequencia, e o contador diz quantas.
static void teste_transbordo(void) {
  LeituraPendente mem[3];
  Fila f;
  filaIniciar(&f, mem, 3);
  for (long long s = 1; s <= 3; s++) assert(filaEmpurrar(&f, lp(s)) == 0);
  assert(filaEmpurrar(&f, lp(4)) == 1);
  assert(filaEmpurrar(&f, lp(5)) == 1);
  assert(f.tamanho == 3 && f.descartadas == 2);

  LeituraPendente lote[3];
  assert(filaEspiar(&f, lote, 3) == 3);
  assert(lote[0].seq == 3 && lote[1].seq == 4 && lote[2].seq == 5);

  // A volta do anel continua certa depois de descartar e empurrar de novo.
  filaDescartar(&f, 2);
  assert(filaEmpurrar(&f, lp(6)) == 0);
  assert(filaEspiar(&f, lote, 3) == 2);
  assert(lote[0].seq == 5 && lote[1].seq == 6);
  puts("ok  transbordo descarta a mais antiga");
}

// O corpo do POST com varias leituras, cortado onde o buffer acaba: o que nao
// coube fica na fila para o proximo lote, em vez de virar JSON truncado.
static void teste_montar_leituras(void) {
  LeituraPendente ls[3] = {lp(7), lp(8), lp(9)};
  char saida[1024];

  size_t n = montarLeituras(saida, sizeof saida, ls, 3, 5000, "cal-x");
  assert(n == 3);
  assert(strncmp(saida, "{\"seq\":7,", 9) == 0);
  assert(strstr(saida, "},{\"seq\":8,") != NULL);
  assert(strstr(saida, "},{\"seq\":9,") != NULL);
  assert(saida[strlen(saida) - 1] == '}');

  // Buffer para uma leitura e pouco: entra uma, inteira, e nada mais.
  char pequeno[200];
  n = montarLeituras(pequeno, sizeof pequeno, ls, 3, 5000, "cal-x");
  assert(n == 1);
  assert(strchr(pequeno, ',') != NULL);   // virgulas internas do objeto
  assert(strstr(pequeno, "},{") == NULL);  // mas nenhum segundo objeto
  assert(pequeno[strlen(pequeno) - 1] == '}');

  // Nem uma cabe: zero, e a saida fica vazia.
  char minusculo[16];
  assert(montarLeituras(minusculo, sizeof minusculo, ls, 3, 5000, "cal-x") == 0);
  assert(minusculo[0] == '\0');
  puts("ok  montar leituras");
}

// O lote inteiro, pronto para o backend: a ultima linha da saida pode ser
// colada num curl contra o servidor real (ver teste_payload.c).
static void teste_lote_completo(void) {
  LeituraPendente ls[2] = {lp(10), lp(11)};
  char leituras[512], corpo[640];
  assert(montarLeituras(leituras, sizeof leituras, ls, 2, 5000, "cal-x") == 2);
  assert(montarLote(corpo, sizeof corpo, "tensio-01", leituras) > 0);
  assert(strncmp(corpo, "{\"device_id\":\"tensio-01\",\"readings\":[{", 38) == 0);
  assert(strcmp(corpo + strlen(corpo) - 2, "]}") == 0);
  puts("ok  lote completo");
  puts(corpo);
}

// So FALHOU guarda o lote. Errar para o lado de descartar perderia dado por
// um soluco de rede; errar para o lado de guardar travaria a fila num 4xx.
static void teste_resultado_do_status(void) {
  assert(resultadoDoStatus(200) == ENTREGUE);
  assert(resultadoDoStatus(400) == RECUSADO);
  assert(resultadoDoStatus(401) == RECUSADO);  // token errado
  assert(resultadoDoStatus(403) == RECUSADO);  // no desativado
  assert(resultadoDoStatus(500) == FALHOU);
  assert(resultadoDoStatus(503) == FALHOU);
  assert(resultadoDoStatus(308) == FALHOU);    // redirecionamento: configuracao
  assert(resultadoDoStatus(-1) == FALHOU);     // HTTPC_ERROR_CONNECTION_REFUSED
  assert(resultadoDoStatus(-11) == FALHOU);    // HTTPC_ERROR_READ_TIMEOUT
  puts("ok  resultado do status");
}

int main(void) {
  (void) lerCampoInteiro;  // vem do payload.h; aqui nao se usa
  teste_ordem_e_descarte();
  teste_transbordo();
  teste_montar_leituras();
  teste_resultado_do_status();
  teste_lote_completo();
  return 0;
}
