import { MutationObserver, QueryObserver } from '@tanstack/react-query';
import type { QueryClient } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { ErroAPI, SEM_REDE } from './api';
import { criarClienteQuery } from './clienteQuery';
import { CHAVE_SESSAO } from './sessao';

/* O LOOP QUE ESTES TESTES PRENDEM.
 *
 * O 401 do proprio /me chegava ao onError global, que invalidava a sessao, que
 * refazia o /me, que voltava 401 -- sem fim, ~800 requisicoes por segundo. Sem
 * dado em cache, cada nova tentativa devolve a query a `pending`, e o porteiro
 * mostrava "Carregando..." para sempre. Com dado em cache (sessao que venceu
 * com a lista aberta), a query ficava com erro E dado velho ao mesmo tempo:
 * ExigeSessao mandava para /entrar pelo erro, SemSessao devolvia para / pelo
 * dado. Medido na @tanstack/query-core 5.102.8 antes da correcao. */

/** Deixa a fila de microtarefas e os timers de 1 ms das falsas requisicoes
 *  andarem. Com o loop, 30 ms bastam para dezenas de chamadas a /me. */
const assentar = () => new Promise((r) => setTimeout(r, 30));

const recusa = (status: number, caminho: string) =>
  new ErroAPI(status, caminho, `recusado com ${status}`);

let cliente: QueryClient;
let vivo: boolean;
let chamadasMe: number;
const desinscrever: Array<() => void> = [];

function observarSessao() {
  const obs = new QueryObserver(cliente, {
    queryKey: CHAVE_SESSAO,
    queryFn: async () => {
      chamadasMe++;
      await new Promise((r) => setTimeout(r, 1));
      if (vivo) return { usuario: { nome: 'demo' }, talhoes: [] };
      throw recusa(401, '/me');
    },
    staleTime: 5 * 60 * 1000, // o mesmo de useSessao
  });
  desinscrever.push(obs.subscribe(() => {}));
  return obs;
}

function observarLista(falha: () => ErroAPI | null) {
  const obs = new QueryObserver(cliente, {
    queryKey: ['devices'],
    queryFn: async () => {
      await new Promise((r) => setTimeout(r, 1));
      const erro = falha();
      if (erro) throw erro;
      return { count: 1, devices: ['no-1'] };
    },
    retry: false, // a repeticao de SEM_REDE nao e o que se mede aqui
  });
  desinscrever.push(obs.subscribe(() => {}));
  return obs;
}

beforeEach(() => {
  cliente = criarClienteQuery();
  vivo = false;
  chamadasMe = 0;
});

afterEach(() => {
  desinscrever.splice(0).forEach((fim) => fim());
  cliente.clear();
});

describe('sessao vencida no cliente de consultas', () => {
  it('sem sessao, o 401 do proprio /me nao refaz o /me', async () => {
    const sessao = observarSessao();
    await assentar();

    expect(chamadasMe).toBe(1);
    expect(sessao.getCurrentResult().status).toBe('error');
  });

  it('senha errada no login (401) nao toca no /me de quem nao tem sessao', async () => {
    observarSessao();
    await assentar();

    const login = new MutationObserver(cliente, {
      mutationFn: async () => {
        throw recusa(401, '/login');
      },
    });
    await login.mutate().catch(() => {});
    await assentar();

    expect(chamadasMe).toBe(1);
  });

  it('sessao que vence com a lista aberta volta sem dado, e a lista vai junto', async () => {
    vivo = true;
    const sessao = observarSessao();
    const lista = observarLista(() => (vivo ? null : recusa(401, '/devices')));
    await assentar();
    expect(sessao.getCurrentResult().data).toBeDefined();
    expect(lista.getCurrentResult().data).toBeDefined();

    vivo = false;
    await lista.refetch();
    await assentar();

    // Uma consulta a /me para confirmar que acabou, e nenhuma depois.
    expect(chamadasMe).toBe(2);
    const fim = sessao.getCurrentResult();
    expect(fim.status).toBe('error');
    // Sem dado velho: e ele que fazia SemSessao devolver para /.
    expect(fim.data).toBeUndefined();
    // Os talhoes de quem saiu nao ficam no cache para o proximo login.
    expect(cliente.getQueryData(['devices'])).toBeUndefined();
  });

  it.each([
    ['sem rede', SEM_REDE],
    ['erro do servidor', 500],
    ['no de outro usuario', 404],
  ])('%s na lista nao derruba a sessao', async (_nome, status) => {
    vivo = true;
    const sessao = observarSessao();
    await assentar();

    const lista = observarLista(() => recusa(status, '/devices'));
    await assentar();

    expect(lista.getCurrentResult().status).toBe('error');
    expect(chamadasMe).toBe(1);
    expect(sessao.getCurrentResult().data).toBeDefined();
  });
});
