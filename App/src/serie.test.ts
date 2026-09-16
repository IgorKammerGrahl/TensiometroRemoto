import { describe, expect, it } from 'vitest';
import { FATOR_LACUNA, periodoTipico, segmentarSerie } from './serie';

const BUCKET = 300; // 5 min
const T0 = Date.parse('2026-08-27T10:00:00Z');

/** Ponto no minuto informado, contado a partir de T0. */
function p(minutos: number) {
  return { t: new Date(T0 + minutos * 60_000).toISOString() };
}

const rotulos = (segmentos: { t: string }[][]) => segmentos.map((s) => s.length);

describe('segmentarSerie', () => {
  it('devolve lista vazia para serie vazia', () => {
    expect(segmentarSerie([], BUCKET)).toEqual([]);
  });

  it('devolve um segmento de um ponto para serie de um ponto', () => {
    expect(rotulos(segmentarSerie([p(0)], BUCKET))).toEqual([1]);
  });

  it('mantem um unico segmento quando os baldes sao consecutivos', () => {
    const pontos = [p(0), p(5), p(10), p(15)];
    expect(rotulos(segmentarSerie(pontos, BUCKET))).toEqual([4]);
  });

  // O PONTO DE VERIFICACAO DA ETAPA 1: dt > 1,5 x bucket_s produz dois
  // segmentos, nao um. Um segmento so seria uma reta desenhada por cima de
  // uma hora sem nenhuma medicao.
  it('quebra em dois segmentos quando ha lacuna', () => {
    const pontos = [p(0), p(5), p(65), p(70)]; // 55 min sem leitura
    const segmentos = segmentarSerie(pontos, BUCKET);
    expect(rotulos(segmentos)).toEqual([2, 2]);
    expect(segmentos[0]?.[1]?.t).toBe(p(5).t);
    expect(segmentos[1]?.[0]?.t).toBe(p(65).t);
  });

  // A FRONTEIRA EXATA. O contrato do servidor e "lacuna quando
  // dt > 1,5 x bucket_s", entao dt IGUAL a 1,5 x bucket_s ainda liga. Este e
  // o caractere que alguem "corrige" de <= para < sem perceber que mudou a
  // regra; o teste existe para que isso apareca.
  it('liga os pontos exatamente em 1,5 x bucket_s', () => {
    const dtLimite = (BUCKET * FATOR_LACUNA) / 60; // 7,5 min
    const pontos = [p(0), p(dtLimite)];
    expect(rotulos(segmentarSerie(pontos, BUCKET))).toEqual([2]);
  });

  it('quebra um milissegundo acima de 1,5 x bucket_s', () => {
    const limiteMs = BUCKET * FATOR_LACUNA * 1000;
    const pontos = [
      { t: new Date(T0).toISOString() },
      { t: new Date(T0 + limiteMs + 1).toISOString() },
    ];
    expect(rotulos(segmentarSerie(pontos, BUCKET))).toEqual([1, 1]);
  });

  it('produz varios segmentos quando ha varias lacunas', () => {
    const pontos = [p(0), p(5), p(60), p(65), p(70), p(200)];
    expect(rotulos(segmentarSerie(pontos, BUCKET))).toEqual([2, 3, 1]);
  });

  it('preserva todos os pontos e a ordem', () => {
    const pontos = [p(0), p(5), p(60), p(65)];
    const achatado = segmentarSerie(pontos, BUCKET).flat();
    expect(achatado).toEqual(pontos);
  });

  // Fail-closed: bucket invalido nao permite provar contiguidade de nada,
  // entao nada e ligado. Mostrar pontos soltos e honesto; ligar sem criterio,
  // nao.
  it('isola cada ponto quando o bucket e invalido', () => {
    for (const ruim of [0, -300, Number.NaN]) {
      expect(rotulos(segmentarSerie([p(0), p(5), p(10)], ruim))).toEqual([1, 1, 1]);
    }
  });

  // NaN reprova toda comparacao. A condicao esta escrita como "e contiguo?",
  // entao NaN cai no lado de quebrar. Se estivesse escrita como "e lacuna?",
  // NaN concluiria "nao e lacuna" e ligaria a linha atravessando um carimbo
  // que nao da nem para ler.
  it('quebra o segmento em carimbo de tempo invalido', () => {
    const pontos = [p(0), { t: 'nao-e-data' }, p(10)];
    expect(rotulos(segmentarSerie(pontos, BUCKET))).toEqual([1, 1, 1]);
  });

  it('quebra em pontos fora de ordem ou repetidos', () => {
    expect(rotulos(segmentarSerie([p(10), p(5)], BUCKET))).toEqual([1, 1]);
    expect(rotulos(segmentarSerie([p(0), p(0)], BUCKET))).toEqual([1, 1]);
  });

  it('acompanha a largura do balde', () => {
    const pontos = [p(0), p(30)]; // 30 min de intervalo
    expect(rotulos(segmentarSerie(pontos, 300))).toEqual([1, 1]); // limite 7,5 min
    expect(rotulos(segmentarSerie(pontos, 3600))).toEqual([2]); // limite 90 min
  });

  it('carrega os campos extras do ponto sem alterar', () => {
    const pontos = [
      { t: p(0).t, kpa_med: -35, zona: 'alerta' as const },
      { t: p(60).t, kpa_med: -70, zona: 'estresse' as const },
    ];
    const segmentos = segmentarSerie(pontos, BUCKET);
    expect(segmentos[0]?.[0]?.kpa_med).toBe(-35);
    expect(segmentos[1]?.[0]?.zona).toBe('estresse');
  });
});


