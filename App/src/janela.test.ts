import { describe, expect, it } from 'vitest';
import { JANELA_PADRAO, JANELAS, janelaDe, parametros } from './janela';

const ANCORA = Date.parse('2026-09-02T18:00:00.000Z');
const j = (chave: string) => janelaDe(chave);

describe('janelaDe', () => {
  it('cai no padrao de 24 h para chave ausente ou desconhecida', () => {
    // A chave vem da URL, que o usuario pode digitar. Chave estranha nao pode
    // deixar a tela sem janela nenhuma.
    expect(janelaDe(null).chave).toBe('24h');
    expect(janelaDe(undefined).chave).toBe('24h');
    expect(janelaDe('ontem').chave).toBe('24h');
    expect(janelaDe('').chave).toBe('24h');
    expect(JANELA_PADRAO.chave).toBe('24h');
  });

  it('reconhece as tres janelas', () => {
    expect(JANELAS.map((x) => x.chave)).toEqual(['24h', '7d', '30d']);
  });
});

describe('parametros', () => {
  // O PONTO: `to` NUNCA aparece. Quem decide o fim da janela e o relogio do
  // servidor, nao o do celular -- ver o cabecalho de janela.ts e a latencia
  // minima de -0,123 s medida no ensaio de 27/08.
  it('nunca manda `to`', () => {
    for (const janela of JANELAS) {
      expect(parametros(janela, ANCORA)).not.toHaveProperty('to');
      expect(parametros(janela, null)).not.toHaveProperty('to');
    }
  });

  it('sem ancora nao manda nada, e o servidor aplica o padrao dele', () => {
    expect(parametros(j('7d'), null)).toEqual({});
    expect(parametros(j('24h'), null)).toEqual({});
  });

  it('conta a duracao para tras a partir da ancora do servidor', () => {
    expect(parametros(j('24h'), ANCORA)).toEqual({ from: '2026-09-01T18:00:00.000Z' });
    expect(parametros(j('7d'), ANCORA)).toEqual({ from: '2026-08-26T18:00:00.000Z' });
    expect(parametros(j('30d'), ANCORA)).toEqual({ from: '2026-08-03T18:00:00.000Z' });
  });

  it('manda RFC3339 em UTC, que e o que horaParam aceita', () => {
    const { from } = parametros(j('7d'), ANCORA);
    expect(from).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/);
    expect(Number.isNaN(Date.parse(from!))).toBe(false);
  });
});
