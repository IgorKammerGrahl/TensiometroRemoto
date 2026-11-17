import type { Situacao } from './leitura';
import type { Veredito } from './veredito';

// O "visual" de uma resposta: qual marca, faixa e pictograma a tela desenha.
// Separado da regra (veredito.ts) porque e apresentacao, e da zona (zona.ts)
// porque tem estados que nao sao zona: sem limites, sem leitura, desligado.
// Cada um com o seu nome, para nenhum ausente parecer um grau da escala.

export type Visual =
  | 'conforto'
  | 'alerta'
  | 'estresse'
  | 'sem-limites'
  | 'indeterminado'
  | 'sem-leitura'
  | 'desligado';

export function visualDoVeredito(v: Veredito): Visual {
  switch (v.tipo) {
    case 'zona':
      return v.zona;
    case 'sem-limites':
      return 'sem-limites';
    case 'indeterminado':
      return 'indeterminado';
    case 'poucos-dados':
      return 'sem-leitura';
    case 'sem-sensor':
      return 'desligado';
  }
}

export function visualDaSituacao(s: Situacao): Visual {
  switch (s.tipo) {
    case 'desativado':
      return 'desligado';
    case 'nunca':
    case 'velha':
      return 'sem-leitura';
    case 'atual': {
      // unknown de proposito: o JSON da rede pode trazer uma zona que esta
      // versao nao conhece, e ela nao pode virar cor nenhuma.
      const z: unknown = s.zona;
      if (z === 'conforto' || z === 'alerta' || z === 'estresse') return z;
      return z === null ? 'sem-limites' : 'indeterminado';
    }
  }
}
