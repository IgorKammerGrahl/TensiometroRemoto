import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect, useId, useState } from 'react';
import { Link, useNavigate } from 'react-router';
import { api } from '../api';
import { explicarFalhaDeNo } from '../erros';
import { useSessao } from '../sessao';

// Cadastro de no (RF10).
//
// ESTA TELA E SOBRE A ORDEM DAS OPERACOES, NAO SOBRE O FORMULARIO.
//
// O 201 traz o token em claro. E a unica vez que ele existe em algum lugar
// legivel: `devices.token_hash` e irreversivel e nao ha endpoint que recupere
// um token existente. Se a tela deixar esse valor escapar antes de a pessoa
// guarda-lo, o no cadastrado nao autentica -- e como o `id` ja esta ocupado,
// nem repetir o cadastro resolve sem antes desfazer o anterior.
//
// Dai as tres decisoes deste arquivo, todas sobre o que a tela NAO faz:
//
//   1. `onSuccess` NAO NAVEGA. O caminho a frente e um botao que a pessoa
//      aperta depois de guardar o token. Redirecionar para o no recem-criado
//      seria a coisa mais natural do mundo e apagaria o token no caminho.
//   2. Enquanto o token esta na tela, NAO HA LINK DE VOLTA. Nao e um link
//      escondido: ele nao e renderizado, entao nao existe para o teclado nem
//      para o leitor de tela. O unico jeito de sair da tela e o botao que diz
//      que o token foi guardado.
//   3. Recarregar a pagina passa pelo aviso nativo do navegador
//      (`beforeunload`), que so fica armado enquanto o token esta visivel.
//
// O token mora em `criar.data` e em nenhum outro lugar. Nao ha copia em
// estado, nem em armazenamento do navegador -- guardar credencial em
// `localStorage` trocaria uma perda possivel por um vazamento permanente.
// Consequencia pratica para quem for mexer aqui: um `criar.reset()`, uma
// `key` que remonte esta tela ou um `queryClient.clear()` no meio do caminho
// destroem o token. Nao ha como recupera-lo depois.

