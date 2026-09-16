import type { UltimaLeitura, Zona } from './api';
import { formatarDuracao, idadeSegundos, leituraVelha } from './tempo';

// Situacao da leitura -- a pergunta que vem ANTES da zona.
//
// A ORDEM DAS VERIFICACOES E A REGRA. Nao reordenar sem ler isto inteiro.
//
//   1. no desativado       -> silencio esperado, nao e falha
//   2. nunca reportou      -> nao existe numero, nada abaixo se decide
//   3. leitura velha       -> existe numero, mas ele nao descreve o agora
//   4. leitura atual       -> so aqui a zona chega a tela
//
// A zona vem preenchida do servidor nos casos 3 e 4 igualmente, e e correta
// nos dois: ela classifica a medicao que existiu. O que muda e se essa
// medicao ainda vale como estado do solo. Deixar a zona passar no caso 3
// pintaria o indicador de conforto sobre um numero de seis horas atras, e o
// produtor decide irrigar a partir dele. Indicador nenhum e melhor que
// indicador verde sobre dado morto -- mesmo criterio que ja vale no backend.
//
// Cada passo pergunta "isto ainda se le como estado atual?", nunca "isto esta
// quebrado?". A diferenca importa porque a idade pode ser NaN (carimbo
// malformado) e NaN reprova toda comparacao: escrito assim, NaN cai para o
// lado seguro sozinho. Mesma disciplina de serie.ts.
//
// A INVERSAO NAO QUEBRA NADA VISIVELMENTE -- a tela continua montando, so
// passa a mentir -- e por isso a ordem esta escrita aqui e nao so no codigo.
//
// O caso 1 e o mais facil de julgar dispensavel e nao e: a ingestao recusa no
// desativado com 403 (http.go), entao um no desativado FICA velho por
// construcao. Sem este passo, desligar um no produziria, algumas horas
// depois, um alarme de "sem dados" sobre exatamente a decisao que o usuario
// tomou de proposito -- e uma viagem ate a lavoura para conferir um no que
// esta desligado porque ele mandou desligar.

export type Situacao =
  /** Desativado pelo usuario. Sem valor: o ultimo numero e historico, e
   *  mostra-lo ao lado de "desativado" convida a le-lo como atual. */
  | { readonly tipo: 'desativado' }
  /** Cadastrado e nunca reportou. DIFERENTE de 'velha': aqui o no pode nunca
   *  ter sido instalado; la ele operava e parou. Acoes diferentes. */
  | { readonly tipo: 'nunca' }
  | { readonly tipo: 'velha'; readonly idadeS: number; readonly kpa: number }
  | {
      readonly tipo: 'atual';
      readonly idadeS: number;
      readonly kpa: number;
      readonly zona: Zona;
    };

export function situacao(
  ativo: boolean,
  ultima: UltimaLeitura | null,
  fetchadoEmMs: number,
  agoraMs: number,
): Situacao {
  if (!ativo) return { tipo: 'desativado' };
  if (ultima === null) return { tipo: 'nunca' };

  const idadeS = idadeSegundos(ultima.medido_ha_s, fetchadoEmMs, agoraMs);
  if (leituraVelha(idadeS)) return { tipo: 'velha', idadeS, kpa: ultima.kpa };

  return { tipo: 'atual', idadeS, kpa: ultima.kpa, zona: ultima.zona };
}

/** Frase de idade. Uma so redacao por estado.
 *
 * Separada de situacao() porque situacao() e decisao e isto e redacao -- e
 * porque as duas telas que a usam (lista e detalhe) precisam da MESMA frase.
 * Duas redacoes para o mesmo estado seriam duas chances de uma delas soar
 * como "tudo bem". */
export function textoDeIdade(s: Situacao): string {
  switch (s.tipo) {
    case 'desativado':
      return 'Nó desativado';
    case 'nunca':
      return 'Ainda não enviou nenhuma leitura';
    case 'velha':
      return `Sem dados há ${formatarDuracao(s.idadeS)}`;
    case 'atual':
      return `há ${formatarDuracao(s.idadeS)}`;
    default: {
      const _exaustivo: never = s;
      void _exaustivo;
      return 'Estado desconhecido';
    }
  }
}
