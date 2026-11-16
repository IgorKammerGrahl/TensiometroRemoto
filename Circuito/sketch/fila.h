// Fila de leituras pendentes: o que o no guarda enquanto a rede esta fora
// (RNF04).
//
// C puro, sem nenhuma dependencia do Arduino, pelo mesmo motivo do
// payload.h: o mesmo codigo compila no host e e testado la, em segundos
// (Circuito/teste/teste_fila.c).
//
// AS QUATRO DECISOES, E POR QUE.
//
// 1. RAM, e nao flash. Cobre o caso do requisito -- a rede cai com o no
//    ligado -- sem escrever na flash a cada leitura. Perde-se num reinicio
//    ou numa falta de energia; guardar em LittleFS e o passo seguinte, e fica
//    registrado como limite.
//
// 2. Cheia, sai a MAIS ANTIGA. Para decidir a irrigacao, a leitura de agora
//    vale mais que a de doze horas atras. A que saiu vira buraco na sequencia,
//    que e o diagnostico que o projeto ja usa para perda; `descartadas` conta
//    quantas.
//
// 3. So o servidor tira da fila. O envio ESPIA um lote; a remocao vem depois
//    da resposta. Se a resposta se perder depois de o servidor gravar, o lote
//    volta no ciclo seguinte e o servidor o reconhece como duplicado pelo par
//    (no, seq) -- a idempotencia que o backend ja tem e o que torna o reenvio
//    seguro.
//
// 4. Sem concorrencia. O loop do Arduino empurra e drena na mesma tarefa, nunca
//    ao mesmo tempo; por isso nao ha trava.

#ifndef FILA_H
#define FILA_H

#include <stddef.h>
#include <time.h>
#include "payload.h"

typedef struct {
  long long seq;
  time_t medidoEm;  // carimbado na medicao, nao no envio
  int rawMV;
  float kPa;
} LeituraPendente;

typedef struct {
  LeituraPendente *itens;
  size_t capacidade;
  size_t inicio;   // indice da mais antiga
  size_t tamanho;
  unsigned long descartadas;  // por transbordo, desde o boot
} Fila;

static void filaIniciar(Fila *f, LeituraPendente *armazenamento, size_t capacidade) {
  f->itens = armazenamento;
  f->capacidade = capacidade;
  f->inicio = 0;
  f->tamanho = 0;
  f->descartadas = 0;
}

// Enfileira no fim. Cheia, descarta a mais antiga e devolve 1; senao, 0.
static int filaEmpurrar(Fila *f, LeituraPendente l) {
  int descartou = 0;
  if (f->tamanho == f->capacidade) {
    f->inicio = (f->inicio + 1) % f->capacidade;
    f->tamanho--;
    f->descartadas++;
    descartou = 1;
  }
  f->itens[(f->inicio + f->tamanho) % f->capacidade] = l;
  f->tamanho++;
  return descartou;
}

// Copia ate `n` das mais antigas, em ordem, SEM remover. Devolve quantas.
static size_t filaEspiar(const Fila *f, LeituraPendente *saida, size_t n) {
  size_t k = n < f->tamanho ? n : f->tamanho;
  for (size_t i = 0; i < k; i++) {
    saida[i] = f->itens[(f->inicio + i) % f->capacidade];
  }
  return k;
}

// Remove as `n` mais antigas: chamado so depois de o servidor responder.
static void filaDescartar(Fila *f, size_t n) {
  if (n > f->tamanho) n = f->tamanho;
  f->inicio = (f->inicio + n) % f->capacidade;
  f->tamanho -= n;
}

// O que fazer com um lote depois da resposta. So FALHOU guarda.
typedef enum {
  ENTREGUE,  // 200: aceitas, duplicadas e recusadas saem todas da fila --
             // reenviar uma recusada daria a mesma recusa, para sempre
  RECUSADO,  // 4xx: o servidor recusou o lote inteiro; repetir o mesmo
             // pedido da a mesma resposta (token errado, no desativado...)
  FALHOU     // sem resposta, 5xx ou redirecionamento: guarda, e o proximo
             // ciclo tenta de novo
} Resultado;

// `codigo` e o do HTTPClient: negativo quando nem houve resposta. Um 3xx cai
// em FALHOU de proposito: e configuracao errada (URL http atras de um proxy
// que redireciona para https), e descartar o dado por isso seria pior que
// guardar ate alguem corrigir.
static Resultado resultadoDoStatus(int codigo) {
  if (codigo == 200) return ENTREGUE;
  if (codigo >= 400 && codigo < 500) return RECUSADO;
  return FALHOU;
}

// Monta os objetos JSON de ate `n` leituras, separados por virgula, para o
// montarLote do payload.h. Para no primeiro que nao couber e devolve quantos
// entraram: o resto fica na fila para o proximo lote, em vez de virar JSON
// truncado. Nenhum coube: devolve 0 e deixa `saida` vazia.
static size_t montarLeituras(char *saida, size_t tamanho, const LeituraPendente *ls,
                             size_t n, int vddMV, const char *calibrationId) {
  size_t usado = 0;
  size_t k = 0;
  if (tamanho > 0) saida[0] = '\0';
  for (; k < n; k++) {
    size_t sep = (k > 0) ? 1 : 0;
    if (usado + sep >= tamanho) break;
    int escrito = montarLeitura(saida + usado + sep, tamanho - usado - sep, ls[k].seq,
                                ls[k].medidoEm, ls[k].rawMV, vddMV, ls[k].kPa, calibrationId);
    if (escrito == 0) break;
    if (sep) saida[usado] = ',';
    usado += sep + (size_t) escrito;
  }
  // Um objeto que nao coube pode ter deixado lixo depois do ultimo inteiro;
  // corta exatamente ao fim do que foi aceito.
  if (usado < tamanho) saida[usado] = '\0';
  return k;
}

#endif  // FILA_H
