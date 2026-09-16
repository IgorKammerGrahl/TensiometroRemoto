// Idade da leitura e sua apresentacao.
//
// Por que este limiar mora no cliente e o da zona nao: a classificacao de
// zona tem uma inversao de sinal que se escreve errado com facilidade, e por
// isso existe uma unica vez, no servidor. "Leitura velha" nao tem armadilha
// de sinal -- e um limiar de apresentacao, e o servidor nao tem opiniao sobre
// ele. (Registro de desenho de 24/08, secao 4.5.)

/** Acima disso a leitura para de valer como estado atual.
 *
 * LIMITACAO CONHECIDA -- constante fixa onde caberia um valor por no.
 *
 * O valor correto e funcao do intervalo de amostragem do proprio no: algo
 * como 3x o periodo. O ensaio de bancada amostra a cada 60 s, para o qual 30
 * min e folgado. Um no em deep sleep de 30 min, que e o alvo da etapa 2 do
 * firmware, apareceria PERMANENTEMENTE como velho sob este mesmo limiar --
 * a interface acusaria falha o tempo todo num no funcionando normalmente.
 *
 * A API nao expoe o periodo de amostragem hoje. Quando expuser, este numero
 * vira `3 * device.intervalo_s` e a constante sai. Ate la e um teto conhecido,
 * nao um esquecimento. Registrado tambem no README.
 */
export const LIMIAR_LEITURA_VELHA_S = 30 * 60;

/** Idade da leitura, ancorada no servidor.
 *
 * O RELOGIO LOCAL NAO E FONTE DE VERDADE E ESTE E O PONTO DA FUNCAO.
 *
 * `medido_ha_s` vem calculado pelo servidor, que compara now() com
 * measured_at dentro do mesmo processo. Calcular a idade aqui como
 * `Date.now() - measured_at` trocaria isso por uma subtracao entre dois
 * relogios distintos: o do navegador e o do no. O ensaio de 17 h de 26-27/08
 * mediu latencia MINIMA de -0,123 s -- received_at anterior a measured_at --
 * o que so e possivel porque os carimbos vem de fontes que nao estao
 * sincronizadas. Um ESP32 sem RTC, dependente de NTP a cada boot, e onde esse
 * desvio nasce.
 *
 * Aqui o valor do servidor e a ancora e o relogio local mede apenas o
 * DECORRIDO desde o fetch -- um intervalo, nao um instante. Diferenca
 * constante entre relogios se cancela na subtracao.
 *
 * Math.max(0, ...) no decorrido: se o relogio local andar para tras (correcao
 * de NTP, usuario mudando a hora), a leitura nao pode REJUVENESCER. Idade so
 * cresce. Fail-closed: o erro possivel passa a ser mostrar um dado como mais
 * velho do que e, nunca como mais novo.
 */
export function idadeSegundos(
  medidoHaS: number,
  fetchadoEmMs: number,
  agoraMs: number,
): number {
  const decorrido = Math.max(0, (agoraMs - fetchadoEmMs) / 1000);
  // medido_ha_s pode chegar negativo: o backend aceita measured_at ate
  // agora+2h, entao um no com relogio adiantado produz leitura "do futuro".
  // Ela chegou agora de qualquer forma -- e recente, nao futura.
  return Math.max(0, medidoHaS + decorrido);
}

export function leituraVelha(idadeS: number): boolean {
  return idadeS > LIMIAR_LEITURA_VELHA_S;
}

/** Duracao legivel, SEM a preposicao.
 *
 * Devolver "6 h" e nao "ha 6 h" e o que deixa a mesma funcao servir aos dois
 * enunciados que a interface precisa: "ha 6 h" para uma leitura que ainda
 * vale e "sem dados ha 6 h" para uma que nao vale mais. Uma funcao que ja
 * embutisse "ha" obrigaria a segunda frase a ser montada por concatenacao
 * torta ou por uma segunda funcao quase igual.
 */
export function formatarDuracao(segundos: number): string {
  const s = Math.max(0, Math.floor(segundos));
  if (s < 60) return 'menos de 1 min';

  const min = Math.floor(s / 60);
  if (min < 60) return `${min} min`;

  const h = Math.floor(min / 60);
  // Horas ate 48, nao ate 24: "sem dados ha 36 h" diz mais ao produtor do que
  // "sem dados ha 1 dia". O preco e que a forma singular nunca aparece --
  // dias so comecam em 2 -- e por isso o plural aqui e incondicional.
  if (h < 48) return `${h} h`;

  return `${Math.floor(h / 24)} dias`;
}
