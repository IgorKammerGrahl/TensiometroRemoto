// Geometria da regua de escala.
//
// A REGUA TEM DOIS ELEMENTOS SEPARAVEIS E ELES FALHAM SEPARADAMENTE.
//
//   1. a escala (0 a -80 kPa) e a posicao da leitura nela -- medicao pura,
//      nao depende de limiar nenhum;
//   2. as bandas de conforto/alerta/estresse -- classificacao, depende dos
//      dois limiares do talhao.
//
// Talhao sem faixa configurada perde (2) e MANTEM (1). Suprimir a regua
// inteira descartaria junto a posicao, que continua sendo dado que o servidor
// mandou: o produtor ainda le "perto de zero" ou "perto do fundo de escala"
// sem saber em que zona isso cai para a cultura dele. O que nao pode acontecer
// e inventar fronteira onde nao ha limiar -- e por isso bandas() devolve lista
// vazia em vez de um palpite. Mesma logica da hachura `vazado` em zona.ts:
// o contorno existe, o preenchimento e que falta.
//
// (Decisao de 02/09. A alternativa considerada -- nao desenhar nada -- foi
// descartada porque so ela, em toda a interface, faria falta de configuracao
// remover um componente em vez de marca-lo.)

/** Topo e fundo da escala, em kPa.
 *
 * NAO SAO NUMEROS ESCOLHIDOS POR GOSTO: sao exatamente o dominio em que os
 * limiares podem existir. A migration 0002 restringe kpa_alerta e
 * kpa_estresse a BETWEEN -80 AND 0, entao toda fronteira de zona desenhavel
 * cai dentro desta janela, sempre.
 *
 * As LEITURAS tem faixa mais larga -- readings.kpa aceita BETWEEN -100 AND 10
 * (migration 0001; kPaMin/kPaMax em http.go) -- entao leitura fora da escala
 * nao e hipotese: e o solo encharcado acima de 0 e o tensiometro cavitado
 * abaixo de -80. Por isso posicao() grampeia E AVISA que grampeou, em vez de
 * deixar o entalhe sair da regua em silencio.
 *
 * Escala FIXA de proposito. Uma escala que se ajustasse ao talhao faria duas
 * telas do mesmo aplicativo nao serem comparaveis entre si -- e a mesma altura
 * de entalhe significaria umidades diferentes em nos diferentes.
 */
export const TOPO_KPA = 0;
export const FUNDO_KPA = -80;

export type ForaDeEscala = 'acima' | 'abaixo' | null;

export type Posicao = {
  /** 0 = topo (0 kPa, umido) · 1 = fundo (-80 kPa, seco).
   *  PIOR PARA BAIXO: a fracao cresce junto com a severidade, o que deixa o
   *  mapeamento para pixel ser uma multiplicacao direta pela altura, sem um
   *  `1 -` no meio que alguem inverteria depois. */
  readonly fracao: number;
  /** Nao-nulo quando a leitura saiu da janela desenhavel. A regua marca o
   *  entalhe na borda, e quem desenha precisa dizer que ele esta na borda
   *  porque foi grampeado -- nao porque a leitura vale exatamente 0 ou -80. */
  readonly fora: ForaDeEscala;
};

/** Posicao da leitura na regua, ou null se o valor nao for um numero.
 *
 * Devolver null em vez de NaN e deliberado: NaN se propaga por toda a
 * aritmetica de layout e sai como um `<rect y="NaN">` que o navegador ignora
 * calado -- o entalhe simplesmente nao aparece e nada acusa por que. Com null,
 * o TypeScript obriga quem chama a decidir o que mostrar. */
export function posicao(kpa: number): Posicao | null {
  if (!Number.isFinite(kpa)) return null;

  if (kpa > TOPO_KPA) return { fracao: 0, fora: 'acima' };
  if (kpa < FUNDO_KPA) return { fracao: 1, fora: 'abaixo' };

  return { fracao: fracaoDe(kpa), fora: null };
}

