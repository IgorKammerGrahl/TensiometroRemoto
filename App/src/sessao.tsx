import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router';
import { ErroAPI, SEM_REDE, api } from './api';
import type { RespostaMe } from './api';

export const CHAVE_SESSAO = ['sessao'] as const;

/** A sessao inteira: quem esta logado e a quais talhoes tem acesso.
 *
 * Nao ha token guardado em lugar nenhum do cliente -- a autenticacao vive no
 * cookie HttpOnly, que o JavaScript nao le por definicao. Entao "estou logado?"
 * nao e uma variavel local: e uma pergunta ao servidor, e a resposta e esta
 * query. Nada de localStorage com um booleano `logado`, que dessincroniza da
 * verdade no instante em que a sessao expira do outro lado. */
export function useSessao() {
  return useQuery({
    queryKey: CHAVE_SESSAO,
    queryFn: ({ signal }) => api.me(signal),
    // A sessao desliza 30 dias a cada requisicao no servidor; nao ha por que
    // reconsultar a cada montagem de componente.
    staleTime: 5 * 60 * 1000,
  });
}

export function useLogin() {
  const clienteQuery = useQueryClient();
  return useMutation({
    mutationFn: ({ email, senha }: { email: string; senha: string }) =>
      api.login(email, senha),
    onSuccess: async () => {
      // O login devolve 204 sem corpo: quem e o usuario so se sabe perguntando.
      await clienteQuery.invalidateQueries({ queryKey: CHAVE_SESSAO });
    },
  });
}

export function useLogout() {
  const clienteQuery = useQueryClient();
  return useMutation({
    mutationFn: () => api.logout(),
    // onSettled, nao onSuccess: se o logout falhar por rede, o cookie pode ou
    // nao ter sido derrubado no servidor. Limpar o cache dos dois jeitos evita
    // o pior caso, que e a tela seguir mostrando os talhoes de alguem depois de
    // um "Sair" que a pessoa acredita ter funcionado.
    onSettled: () => clienteQuery.clear(),
  });
}

/** Porteiro das rotas autenticadas.
 *
 * OS TRES DESFECHOS DE FALHA SAO TRATADOS SEPARADAMENTE, e essa e a razao de o
 * componente existir:
 *
 *  401 -> nao ha sessao. Vai para o login, guardando de onde veio.
 *  403 -> ha sessao, a conta esta desativada. NAO vai para o login: mandar essa
 *         pessoa digitar a senha de novo e faze-la repetir para sempre uma
 *         credencial correta contra uma conta que nao vai abrir. A saida e
 *         humana, nao tecnica -- falar com quem administra.
 *  sem rede -> nao se sabe nada sobre a sessao. Nao derruba nada; oferece
 *         tentar de novo. Tratar queda de sinal como logout seria expulsar o
 *         usuario do aplicativo toda vez que ele entrasse num galpao. */
export function ExigeSessao({ children }: { children: ReactNode }) {
  const local = useLocation();
  const sessao = useSessao();

  if (sessao.isPending) return <Carregando />;

  if (sessao.error) {
    const erro = sessao.error;
    const status = erro instanceof ErroAPI ? erro.status : -1;

    if (status === 401) {
      return <Navigate to="/entrar" replace state={{ de: local.pathname }} />;
    }
    if (status === 403) return <ContaDesativada />;
    if (status === SEM_REDE) {
      return <SemConexao aoTentar={() => void sessao.refetch()} />;
    }
    return <FalhaInesperada mensagem={erro.message} aoTentar={() => void sessao.refetch()} />;
  }

  return <>{children}</>;
}

/** O oposto do porteiro: quem ja tem sessao nao ve a tela de login. */
export function SemSessao({ children }: { children: ReactNode }) {
  const sessao = useSessao();
  if (sessao.isPending) return <Carregando />;
  if (sessao.data) return <Navigate to="/" replace />;
  return <>{children}</>;
}

export type { RespostaMe };

// ------------------------------------------------------------------ telas de borda

function Carregando() {
  return (
    <main className="tela-borda">
      <p aria-live="polite">Carregando…</p>
    </main>
  );
}

function ContaDesativada() {
  return (
    <main className="tela-borda">
      <h1>Conta desativada</h1>
      <p>
        O acesso desta conta foi desligado. Procure quem administra o sistema para
        reativá-la.
      </p>
      <p className="secundario">Digitar a senha de novo não resolve.</p>
    </main>
  );
}

function SemConexao({ aoTentar }: { aoTentar: () => void }) {
  return (
    <main className="tela-borda">
      <h1>Sem conexão</h1>
      <p>Não foi possível falar com o servidor. Verifique a rede do aparelho.</p>
      <button type="button" className="botao" onClick={aoTentar}>
        Tentar de novo
      </button>
    </main>
  );
}

function FalhaInesperada({
  mensagem,
  aoTentar,
}: {
  mensagem: string;
  aoTentar: () => void;
}) {
  return (
    <main className="tela-borda">
      <h1>Algo deu errado</h1>
      <p className="secundario">{mensagem}</p>
      <button type="button" className="botao" onClick={aoTentar}>
        Tentar de novo
      </button>
    </main>
  );
}
