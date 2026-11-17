import { QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router';
import { criarClienteQuery } from './clienteQuery';
import { ExigeSessao, SemSessao } from './sessao';
import { Entrar } from './telas/Entrar';
import { No } from './telas/No';
import { NovoNo } from './telas/NovoNo';
import { Talhoes } from './telas/Talhoes';

// Regras de sessao vencida, repeticao e foco: ver clienteQuery.ts.
const clienteQuery = criarClienteQuery();

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
                <Talhoes />
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
