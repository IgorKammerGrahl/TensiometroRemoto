import { describe, expect, it } from 'vitest';
import type { DeviceApp, Talhao, Zona } from './api';
import {
  contar,
  frasesDaSituacao,
  frasesDoVeredito,
  idadeMaisRecente,
  itensDaContagem,
  ordenarPorUrgencia,
  partesDaSintese,
  sintese,
  textoSemLeitura,
  type Veredito,
  veredito,
} from './veredito';

const AGORA = Date.parse('2026-11-16T18:00:00Z');
const COM: Talhao = { id: 't1', nome: 'Talhão 1', cultura: null, kpa_alerta: -30, kpa_estresse: -60 };
const SEM: Talhao = { ...COM, id: 't2', kpa_alerta: null, kpa_estresse: null };

// O kPa e indiferente nestes testes: a zona vem pronta do servidor, e a regra
// nao olha o numero. Todos levam -20 de proposito, inclusive os "secos".
type Caso = Zona | 'velha' | 'nunca' | 'desligado' | 'desconhecida';
let seq = 0;
function no(caso: Caso, t: Talhao = COM, idadeS = 60): DeviceApp {
  seq++;
  const base = { id: `n${seq}`, descricao: `s${seq}`, ativo: true, simulado: false, talhao: t };
  const leitura = (zona: Zona, idade: number) => ({
    measured_at: '2026-11-16T17:59:00Z',
    kpa: -20,
    zona,
    medido_ha_s: idade,
  });
  switch (caso) {
    case 'nunca':
      return { ...base, ultima: null };
    case 'desligado':
      return { ...base, ativo: false, ultima: leitura('conforto', 60) };
    case 'velha':
      return { ...base, ultima: leitura('conforto', 6 * 3600) };
    case 'desconhecida':
      return { ...base, ultima: leitura('saturado' as unknown as Zona, 60) };
    default:
      return { ...base, ultima: leitura(caso, idadeS) };
  }
}
const rep = <T,>(x: T, n: number): T[] => Array.from({ length: n }, () => x);
const de = (casos: Caso[], t: Talhao = COM): Veredito =>
  veredito(
    t,
    casos.map((c) => no(c, t)),
    AGORA,
    AGORA,
  );

describe('veredito: a ordem das verificacoes', () => {
  it('1. todos desligados: nenhum sensor em uso', () => {
    expect(de(['desligado', 'desligado']).tipo).toBe('sem-sensor');
  });

  it('2. sem limites vem antes de qualquer contagem', () => {
    expect(de([null, null], SEM).tipo).toBe('sem-limites');
    expect(de(['nunca', 'nunca'], SEM).tipo).toBe('sem-limites');
  });

  it('3. menos da metade dos ligados com leitura atual: poucos dados', () => {
    expect(de(['conforto', 'nunca', 'velha']).tipo).toBe('poucos-dados');
    expect(de(['nunca']).tipo).toBe('poucos-dados');
  });

  it('3. exatamente metade ainda responde', () => {
    expect(de(['conforto', 'nunca']).tipo).toBe('zona');
  });

  it('3. desligado nao entra no denominador', () => {
    expect(de(['conforto', 'desligado', 'desligado'])).toMatchObject({
      tipo: 'zona',
      zona: 'conforto',
      ligados: 1,
    });
  });

  it('4. leitura atual sem zona num talhao com limites: nao da para dizer', () => {
    expect(de(['conforto', null]).tipo).toBe('indeterminado');
  });

  it('4. zona que este cliente nao conhece nunca vira conforto', () => {
    expect(de(['conforto', 'conforto', 'desconhecida']).tipo).toBe('indeterminado');
  });
});

describe('veredito: mediana superior (lado seco)', () => {
  it('n impar: a do meio', () => {
    expect(de(['conforto', 'alerta', 'estresse'])).toMatchObject({ tipo: 'zona', zona: 'alerta' });
  });

  it('n par: a mais seca das duas do meio', () => {
    expect(de(['conforto', 'estresse'])).toMatchObject({ zona: 'estresse' });
    expect(de(['conforto', 'conforto', 'alerta', 'alerta'])).toMatchObject({ zona: 'alerta' });
  });

  it('1 sensor: vale ele', () => {
    expect(de(['alerta'])).toMatchObject({ zona: 'alerta' });
  });

  it('a ordem de chegada nao importa', () => {
    expect(de(['estresse', 'conforto', 'alerta'])).toMatchObject({ zona: 'alerta' });
  });

  it('vitrine, Boa Vista Talhao 2: 10 umidos, 21 secando, 9 secos', () => {
    const v = de([...rep<Caso>('conforto', 10), ...rep<Caso>('alerta', 21), ...rep<Caso>('estresse', 9)]);
    expect(v).toMatchObject({ tipo: 'zona', zona: 'alerta', complemento: null });
  });
});

