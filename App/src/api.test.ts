import { afterEach, describe, expect, it, vi } from 'vitest';
import { ErroAPI, SEM_REDE, api } from './api';

type Chamada = { url: string; init: RequestInit };

/** Instala um fetch falso e devolve o registro das chamadas. */
function stubFetch(responder: () => Promise<Response> | Response) {
  const chamadas: Chamada[] = [];
  vi.stubGlobal('fetch', (url: string, init: RequestInit) => {
    chamadas.push({ url, init });
    return Promise.resolve(responder());
  });
  return chamadas;
}

const json = (corpo: unknown, status = 200) =>
  new Response(JSON.stringify(corpo), {
    status,
    headers: { 'content-type': 'application/json' },
  });

afterEach(() => vi.unstubAllGlobals());

describe('requisitar', () => {
  it('monta a URL sob /api/v1/app e manda o cookie de mesma origem', async () => {
    const chamadas = stubFetch(() => json({ usuario: {}, talhoes: [] }));
    await api.me();

    expect(chamadas[0]?.url).toBe('/api/v1/app/me');
    // O cookie HttpOnly e a autenticacao inteira. Se credentials sair daqui,
    // toda requisicao volta 401 e nada na tela explica por que.
    expect(chamadas[0]?.init.credentials).toBe('same-origin');
  });

  it('envia login como POST com corpo JSON', async () => {
    const chamadas = stubFetch(() => new Response(null, { status: 204 }));
    await api.login('produtor@exemplo.br', 'segredo');

    expect(chamadas[0]?.init.method).toBe('POST');
    expect(chamadas[0]?.init.body).toBe(
      JSON.stringify({ email: 'produtor@exemplo.br', senha: 'segredo' }),
    );
  });

  it('aceita 204 sem corpo sem tentar desserializar', async () => {
    stubFetch(() => new Response(null, { status: 204 }));
    await expect(api.logout()).resolves.toBeUndefined();
  });

  it('devolve o corpo desserializado em 200', async () => {
    stubFetch(() =>
      json({ usuario: { id: 'u1', email: 'a@b.c', nome: 'Ana' }, talhoes: [] }),
    );
    await expect(api.me()).resolves.toEqual({
      usuario: { id: 'u1', email: 'a@b.c', nome: 'Ana' },
      talhoes: [],
    });
  });

  // 401 e 403 sao respostas diferentes e a interface trata as duas de formas
  // diferentes: 401 volta ao login, 403 nao -- mandar uma conta desativada
  // para o login faz o usuario redigitar a senha para sempre contra uma conta
  // que nunca vai abrir.
  it('preserva o status para o chamador distinguir 401 de 403', async () => {
    stubFetch(() => json({ error: 'credenciais invalidas' }, 401));
    const erro401 = await api.login('a@b.c', 'x').catch((e: unknown) => e);
    expect(erro401).toBeInstanceOf(ErroAPI);
    expect((erro401 as ErroAPI).status).toBe(401);
    expect((erro401 as ErroAPI).message).toBe('credenciais invalidas');

    vi.unstubAllGlobals();
    stubFetch(() => json({ error: 'usuario inativo' }, 403));
    const erro403 = await api.login('a@b.c', 'x').catch((e: unknown) => e);
    expect((erro403 as ErroAPI).status).toBe(403);
  });

  it('cai no statusText quando o corpo do erro nao traz {error}', async () => {
    stubFetch(() => new Response('', { status: 500, statusText: 'Server Error' }));
    const erro = await api.me().catch((e: unknown) => e);
    expect((erro as ErroAPI).message).toBe('Server Error');
  });

  // Falha de rede nao e resposta do servidor. SEM_REDE separa as duas para
  // que "sem conexao" nao derrube a sessao como se fosse 401.
  it('marca falha de rede com status SEM_REDE, nao com um HTTP inventado', async () => {
    vi.stubGlobal('fetch', () => Promise.reject(new TypeError('failed to fetch')));
    const erro = await api.me().catch((e: unknown) => e);
    expect(erro).toBeInstanceOf(ErroAPI);
    expect((erro as ErroAPI).status).toBe(SEM_REDE);
    expect(SEM_REDE).not.toBe(401);
  });

  it('propaga o abort sem transformar em erro de rede', async () => {
    const ctrl = new AbortController();
    vi.stubGlobal('fetch', () =>
      Promise.reject(new DOMException('aborted', 'AbortError')),
    );
    ctrl.abort();
    const erro = await api.me(ctrl.signal).catch((e: unknown) => e);
    expect(erro).toBeInstanceOf(DOMException);
    expect((erro as DOMException).name).toBe('AbortError');
  });

  // O modo de falha da etapa 6: rota desconhecida sob /api/ caindo no
  // fallback de SPA e devolvendo index.html com 200. Sem esta checagem o
  // sintoma seria um erro de sintaxe de JSON sem relacao aparente com a
  // causa.
  it('recusa 200 que nao e JSON em vez de estourar no parse', async () => {
    stubFetch(
      () =>
        new Response('<!doctype html><title>app</title>', {
          status: 200,
          headers: { 'content-type': 'text/html' },
        }),
    );
    const erro = await api.me().catch((e: unknown) => e);
    expect(erro).toBeInstanceOf(ErroAPI);
    expect((erro as ErroAPI).message).toContain('nao e JSON');
    expect((erro as ErroAPI).message).toContain('text/html');
  });
});
