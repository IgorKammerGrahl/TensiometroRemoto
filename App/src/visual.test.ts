import { describe, expect, it } from 'vitest';
import type { DeviceApp, Talhao, Zona } from './api';
import { veredito } from './veredito';
import { visualDaSituacao, visualDoVeredito } from './visual';

const T: Talhao = { id: 't', nome: 'T', cultura: null, kpa_alerta: -30, kpa_estresse: -60 };
const d = (ativo: boolean, zona: Zona | 'nunca'): DeviceApp => ({
  id: 'x',
  descricao: 'x',
  ativo,
  simulado: false,
  talhao: T,
  ultima: zona === 'nunca' ? null : { measured_at: '', kpa: -20, zona, medido_ha_s: 60 },
});

describe('visualDoVeredito', () => {
  it('zona vira a propria zona; o resto, um visual neutro com nome', () => {
    expect(visualDoVeredito(veredito(T, [d(true, 'alerta')], 0, 0))).toBe('alerta');
    expect(
      visualDoVeredito(veredito({ ...T, kpa_alerta: null, kpa_estresse: null }, [d(true, null)], 0, 0)),
    ).toBe('sem-limites');
    expect(visualDoVeredito(veredito(T, [d(true, 'nunca')], 0, 0))).toBe('sem-leitura');
    expect(visualDoVeredito(veredito(T, [d(false, 'conforto')], 0, 0))).toBe('desligado');
    expect(visualDoVeredito(veredito(T, [d(true, null)], 0, 0))).toBe('indeterminado');
  });
});

describe('visualDaSituacao', () => {
  it('um visual por situacao; zona desconhecida nao vira cor', () => {
    expect(visualDaSituacao({ tipo: 'desativado' })).toBe('desligado');
    expect(visualDaSituacao({ tipo: 'nunca' })).toBe('sem-leitura');
    expect(visualDaSituacao({ tipo: 'velha', idadeS: 9999, kpa: -20 })).toBe('sem-leitura');
    expect(visualDaSituacao({ tipo: 'atual', idadeS: 60, kpa: -20, zona: 'estresse' })).toBe('estresse');
    expect(visualDaSituacao({ tipo: 'atual', idadeS: 60, kpa: -20, zona: null })).toBe('sem-limites');
    expect(visualDaSituacao({ tipo: 'atual', idadeS: 60, kpa: -20, zona: 'x' as unknown as Zona })).toBe(
      'indeterminado',
    );
  });
});
