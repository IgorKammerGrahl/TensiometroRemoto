import type { Zona } from '../api';
import type { Situacao } from '../leitura';
import { textoDeIdade } from '../leitura';
import { aparencia } from '../zona';

/** A marca de estado, do jeito que RNF05 exige: cor, forma e palavra juntas.
 *
 * A marca colorida sozinha nunca aparece. Quem le a tela sob sol, com
 * daltonismo, ou numa impressao em preto e branco, encontra a mesma informacao
 * em outros dois canais -- a hachura e o rotulo escrito.
 *
 * O componente nao recebe limiares e nao sabe comparar nada: recebe a zona que
 * o servidor calculou e a traduz. Ver zona.ts. */
export function Estado({ zona }: { zona: Zona }) {
  const { chave, rotulo, hachura } = aparencia(zona);
  return (
    <span className={`estado estado--${chave}`}>
      <span className={`estado__marca hachura--${hachura}`} aria-hidden="true" />
      {rotulo}
    </span>
  );
}

/** Marca da situacao: o indicador de zona quando -- e SOMENTE quando -- ha
 *  leitura atual.
 *
 *  Os outros tres casos nao ganham um cinza da paleta de estado: ganham outra
 *  FORMA. O quadrado tracejado contra o circulo preenchido diz "aqui nao ha
 *  estado a informar" antes de qualquer cor ou texto, e sobrevive ao sol na
 *  tela e a impressao em preto e branco igual as hachuras. O texto diz qual
 *  dos tres e -- desativado, nunca reportou, ou parou de reportar -- porque
 *  as acoes que eles pedem sao diferentes. */
export function MarcaSituacao({ s }: { s: Situacao }) {
  if (s.tipo === 'atual') return <Estado zona={s.zona} />;
  return (
    <span className="estado estado--sem-estado">
      <span className="estado__marca estado__marca--sem-estado" aria-hidden="true" />
      {textoDeIdade(s)}
    </span>
  );
}
