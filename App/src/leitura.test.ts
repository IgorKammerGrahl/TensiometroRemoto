import { describe, expect, it } from 'vitest';
import type { UltimaLeitura } from './api';
import { situacao, textoDeIdade } from './leitura';
import { LIMIAR_LEITURA_VELHA_S } from './tempo';

const T0 = 1_756_800_000_000; // instante do fetch, fixo
const leitura = (medidoHaS: number, kpa: number, zona: UltimaLeitura['zona']): UltimaLeitura => ({
  measured_at: new Date(T0 - medidoHaS * 1000).toISOString(),
  kpa,
  zona,
  medido_ha_s: medidoHaS,
});

describe('situacao', () => {
  it('classifica leitura recente como atual e deixa a zona do servidor passar', () => {
    const s = situacao(true, leitura(60, -22, 'conforto'), T0, T0);
    expect(s).toEqual({ tipo: 'atual', idadeS: 60, kpa: -22, zona: 'conforto' });
  });

  it('preserva zona nula sem confundir com conforto', () => {
    const s = situacao(true, leitura(60, -22, null), T0, T0);
    expect(s.tipo).toBe('atual');
    expect(s.tipo === 'atual' && s.zona).toBeNull();
  });

  it('separa "nunca reportou" de "ficou mudo"', () => {
    expect(situacao(true, null, T0, T0)).toEqual({ tipo: 'nunca' });
    expect(situacao(true, leitura(9 * 3600, -22, 'conforto'), T0, T0).tipo).toBe('velha');
  });

  // O TESTE DA ORDEM, parte 1. A leitura velha carrega zona no contrato e ela
  // e correta sobre a medicao que existiu -- mas descreve o solo de horas
  // atras. Se ela vazar para a Situacao, a tela ganha permissao de pintar o
  // indicador de conforto sobre dado morto. A ausencia do campo e o que torna
  // isso inexpressavel a jusante, do mesmo modo que zona.ts nao aceita limiar.
  it('nao deixa a zona sobreviver a uma leitura velha', () => {
    const s = situacao(true, leitura(9 * 3600, -70, 'estresse'), T0, T0);
    expect(s.tipo).toBe('velha');
    expect('zona' in s).toBe(false);
  });

  // O TESTE DA ORDEM, parte 2. Desativado vence tudo, inclusive uma leitura
  // que acabou de chegar. A ingestao recusa no desativado (403), entao esse
  // caso so aparece na janela entre desativar e a proxima consulta -- mas e
  // exatamente onde a ordem errada apareceria como "esta funcionando".
  it('poe no desativado antes de qualquer leitura', () => {
    expect(situacao(false, leitura(30, -10, 'conforto'), T0, T0)).toEqual({
      tipo: 'desativado',
    });
    expect(situacao(false, null, T0, T0)).toEqual({ tipo: 'desativado' });
  });

  // leituraVelha compara com `>`, entao a igualdade ainda e atual. Um `>=`
  // aqui envelheceria a leitura um instante cedo demais; a fronteira exata e
  // o tipo de coisa que alguem "arruma" nos dois sentidos.
  it('trata o limiar exato como leitura ainda atual', () => {
    expect(situacao(true, leitura(LIMIAR_LEITURA_VELHA_S, -22, 'alerta'), T0, T0).tipo).toBe(
      'atual',
    );
    expect(situacao(true, leitura(LIMIAR_LEITURA_VELHA_S + 1, -22, 'alerta'), T0, T0).tipo).toBe(
      'velha',
    );
  });

  // A idade envelhece na mao do cliente entre um fetch e o proximo repintar:
  // uma leitura recente vira velha sem nova requisicao.
  it('envelhece a leitura com o tempo decorrido desde o fetch', () => {
    const nova = leitura(60, -22, 'conforto');
    expect(situacao(true, nova, T0, T0).tipo).toBe('atual');
    expect(situacao(true, nova, T0, T0 + 3600_000).tipo).toBe('velha');
  });
});

describe('textoDeIdade', () => {
  it('da uma frase distinta para cada situacao', () => {
    const frases = [
      textoDeIdade({ tipo: 'desativado' }),
      textoDeIdade({ tipo: 'nunca' }),
      textoDeIdade({ tipo: 'velha', idadeS: 6 * 3600, kpa: -70 }),
      textoDeIdade({ tipo: 'atual', idadeS: 120, kpa: -22, zona: 'conforto' }),
    ];
    expect(new Set(frases).size).toBe(4);
    expect(frases[2]).toBe('Sem dados há 6 h');
    expect(frases[3]).toBe('há 2 min');
  });

  // RNF05: o produtor nao tem treinamento previo. Nenhuma destas frases pode
  // exigir vocabulario tecnico para ser entendida.
  it('nao usa jargao', () => {
    const proibido = ['kpa', 'potencial matricial', 'tensao', 'zona', 'payload', 'uptime'];
    const frases = [
      textoDeIdade({ tipo: 'desativado' }),
      textoDeIdade({ tipo: 'nunca' }),
      textoDeIdade({ tipo: 'velha', idadeS: 60, kpa: -70 }),
      textoDeIdade({ tipo: 'atual', idadeS: 60, kpa: -22, zona: null }),
    ];
    for (const f of frases) {
      for (const p of proibido) expect(f.toLowerCase()).not.toContain(p);
    }
  });
});
