import { describe, expect, it } from 'vitest';
import { corpoDeCalibracao, dataDoEnsaio, LIMITES, RASCUNHO_VAZIO } from './calibracao';

const bom = {
  ...RASCUNHO_VAZIO,
  vZero: '4.5',
  k: '0.04',
  fatorDivisor: '1.5',
  vddEnsaio: '5000',
};

describe('corpoDeCalibracao', () => {
  it('converte os quatro obrigatorios para numero', () => {
    expect(corpoDeCalibracao(bom)).toEqual({
      v_zero_kpa: 4.5,
      k_v_por_kpa: 0.04,
      fator_divisor: 1.5,
      vdd_ensaio_mv: 5000,
    });
  });

  // O TESTE QUE JUSTIFICA O MODULO. `Number('')` e 0, entao a conversao ingenua
  // manda `r2: 0` -- "o ajuste da reta foi pessimo" -- quando a pessoa apenas
  // nao mediu r2. A interface estaria inventando um dado sobre a qualidade do
  // ensaio, e ele ficaria gravado no banco junto com os coeficientes.
  it('omite opcional em branco em vez de mandar zero', () => {
    const corpo = corpoDeCalibracao(bom)!;
    expect('r2' in corpo).toBe(false);
    expect('rmse_kpa' in corpo).toBe(false);
    expect('nota' in corpo).toBe(false);
  });

  // Zero DIGITADO e diferente de campo vazio, e o servidor aceita os dois
  // valores (r2 e rmse tem piso 0). Se a distincao sumir, some junto a
  // possibilidade de registrar um ensaio que de fato deu r2 = 0.
  it('preserva opcional preenchido com zero', () => {
    const corpo = corpoDeCalibracao({ ...bom, r2: '0', rmse: '0' })!;
    expect(corpo.r2).toBe(0);
    expect(corpo.rmse_kpa).toBe(0);
  });

  it('recusa o corpo quando falta qualquer obrigatorio', () => {
    for (const campo of ['vZero', 'k', 'fatorDivisor', 'vddEnsaio'] as const) {
      expect(corpoDeCalibracao({ ...bom, [campo]: '' }), campo).toBeNull();
      expect(corpoDeCalibracao({ ...bom, [campo]: 'abc' }), campo).toBeNull();
    }
  });

  // k negativo e legitimo: sensor cuja tensao CAI com a succao. O servidor
  // limita |k|, nao o sinal. Um `> 0` que aparecesse aqui rejeitaria metade
  // dos sensores possiveis sem que nada no formulario explicasse por que.
  it('aceita k negativo', () => {
    expect(corpoDeCalibracao({ ...bom, k: '-0.04' })?.k_v_por_kpa).toBe(-0.04);
  });

  it('descarta nota so de espaco', () => {
    expect('nota' in corpoDeCalibracao({ ...bom, nota: '   ' })!).toBe(false);
    expect(corpoDeCalibracao({ ...bom, nota: ' bancada ' })?.nota).toBe('bancada');
  });
});

describe('LIMITES', () => {
  // O teto de v_zero e o que torna o erro de milivolts INDIGITAVEL no campo.
  // Se ele subir para 10000 "para aceitar mV", a defesa do formulario acaba e
  // so resta a checagem fisica do servidor.
  it('mantem v_zero em faixa de volts, nao de milivolts', () => {
    expect(LIMITES.vZeroMaxV).toBe(10);
  });
});

describe('dataDoEnsaio', () => {
  // SEM Date NO CAMINHO. `new Date('2026-09-17')` e meia-noite UTC; em UTC-3
  // o dia exibido voltaria para 16, e o ensaio apareceria feito na vespera.
  it('troca os separadores sem mudar o dia', () => {
    expect(dataDoEnsaio('2026-09-17')).toBe('17/09/2026');
    expect(dataDoEnsaio('2026-01-01')).toBe('01/01/2026');
  });

  it('devolve como veio o que nao for AAAA-MM-DD', () => {
    expect(dataDoEnsaio('')).toBe('');
  });
});
