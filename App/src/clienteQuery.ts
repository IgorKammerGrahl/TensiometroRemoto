import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query';
import type { Query } from '@tanstack/react-query';
import { ErroAPI, SEM_REDE } from './api';
import { CHAVE_SESSAO } from './sessao';

/** O cliente de consultas do aplicativo inteiro. Fabrica, e nao constante de
 *  modulo, para que cada teste tenha o seu (clienteQuery.test.ts). */
export function criarClienteQuery(): QueryClient {
  const cliente: QueryClient = new QueryClient({
    /* A SESSAO PODE MORRER COM O APLICATIVO ABERTO.
     *
     * O porteiro das rotas decide a partir de /me, que fica em cache. Se a
     * sessao expirar enquanto a lista esta na tela, e /devices voltar 401, o
     * porteiro nao fica sabendo: o usuario ganha uma faixa de erro generica e
     * nenhum caminho de volta ao login.
     *
     * Aqui em vez de em cada tela porque e uma regra do transporte, nao de uma
     * tela -- e porque a versao por tela seria copiada errada na quinta. O
     * reset refaz /me, que responde 401, e o porteiro redireciona. As
     * condicoes de parada estao em devolverAoLogin. */
    queryCache: new QueryCache({
      onError: (erro, query) => devolverAoLogin(cliente, erro, query),
    }),
    /* E a sessao tambem pode morrer entre abrir um formulario e salvar. O
     * mutationCache existe porque `queryCache.onError` NAO ve mutacao: sem esta
     * copia, um 401 ao salvar viraria uma mensagem de erro tecnica dentro do
     * formulario e o usuario ficaria tentando de novo numa sessao que acabou. */
    mutationCache: new MutationCache({
      onError: (erro) => devolverAoLogin(cliente, erro, null),
    }),
    defaultOptions: {
      queries: {
        /* SO REPETE O QUE VALE A PENA REPETIR.
         *
         * O padrao da biblioteca (tres tentativas para qualquer erro) esta errado
         * aqui de duas maneiras. Repetir um 401 nao vai fazer aparecer uma sessao
         * que nao existe -- so atrasa o redirecionamento para o login em alguns
         * segundos de tela parada. Repetir um 404 nao vai fazer aparecer um no que
         * nao e do usuario. Falha de rede, essa sim, e frequentemente transitoria:
         * o celular anda pela lavoura e o sinal vai e volta.
         *
         * Por isso a repeticao e restrita a SEM_REDE, e e a existencia desse
         * codigo separado -- em vez de um status HTTP inventado -- que torna a
         * condicao escrevivel. */
        retry: (tentativa, erro) =>
          erro instanceof ErroAPI && erro.status === SEM_REDE && tentativa < 2,
        retryDelay: (tentativa) => 1000 * 2 ** tentativa,
        /* Telemetria envelhece. Voltar para o aplicativo depois de deixa-lo aberto
         * no bolso deve trazer dado de agora, nao o dado de quando a tela abriu.
         * (E quando nao trouxer, `medido_ha_s` conta a verdade -- ver tempo.ts.) */
        refetchOnWindowFocus: true,
      },
    },
  });
  return cliente;
}

/** Sessao vencida volta para o login, venha o 401 de uma leitura ou de uma
 *  escrita. Uma funcao so para as duas porque a regra e do transporte: ver a
 *  condicao escrita duas vezes seria o comeco de ela divergir em uma delas.
 *
 *  AS DUAS PRIMEIRAS SAIDAS SAO O QUE IMPEDE O LOOP, e a ordem importa.
 *
 *  1. O erro e do proprio /me: nada a fazer, o porteiro (ExigeSessao) ja le
 *     esse erro e redireciona. Refazer o /me aqui foi o loop sem fim: 401,
 *     refaz, 401, refaz -- e, sem dado em cache, cada tentativa devolve a
 *     query a `pending`, "Carregando..." para sempre.
 *  2. Nao ha sessao em cache: nao ha o que derrubar. E o 401 de senha errada
 *     no login. Refazer o /me ali tirava o formulario da tela (o porteiro
 *     passa a `pending`) e o trazia de volta vazio, sem a mensagem de erro.
 *  3. Havia sessao e ela acabou: reset, nao invalidate. O invalidate guarda o
 *     dado velho junto do erro novo, e com os dois ao mesmo tempo ExigeSessao
 *     manda para /entrar e SemSessao devolve para /, sem fim. O reset e de
 *     TODAS as consultas pelo mesmo motivo do clear() do logout: os talhoes
 *     de quem saiu nao ficam no cache para o proximo login no aparelho. As
 *     consultas ainda na tela refazem e voltam 401, e param na saida 2.
 *
 *  Ver clienteQuery.test.ts. */
function devolverAoLogin(
  cliente: QueryClient,
  erro: unknown,
  query: Query<unknown, unknown, unknown, readonly unknown[]> | null,
) {
  if (!(erro instanceof ErroAPI && (erro.status === 401 || erro.status === 403))) return;

  const sessao = cliente.getQueryCache().find({ queryKey: CHAVE_SESSAO, exact: true });
  if (query !== null && query === sessao) return; // 1
  if (sessao?.state.data === undefined) return; // 2

  void cliente.resetQueries(); // 3
}