describe('complemento da minoria mais seca', () => {
  it('3 sensores, 1 seco: avisa', () => {
    expect(de(['conforto', 'conforto', 'estresse'])).toMatchObject({
      zona: 'conforto',
      complemento: { zona: 'estresse', n: 1 },
    });
  });

  it('40 sensores, 15 secos: avisa; 1 seco: nao', () => {
    expect(de([...rep<Caso>('conforto', 25), ...rep<Caso>('estresse', 15)])).toMatchObject({
      zona: 'conforto',
      complemento: { zona: 'estresse', n: 15 },
    });
    expect(de([...rep<Caso>('conforto', 39), 'estresse'])).toMatchObject({ complemento: null });
  });

  it('secando so complementa o umido, e so se nao houver seco bastante', () => {
    expect(de([...rep<Caso>('conforto', 3), 'alerta'])).toMatchObject({
      zona: 'conforto',
      complemento: { zona: 'alerta', n: 1 },
    });
  });

  it('com 1 ou 2 sensores nao acontece: a mediana superior ja e o mais seco', () => {
    expect(de(['conforto', 'estresse'])).toMatchObject({ complemento: null });
    expect(de(['conforto', 'alerta'])).toMatchObject({ complemento: null });
    expect(de(['estresse'])).toMatchObject({ complemento: null });
  });
});

describe('frasesDoVeredito', () => {
  it('as tres zonas', () => {
    expect(frasesDoVeredito(de(['conforto']))).toEqual({
      principal: 'Solo úmido',
      linhas: ['Não precisa irrigar agora'],
    });
    expect(frasesDoVeredito(de(['alerta']))).toEqual({
      principal: 'Solo secando',
      linhas: ['Irrigar nos próximos dias'],
    });
    expect(frasesDoVeredito(de(['estresse']))).toEqual({
      principal: 'Solo seco',
      linhas: ['Irrigar agora'],
    });
  });

  it('o complemento vem antes da acao, e a acao continua a da mediana', () => {
    const v = de([...rep<Caso>('conforto', 25), ...rep<Caso>('estresse', 15)]);
    expect(frasesDoVeredito(v)).toEqual({
      principal: 'Solo úmido',
      linhas: ['mas 15 sensores secos', 'Não precisa irrigar agora'],
    });
    expect(frasesDoVeredito(de(['conforto', 'conforto', 'estresse'])).linhas[0]).toBe('mas 1 sensor seco');
    expect(frasesDoVeredito(de([...rep<Caso>('conforto', 3), 'alerta'])).linhas[0]).toBe(
      'mas 1 sensor secando',
    );
  });

  it('os casos sem resposta, com contagem e plural', () => {
    expect(frasesDoVeredito(de([null], SEM))).toEqual({
      principal: 'Limites não definidos',
      linhas: ['Não dá para dizer se precisa irrigar'],
    });
    expect(frasesDoVeredito(de(['conforto', 'nunca', 'nunca', 'velha']))).toEqual({
      principal: 'Sem leitura recente',
      linhas: ['3 de 4 sensores sem leitura recente'],
    });
    expect(frasesDoVeredito(de(['conforto', null]))).toEqual({
      principal: 'Não dá para dizer',
      linhas: ['Leitura sem classificação'],
    });
    expect(frasesDoVeredito(de(['desligado']))).toEqual({
      principal: 'Nenhum sensor em uso',
      linhas: ['O sensor deste talhão está desligado'],
    });
    expect(frasesDoVeredito(de(['desligado', 'desligado'])).linhas).toEqual([
      'Os 2 sensores deste talhão estão desligados',
    ]);
  });
});

describe('textoSemLeitura', () => {
  it('conta os ligados sem leitura atual', () => {
    expect(textoSemLeitura(de(['conforto', 'nunca']))).toBe('1 de 2 sensores sem leitura recente');
    expect(textoSemLeitura(de(['conforto', 'conforto']))).toBeNull();
  });
});

describe('itensDaContagem', () => {
  it('de umido para seco, depois sem leitura; zeros fora; plural', () => {
    const c = contar([no('estresse'), no('conforto'), no('alerta'), no('alerta'), no('nunca')], AGORA, AGORA);
    expect(itensDaContagem(c)).toEqual([
      { chave: 'conforto', n: 1, texto: 'úmido' },
      { chave: 'alerta', n: 2, texto: 'secando' },
      { chave: 'estresse', n: 1, texto: 'seco' },
      { chave: 'sem-leitura', n: 1, texto: 'sem leitura recente' },
    ]);
    const c2 = contar([no('conforto'), no('conforto'), no('estresse'), no('estresse')], AGORA, AGORA);
    expect(itensDaContagem(c2).map((i) => i.texto)).toEqual(['úmidos', 'secos']);
  });
});

