import type { Visual } from '../visual';
import { aparencia } from '../zona';

/** Classes de cor + hachura de um visual. Zonas: a cor e a hachura validadas
 *  (zona.ts). Sem limites e indeterminado: vazado, como "sem limites" sempre
 *  foi. Sem leitura e desligado: a forma de "aqui nao ha estado" (quadrado
 *  tracejado), nao uma cor. */
export function classesDoVisual(visual: Visual): string {
  switch (visual) {
    case 'conforto':
    case 'alerta':
    case 'estresse':
      return `estado--${visual} hachura--${aparencia(visual).hachura}`;
    case 'sem-limites':
    case 'indeterminado':
      return 'estado--nao-configurado hachura--vazado';
    case 'sem-leitura':
    case 'desligado':
      return 'sem-estado';
  }
}

/** A marca grande ao lado da resposta. Decorativa para leitor de tela: a
 *  palavra ao lado e a informacao. */
export function MarcaVeredito({ visual, pequena = false }: { visual: Visual; pequena?: boolean }) {
  return (
    <span
      className={`marca-veredito${pequena ? ' marca-veredito--pequena' : ''} ${classesDoVisual(visual)}`}
      aria-hidden="true"
    />
  );
}
