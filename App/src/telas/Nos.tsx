import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router';
import { api, ErroAPI, SEM_REDE } from '../api';
import { MarcaSituacao } from '../componentes/Estado';
import { textoKpa } from '../escala';
import { situacao, textoDeIdade } from '../leitura';
import { useAgora } from '../relogio';
import { useLogout, useSessao } from '../sessao';
import { aparencia } from '../zona';

// Lista de nos (UC02/UC05). A tela que responde "de qual talhao eu cuido
// agora?" -- e a resposta e a ORDEM.


export function Nos() {
  const agora = useAgora();
  const sessao = useSessao();
  const logout = useLogout();

  const q = useQuery({
    queryKey: ['devices'],
    queryFn: ({ signal }) => api.devices(signal),
    // Telemetria de campo nao muda a cada segundo, mas 30 s mantem a lista
    // viva sem transformar a tela aberta num pedido por minuto.
    staleTime: 30_000,
  });

  return (
    <div className="pagina">
      <header className="cabecalho">
        <div>
          <h1>Nós</h1>
          {sessao.data && <p className="secundario">{sessao.data.usuario.nome}</p>}
        </div>
        <button
          type="button"
          className="botao botao--contorno"
          onClick={() => logout.mutate()}
          disabled={logout.isPending}
        >
          Sair
        </button>
      </header>

      {q.isPending && <p className="secundario">Carregando…</p>}

      {q.isError && (
        <div className="aviso" role="alert">
          {q.error instanceof ErroAPI && q.error.status === SEM_REDE
            ? 'Sem conexão com o servidor. A lista pode estar desatualizada.'
            : 'Não foi possível carregar os nós agora.'}
        </div>
      )}

      {q.data?.devices.length === 0 && (
        <p className="secundario">
          Nenhum nó instalado nos seus talhões ainda. Cadastre o primeiro abaixo, ou peça a quem
          instalou os aparelhos que faça isso.
        </p>
      )}

      {q.data && q.data.devices.length > 0 && (
        <>
          {/* A ORDEM VEM DO SERVIDOR E FICA COMO VEIO.
              ORDER BY kpa ASC: mais negativo primeiro, ou seja, o talhao mais
              seco no topo. Nao ha .sort() nesta tela, e nao deve haver: seria
              a segunda implementacao da regra de sinal invertido, e a que
              ninguem testa. Ordenar por kpa DESC pareceria certo em qualquer
              revisao rapida e poria o no mais critico no fim da lista. */}
          <ul className="lista">
            {q.data.devices.map((d) => {
              const s = situacao(d.ativo, d.ultima, q.dataUpdatedAt, agora);
              // Sem leitura atual e talhao sem faixa recebem a MESMA enfase, e
              // isso e proposital: os dois significam "nao sei o estado deste
              // talhao". Dar a um deles um tratamento mais discreto que ao
              // outro criaria um degrau de gravidade entre duas ausencias que
              // nao tem gravidade -- so falta de informacao.
              const enfase = s.tipo === 'atual' ? aparencia(s.zona).enfase : 'pendencia';
              return (
                <li key={d.id}>
                  <Link to={`/no/${encodeURIComponent(d.id)}`} className={`cartao cartao--enfase-${enfase}`}>
                    <span className="cartao__corpo">
                      <strong>{d.descricao || d.id}</strong>
                      <span className="secundario">
                        {d.talhao.nome}
                        {d.talhao.cultura ? ` · ${d.talhao.cultura}` : ''}
                      </span>
                      <MarcaSituacao s={s} />
                    </span>

                    {/* Numero SO no caso atual.
                        Um valor cinza ao lado de "sem dados ha 6 h" seria
                        lido como o estado do talhao por qualquer pessoa
                        passando o olho na lista -- e a lista existe
                        exatamente para ser lida de relance. O ultimo valor de
                        um no mudo continua acessivel no grafico do detalhe,
                        onde o eixo do tempo deixa claro que e passado. */}
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
                </li>
              );
            })}
          </ul>

          <p className="nota secundario">
            Os nós aparecem do mais seco para o mais úmido. <strong>kPa</strong> mede o esforço da
            planta para tirar água do solo: quanto mais negativo, mais seco.
          </p>
        </>
      )}

      {/* Fora dos dois blocos condicionais de proposito: o cadastro precisa
          existir tanto para quem ainda nao tem no nenhum quanto para quem esta
          instalando o proximo. Depois da lista, e nao no cabecalho, porque a
          pergunta desta tela e "de qual talhao eu cuido agora?" -- cadastrar e
          o que se faz depois de responder, nao antes. */}
      <p className="nota">
        <Link to="/nos/novo" className="botao botao--contorno botao--largo">
          Cadastrar nó
        </Link>
      </p>
    </div>
  );
}
