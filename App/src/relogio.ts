import { useEffect, useState } from 'react';

/** Relogio que avanca sozinho.
 *
 * Existe porque a passagem de "leitura atual" para "leitura velha" e uma
 * mudanca de ESTADO que acontece sem nenhuma resposta nova do servidor: a
 * idade cresce entre um fetch e o proximo. Sem este tique, um no que ficou
 * mudo continuaria mostrando "ha 29 min" e o indicador de zona aceso ate o
 * usuario puxar a tela -- justamente o intervalo em que a interface estaria
 * afirmando algo que ja nao vale.
 *
 * 30 s e granularidade suficiente: a menor unidade que a interface imprime e
 * o minuto, e o limiar de leitura velha esta em 30 min.
 */
export function useAgora(intervaloMs = 30_000): number {
  const [agora, setAgora] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setAgora(Date.now()), intervaloMs);
    return () => clearInterval(id);
  }, [intervaloMs]);
  return agora;
}
