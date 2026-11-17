import type { Zona } from '../api';
import { bandas, FUNDO_KPA, posicao, textoKpa } from '../escala';
import type { ContagemTalhao, GrupoTalhao } from '../talhoes';
import { aparencia } from '../zona';

// A TIRA DO TALHAO: a regua do detalhe, deitada, com um traco por no.
//
// Com tres nos, a lista basta. Com quarenta, a lista vira rolagem, e a
// pergunta "como esta este talhao?" so se responde lendo cartao por cartao. A
// tira responde de relance: os tracos se acumulam onde os nos estao, e a
// sobreposicao escurece -- densidade sem calculo nenhum, porque cada traco e
// semitransparente e o navegador soma.
//
// Mesma escala (0 a -80, pior para a DIREITA, como pior para baixo na regua),
// mesmas faixas, mesmas hachuras: quem aprendeu a regua le a tira. A cor
// continua morando so no preenchimento das faixas; os tracos sao tinta.
//
// Traco so de leitura ATUAL (ver agruparPorTalhao). Os nos sem leitura atual
// aparecem na contagem, por escrito, e nao na escala.

/** Ordem de gravidade: quem precisa de acao primeiro. O texto reusa as
 *  palavras dos rotulos de estado (Seco, Secando, Umido) para que a
 *  contagem e o cartao digam a mesma coisa, so que na forma de uma frase
 *  que aceita numero na frente. */
const ESTADOS: ReadonlyArray<{
  campo: keyof ContagemTalhao;
  zona: Zona | 'sem-leitura';
  texto: (n: number) => string;
}> = [
  { campo: 'estresse', zona: 'estresse', texto: (n) => (n === 1 ? 'seco' : 'secos') },
  { campo: 'alerta', zona: 'alerta', texto: () => 'secando' },
  { campo: 'conforto', zona: 'conforto', texto: (n) => (n === 1 ? 'úmido' : 'úmidos') },
  { campo: 'semFaixa', zona: null, texto: () => 'sem limites para classificar' },
  { campo: 'semLeitura', zona: 'sem-leitura', texto: () => 'sem leitura recente' },
];

export function TiraTalhao({ g }: { g: GrupoTalhao }) {
  const faixas = bandas(g.talhao.kpa_alerta, g.talhao.kpa_estresse);
  const limiares =
    faixas.length > 0
      ? [
          { kpa: g.talhao.kpa_alerta!, fracao: faixas[0]!.ate },
          { kpa: g.talhao.kpa_estresse!, fracao: faixas[1]!.ate },
        ]
      : [];

  const n = g.leituras.length;
  // A lista vem do mais seco para o mais umido: extremos sem varrer nada.
  const descricao =
    n === 0
      ? 'Nenhum sensor deste talhão tem leitura recente.'
      : `Leituras recentes de ${n} ${n === 1 ? 'sensor' : 'sensores'}, de ${textoKpa(
          Math.max(...g.leituras),
        )} a ${textoKpa(Math.min(...g.leituras))} kPa.`;

  return (
    <div className="tira">
      <figure className="tira__figura" role="img" aria-label={descricao}>
        <div className={`tira__tracos${faixas.length === 0 ? ' tira__tracos--sem-faixa' : ''}`}>
          {limiares.map((l) => (
            <span key={l.kpa} className="tira__limiar" style={{ left: `${l.fracao * 100}%` }} />
          ))}
          {g.leituras.map((kpa, i) => {
            const p = posicao(kpa);
            return p ? <span key={i} className="tira__traco" style={{ left: `${p.fracao * 100}%` }} /> : null;
          })}
        </div>
        <div className={`tira__faixas${faixas.length === 0 ? ' tira__faixas--sem-faixa' : ''}`}>
          {faixas.map((b) => (
            <span
              key={b.chave}
              className={`tira__faixa regua__banda--${b.chave} hachura--${aparencia(b.chave).hachura}`}
              style={{ left: `${b.de * 100}%`, width: `${(b.ate - b.de) * 100}%` }}
            />
          ))}
        </div>
        <div className="tira__eixo" aria-hidden="true">
          <span className="tira__eixo-ponta">
            <strong className="numero">0</strong> encharcado
          </span>
          {limiares.map((l) => (
            <span key={l.kpa} className="tira__eixo-limiar numero" style={{ left: `${l.fracao * 100}%` }}>
              {textoKpa(l.kpa)}
            </span>
          ))}
          <span className="tira__eixo-ponta tira__eixo-ponta--fim">
            seco <strong className="numero">{textoKpa(FUNDO_KPA)}</strong>
          </span>
        </div>
      </figure>

      <ul className="tira__contagem">
        {ESTADOS.filter((e) => g.contagem[e.campo] > 0).map((e) => {
          const qtd = g.contagem[e.campo];
          if (e.zona === 'sem-leitura') {
            return (
              <li key={e.campo} className="estado estado--sem-estado">
                <span className="estado__marca estado__marca--sem-estado" aria-hidden="true" />
                <span className="numero">{qtd}</span> {e.texto(qtd)}
              </li>
            );
          }
          const a = aparencia(e.zona);
          return (
            <li key={e.campo} className={`estado estado--${a.chave}`}>
              <span className={`estado__marca hachura--${a.hachura}`} aria-hidden="true" />
              <span className="numero">{qtd}</span> {e.texto(qtd)}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
