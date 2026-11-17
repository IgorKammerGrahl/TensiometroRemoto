import { useQuery } from '@tanstack/react-query';
import { useId, useMemo, useState } from 'react';
import { Link } from 'react-router';
import { api } from '../api';
import { CartaoNo } from '../componentes/CartaoNo';
import { CartaoTalhao } from '../componentes/CartaoTalhao';
import { LinhaDeEstado } from '../componentes/LinhaDeEstado';
import { situacao } from '../leitura';
import { useAgora } from '../relogio';
import { useLogout, useSessao } from '../sessao';
import { agruparPorTalhao, buscarNos } from '../talhoes';
import { dataPorExtenso } from '../tempo';
import { ordenarPorUrgencia, partesDaSintese, veredito } from '../veredito';

// Tela inicial (UC02/UC05): "preciso irrigar?", talhao por talhao.
//
// A RESPOSTA E POR TALHAO E VEM DE veredito.ts. A ordem dos cartoes e por
// urgencia da resposta (ordenarPorUrgencia), estavel sobre a ordem do
// servidor -- e nao por kPa: ordenar kPa aqui seria a segunda implementacao
// da regra de sinal invertido. O trade-off com a memoria espacial (talhoes
// trocam de lugar quando a resposta muda) esta na spec e vai para o teste
// com pessoas.

/** A partir de quantos sensores a busca aparece. Com poucos, so ocupa espaco. */
const BUSCA_A_PARTIR_DE = 9;
/** Teto de cartoes no resultado da busca: uma letra so casaria com centenas. */
const MAX_ACHADOS = 50;

export function Talhoes() {
  const agora = useAgora();
  const sessao = useSessao();
  const logout = useLogout();
  const idBusca = useId();
  const [termo, setTermo] = useState('');

  const q = useQuery({
    queryKey: ['devices'],
    queryFn: ({ signal }) => api.devices(signal),
    staleTime: 30_000,
    // A cadencia de envio do no: pedir mais vezes nao traz leitura nova. Em
    // segundo plano o TanStack nao pede (refetchIntervalInBackground).
    refetchInterval: 60_000,
  });

  const devices = q.data?.devices;
  const cartoes = useMemo(() => {
    if (!devices) return [];
    const comResposta = agruparPorTalhao(devices, q.dataUpdatedAt, agora).map((g) => ({
      g,
      v: veredito(g.talhao, g.nos, q.dataUpdatedAt, agora),
    }));
    return ordenarPorUrgencia(comResposta, (c) => c.v);
  }, [devices, q.dataUpdatedAt, agora]);
  const buscando = termo.trim() !== '';
  const achados = devices && buscando ? buscarNos(devices, termo) : [];

  return (
    <div className="pagina">
      <header className="cabecalho cabecalho--dia">
        <h1 className="so-leitor-de-tela">Talhões</h1>
        <p className="dia">{dataPorExtenso(agora)}</p>
        {devices && (
          <ul className="sintese">
            {partesDaSintese(cartoes.map((c) => c.v)).map((p) => (
              <li key={p}>{p}</li>
            ))}
          </ul>
        )}
      </header>

      <LinhaDeEstado q={q} agora={agora} carregando="Carregando os talhões…" />

      {devices?.length === 0 && (
        <p className="secundario">
          Cadastre o primeiro sensor abaixo, ou peça a quem instalou os aparelhos que faça isso.
        </p>
      )}

      {devices && devices.length >= BUSCA_A_PARTIR_DE && (
        <div className="campo busca">
          <label htmlFor={idBusca}>Procurar sensor ou talhão</label>
          <input
            id={idBusca}
            type="search"
            autoComplete="off"
            value={termo}
            onChange={(e) => setTermo(e.target.value)}
          />
        </div>
      )}

      {buscando ? (
        <section aria-label="Resultado da busca">
          <p className="secundario" aria-live="polite">
            {achados.length === 0
              ? 'Nenhum sensor com esse nome, código ou talhão.'
              : `${achados.length} ${achados.length === 1 ? 'sensor encontrado' : 'sensores encontrados'}, do mais seco para o mais úmido.`}
          </p>
          <ul className="lista">
            {achados.slice(0, MAX_ACHADOS).map((d) => (
              <li key={d.id}>
                <CartaoNo d={d} s={situacao(d.ativo, d.ultima, q.dataUpdatedAt, agora)} />
              </li>
            ))}
          </ul>
          {achados.length > MAX_ACHADOS && (
            <p className="nota secundario">
              Mostrando os {MAX_ACHADOS} primeiros. Escreva mais do nome para achar os outros.
            </p>
          )}
        </section>
      ) : (
        <ul className="cartoes-talhao">
          {cartoes.map(({ g, v }) => (
            <li key={g.talhao.id}>
              <CartaoTalhao g={g} v={v} fetchadoEm={q.dataUpdatedAt} agora={agora} />
            </li>
          ))}
        </ul>
      )}

      {devices && devices.length > 0 && (
        <p className="nota secundario">
          Os talhões aparecem do que mais precisa de água para o que menos. A resposta de cada um vale
          para a maioria dos seus sensores, pelos limites de irrigação do talhão.
        </p>
      )}

      {/* Ate existir Ajustes (Task 10): cadastrar e sair ficam aqui, no fim --
          a pergunta desta tela e "preciso irrigar?", e cadastrar ou sair e o
          que se faz depois de responder. */}
      <p className="nota">
        <Link to="/nos/novo" className="botao botao--contorno botao--largo">
          Cadastrar sensor
        </Link>
      </p>
      <p>
        {sessao.data && <span className="secundario">{sessao.data.usuario.nome} · </span>}
        <button
          type="button"
          className="botao botao--contorno"
          onClick={() => logout.mutate()}
          disabled={logout.isPending}
        >
          Sair
        </button>
      </p>
    </div>
  );
}