/** kPa dentro da escala -> fracao. UM SO LUGAR, usado tambem por bandas().
 *
 * Nao e economia de linha: e o que garante que a fronteira de uma banda e a
 * posicao do entalhe no mesmo limiar sejam o MESMO numero. Duas expressoes
 * equivalentes divergiriam no dia em que uma delas ganhasse um ajuste de meio
 * pixel, e a regua passaria a mostrar o entalhe fora da banda que o
 * classifica.
 *
 * Os dois operandos sao negativos (ou zero), entao a divisao ja sai positiva
 * e crescente para baixo -- nao ha inversao de sinal escondida aqui. O `+ 0`
 * normaliza o -0 que `0 / -80` produz: invisivel no SVG, mas -0 nao e igual a
 * 0 sob Object.is, e um dia isso vira um teste falhando pelo motivo errado. */
function fracaoDe(kpa: number): number {
  return kpa / FUNDO_KPA + 0;
}

export type ChaveBanda = 'conforto' | 'alerta' | 'estresse';

export type Banda = {
  readonly chave: ChaveBanda;
  /** Fracoes, topo e base. `de` < `ate` sempre. */
  readonly de: number;
  readonly ate: number;
};

/** As tres bandas, ou lista vazia quando nao ha faixa desenhavel.
 *
 * LISTA VAZIA E O CAMINHO DE FALHA E O CAMINHO NORMAL AO MESMO TEMPO, de
 * proposito: quem desenha so precisa saber "tem banda ou nao tem", e o caso
 * de limiar ausente e o de limiar corrompido produzem a mesma regua honesta.
 *
 * A guarda repete a restricao do banco (talhoes_faixas_ck: os dois nulos ou
 * os dois preenchidos, kpa_estresse < kpa_alerta, ambos entre -80 e 0) porque
 * o banco protege a tabela e nao o fio. Se um dia chegar um par invertido, o
 * desenho ingenuo pintaria conforto sobre a faixa de estresse -- verde onde
 * deveria estar vermelho, que e exatamente o erro que a interface inteira
 * existe para nao cometer. Sem faixa confiavel, nenhuma faixa.
 *
 * NaN reprova toda comparacao, entao cai na lista vazia sem tratamento
 * especial -- a condicao esta escrita como "e desenhavel?", nunca como
 * "e invalido?". */
export function bandas(
  kpaAlerta: number | null,
  kpaEstresse: number | null,
): Banda[] {
  if (kpaAlerta === null || kpaEstresse === null) return [];

  const desenhavel =
    FUNDO_KPA <= kpaEstresse && kpaEstresse < kpaAlerta && kpaAlerta <= TOPO_KPA;
  if (!desenhavel) return [];

  const a = fracaoDe(kpaAlerta);
  const e = fracaoDe(kpaEstresse);

  return [
    { chave: 'conforto', de: 0, ate: a },
    { chave: 'alerta', de: a, ate: e },
    { chave: 'estresse', de: e, ate: 1 },
  ];
}

/** kPa como o produtor le: uma casa decimal e SINAL EXPLICITO no positivo.
 *
 * Quase toda leitura e negativa, e essa uniformidade e justamente o risco:
 * quem passa o olho por uma coluna de "-31,7", "-49,8", "-68,4" le o proximo
 * numero ja esperando o menos, e "0,6" vira "-0,6" na leitura rapida. Os dois
 * ficam perto de zero na escala, mas significam coisas opostas -- um e solo
 * saturado ou sensor fora do lugar, o outro e solo comecando a secar.
 *
 * `signDisplay: 'exceptZero'` e do Intl, nao invencao local: poe o sinal nos
 * dois lados e deixa o zero sem sinal, que e o certo -- zero nao e positivo
 * nem negativo, e -0 (que `0 / -80` produz, ver fracaoDe) tambem sai sem.
 *
 * Vive aqui porque este e o modulo que ja e dono do eixo de kPa. Estava
 * copiado em quatro telas, que e o numero de lugares onde uma regra de sinal
 * seria aplicada em tres e esquecida no quarto. */
export function textoKpa(kpa: number): string {
  return kpa.toLocaleString('pt-BR', { maximumFractionDigits: 1, signDisplay: 'exceptZero' });
}
