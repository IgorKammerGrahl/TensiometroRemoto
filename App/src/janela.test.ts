import { describe, expect, it } from 'vitest';
import {
  intervaloInicial,
  intervaloValido,
  JANELA_PADRAO,
  JANELAS,
  janelaDe,
  parametros,
  parametrosIntervalo,
  tituloIntervalo,
} from './janela';

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

// RF07: intervalo de datas escolhido pelo produtor. O fuso dos testes e
// America/Sao_Paulo (UTC-3, sem horario de verao), fixado no vite.config.ts.
describe('parametrosIntervalo', () => {
  const HOJE = '2026-11-16';

  it('vai da meia-noite local do primeiro dia a meia-noite seguinte ao ultimo', () => {
    expect(parametrosIntervalo({ de: '2026-09-01', ate: '2026-09-16' }, HOJE)).toEqual({
      from: '2026-09-01T03:00:00.000Z',
      to: '2026-09-17T03:00:00.000Z',
    });
  });

  it('um dia so e um dia inteiro', () => {
    expect(parametrosIntervalo({ de: '2026-09-05', ate: '2026-09-05' }, HOJE)).toEqual({
      from: '2026-09-05T03:00:00.000Z',
      to: '2026-09-06T03:00:00.000Z',
    });
  });

  it('o fim atravessa mes e ano', () => {
    expect(parametrosIntervalo({ de: '2025-12-30', ate: '2025-12-31' }, HOJE).to).toBe(
      '2026-01-01T03:00:00.000Z',
    );
  });

  // Intervalo que termina hoje: o fim e o agora do servidor, como nas
  // janelas relativas. Mandar meia-noite de amanha pediria o futuro.
  it('terminando hoje, nao manda `to`', () => {
    expect(parametrosIntervalo({ de: '2026-11-10', ate: HOJE }, HOJE)).toEqual({
      from: '2026-11-10T03:00:00.000Z',
    });
  });
});

describe('intervaloValido', () => {
  const HOJE = '2026-11-16';
  it('aceita de <= ate <= hoje', () => {
    expect(intervaloValido({ de: '2026-09-01', ate: '2026-09-16' }, HOJE)).toBe(true);
    expect(intervaloValido({ de: HOJE, ate: HOJE }, HOJE)).toBe(true);
  });
  it('recusa invertido, futuro e vazio', () => {
    expect(intervaloValido({ de: '2026-09-16', ate: '2026-09-01' }, HOJE)).toBe(false);
    expect(intervaloValido({ de: '2026-11-10', ate: '2026-11-17' }, HOJE)).toBe(false);
    expect(intervaloValido({ de: '', ate: '2026-09-01' }, HOJE)).toBe(false);
    expect(intervaloValido({ de: '2026-9-1', ate: '2026-09-02' }, HOJE)).toBe(false);
  });
});

describe('intervaloInicial e tituloIntervalo', () => {
  it('sao os 7 dias que terminam no dia do instante dado, no fuso local', () => {
    // 02/09 01:30 em Sao Paulo ainda e 02/09, embora em UTC ja seja 02/09 04:30.
    expect(intervaloInicial(Date.parse('2026-09-02T04:30:00Z'))).toEqual({
      de: '2026-08-27',
      ate: '2026-09-02',
    });
    // 23:30 de 01/09 em Sao Paulo e 02:30 de 02/09 em UTC: o dia e 01/09.
    expect(intervaloInicial(Date.parse('2026-09-02T02:30:00Z')).ate).toBe('2026-09-01');
  });

  it('escreve as datas como o produtor le', () => {
    expect(tituloIntervalo({ de: '2026-09-01', ate: '2026-09-16' })).toBe('De 01/09/2026 a 16/09/2026');
    expect(tituloIntervalo({ de: '2026-09-05', ate: '2026-09-05' })).toBe('Dia 05/09/2026');
  });
});
