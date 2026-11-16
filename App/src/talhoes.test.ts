import { describe, expect, it } from 'vitest';
import type { DeviceApp, Talhao, Zona } from './api';
import { agruparPorTalhao, buscarNos } from './talhoes';

const AGORA = Date.parse('2026-11-16T18:00:00Z');

const talhao = (id: string, nome: string): Talhao => ({
  id,
  nome,
  cultura: null,
  kpa_alerta: -30,
  kpa_estresse: -60,
});
const NORTE = talhao('t-norte', 'Talhão Norte');
const SUL = talhao('t-sul', 'Talhão Sul');

let seq = 0;
function no(
  t: Talhao,
  kpa: number | null,
  zona: Zona = null,
  extra: Partial<DeviceApp> & { idadeS?: number } = {},
): DeviceApp {
  const { idadeS = 60, ...resto } = extra;
  seq++;
  return {
    id: `no-${seq}`,
    descricao: `nó ${seq}`,
    ativo: true,
    simulado: false,
    talhao: t,
    ultima:
      kpa === null
        ? null
        : { measured_at: '2026-11-16T17:59:00Z', kpa, zona, medido_ha_s: idadeS },
    ...resto,
  };
}

describe('agruparPorTalhao', () => {
  it('ordena os talhoes pela primeira aparicao na lista do servidor, sem .sort()', () => {
    // O servidor manda do mais seco para o mais umido. O talhao do no mais
    // seco vem primeiro, e dentro de cada um a ordem e a que veio.
    const lista = [
      no(SUL, -70, 'estresse'),
      no(NORTE, -40, 'alerta'),
      no(SUL, -20, 'conforto'),
      no(NORTE, -10, 'conforto'),
    ];
    const g = agruparPorTalhao(lista, AGORA, AGORA);
    expect(g.map((x) => x.talhao.id)).toEqual(['t-sul', 't-norte']);
    expect(g[0]!.nos.map((n) => n.ultima?.kpa)).toEqual([-70, -20]);
    expect(g[1]!.nos.map((n) => n.ultima?.kpa)).toEqual([-40, -10]);
  });

  it('conta por estado, e quem nao tem leitura atual fica fora das zonas', () => {
    const SEM = { ...talhao('t-sem', 'Sem faixa'), kpa_alerta: null, kpa_estresse: null };
    const lista = [
      no(NORTE, -70, 'estresse'),
      no(NORTE, -40, 'alerta'),
      no(NORTE, -45, 'alerta', { idadeS: 3 * 3600 }), // velha: 3 h
      no(NORTE, -10, 'conforto'),
      no(NORTE, null), // nunca reportou
      no(NORTE, -12, 'conforto', { ativo: false }), // desativado
      no(SEM, -33, null),
    ];
    const [norte, sem] = agruparPorTalhao(lista, AGORA, AGORA);
    expect(norte!.contagem).toEqual({
      conforto: 1,
      alerta: 1,
      estresse: 1,
      semFaixa: 0,
      semLeitura: 3,
    });
    expect(sem!.contagem.semFaixa).toBe(1);
  });

  it('so desenha traco de leitura ATUAL: valor velho na tira seria afirmar o presente', () => {
    const lista = [
      no(NORTE, -70, 'estresse'),
      no(NORTE, -45, 'alerta', { idadeS: 3 * 3600 }),
      no(NORTE, null),
      no(NORTE, -10, 'conforto'),
    ];
    expect(agruparPorTalhao(lista, AGORA, AGORA)[0]!.leituras).toEqual([-70, -10]);
  });

  it('conta os simulados, para a tela dizer quantos sao', () => {
    const lista = [no(NORTE, -10, 'conforto', { simulado: true }), no(NORTE, -20, 'conforto')];
    expect(agruparPorTalhao(lista, AGORA, AGORA)[0]!.simulados).toBe(1);
  });
});

describe('buscarNos', () => {
  const lista = [
    no(NORTE, -10, 'conforto', { descricao: 'Cabeceira do pivô', id: 'tensio-07' }),
    no(SUL, -20, 'conforto', { descricao: 'Baixada', id: 'tensio-12' }),
  ];

  it('acha por descricao, id ou talhao, sem diferenciar acento nem caixa', () => {
    expect(buscarNos(lista, 'pivo').map((n) => n.id)).toEqual(['tensio-07']);
    expect(buscarNos(lista, 'TENSIO-12').map((n) => n.id)).toEqual(['tensio-12']);
    expect(buscarNos(lista, 'talhao sul').map((n) => n.id)).toEqual(['tensio-12']);
  });

  it('termo vazio ou so espacos devolve a lista inteira, na mesma ordem', () => {
    expect(buscarNos(lista, '   ')).toEqual(lista);
  });
});
