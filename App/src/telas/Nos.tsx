import { useQuery } from '@tanstack/react-query';
import { useId, useMemo, useState } from 'react';
import { Link } from 'react-router';
import { api, ErroAPI, SEM_REDE } from '../api';
import { CartaoNo } from '../componentes/CartaoNo';
import { TiraTalhao } from '../componentes/TiraTalhao';
import { situacao } from '../leitura';
import { useAgora } from '../relogio';
import { useLogout, useSessao } from '../sessao';
import { agruparPorTalhao, buscarNos, type GrupoTalhao } from '../talhoes';

// Lista de nos (UC02/UC05). A tela que responde "de qual talhao eu cuido
// agora?" -- e a resposta e a ORDEM.
//
// A ORDEM VEM DO SERVIDOR E FICA COMO VEIO.
// ORDER BY kpa ASC: mais negativo primeiro, ou seja, o mais seco no topo. Nao
// ha .sort() nesta tela, e nao deve haver: seria a segunda implementacao da
// regra de sinal invertido, e a que ninguem testa. Ordenar por kpa DESC
// pareceria certo em qualquer revisao rapida e poria o no mais critico no fim.
// O agrupamento por talhao (talhoes.ts) preserva essa ordem nos dois niveis.

/** Cartoes visiveis por talhao antes de "mostrar os outros": os mais secos. */
const PRIMEIROS = 3;
/** A partir de quantos nos a busca aparece. Com poucos, ela so ocupa espaco. */
const BUSCA_A_PARTIR_DE = 9;
/** Teto de cartoes no resultado da busca: uma letra so casaria com centenas. */
const MAX_ACHADOS = 50;

export function Nos() {
  const agora = useAgora();
  const sessao = useSessao();
  const logout = useLogout();
  const idBusca = useId();
  const [termo, setTermo] = useState('');

  const q = useQuery({
    queryKey: ['devices'],
    queryFn: ({ signal }) => api.devices(signal),
    // Telemetria de campo nao muda a cada segundo, mas 30 s mantem a lista
    // viva sem transformar a tela aberta num pedido por minuto.
    staleTime: 30_000,
  });

  const devices = q.data?.devices;
  const grupos = useMemo(
    () => (devices ? agruparPorTalhao(devices, q.dataUpdatedAt, agora) : []),
    [devices, q.dataUpdatedAt, agora],
  );
  const buscando = termo.trim() !== '';
  const achados = devices && buscando ? buscarNos(devices, termo) : [];

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

      {devices?.length === 0 && (
        <p className="secundario">
          Nenhum nó instalado nos seus talhões ainda. Cadastre o primeiro abaixo, ou peça a quem
          instalou os aparelhos que faça isso.
        </p>
      )}

      {devices && devices.length > 0 && (
        <>
          <p className="secundario">
            <span className="numero">{devices.length}</span> {devices.length === 1 ? 'nó' : 'nós'} em{' '}
            <span className="numero">{grupos.length}</span> {grupos.length === 1 ? 'talhão' : 'talhões'}
          </p>

          {devices.length >= BUSCA_A_PARTIR_DE && (
            <div className="campo busca">
              <label htmlFor={idBusca}>Buscar nó ou talhão</label>
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
                  ? 'Nenhum nó com esse nome, código ou talhão.'
                  : `${achados.length} ${achados.length === 1 ? 'nó encontrado' : 'nós encontrados'}, do mais seco para o mais úmido.`}
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
            <div className="talhoes">
              {grupos.map((g) => (
                <GrupoDoTalhao key={g.talhao.id} g={g} fetchadoEm={q.dataUpdatedAt} agora={agora} />
              ))}
            </div>
          )}

          <p className="nota secundario">
            Os talhões aparecem a partir do que tem o nó mais seco, e os nós de cada um, do mais seco
            para o mais úmido. <strong>kPa</strong> mede o esforço da planta para tirar água do solo:
            quanto mais negativo, mais seco.
          </p>
        </>
      )}

      {/* Fora dos blocos condicionais de proposito: o cadastro precisa existir
          tanto para quem ainda nao tem no nenhum quanto para quem esta
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

function GrupoDoTalhao({ g, fetchadoEm, agora }: { g: GrupoTalhao; fetchadoEm: number; agora: number }) {
  const idTitulo = useId();
  const [todos, setTodos] = useState(false);
  const n = g.nos.length;
  const visiveis = todos ? g.nos : g.nos.slice(0, PRIMEIROS);

  const simulados =
    g.simulados === 0 ? null : g.simulados === n ? (n === 1 ? 'simulado' : 'todos simulados') : `${g.simulados} simulados`;
  const detalhes = [g.talhao.cultura, `${n} ${n === 1 ? 'nó' : 'nós'}`, simulados].filter(Boolean).join(' · ');

  return (
    <section className="talhao" aria-labelledby={idTitulo}>
      <header className="talhao__cabecalho">
        <h2 id={idTitulo}>{g.talhao.nome}</h2>
        <p className="secundario">{detalhes}</p>
      </header>

      {/* O resumo existe para representar os nos que nao estao na tela. Com
          todos os cartoes visiveis, a tira e a contagem so repetiriam o que
          esta logo abaixo -- e uma tira vazia sobre um unico no parado
          ocuparia meia tela para dizer nada. */}
      {n > PRIMEIROS && <TiraTalhao g={g} />}

      <ul className="lista">
        {visiveis.map((d) => (
          <li key={d.id}>
            <CartaoNo d={d} s={situacao(d.ativo, d.ultima, fetchadoEm, agora)} comTalhao={false} />
          </li>
        ))}
      </ul>

      {n > PRIMEIROS && (
        <button
          type="button"
          className="botao botao--contorno botao--largo"
          aria-expanded={todos}
          onClick={() => setTodos((t) => !t)}
        >
          {todos ? 'Mostrar só os mais secos' : `Mostrar os outros ${n - PRIMEIROS} nós`}
        </button>
      )}
    </section>
  );
}
