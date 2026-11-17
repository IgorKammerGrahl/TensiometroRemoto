import { describe, expect, it } from 'vitest';
import { corDaBarra, escolhaDe, esquemaDe } from './tema';

describe('escolhaDe', () => {
  it('sem nada guardado, o tema e o claro', () => {
    expect(escolhaDe(null)).toBe('claro');
  });
  it('le as duas escolhas que mudam alguma coisa', () => {
    expect(escolhaDe('escuro')).toBe('escuro');
    expect(escolhaDe('sistema')).toBe('sistema');
  });
  it('valor estranho no armazenamento cai no claro, nunca no escuro', () => {
    expect(escolhaDe('dark')).toBe('claro');
    expect(escolhaDe('')).toBe('claro');
  });
});

describe('esquemaDe', () => {
  it('mapeia cada escolha para o color-scheme', () => {
    // `only`: impede o Chrome de escurecer o claro por algoritmo (tema.ts).
    expect(esquemaDe('claro')).toBe('only light');
    expect(esquemaDe('escuro')).toBe('dark');
    expect(esquemaDe('sistema')).toBe('light dark');
  });
});

describe('corDaBarra', () => {
  it('claro e escuro pintam as duas metas iguais', () => {
    expect(corDaBarra('claro', false)).toBe('#f7f5f0');
    expect(corDaBarra('claro', true)).toBe('#f7f5f0');
    expect(corDaBarra('escuro', false)).toBe('#131210');
    expect(corDaBarra('escuro', true)).toBe('#131210');
  });
  it('igual ao celular devolve cada meta a sua cor', () => {
    expect(corDaBarra('sistema', false)).toBe('#f7f5f0');
    expect(corDaBarra('sistema', true)).toBe('#131210');
  });
});
