import { QueryClientProvider } from '@tanstack/react-query';
import { useEffect } from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router';
import { criarClienteQuery } from './clienteQuery';
import { ExigeSessao, SemSessao } from './sessao';
import { Entrar } from './telas/Entrar';
import { No } from './telas/No';
import { NovoNo } from './telas/NovoNo';
import { TelaTalhao } from './telas/Talhao';
import { Talhoes } from './telas/Talhoes';

// Regras de sessao vencida, repeticao e foco: ver clienteQuery.ts.
const clienteQuery = criarClienteQuery();

/** Cada tela abre no topo. O BrowserRouter mantem a rolagem da tela anterior:
 *  tocar num cartao no fim da lista abria o talhao ja rolado ate o fim, com a
 *  resposta -- o motivo de abrir a tela -- fora de vista. So na troca de
 *  caminho: o voltar do navegador continua devolvendo a posicao que ele
 *  guardou. */
function RolarAoTopo() {
  const { pathname } = useLocation();
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [pathname]);
  return null;
}

export function App() {
  return (
    <QueryClientProvider client={clienteQuery}>
      <BrowserRouter>
        <RolarAoTopo />
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
                <Talhoes />
              </ExigeSessao>
            }
          />
          <Route
            path="/talhao/:id"
            element={
              <ExigeSessao>
                <TelaTalhao />
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
