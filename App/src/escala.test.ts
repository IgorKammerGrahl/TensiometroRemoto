import { describe, expect, it } from 'vitest';
import { bandas, FUNDO_KPA, posicao, textoKpa, TOPO_KPA } from './escala';

describe('posicao', () => {
  // O TESTE DA DIREcAO. kPa e negativo e mais negativo e pior; a fracao tem
  // de CRESCER com a severidade para que "pior para baixo" seja verdade na
  // tela. Se alguem trocar o sinal da divisao, o entalhe de um solo seco sobe
  // para o topo da regua e a regua passa a dizer o contrario do que mede --
  // sem quebrar nada, sem erro no console.
  it('cresce conforme o solo seca', () => {
    const umido = posicao(-5);
    const meio = posicao(-40);
    const seco = posicao(-75);
    expect(umido && meio && seco).toBeTruthy();
    expect(umido!.fracao).toBeLessThan(meio!.fracao);
    expect(meio!.fracao).toBeLessThan(seco!.fracao);
  });

  it('ancora 0 kPa no topo e o fundo de escala na base', () => {
    expect(posicao(TOPO_KPA)).toEqual({ fracao: 0, fora: null });
    expect(posicao(FUNDO_KPA)).toEqual({ fracao: 1, fora: null });
    expect(posicao(-40)).toEqual({ fracao: 0.5, fora: null });
  });

  // Faixas reais, nao hipoteticas: readings.kpa aceita BETWEEN -100 AND 10,
  // enquanto a escala vai de 0 a -80. Os dois extremos do banco caem fora.
  it('grampeia leitura fora da escala e diz que grampeou', () => {
    expect(posicao(-100)).toEqual({ fracao: 1, fora: 'abaixo' });
    expect(posicao(10)).toEqual({ fracao: 0, fora: 'acima' });
  });

  // Distingue "esta na borda porque vale -80" de "esta na borda porque foi
  // grampeada". Sem isso a regua mostraria o mesmo entalhe para as duas.
  it('so marca fora de escala quando de fato passou do limite', () => {
    expect(posicao(-80)?.fora).toBeNull();
    expect(posicao(-80.1)?.fora).toBe('abaixo');
    expect(posicao(0)?.fora).toBeNull();
    expect(posicao(0.1)?.fora).toBe('acima');
  });

  // null e nao NaN: NaN atravessaria a aritmetica de layout e sairia como um
  // atributo SVG invalido, que o navegador ignora em silencio.
  it('devolve null para valor nao finito', () => {
    expect(posicao(Number.NaN)).toBeNull();
    expect(posicao(Number.POSITIVE_INFINITY)).toBeNull();
    expect(posicao(Number.NEGATIVE_INFINITY)).toBeNull();
  });
});

describe('bandas', () => {
  it('devolve as tres bandas contiguas, conforto no topo', () => {
    const b = bandas(-30, -60);
    expect(b.map((x) => x.chave)).toEqual(['conforto', 'alerta', 'estresse']);
    expect(b[0]!.de).toBe(0);
    expect(b[2]!.ate).toBe(1);
    // Contiguas e sem sobreposicao: o fim de uma e o inicio da seguinte.
    expect(b[0]!.ate).toBe(b[1]!.de);
    expect(b[1]!.ate).toBe(b[2]!.de);
    // E crescentes -- nenhuma banda de altura negativa.
    for (const x of b) expect(x.de).toBeLessThan(x.ate);
  });

  // A fronteira desenhada tem de cair onde a linha de referencia do grafico
  // cai. Se divergirem, a regua e o grafico contam historias diferentes sobre
  // o mesmo limiar.
  it('poe a fronteira exatamente na posicao do limiar', () => {
    const b = bandas(-30, -60);
    expect(b[0]!.ate).toBe(posicao(-30)!.fracao);
    expect(b[1]!.ate).toBe(posicao(-60)!.fracao);
  });

  it('nao desenha banda nenhuma quando falta limiar', () => {
    expect(bandas(null, null)).toEqual([]);
    expect(bandas(-30, null)).toEqual([]);
    expect(bandas(null, -60)).toEqual([]);
  });

  // FAIL-CLOSED. O banco garante kpa_estresse < kpa_alerta (talhoes_faixas_ck),
  // mas o banco protege a tabela e nao o fio. Um par invertido chegando aqui
  // pintaria conforto sobre a faixa de estresse: verde onde deveria estar
  // vermelho. Sem faixa confiavel, nenhuma faixa.
  it('recusa par invertido em vez de pintar ao contrario', () => {
    expect(bandas(-60, -30)).toEqual([]);
  });

  it('recusa limiares iguais -- nao ha faixa de alerta com altura zero', () => {
    expect(bandas(-30, -30)).toEqual([]);
  });

  it('recusa limiar fora da escala desenhavel', () => {
    expect(bandas(-30, -90)).toEqual([]);
    expect(bandas(10, -60)).toEqual([]);
  });

  it('recusa valor nao numerico', () => {
    expect(bandas(Number.NaN, -60)).toEqual([]);
    expect(bandas(-30, Number.NaN)).toEqual([]);
  });

  // Legal pelo banco (BETWEEN -80 AND 0 inclui os extremos) e legal aqui: o
  // talhao inteiro e alerta e estresse, sem faixa de conforto.
  it('aceita conforto de altura zero quando o alerta comeca em 0 kPa', () => {
    const b = bandas(0, -60);
    expect(b).toHaveLength(3);
    expect(b[0]).toEqual({ chave: 'conforto', de: 0, ate: 0 });
  });
});

// O SINAL E A INFORMACAO, NAO O ENFEITE. Quase toda leitura e negativa; a
// positiva e rara e significa o oposto (solo saturado, ou sensor fora do
// lugar). Sem o "+", "0,6" e "-0,6" ficam a um caractere de distancia numa
// coluna onde o olho ja aprendeu a esperar o menos.
describe('textoKpa', () => {
  it('marca o positivo com sinal', () => {
    // Leitura real do tensio-01 aberto a atmosfera, 02/09/2026.
    expect(textoKpa(0.6)).toBe('+0,6');
    expect(textoKpa(10)).toBe('+10'); // teto do CHECK do banco
  });

  it('mantem o negativo como sempre foi', () => {
    expect(textoKpa(-49.7625)).toBe('-49,8');
    expect(textoKpa(FUNDO_KPA)).toBe('-80');
  });

  it('nao poe sinal no zero, que nao e nem um nem outro', () => {
    expect(textoKpa(TOPO_KPA)).toBe('0');
    expect(textoKpa(0)).toBe('0');
    expect(textoKpa(-0)).toBe('0'); // -0 sai de 0 / -80; ver fracaoDe
  });

  it('arredonda para uma casa, em virgula', () => {
    expect(textoKpa(-31.74)).toBe('-31,7');
    expect(textoKpa(-31.75)).toBe('-31,8');
  });
});
