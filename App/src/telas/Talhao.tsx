import { useQuery } from '@tanstack/react-query';
import { useId, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router';
import { api } from '../api';
import { BarraProporcao } from '../componentes/BarraProporcao';
import { CartaoNo } from '../componentes/CartaoNo';
import { Faixa } from '../componentes/Faixa';
import { LinhaDeEstado } from '../componentes/LinhaDeEstado';
import { MarcaVeredito } from '../componentes/MarcaVeredito';
import { TiraTalhao } from '../componentes/TiraTalhao';
import { textoKpa } from '../escala';
import { situacao } from '../leitura';
import { useAgora } from '../relogio';
import { agruparPorTalhao, buscarNos } from '../talhoes';
import { frasesDoVeredito, veredito } from '../veredito';
import { visualDoVeredito } from '../visual';

// Um talhao: a resposta, de onde ela vem, e os sensores. Mesma consulta da
// tela inicial (['devices']): nenhuma rota nova na API, e ir e voltar entre as
// duas telas nao pede nada ao servidor.

/** A partir de quantos sensores a busca aparece. Com poucos, so ocupa espaco. */
const BUSCA_A_PARTIR_DE = 9;
/** A partir de quantos sensores ligados a barra de proporcao resume o talhao. */
const BARRA_A_PARTIR_DE = 4;

export function TelaTalhao() {
  const { id = '' } = useParams();
  const agora = useAgora();
  const idBusca = useId();
  const [termo, setTermo] = useState('');

  const q = useQuery({
    queryKey: ['devices'],
    queryFn: ({ signal }) => api.devices(signal),
    staleTime: 30_000,
    refetchInterval: 60_000,
  });

  const g = useMemo(
    () =>
      q.data
        ? (agruparPorTalhao(q.data.devices, q.dataUpdatedAt, agora).find((x) => x.talhao.id === id) ?? null)
        : null,
    [q.data, q.dataUpdatedAt, agora, id],
  );
  const v = g ? veredito(g.talhao, g.nos, q.dataUpdatedAt, agora) : null;
  const frases = v ? frasesDoVeredito(v) : null;
  const buscando = termo.trim() !== '';
  const nos = g ? (buscando ? buscarNos(g.nos, termo) : g.nos) : [];
  const ligados = g ? g.nos.filter((d) => d.ativo).length : 0;
  const todosSimulados = g ? g.simulados > 0 && g.simulados === g.nos.length : false;
  const comBarra = ligados >= BARRA_A_PARTIR_DE && v?.tipo !== 'sem-limites';

  return (
    <div className="pagina">
      <header className="cabecalho cabecalho--detalhe">
        <Link to="/" className="voltar">
          ← Talhões
        </Link>
      </header>

      <LinhaDeEstado q={q} agora={agora} carregando="Carregando o talhão…" />

      {q.data && !g && (
        <div className="aviso" role="alert">
          Este talhão não existe ou não está entre os seus.
        </div>
      )}

      {g && v && frases && (
        <div className="pilha conteudo">
          <div>
            <h1>{g.talhao.nome}</h1>
            <p className="secundario">
              {[
                g.talhao.cultura,
                `${g.nos.length} ${g.nos.length === 1 ? 'sensor' : 'sensores'}`,
                todosSimulados ? 'dados simulados' : null,
              ]
                .filter(Boolean)
                .join(' · ')}
            </p>
          </div>

          <section className="agora" aria-label="Resposta do talhão">
            <div className="resposta">
              <MarcaVeredito visual={visualDoVeredito(v)} />
              <div className="resposta__texto">
                <p className="resposta__principal">{frases.principal}</p>
                {frases.linhas.map((l) => (
                  <p key={l} className="resposta__linha">
                    {l}
                  </p>
                ))}
              </div>
            </div>
            {v.tipo === 'zona' && g.talhao.kpa_alerta !== null && g.talhao.kpa_estresse !== null && (
              /* De onde vem a resposta. Os limites aqui so sao EXIBIDOS, nunca
                 comparados: a zona veio do servidor (ver veredito.ts). */
              <p className="secundario">
                Pelos limites de irrigação deste talhão: secando a partir de{' '}
                <span className="numero">{textoKpa(g.talhao.kpa_alerta)}</span> kPa, seco a partir de{' '}
                <span className="numero">{textoKpa(g.talhao.kpa_estresse)}</span> kPa. Vale o que diz a
                maioria dos sensores.
              </p>
            )}
            {comBarra && <BarraProporcao c={v.contagem} />}
          </section>

          {g.nos.length > 1 && (
            <section className="pilha conteudo">
              <h2>Onde está cada sensor</h2>
              <TiraTalhao g={g} semContagem={comBarra} />
            </section>
          )}

          <section className="pilha conteudo">
            <h2>Sensores</h2>
            {g.nos.length >= BUSCA_A_PARTIR_DE && (
              <div className="campo busca">
                <label htmlFor={idBusca}>Procurar sensor</label>
                <input
                  id={idBusca}
                  type="search"
                  autoComplete="off"
                  value={termo}
                  onChange={(e) => setTermo(e.target.value)}
                />
              </div>
            )}
            {buscando && nos.length === 0 && (
              <p className="secundario" aria-live="polite">
                Nenhum sensor com esse nome ou código.
              </p>
            )}
            <ul className="lista">
              {nos.map((d) => (
                <li key={d.id}>
                  <CartaoNo
                    d={d}
                    s={situacao(d.ativo, d.ultima, q.dataUpdatedAt, agora)}
                    comTalhao={false}
                    comSelo={!todosSimulados}
                  />
                </li>
              ))}
            </ul>
          </section>

          {/* Fechado no fim, como na tela do sensor; aberto quando falta,
              porque ai e o conserto do "Limites nao definidos" la de cima. */}
          <details className="dobra" open={v.tipo === 'sem-limites'}>
            <summary>Limites de irrigação</summary>
            <Faixa talhao={g.talhao} />
          </details>
        </div>
      )}
    </div>
  );
}
