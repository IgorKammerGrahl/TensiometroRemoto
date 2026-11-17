/** Selo de no simulado: a serie inteira dele e gerada por software (cmd/seed
 *  ou cmd/simulador), nao medida por um sensor.
 *
 *  Existe porque o trabalho aceita dado simulado SE declarado. Um print da
 *  lista com vinte nos e lido como vinte sensores por quem nao viu o texto;
 *  o selo leva a declaracao junto com o print.
 *
 *  So tinta e contorno, sem cor: cor e reservada para estado (tokens.css), e
 *  "simulado" e procedencia, nao estado. Um selo colorido competiria com a
 *  marca de zona logo abaixo dele. O tracejado e o mesmo vocabulario de
 *  "aqui nao ha medida" da marca de ausencia de estado. */
export function SeloSimulado() {
  return (
    <span className="selo-simulado" title="Leituras geradas por software, não medidas em campo">
      Simulado
    </span>
  );
}
