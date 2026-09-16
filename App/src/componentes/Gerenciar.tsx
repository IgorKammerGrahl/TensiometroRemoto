import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useId, useState } from 'react';
import { api, type DeviceApp } from '../api';
import { explicarFalhaDeNo } from '../erros';
import { useSessao } from '../sessao';

// Edicao, mudanca de talhao e desativacao do no (RF10).
//
// SAO DOIS CONTROLES SEPARADOS PORQUE SAO DOIS RISCOS SEPARADOS.
//
// Trocar a descricao e reversivel e sem consequencia; desativar faz o servidor
// RECUSAR as leituras do no, e o que ele medir nesse periodo nao entra no
// historico depois. O estado volta com um clique, o dado nao volta nunca.
// Por isso a desativacao tem confirmacao propria e a edicao nao: confirmacao
// em tudo vira clique automatico, e ai ela nao protege mais o que importa.
//
// O que a confirmacao mostra e a CONSEQUENCIA, nao a pergunta. "Tem certeza?"
// nao informa nada a quem ja clicou -- quem clica em desativar tem certeza de
// que quer desativar; o que a pessoa pode nao saber e que o historico fica com
// um buraco do tamanho do periodo desligado.
//
// A mudanca de talhao exige DUAS concessoes no servidor e o 404 nao diz qual
// faltou. Ver erros.ts: a mensagem nomeia as possibilidades e nao escolhe.

export function Gerenciar({ dev }: { dev: DeviceApp }) {
  const cliente = useQueryClient();
  const sessao = useSessao();
  const idDescricao = useId();
  const idTalhao = useId();

  const [aberto, setAberto] = useState(false);
  const [confirmando, setConfirmando] = useState(false);
  const [form, setForm] = useState({ descricao: dev.descricao, talhao: dev.talhao.id });

  /* O talhao decide os limiares, e os limiares decidem a zona de cada ponto da
   * serie -- inclusive do resumo em texto do grafico, que e o que um leitor de
   * tela anuncia. Deixar a serie em cache depois de mover o no manteria na
   * tela uma classificacao feita com os limiares do talhao antigo.
   *
   * Mesma armadilha descrita em Faixa.tsx, pela mesma razao: o desenho se
   * corrige sozinho e o texto nao, entao o defeito nao aparece para quem
   * enxerga a tela. As tres chaves andam juntas de proposito. */
  const invalidar = () => {
    void cliente.invalidateQueries({ queryKey: ['device'] });
    void cliente.invalidateQueries({ queryKey: ['devices'] });
    void cliente.invalidateQueries({ queryKey: ['serie'] });
  };

  const salvar = useMutation({
    mutationFn: (corpo: { descricao?: string; talhao_id?: string }) =>
      api.patchDevice(dev.id, corpo),
    onSuccess: () => {
      invalidar();
      setAberto(false);
    },
  });

  const ativacao = useMutation({
    mutationFn: (ativo: boolean) => api.patchDevice(dev.id, { ativo }),
    onSuccess: () => {
      invalidar();
      setConfirmando(false);
    },
  });

  const talhoes = sessao.data?.talhoes ?? [];

  if (!aberto) {
    return (
      <div className="painel painel--fechado">
        <div className="painel__acoes">
          <button type="button" className="botao botao--contorno" onClick={() => setAberto(true)}>
            Editar este nó
          </button>
        </div>

        <Ativacao
          dev={dev}
          confirmando={confirmando}
          setConfirmando={setConfirmando}
          ativacao={ativacao}
        />
      </div>
    );
  }

  return (
    <div className="painel">
      <form
        className="painel__forma"
        onSubmit={(e) => {
          e.preventDefault();
          /* So o que mudou entra no corpo. Campo ausente e campo preservado
           * no servidor, entao mandar o talhao numa edicao de descricao nao
           * seria inofensivo: passaria a exigir a concessao de destino para
           * uma operacao que nao tem destino, e um 404 por acesso a talhao
           * apareceria como "a descricao nao foi salva". */
          const corpo: { descricao?: string; talhao_id?: string } = {};
          const descricao = form.descricao.trim();
          if (descricao !== dev.descricao) corpo.descricao = descricao;
          if (form.talhao !== dev.talhao.id) corpo.talhao_id = form.talhao;
          if (Object.keys(corpo).length === 0) {
            // Nada mudou. Um PATCH vazio voltaria 200 sem fazer nada, e
            // "salvo" sobre coisa nenhuma e a afirmacao falsa que este
            // projeto ja removeu uma vez (ver Faixa.tsx).
            setAberto(false);
            return;
          }
          salvar.mutate(corpo);
        }}
      >
        <h2 className="painel__titulo">Dados do nó</h2>

        <div className="campo">
          <label htmlFor={idDescricao}>Descrição</label>
          <input
            id={idDescricao}
            type="text"
            required
            value={form.descricao}
            onChange={(e) => setForm((f) => ({ ...f, descricao: e.target.value }))}
          />
          <span className="dica">É o que aparece na lista de nós.</span>
        </div>

        <div className="campo">
          <label htmlFor={idTalhao}>Talhão</label>
          <select
            id={idTalhao}
            required
            value={form.talhao}
            onChange={(e) => setForm((f) => ({ ...f, talhao: e.target.value }))}
          >
            {talhoes.map((t) => (
              <option key={t.id} value={t.id}>
                {t.nome}
                {t.cultura ? ` · ${t.cultura}` : ''}
              </option>
            ))}
          </select>
          <span className="dica">
            Mudar o talhão muda quem enxerga este nó e os limites que classificam as leituras
            dele.
          </span>
        </div>

        {salvar.isError && (
          <p className="painel__erro" role="alert">
            <strong>As mudanças não foram salvas.</strong>{' '}
            {explicarFalhaDeNo(salvar.error, 'edicao')}
          </p>
        )}

        <div className="painel__acoes">
          <button type="submit" className="botao" disabled={salvar.isPending}>
            {salvar.isPending ? 'Salvando…' : 'Salvar'}
          </button>
          <button
            type="button"
            className="botao botao--contorno"
            disabled={salvar.isPending}
            onClick={() => {
              // Cancelar devolve os campos ao que o servidor tem. Sem isso o
              // rascunho abandonado reapareceria na proxima abertura como se
              // fosse o estado atual do no.
              setForm({ descricao: dev.descricao, talhao: dev.talhao.id });
              setAberto(false);
            }}
          >
            Cancelar
          </button>
        </div>
      </form>

      <Ativacao
        dev={dev}
        confirmando={confirmando}
        setConfirmando={setConfirmando}
        ativacao={ativacao}
      />
    </div>
  );
}

