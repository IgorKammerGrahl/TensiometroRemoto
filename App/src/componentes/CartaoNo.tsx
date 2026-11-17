import { Link } from 'react-router';
import type { DeviceApp } from '../api';
import { textoKpa } from '../escala';
import { type Situacao, textoDeIdade } from '../leitura';
import { aparencia } from '../zona';
import { MarcaSituacao } from './Estado';
import { SeloSimulado } from './Simulado';

/** Um no na lista: nome, talhao, estado e, so se a leitura e atual, o valor.
 *  Saiu de Nos.tsx quando a lista passou a aparecer em dois lugares (dentro de
 *  cada talhao e no resultado da busca) -- uma copia so do cartao. */
/** `comSelo={false}` dentro de um talhao todo simulado: o cartao do talhao ja
 *  declara, e repetir o selo em cada sensor e ruido. */
export function CartaoNo({
  d,
  s,
  comTalhao = true,
  comSelo = true,
}: {
  d: DeviceApp;
  s: Situacao;
  comTalhao?: boolean;
  comSelo?: boolean;
}) {
  // Sem leitura atual e talhao sem faixa recebem a MESMA enfase, e isso e
  // proposital: os dois significam "nao sei o estado deste talhao". Dar a um
  // deles um tratamento mais discreto que ao outro criaria um degrau de
  // gravidade entre duas ausencias que nao tem gravidade -- so falta de
  // informacao.
  const enfase = s.tipo === 'atual' ? aparencia(s.zona).enfase : 'pendencia';
  return (
    <Link to={`/no/${encodeURIComponent(d.id)}`} className={`cartao cartao--enfase-${enfase}`}>
      <span className="cartao__corpo">
        <strong>
          {d.descricao || d.id}
          {d.simulado && comSelo && <SeloSimulado />}
        </strong>
        {/* Dentro do grupo do talhao o nome do talhao ja esta no titulo;
            repeti-lo em cada cartao e ruido. Na busca, que mistura talhoes,
            ele volta. */}
        {comTalhao && (
          <span className="secundario">
            {d.talhao.nome}
            {d.talhao.cultura ? ` · ${d.talhao.cultura}` : ''}
          </span>
        )}
        <MarcaSituacao s={s} />
      </span>

      {/* Numero SO no caso atual.
          Um valor cinza ao lado de "sem dados ha 6 h" seria lido como o estado
          do talhao por qualquer pessoa passando o olho na lista -- e a lista
          existe exatamente para ser lida de relance. O ultimo valor de um no
          mudo continua acessivel no grafico do detalhe, onde o eixo do tempo
          deixa claro que e passado. */}
      <span className="cartao__valor">
        {s.tipo === 'atual' && (
          <>
            <span className="numero">{textoKpa(s.kpa)}</span>
            <span className="cartao__unidade"> kPa</span>
            <span className="secundario">{textoDeIdade(s)}</span>
          </>
        )}
      </span>
    </Link>
  );
}
