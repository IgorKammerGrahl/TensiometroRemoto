import {
  MutationCache,
  QueryCache,
  QueryClient,
  QueryClientProvider,
} from '@tanstack/react-query';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router';
import { ErroAPI, SEM_REDE } from './api';
import { CHAVE_SESSAO, ExigeSessao, SemSessao } from './sessao';
import { Entrar } from './telas/Entrar';
import { No } from './telas/No';
import { NovoNo } from './telas/NovoNo';
import { Nos } from './telas/Nos';

const clienteQuery = new QueryClient({
  /* A SESSAO PODE MORRER COM O APLICATIVO ABERTO.
   *
   * O porteiro das rotas decide a partir de /me, que fica em cache. Se a
   * sessao expirar enquanto a lista esta na tela, e /devices voltar 401, o
   * porteiro nao fica sabendo: o usuario ganha uma faixa de erro generica e
   * nenhum caminho de volta ao login.
   *
   * Aqui em vez de em cada tela porque e uma regra do transporte, nao de uma
   * tela -- e porque a versao por tela seria copiada errada na quinta. A
   * invalidacao refaz /me, que responde 401, e o porteiro redireciona. */
  queryCache: new QueryCache({
    onError: (erro) => devolverAoLogin(erro),
  }),
  /* E a sessao tambem pode morrer entre abrir um formulario e salvar. O
   * mutationCache existe porque `queryCache.onError` NAO ve mutacao: sem esta
   * copia, um 401 ao salvar viraria uma mensagem de erro tecnica dentro do
   * formulario e o usuario ficaria tentando de novo numa sessao que acabou. */
  mutationCache: new MutationCache({
    onError: (erro) => devolverAoLogin(erro),
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

/** Sessao vencida volta para o login, venha o 401 de uma leitura ou de uma
 *  escrita. Uma funcao so para as duas porque a regra e do transporte: ver a
 *  condicao escrita duas vezes seria o comeco de ela divergir em uma delas. */
function devolverAoLogin(erro: unknown) {
  if (erro instanceof ErroAPI && (erro.status === 401 || erro.status === 403)) {
    clienteQuery.invalidateQueries({ queryKey: CHAVE_SESSAO });
  }
}

export function App() {
  return (
    <QueryClientProvider client={clienteQuery}>
      <BrowserRouter>
        <Routes>
          <Route
            path="/entrar"
            element={
              <SemSessao>
                <Entrar />
              </SemSessao>
            }
          />
          <Route
            path="/"
            element={
              <ExigeSessao>
                <Nos />
              </ExigeSessao>
            }
          />
          {/* Antes de /no/:id na leitura, mas nao ha ambiguidade: os
              prefixos sao diferentes (/nos/ e /no/). */}
          <Route
            path="/nos/novo"
            element={
              <ExigeSessao>
                <NovoNo />
              </ExigeSessao>
            }
          />
          <Route
            path="/no/:id"
            element={
              <ExigeSessao>
                <No />
              </ExigeSessao>
            }
          />
          {/* Rota desconhecida volta para a raiz. O servidor ja devolve
              index.html para qualquer caminho fora de /api (etapa 6), entao
              quem chega aqui digitou errado ou seguiu um link velho. */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
