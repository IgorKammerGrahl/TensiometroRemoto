import { describe, expect, it } from 'vitest';
import type { Zona } from './api';
import { aparencia } from './zona';

describe('aparencia', () => {
  // O ponto de verificacao da etapa 1: os quatro valores produzem quatro
  // saidas distintas, incluindo rotulo textual distinto. Se duas zonas
  // colapsarem no mesmo rotulo, a interface passa a comunicar estado apenas
  // por cor -- exatamente o que RNF05 proibe.
  it('da quatro saidas distintas para os quatro valores de Zona', () => {
    const zonas: Zona[] = ['conforto', 'alerta', 'estresse', null];
    const saidas = zonas.map(aparencia);

    expect(new Set(saidas.map((a) => a.chave)).size).toBe(4);
    expect(new Set(saidas.map((a) => a.rotulo)).size).toBe(4);
    expect(new Set(saidas.map((a) => a.hachura)).size).toBe(4);
    expect(new Set(saidas.map((a) => a.enfase)).size).toBe(4);
  });

  it('nao deixa rotulo nem explicacao vazios', () => {
    for (const zona of ['conforto', 'alerta', 'estresse', null] as Zona[]) {
      const a = aparencia(zona);
      expect(a.rotulo.length).toBeGreaterThan(0);
      expect(a.explicacao.length).toBeGreaterThan(0);
    }
  });

  // Zona nula NAO E CONFORTO. Este e o teste que quebra se alguem "limpar" o
  // case null mandando-o para CONFORTO por parecer o estado neutro.
  it('trata zona nula como pendencia, nunca como conforto', () => {
    const nula = aparencia(null);
    expect(nula.chave).toBe('nao-configurado');
    expect(nula.enfase).toBe('pendencia');
    expect(nula.chave).not.toBe(aparencia('conforto').chave);
    expect(nula.rotulo).not.toBe(aparencia('conforto').rotulo);
  });

  it('marca alerta e estresse com enfase, conforto sem', () => {
    expect(aparencia('conforto').enfase).toBe('nenhuma');
    expect(aparencia('alerta').enfase).toBe('atencao');
    expect(aparencia('estresse').enfase).toBe('grave');
  });

  // Os tipos nao valem nada sobre JSON vindo da rede. Um quinto valor de zona
  // no servidor nao pode virar verde neste cliente.
  it('cai em estado desconhecido, e nao em conforto, para valor fora da uniao', () => {
    const forasteira = aparencia('umido_demais' as unknown as Zona);
    expect(forasteira.chave).toBe('nao-configurado');
    expect(forasteira.rotulo).toBe('Estado desconhecido');
    expect(forasteira.rotulo).not.toBe(aparencia(null).rotulo);
  });

  // A interface e para produtor rural sem treinamento previo (RNF05).
  it('nao usa jargao tecnico nos textos', () => {
    const proibidos = ['potencial matricial', 'kpa', 'tensao matricial', 'zona'];
    for (const zona of ['conforto', 'alerta', 'estresse', null] as Zona[]) {
      const texto = `${aparencia(zona).rotulo} ${aparencia(zona).explicacao}`.toLowerCase();
      for (const termo of proibidos) expect(texto).not.toContain(termo);
    }
  });
});
