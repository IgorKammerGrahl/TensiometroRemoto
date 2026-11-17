import type { ReactNode } from 'react';
import type { Visual } from '../visual';

// Pictogramas de solo: o quarto canal do estado, para quem le pouco.
//
// Sempre AO LADO da palavra, nunca no lugar dela: sao aria-hidden, porque a
// palavra e a informacao. Desenhados em tinta sobre o miolo claro da marca --
// a cor e a hachura ficam no anel em volta --, entao continuam legiveis no
// sol, em preto e branco e com daltonismo.
//
// Grade de 48, traco unico de 3,5, cantos arredondados, pensado para 64 px.
// A gota e o elemento que muda: tres gotas, uma, nenhuma e o solo rachado.

/** Gota com a ponta em (cx, topo); `e` escala o tamanho (1 = 12 de altura). */
const gota = (cx: number, topo: number, e: number) =>
  `M${cx} ${topo}C${cx} ${topo} ${cx - 5 * e} ${topo + 6 * e} ${cx - 5 * e} ${topo + 9 * e}` +
  `A${5 * e} ${5 * e} 0 0 0 ${cx + 5 * e} ${topo + 9 * e}C${cx + 5 * e} ${topo + 6 * e} ${cx} ${topo} ${cx} ${topo}Z`;

/** Torrao: um monte de terra com base reta. */
const TORRAO = 'M3 43C4 35 9 31 15 32C18 26 30 25 33 30C39 29 45 34 45 43Z';

/** Interrogacao: "nao da para dizer". */
const PERGUNTA = (
  <>
    <path d="M19 13a5 5 0 1 1 7.5 4.3C24.8 18.4 24 19.6 24 21.5" />
    <path d="M24 26.5v0.5" />
  </>
);

const DESENHOS: Readonly<Record<Visual, ReactNode>> = {
  // Umido: tres gotas sobre a terra.
  conforto: (
    <>
      <path d={TORRAO} />
      <path d={gota(11, 5, 1)} />
      <path d={gota(24, 1, 1.2)} />
      <path d={gota(37, 5, 1)} />
    </>
  ),
  // Secando: uma gota so.
  alerta: (
    <>
      <path d={TORRAO} />
      <path d={gota(24, 1, 1.6)} />
    </>
  ),
  // Seco: terra rachada, nenhuma gota.
  estresse: (
    <>
      <path d={TORRAO} />
      <path d="M16 32l4 5-3 6M31 30l-3 6 4 7" />
      {/* O sol em cima: secou. */}
      <circle cx="24" cy="13" r="5" />
      <path d="M24 2v2M13 13h-2M37 13h-2M16 5l1.5 1.5M32 5l-1.5 1.5" />
    </>
  ),
  // Sem limites: terra em tracejado e a pergunta em cima.
  'sem-limites': (
    <>
      <path d={TORRAO} strokeDasharray="4 4" />
      {PERGUNTA}
    </>
  ),
  indeterminado: (
    <>
      <path d={TORRAO} strokeDasharray="4 4" />
      {PERGUNTA}
    </>
  ),
  // Sem leitura recente: um relogio.
  'sem-leitura': (
    <>
      <circle cx="24" cy="24" r="16" />
      <path d="M24 14v10l7 4" />
    </>
  ),
  // Desligado: tomada fora do lugar.
  desligado: (
    <>
      <path d="M18 7v8M30 7v8" />
      <path d="M13 15h22v7a11 11 0 0 1-22 0z" />
      <path d="M24 33v4M24 41v1" />
    </>
  ),
};

export function Pictograma({ visual }: { visual: Visual }) {
  return (
    <svg
      className="pictograma"
      viewBox="0 0 48 48"
      aria-hidden="true"
      focusable="false"
      fill="none"
      stroke="currentColor"
      strokeWidth={3.5}
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {DESENHOS[visual]}
    </svg>
  );
}
