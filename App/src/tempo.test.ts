import { describe, expect, it } from 'vitest';
import {
  LIMIAR_LEITURA_VELHA_S,
  formatarDuracao,
  idadeSegundos,
  leituraVelha,
} from './tempo';

describe('idadeSegundos', () => {
  it('devolve a idade do servidor quando nao decorreu tempo local', () => {
    expect(idadeSegundos(120, 1_000_000, 1_000_000)).toBe(120);
  });

  it('soma o tempo decorrido desde o fetch', () => {
    // 120 s no servidor + 90 s de tela aberta = 210 s.
    expect(idadeSegundos(120, 1_000_000, 1_090_000)).toBe(210);
  });

  // O PONTO DA FUNCAO. O relogio do cliente pode estar a horas de distancia
  // do relogio do servidor; o ensaio de 17 h registrou latencia MINIMA de
  // -0,123 s, ou seja, os carimbos ja discordam entre si. Ancorar em
  // medido_ha_s e medir so o INTERVALO local faz a diferenca constante entre
  // os relogios se cancelar.
  it('e imune a desvio constante entre o relogio do cliente e o do servidor', () => {
    const desvio = 3 * 60 * 60 * 1000; // cliente 3 h adiantado
    const semDesvio = idadeSegundos(300, 1_000_000, 1_060_000);
    const comDesvio = idadeSegundos(300, 1_000_000 + desvio, 1_060_000 + desvio);
    expect(comDesvio).toBe(semDesvio);
  });

  it('nao rejuvenesce a leitura se o relogio local andar para tras', () => {
    // Correcao de NTP no meio da sessao: agora < fetchadoEm.
    expect(idadeSegundos(600, 1_000_000, 900_000)).toBe(600);
  });

  it('trata idade negativa do servidor como zero', () => {
    // O backend aceita measured_at ate 2 h no futuro, entao medido_ha_s pode
    // chegar negativo. "Leitura daqui a 1 h" nao e uma frase exibivel.
    expect(idadeSegundos(-3600, 1_000_000, 1_000_000)).toBe(0);
  });
});

describe('leituraVelha', () => {
  // A fronteira exata: `>` e nao `>=`. Exatamente no limiar a leitura ainda
  // vale; um segundo depois, nao.
  it('nao marca como velha exatamente no limiar', () => {
    expect(leituraVelha(LIMIAR_LEITURA_VELHA_S)).toBe(false);
  });

  it('marca como velha um segundo depois do limiar', () => {
    expect(leituraVelha(LIMIAR_LEITURA_VELHA_S + 1)).toBe(true);
  });

  it('nao marca como velha uma leitura recente', () => {
    expect(leituraVelha(0)).toBe(false);
    expect(leituraVelha(60)).toBe(false);
  });
});

describe('formatarDuracao', () => {
  it('cobre as fronteiras de cada unidade', () => {
    expect(formatarDuracao(0)).toBe('menos de 1 min');
    expect(formatarDuracao(59)).toBe('menos de 1 min');
    expect(formatarDuracao(60)).toBe('1 min');
    expect(formatarDuracao(3599)).toBe('59 min');
    expect(formatarDuracao(3600)).toBe('1 h');
    expect(formatarDuracao(6 * 3600)).toBe('6 h');
    expect(formatarDuracao(47 * 3600 + 3599)).toBe('47 h');
    expect(formatarDuracao(48 * 3600)).toBe('2 dias');
    expect(formatarDuracao(10 * 24 * 3600)).toBe('10 dias');
  });

  it('nao produz duracao negativa', () => {
    expect(formatarDuracao(-500)).toBe('menos de 1 min');
  });

  // A funcao devolve a duracao sem preposicao para servir aos dois
  // enunciados da interface: "ha 6 h" e "sem dados ha 6 h".
  it('nao embute preposicao', () => {
    expect(formatarDuracao(6 * 3600).startsWith('ha ')).toBe(false);
  });
});
