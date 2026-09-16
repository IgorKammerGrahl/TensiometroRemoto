import { bandas, FUNDO_KPA, posicao, textoKpa, TOPO_KPA } from '../escala';
import { aparencia } from '../zona';

// A regua de escala. E a ancora visual do aplicativo -- o icone do PWA e ela
// reduzida ao essencial -- e a unica tela em que a grandeza medida aparece
// como POSICAO e nao como numero.
//
// Ela nao recebe zona. A banda em que o entalhe cai e geometria; a zona e a
// classificacao do servidor e vive no indicador ao lado. Passar a zona para
// ca abriria caminho para "pintar o entalhe conforme a zona", que e a
// classificacao acontecendo duas vezes com duas fontes.
//
// Em HTML e nao em SVG de proposito: as hachuras ja existem como classe CSS
// (.hachura--*), os rotulos herdam a tipografia real do documento em vez de
// escalarem junto com um viewBox, e nao ha <pattern> nem defs para manter.

type Props = {
  /** kPa a marcar, ou null quando nao ha leitura ATUAL.
   *
   *  null cobre no desativado, no que nunca reportou e leitura velha. Nos
   *  tres a escala e as bandas continuam desenhadas -- elas descrevem o
   *  talhao, nao o instante -- e o que some e o entalhe. Marcar a ultima
   *  posicao conhecida em cinza pareceria cuidadoso e seria o mesmo erro de
   *  sempre: um numero morto ocupando o lugar reservado ao estado atual. O
   *  historico de um no mudo esta no grafico, que e o componente cujo
   *  assunto e o passado. */
  kpa: number | null;
  kpaAlerta: number | null;
  kpaEstresse: number | null;
};


export function Regua({ kpa, kpaAlerta, kpaEstresse }: Props) {
  const faixas = bandas(kpaAlerta, kpaEstresse);
  const marca = kpa === null ? null : posicao(kpa);
  const configurada = faixas.length > 0;

  return (
    <figure className="regua">
      <div className="regua__corpo">
        <div
          className={`regua__escala${configurada ? '' : ' regua__escala--sem-faixa'}`}
          role="img"
          aria-label={descrever(kpa, marca, faixas.length > 0)}
        >
          {faixas.map((b) => (
            <div
              key={b.chave}
              className={`regua__banda regua__banda--${b.chave} hachura--${aparencia(b.chave).hachura}`}
              style={{ top: `${b.de * 100}%`, height: `${(b.ate - b.de) * 100}%` }}
            />
          ))}

          {/* Fronteiras rotuladas com o limiar bruto. E o unico consumidor
              legitimo de kpa_alerta/kpa_estresse junto com as linhas do
              grafico -- e aqui eles sao rotulo, nunca operando. */}
          {faixas.slice(0, -1).map((b) => (
            <div key={`f-${b.chave}`} className="regua__fronteira" style={{ top: `${b.ate * 100}%` }} />
          ))}

          {marca && (
            <div className="regua__entalhe" style={{ top: `${marca.fracao * 100}%` }}>
              <span className="regua__entalhe-valor numero">
                {textoKpa(kpa!)}
                <span className="regua__unidade"> kPa</span>
              </span>
            </div>
          )}
        </div>

        <div className="regua__eixo">
          <span>
            <strong className="numero">0</strong> solo encharcado
          </span>
          {faixas.slice(0, -1).map((b, i) => (
            <span
              key={`r-${b.chave}`}
              className="regua__eixo-limiar"
              style={{ top: `${b.ate * 100}%` }}
            >
              <strong className="numero">{textoKpa(i === 0 ? kpaAlerta! : kpaEstresse!)}</strong>
            </span>
          ))}
          <span>
            <strong className="numero">{textoKpa(FUNDO_KPA)}</strong> solo seco
          </span>
        </div>
      </div>

      <figcaption className="regua__legenda">
        {configurada ? (
          <ul className="regua__chaves">
            {faixas.map((b) => {
              const a = aparencia(b.chave);
              return (
                <li key={b.chave} className={`estado estado--${a.chave}`}>
                  <span className={`estado__marca hachura--${a.hachura}`} aria-hidden="true" />
                  {a.rotulo}
                </li>
              );
            })}
          </ul>
        ) : (
          <p className="regua__sem-faixa">
            <strong>{aparencia(null).rotulo}.</strong> {aparencia(null).explicacao} A escala
            continua valendo: o traço mostra onde a leitura está entre encharcado e seco.
          </p>
        )}
        {marca?.fora === 'abaixo' && (
          <p className="regua__fora">
            A leitura passou do fim da escala. O traço está no limite do desenho, não no valor.
          </p>
        )}
        {marca?.fora === 'acima' && (
          <p className="regua__fora">
            A leitura está acima de {textoKpa(TOPO_KPA)}, fora da escala — solo saturado ou
            sensor fora do lugar.
          </p>
        )}
      </figcaption>
    </figure>
  );
}

/** Texto para quem nao ve o desenho. Diz a mesma coisa que a figura, nao
 *  menos: sem isto a regua e informacao exclusiva de quem enxerga. */
function descrever(kpa: number | null, marca: ReturnType<typeof posicao>, temFaixa: boolean): string {
  const escala = `Escala de ${textoKpa(TOPO_KPA)} a ${textoKpa(FUNDO_KPA)} quilopascal, do solo encharcado no topo ao solo seco embaixo.`;
  const faixa = temFaixa
    ? ' As faixas de umidade boa, secando e seco estão marcadas.'
    : ' As faixas de umidade não estão configuradas para este talhão.';
  if (kpa === null || marca === null) return `${escala}${faixa} Sem leitura atual para marcar.`;
  return `${escala}${faixa} Leitura atual em ${textoKpa(kpa)} quilopascal.`;
}
