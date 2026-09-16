// Segmentacao da serie por lacuna.
//
// O PROBLEMA: balde sem leitura nao gera ponto na resposta. Se o grafico
// ligar os pontos atraves do buraco, ele desenha uma reta onde houve
// AUSENCIA DE DADO -- inventa a medicao que faltou, e a inventa suave, sem
// nenhuma marca de que foi inventada. Um no offline por doze horas viraria
// uma interpolacao limpa entre a ultima leitura antes de cair e a primeira
// depois de voltar.
//
// BUCKET_S NAO E O PERIODO DE AMOSTRAGEM. Esta distincao custou uma tela
// errada contra o backend real e por isso esta escrita aqui.
//   bucket_s  = a resolucao com que o servidor AGREGA (escolha de exibicao;
//               hoje 60 s por padrao, para qualquer janela);
//   o periodo = de quanto em quanto tempo o NO reporta (propriedade do
//               dispositivo, que a API ainda nao expoe).
// Quando o no reporta mais devagar que o balde -- tensio-demo amostra a cada
// 15 min contra balde de 60 s --, 14 de cada 15 baldes saem vazios e TODO
// intervalo entre pontos consecutivos ultrapassa 1.5 x bucket_s. A regra
// antiga quebrava a serie em 77 trechos de um ponto e a legenda anunciava
// "76 trechos sem dados": exatamente o pecado que a segmentacao existe para
// impedir, so que invertido -- em vez de inventar dado onde faltou, inventa
// falta onde o dado esta inteiro.
//
// O periodo entao vem do PROPRIO DADO, que e o unico lugar onde ele existe
// hoje: a mediana dos intervalos observados. Nao e palpite -- e medicao do
// ritmo que a serie tem. Quando a API expuser o periodo do no (ver README,
// mesma causa raiz do limiar de leitura velha), ele substitui a mediana e
// esta funcao encolhe.

/** dt > FATOR_LACUNA * periodo e lacuna.
 *
 * 1.5 fica entre um periodo e dois: uma lacuna e o no ter PERDIDO pelo menos
 * um reporte, e perder um reporte da dt ~= 2 x periodo. A margem acima de 1.0
 * absorve o jitter real da amostragem -- o no de bancada reporta a 60 s e
 * entrega 61, 62 e 63 s com frequencia. */
export const FATOR_LACUNA = 1.5;

/** O ritmo da serie, em segundos: mediana dos intervalos observados.
 *
 * MEDIANA, nao media, e nao o maior nem o menor. A media seria puxada pela
 * propria lacuna que queremos detectar -- uma parada de 12 h no meio de dados
 * de 15 min levantaria o limite acima da parada e a linha atravessaria o
 * buraco. A mediana so se desloca se MAIS DA METADE dos intervalos forem
 * longos, e nesse caso o ritmo da serie realmente e o longo.
 *
 * Com n par toma-se a mediana INFERIOR, nao a media das duas centrais: puxa o
 * limite para baixo, e para baixo e o lado seguro (quebra a mais, nunca liga a
 * mais). Pelo mesmo motivo bucketS e piso -- serie curta demais para ter ritmo
 * cai no limite menor, que separa, em vez de num limite grande que ligaria.
 *
 * DUAS RESSALVAS CONHECIDAS. Nenhuma das duas justifica mudar o criterio hoje;
 * as duas justificam reconferir quando a condicao aparecer.
 *
 * 1. A MEDIANA E DO HISTORICO VISIVEL, E UMA JANELA PODE CONTER DOIS REGIMES.
 *    Hoje cada no tem uma cadencia so, entao a mediana descreve a serie
 *    inteira. Quando a etapa 2 do firmware trocar 60 s por deep sleep de
 *    dezenas de minutos, uma janela que cruze a transicao mistura os dois
 *    ritmos numa mediana que nao descreve nenhum dos dois. Qual lado perde
 *    depende de qual regime ocupa mais da metade da janela: mediana presa no
 *    regime rapido quebra a parte lenta em pontos soltos; presa no regime
 *    lento, atravessa lacunas reais da parte rapida -- e esse e o lado que
 *    mente. Reconferir numa janela que cruze a transicao, quando ela existir.
 *
 * 2. O BALDE TEM GRADE FIXA E A AMOSTRAGEM NAO SE ALINHA A ELA. Medido no
 *    tensio-01 em 02/09/2026: reporta a ~61 s contra balde de 60 s, todos os
 *    baldes com n=1, e a deriva acumula ate um balde ficar vazio -- 3 saltos
 *    de 120 s em 247 pontos, sem que o no tenha perdido reporte nenhum. Como
 *    perder UM reporte tambem da ~120 s, os dois casos sao indistinguiveis na
 *    serie agregada, e com FATOR_LACUNA 1.5 os dois quebram. O erro cai do
 *    lado seguro (quebra a mais, nunca liga a mais), e e o lado que fica: subir
 *    o fator para ~2.5 inverteria isso -- passaria a desenhar reta por cima de
 *    um reporte que realmente nao existiu, que a 15 min de cadencia e meia hora
 *    de dado inventado. O fator decide SE DESENHA A LINHA e continua em 1.5.
 *    O que estava errado era a legenda, que AFIRMAVA QUANTAS paradas houve --
 *    numero que ninguem tem como saber daqui. Corrigido em Grafico.tsx.
 *
 * O RESOLVEDOR DEFINITIVO DAS DUAS E O MESMO, E NAO E DESTE LADO. Com o periodo
 * de amostragem no contrato, o limiar vira funcao do periodo do NO em vez da
 * grade do balde: o artefato de alinhamento (2) desaparece porque a grade deixa
 * de ser referencia, e a transicao de regime (1) deixa de precisar de mediana
 * porque o periodo passa a ser declarado, nao inferido. E trabalho de backend
 * -- expor o campo --, fora do escopo da PWA. Ate la, a mediana e a melhor
 * aproximacao disponivel a partir do que a resposta traz. */