export function NovoNo() {
  const sessao = useSessao();
  const cliente = useQueryClient();
  const navegar = useNavigate();
  const idNo = useId();
  const idDescricao = useId();
  const idTalhao = useId();
  const idToken = useId();

  const [form, setForm] = useState({ id: '', descricao: '', talhao: '' });
  const [copia, setCopia] = useState<'ocioso' | 'copiado' | 'falhou'>('ocioso');

  const criar = useMutation({
    mutationFn: (corpo: { id: string; descricao: string; talhao_id: string }) =>
      api.criarDevice(corpo),
    onSuccess: () => {
      // Invalidar a lista e seguro: refazer uma consulta nao desmonta esta
      // tela nem toca no resultado da mutacao. Navegar seria o contrario.
      void cliente.invalidateQueries({ queryKey: ['devices'] });
    },
  });

  const criado = criar.data ?? null;

  /* Fechar ou recarregar com o token na tela e a perda que nenhum cuidado de
   * dentro do aplicativo evita -- entao o aviso e o nativo do navegador. Fica
   * armado so enquanto o token esta visivel: um `beforeunload` permanente
   * treinaria a pessoa a dispensar o aviso sem ler, e ele existe justamente
   * para o unico momento em que ha algo a perder. */
  useEffect(() => {
    if (!criado) return;
    const avisar = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener('beforeunload', avisar);
    return () => window.removeEventListener('beforeunload', avisar);
  }, [criado]);

  const talhoes = sessao.data?.talhoes ?? [];

  // A area de transferencia exige contexto seguro: em HTTP de rede local --
  // que e como esta PWA e aberta no celular em campo -- `navigator.clipboard`
  // simplesmente nao existe. O botao entao nao e renderizado, em vez de
  // aparecer e nao funcionar. O token continua selecionavel do mesmo jeito,
  // que e o caminho que sempre existe.
  const podeCopiar = typeof navigator.clipboard?.writeText === 'function';

  if (criado) {
    return (
      <div className="pagina">
        <div className="pilha conteudo">
          <h1>Nó cadastrado</h1>

          <section className="token" aria-labelledby={idToken}>
            <h2 id={idToken} className="token__titulo">
              Esta é a única vez que este token aparece
            </h2>
            <p>
              Copie e guarde agora. Ele não fica salvo em nenhum lugar que possa mostrá-lo de
              novo — nem nesta tela, nem no servidor. Se ele se perder, o jeito de recuperar o
              nó é cadastrar outro.
            </p>

            <label className="token__rotulo" htmlFor={`${idToken}-valor`}>
              Token de {criado.device.descricao || criado.device.id}
            </label>
            {/* Input em vez de texto: seleciona com um toque no celular,
                aceita Ctrl+C no teclado e nao quebra a linha no meio do
                valor. Somente leitura porque editar aqui nao mudaria o que
                o servidor gravou -- so estragaria a copia. */}
            <input
              id={`${idToken}-valor`}
              className="numero token__valor"
              type="text"
              readOnly
              value={criado.token}
              spellCheck={false}
              autoComplete="off"
              onFocus={(e) => e.currentTarget.select()}
            />

            {podeCopiar && (
              <button
                type="button"
                className="botao botao--contorno"
                onClick={() => {
                  void navigator.clipboard.writeText(criado.token).then(
                    () => setCopia('copiado'),
                    // A recusa e dita. Um catch silencioso deixaria a pessoa
                    // seguir achando que copiou -- e ela so descobriria que
                    // nao com o no ja instalado no campo.
                    () => setCopia('falhou'),
                  );
                }}
              >
                Copiar o token
              </button>
            )}

            {/* aria-live porque a confirmacao da copia e a unica resposta a
                um clique que nao muda nada visivel no resto da tela. */}
            <p className="token__aviso" aria-live="polite">
              {copia === 'copiado' && 'Token copiado.'}
              {copia === 'falhou' &&
                'Não foi possível copiar automaticamente. Selecione o token acima e copie à mão.'}
            </p>

            <p className="secundario">{criado.aviso}</p>
          </section>

          {/* O unico caminho para fora. Substitui a entrada no historico em
              vez de empilhar: voltar para ca depois nao traria o token de
              volta, so um formulario vazio com cara de que algo se perdeu. */}
          <button
            type="button"
            className="botao botao--largo"
            onClick={() =>
              navegar(`/no/${encodeURIComponent(criado.device.id)}`, { replace: true })
            }
          >
            Já guardei o token
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="pagina">
      <header className="cabecalho cabecalho--detalhe">
        <Link to="/" className="voltar">
          ← Nós
        </Link>
      </header>

      <div className="pilha conteudo">
        <h1>Cadastrar nó</h1>

        {talhoes.length === 0 ? (
          <p className="secundario">
            Você ainda não tem nenhum talhão. Todo nó nasce dentro de um talhão — sem isso ele
            não aparece para ninguém, nem para quem o cadastrou. Procure quem administra o
            sistema.
          </p>
        ) : (
          <form
            className="painel"
            onSubmit={(e) => {
              e.preventDefault();
              criar.mutate({
                id: form.id.trim(),
                descricao: form.descricao.trim(),
                talhao_id: form.talhao,
              });
            }}
          >
            <div className="campo">
              <label htmlFor={idNo}>Identificador do nó</label>
              {/* maxLength e o limite do servidor (64), nao um palpite: o
                  campo simplesmente nao deixa passar do que ele aceita. O
                  formato em si nao e restringido aqui porque o servidor
                  tambem nao restringe -- inventar um `pattern` seria criar
                  uma regra que so existe nesta tela. */}
              <input
                id={idNo}
                type="text"
                required
                maxLength={64}
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
                value={form.id}
                onChange={(e) => setForm((f) => ({ ...f, id: e.target.value }))}
              />
              <span className="dica">
                O mesmo nome gravado no aparelho. Não pode ser mudado depois.
              </span>
            </div>

            <div className="campo">
              <label htmlFor={idDescricao}>Descrição</label>
              <input
                id={idDescricao}
                type="text"
                required
                value={form.descricao}
                onChange={(e) => setForm((f) => ({ ...f, descricao: e.target.value }))}
              />
              <span className="dica">
                É o que aparece na lista. Descreva onde ele está, para reconhecer de relance.
              </span>
            </div>

            <div className="campo">
              <label htmlFor={idTalhao}>Talhão</label>
              {/* Sem opcao pre-selecionada: o talhao decide quem enxerga o no
                  e quais limiares o classificam. Escolher por conta propria e
                  decidir isso no lugar de quem cadastra. A opcao vazia com
                  `required` faz o navegador cobrar a escolha. */}
              <select
                id={idTalhao}
                required
                value={form.talhao}
                onChange={(e) => setForm((f) => ({ ...f, talhao: e.target.value }))}
              >
                <option value="">Escolha o talhão…</option>
                {talhoes.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.nome}
                    {t.cultura ? ` · ${t.cultura}` : ''}
                  </option>
                ))}
              </select>
              <span className="dica">
                Só aparecem os talhões que são seus. O nó fica visível para quem tem acesso a
                esse talhão.
              </span>
            </div>

            {criar.isError && (
              <p className="painel__erro" role="alert">
                <strong>O nó não foi cadastrado.</strong>{' '}
                {explicarFalhaDeNo(criar.error, 'cadastro')}
              </p>
            )}

            <div className="painel__acoes">
              <button type="submit" className="botao" disabled={criar.isPending}>
                {criar.isPending ? 'Cadastrando…' : 'Cadastrar'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