// ------------------------------------------------------------------ ativacao

type Ativacao = {
  isPending: boolean;
  isError: boolean;
  error: unknown;
  mutate: (ativo: boolean) => void;
};

function Ativacao({
  dev,
  confirmando,
  setConfirmando,
  ativacao,
}: {
  dev: DeviceApp;
  confirmando: boolean;
  setConfirmando: (v: boolean) => void;
  ativacao: Ativacao;
}) {
  return (
    <div className="painel__ativacao">
      {dev.ativo ? (
        confirmando ? (
          /* A confirmacao e um bloco de texto com dois botoes, nao uma janela
           * modal. Modal exigiria prender o foco e devolver ao lugar certo no
           * fechamento -- codigo que, feito pela metade, deixa quem usa
           * teclado ou leitor de tela preso atras de um dialogo. O bloco
           * inline nao tem esse problema porque nunca tira o foco de onde ele
           * esta. */
          <div className="painel__confirma" role="group" aria-label="Confirmar desativação">
            <p>
              <strong>Desativar {dev.descricao || dev.id}?</strong> O servidor passa a recusar as
              leituras deste nó. O que ele medir enquanto estiver desativado não entra no
              histórico, nem depois que ele voltar.
            </p>
            <div className="painel__acoes">
              <button
                type="button"
                className="botao"
                disabled={ativacao.isPending}
                onClick={() => ativacao.mutate(false)}
              >
                {ativacao.isPending ? 'Desativando…' : 'Desativar mesmo assim'}
              </button>
              <button
                type="button"
                className="botao botao--contorno"
                disabled={ativacao.isPending}
                onClick={() => setConfirmando(false)}
              >
                Manter ativo
              </button>
            </div>
          </div>
        ) : (
          <div className="painel__acoes">
            <button
              type="button"
              className="botao botao--contorno"
              onClick={() => setConfirmando(true)}
            >
              Desativar este nó
            </button>
          </div>
        )
      ) : (
        /* Reativar nao pergunta nada: nao ha o que perder, e o estado atual
         * ja e o que causa perda. */
        <div className="painel__acoes">
          <button
            type="button"
            className="botao"
            disabled={ativacao.isPending}
            onClick={() => ativacao.mutate(true)}
          >
            {ativacao.isPending ? 'Reativando…' : 'Reativar este nó'}
          </button>
        </div>
      )}

      {ativacao.isError && (
        <p className="painel__erro" role="alert">
          <strong>O nó continua como estava.</strong>{' '}
          {explicarFalhaDeNo(ativacao.error, 'edicao')}
        </p>
      )}
    </div>
  );
}