export function periodoTipico(pontos: readonly { t: string }[], bucketS: number): number {
  const dts: number[] = [];
  for (let i = 1; i < pontos.length; i++) {
    const dt = (Date.parse(pontos[i]!.t) - Date.parse(pontos[i - 1]!.t)) / 1000;
    // `dt > 0` e nao `!(dt <= 0)`: carimbo malformado da NaN, NaN reprova a
    // comparacao e o intervalo fica de fora do calculo do ritmo.
    if (dt > 0 && Number.isFinite(dt)) dts.push(dt);
  }
  // UM intervalo nao e ritmo, e uma amostra: com dois pontos a mediana E o
  // proprio intervalo, e qualquer par -- inclusive um par separado por doze
  // horas -- passaria a ser contiguo. Sem ritmo para medir, cai no balde, que
  // e o limite menor: separa em vez de ligar.
  if (dts.length < 2) return bucketS;
  dts.sort((a, b) => a - b);
  const mediana = dts[Math.floor((dts.length - 1) / 2)]!;
  return Math.max(bucketS, mediana);
}

/** Quebra a serie em trechos contiguos.
 *
 * Cada trecho vira um subpath do gerador de linha. Um trecho por lacuna, e o
 * grafico simplesmente nao tem o que desenhar no intervalo -- que e o
 * desenho correto da ausencia.
 *
 * TODA CONDICAO AQUI E ESCRITA COMO "E CONTIGUO?", NUNCA COMO "E LACUNA?".
 * A diferenca importa porque dt pode ser NaN (carimbo malformado) e NaN
 * reprova toda comparacao: `NaN > limite` e false, entao a forma negativa
 * concluiria "nao e lacuna" e ligaria a linha. Na forma afirmativa, NaN
 * reprova `dt > 0` e o ponto e separado. Fail-closed: na duvida, nao ligue.
 */
export function segmentarSerie<P extends { t: string }>(
  pontos: readonly P[],
  bucketS: number,
): P[][] {
  if (pontos.length === 0) return [];

  // bucket invalido (zero, negativo, NaN) tornaria o limite sem sentido.
  // Isolar cada ponto e o desfecho honesto: o grafico mostra as medicoes e
  // nenhuma ligacao entre elas, em vez de ligacoes que nao podemos justificar.
  // `!(x > 0)` e nao `x <= 0` para pegar NaN junto.
  if (!(bucketS > 0)) return pontos.map((p) => [p]);

  const limite = periodoTipico(pontos, bucketS) * FATOR_LACUNA;
  const segmentos: P[][] = [];
  let atual: P[] = [];
  let anteriorMs = Number.NaN;

  for (const ponto of pontos) {
    const ms = Date.parse(ponto.t);
    const dt = (ms - anteriorMs) / 1000;

    // dt > 0 tambem cobre desordem e carimbos repetidos. A serie chega
    // ordenada por t crescente; se nao chegar, ligar dois pontos fora de
    // ordem desenharia a linha voltando no tempo.
    const contiguo = atual.length > 0 && dt > 0 && dt <= limite;

    if (!contiguo && atual.length > 0) {
      segmentos.push(atual);
      atual = [];
    }
    atual.push(ponto);
    anteriorMs = ms;
  }

  segmentos.push(atual);
  return segmentos;
}