/** Serie a partir dos deslocamentos em SEGUNDOS desde T0. */
function serie(...segundos: number[]) {
  return segundos.map((s) => ({ t: new Date(T0 + s * 1000).toISOString() }));
}

// A REGRESSAO QUE O DUBLE NAO PEGA.
//
// Todos os testes acima usam pontos espacados exatamente um balde, porque era
// assim que o backend falso gerava a serie. O backend real devolve bucket_s=60
// para qualquer janela e o no amostra a cada 15 min: nenhum ponto fica a um
// balde do vizinho e a regra antiga -- 1,5 x bucket_s -- declarava lacuna em
// todo intervalo. Os numeros aqui sao os medidos no banco de desenvolvimento
// em 02/09/2026, nao inventados.
describe('periodoTipico', () => {
  it('encontra o ritmo real quando o no reporta mais devagar que o balde', () => {
    // tensio-demo: 1236 intervalos de 15 min, balde de 60 s.
    const pontos = serie(0, 900, 1800, 2700, 3600);
    expect(periodoTipico(pontos, 60)).toBe(900);
  });

  it('nao deixa a lacuna levantar o proprio limite', () => {
    // Mesmo ritmo de 15 min com a parada de 7h15 que o seed grava. Se isto
    // usasse media, o limite subiria acima da parada e a linha a atravessaria.
    const pontos = serie(0, 900, 1800, 27900, 28800, 29700);
    expect(periodoTipico(pontos, 60)).toBe(900);
    expect(rotulos(segmentarSerie(pontos, 60))).toEqual([3, 3]);
  });

  it('absorve o jitter de amostragem do no fisico', () => {
    // tensio-01 reporta a 60 s e entrega 61, 62 e 63 s com frequencia.
    const pontos = serie(0, 60, 121, 183, 244, 304);
    expect(rotulos(segmentarSerie(pontos, 60))).toEqual([6]);
  });

  it('separa quando o no fisico perde um reporte', () => {
    // 7m45 no meio de dados de 60 s -- o no ficou mudo, e isso se ve.
    const pontos = serie(0, 60, 121, 586, 646, 707);
    expect(rotulos(segmentarSerie(pontos, 60))).toEqual([3, 3]);
  });

  it('usa o balde como piso quando o no reporta mais rapido que o balde', () => {
    // Impossivel na pratica -- a agregacao garante um ponto por balde no
    // maximo -- mas o piso e o que impede um limite menor que o balde.
    expect(periodoTipico(serie(0, 10, 20, 30), 300)).toBe(300);
  });

  it('cai no balde quando ha um intervalo so, porque um intervalo nao e ritmo', () => {
    // FAIL-CLOSED: com dois pontos a mediana E o intervalo entre eles, e sem
    // este piso qualquer par -- inclusive um par separado por doze horas --
    // viraria um segmento so.
    expect(periodoTipico(serie(0, 43_200), 60)).toBe(60);
    expect(rotulos(segmentarSerie(serie(0, 43_200), 60))).toEqual([1, 1]);
  });

  it('cai no balde quando nao ha intervalo nenhum', () => {
    expect(periodoTipico([], 60)).toBe(60);
    expect(periodoTipico(serie(0), 60)).toBe(60);
  });

  it('ignora carimbo malformado no calculo do ritmo', () => {
    const pontos = [...serie(0, 900, 1800), { t: 'nao e data' }, ...serie(2700, 3600)];
    expect(periodoTipico(pontos, 60)).toBe(900);
  });
});