describe('idadeMaisRecente', () => {
  it('a menor idade entre as leituras atuais; nada atual, null', () => {
    expect(
      idadeMaisRecente([no('conforto', COM, 300), no('alerta', COM, 120), no('velha')], AGORA, AGORA),
    ).toBe(120);
    expect(idadeMaisRecente([no('nunca'), no('velha')], AGORA, AGORA)).toBeNull();
  });
});

describe('ordenarPorUrgencia', () => {
  it('seco, secando, sem leitura, indeterminado, sem limites, umido, sem sensor; empate fica', () => {
    const itens = [
      { nome: 'umido-a', v: de(['conforto']) },
      { nome: 'sem-sensor', v: de(['desligado']) },
      { nome: 'sem-limites', v: de([null], SEM) },
      { nome: 'secando', v: de(['alerta']) },
      { nome: 'umido-b', v: de(['conforto']) },
      { nome: 'indeterminado', v: de(['conforto', null]) },
      { nome: 'seco', v: de(['estresse']) },
      { nome: 'poucos', v: de(['nunca']) },
    ];
    expect(ordenarPorUrgencia(itens, (i) => i.v).map((i) => i.nome)).toEqual([
      'seco',
      'secando',
      'poucos',
      'indeterminado',
      'sem-limites',
      'umido-a',
      'umido-b',
      'sem-sensor',
    ]);
  });
});

describe('sintese', () => {
  it('nenhum talhao', () => {
    expect(sintese([])).toBe('Nenhum sensor instalado nos seus talhões ainda');
  });
  it('todos umidos', () => {
    expect(sintese([de(['conforto']), de(['conforto'])])).toBe('Nenhum talhão precisa de água agora');
  });
  it('mistura, na ordem de urgencia, com plural', () => {
    expect(
      sintese([de(['conforto']), de(['alerta']), de([null], SEM), de(['estresse']), de(['alerta'])]),
    ).toBe(
      '1 talhão para irrigar agora · 2 talhões para irrigar nos próximos dias · 1 talhão sem limites · 1 talhão com solo úmido',
    );
  });
  it('em partes, uma por situacao, para a tela mostrar uma por linha', () => {
    expect(partesDaSintese([de(['estresse']), de(['conforto']), de(['conforto'])])).toEqual([
      '1 talhão para irrigar agora',
      '2 talhões com solo úmido',
    ]);
    expect(partesDaSintese([])).toEqual(['Nenhum sensor instalado nos seus talhões ainda']);
  });
  it('poucos dados e indeterminado somam como sem leitura; sem sensor no fim', () => {
    expect(sintese([de(['nunca']), de(['conforto', null]), de(['desligado'])])).toBe(
      '2 talhões sem leitura recente · 1 talhão sem sensor em uso',
    );
  });
});

describe('frasesDaSituacao (um sensor)', () => {
  it('cada situacao tem a sua frase', () => {
    expect(frasesDaSituacao({ tipo: 'atual', idadeS: 60, kpa: -20, zona: 'estresse' })).toEqual({
      principal: 'Solo seco',
      linhas: ['Irrigar agora'],
    });
    expect(frasesDaSituacao({ tipo: 'atual', idadeS: 60, kpa: -20, zona: null })).toEqual({
      principal: 'Limites não definidos',
      linhas: ['Não dá para dizer se precisa irrigar'],
    });
    expect(frasesDaSituacao({ tipo: 'velha', idadeS: 6 * 3600, kpa: -20 })).toEqual({
      principal: 'Sem leitura recente',
      linhas: ['Última leitura há 6 h · não dá para dizer como está agora'],
    });
    expect(frasesDaSituacao({ tipo: 'nunca' }).principal).toBe('Ainda sem leitura');
    expect(frasesDaSituacao({ tipo: 'desativado' })).toEqual({
      principal: 'Desligado',
      linhas: ['Este sensor foi desligado e não envia leituras'],
    });
  });
  it('zona desconhecida: nao da para dizer', () => {
    const s = { tipo: 'atual', idadeS: 60, kpa: -20, zona: 'saturado' as unknown as Zona } as const;
    expect(frasesDaSituacao(s).principal).toBe('Não dá para dizer');
  });
});
