import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { Link } from 'react-router';
import { api } from '../api';
import { CartaoTalhao } from '../componentes/CartaoTalhao';
import { LinhaDeEstado } from '../componentes/LinhaDeEstado';
import { useAgora } from '../relogio';
import { useLogout, useSessao } from '../sessao';
import { agruparPorTalhao } from '../talhoes';
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

export function Talhoes() {
  const agora = useAgora();
  const sessao = useSessao();
  const logout = useLogout();

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

      {/* Sem busca aqui: ela fica na pagina de cada talhao, onde a lista de
          sensores esta. A tela inicial so responde "preciso irrigar?". */}
      <ul className="cartoes-talhao">
        {cartoes.map(({ g, v }) => (
          <li key={g.talhao.id}>
            <CartaoTalhao g={g} v={v} fetchadoEm={q.dataUpdatedAt} agora={agora} />
          </li>
        ))}
      </ul>

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
