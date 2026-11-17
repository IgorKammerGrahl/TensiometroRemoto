import { type Contagem, itensDaContagem } from '../veredito';
import { aparencia } from '../zona';

/** Resumo de 4 ou mais sensores: a proporcao em cor + hachura, e a contagem
 *  em texto embaixo. A barra e aria-hidden: o texto visivel ja diz tudo, e
 *  diz igual para quem le e para quem ouve. */
export function BarraProporcao({ c }: { c: Contagem }) {
  const itens = itensDaContagem(c);
  if (itens.length === 0) return null;
  return (
    <div className="proporcao">
      <div className="proporcao__barra" aria-hidden="true">
        {itens.map((i) => (
          <span
            key={i.chave}
            className={`proporcao__segmento ${
              i.chave === 'sem-leitura'
                ? 'proporcao__segmento--sem-leitura'
                : `estado--${i.chave} hachura--${aparencia(i.chave).hachura}`
            }`}
            style={{ flex: `${i.n} 1 0` }}
          />
        ))}
      </div>
      <p className="proporcao__legenda">
        {itens.map((i) => (
          <span
            key={i.chave}
            className={`estado ${i.chave === 'sem-leitura' ? 'estado--sem-estado' : `estado--${i.chave}`}`}
          >
            <span
              className={`estado__marca ${
                i.chave === 'sem-leitura' ? 'estado__marca--sem-estado' : `hachura--${aparencia(i.chave).hachura}`
              }`}
              aria-hidden="true"
            />
            <span className="numero">{i.n}</span> {i.texto}
          </span>
        ))}
      </p>
    </div>
  );
}
