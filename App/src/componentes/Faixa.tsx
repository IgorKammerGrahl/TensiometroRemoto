import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useId, useState } from 'react';
import { api, ErroAPI, SEM_REDE, type Talhao } from '../api';
import { FUNDO_KPA, textoKpa, TOPO_KPA } from '../escala';
import { aparencia } from '../zona';

// Configuracao dos limiares do talhao (RF07/UC04).
//
// A INVERSAO NAO E DETECTADA AQUI. ELA E INEXPRIMIVEL.
//
// Ha duas coisas distintas e so uma delas e deste lado:
//   VALIDACAO -- decidir se o par e aceitavel. E do servidor (validarFaixa) e
//     do CHECK talhoes_faixas_ck. A mensagem de erro dele e a autoridade e
//     aparece na tela como veio.
//   AFFORDANCE -- impedir que o formulario CONSTRUA um par invalido. E daqui,
//     e nao duplica regra nenhuma.
//
// A affordance sai do `min`/`max` nativo do input numerico, cruzados: o teto do
// campo de "seco" e o valor corrente de "secando", e o piso de "secando" e o
// valor corrente de "seco". Nao existe `estresse < alerta` escrito em lugar
// nenhum deste arquivo -- existe um intervalo que o navegador nao deixa sair.
// Se um dia aparecer um `<` entre os dois aqui, a regra foi duplicada e a
// duplicata vai divergir do CHECK sem avisar.
//
// De brinde vem a acessibilidade: `min`/`max` viram aria-valuemin/max no leitor
// de tela e o passo do teclado respeita os limites, sem nada escrito.
//
// O QUE O NAVEGADOR NAO RESOLVE, E POR ISSO O ERRO DO SERVIDOR CONTINUA TRATADO:
//   - igualdade exata. `min`/`max` sao inclusivos; o CHECK exige estresse
//     ESTRITAMENTE menor. Derivar `max={alerta - passo}` seria escrever a
//     regra da estritude aqui, entao a igualdade fica para o servidor recusar.
//   - o campo pode chegar por outro caminho que nao este formulario.
//   - a faixa de -80 a 0 e a exigencia de vir o par completo, que o `required`
//     e o `min`/`max` de escala cobrem no caminho feliz e nao garantem.

/** Ambos preenchidos, e nunca um so. O estado "um preenchido" nao existe no
 *  formulario porque os dois campos sao `required` -- sem isso a tela
 *  ofereceria um par que o CHECK recusa. */
type Rascunho = { alerta: string; estresse: string };

const paraNumero = (v: string): number | null => {
  const t = v.trim();
  if (t === '') return null;
  const n = Number(t);
  return Number.isFinite(n) ? n : null;
};

export function Faixa({ talhao }: { talhao: Talhao }) {
  const cliente = useQueryClient();
  const idAlerta = useId();
  const idEstresse = useId();
  const [aberto, setAberto] = useState(false);
  const [rascunho, setRascunho] = useState<Rascunho>({
    alerta: talhao.kpa_alerta === null ? '' : String(talhao.kpa_alerta),
    estresse: talhao.kpa_estresse === null ? '' : String(talhao.kpa_estresse),
  });

  const salvar = useMutation({
    mutationFn: (corpo: { kpa_alerta: number | null; kpa_estresse: number | null }) =>
      api.patchTalhao(talhao.id, corpo),
    onSuccess: () => {
      // Os limiares mudam a zona de TODO no do talhao, nao so o que esta na
      // tela. Quem recalcula e o servidor -- aqui so se joga fora o que ficou
      // velho, incluindo a lista, que e ordenada por kpa mas rotulada por zona.
      void cliente.invalidateQueries({ queryKey: ['devices'] });
      void cliente.invalidateQueries({ queryKey: ['device'] });
      /* A SERIE TAMBEM, E ESTA LINHA E A MENOS OBVIA DAS TRES.
       *
       * Cada ponto vem com a zona ja classificada pelo servidor, e o resumo
       * em texto do grafico -- o que um leitor de tela anuncia -- e escrito
       * a partir dela. O desenho se corrige sozinho, porque as bandas vem
       * dos limiares novos; o texto nao, porque vem do cache antigo.
       *
       * O defeito, entao, e invisivel para quem enxerga a tela: o grafico
       * fica certo e a descricao fica errada, e as duas so se contradizem
       * para quem depende da segunda. Sobreviveu a revisao visual uma vez
       * (achado na etapa 5); qualquer refatoracao que "limpe" esta linha por
       * parecer redundante com ['device'] o traz de volta do mesmo jeito. */
      void cliente.invalidateQueries({ queryKey: ['serie'] });
      setAberto(false);
    },
  });

  const alertaNum = paraNumero(rascunho.alerta);
  const estresseNum = paraNumero(rascunho.estresse);
  const configurado = talhao.kpa_alerta !== null && talhao.kpa_estresse !== null;

  // NAO EXISTE BOTAO DE REMOVER A FAIXA, PORQUE O PATCH NAO SABE APAGAR.
  //
  // Ele existiu, foi testado contra o servidor de verdade e devolveu 200 sem
  // mudar nada. A causa esta no Go: os campos sao `*float32`, e `json.Unmarshal`
  // deixa o ponteiro nil tanto para `null` explicito quanto para chave ausente
  // -- os dois casos ficam indistinguiveis. `AtualizarTalhao` entao faz
  // `COALESCE($4, t.kpa_alerta)`, que le nil como "mantenha o que esta la".
  // Enviar {null, null} e, para aquele endpoint, o mesmo que enviar {}.
  //
  // Um botao que devolve sucesso e nao faz nada e pior que botao nenhum: a
  // interface afirma um fato falso, que e o erro que este projeto persegue no
  // grafico e no indicador. Entao ele saiu. Voltar a faixa para nao-configurada
  // hoje so pelo banco.
  //
  // Conserto e do backend, e e pequeno: distinguir ausente de nulo (ponteiro
  // duplo, json.RawMessage ou um sentinela) e trocar o COALESCE. Fora do escopo
  // da PWA, registrado no README.

  if (!aberto) {
    return (
      <div className="painel painel--fechado">
        <button type="button" className="botao botao--contorno" onClick={() => setAberto(true)}>
          {configurado ? 'Alterar os limites do talhão' : 'Definir os limites deste talhão'}
        </button>
      </div>
    );
  }

  return (
    <form
      className="painel"
      onSubmit={(e) => {
        e.preventDefault();
        // O `required` ja barra o par incompleto; esta linha existe porque
        // {null, null} seria aceito com 200 e nao mudaria nada (ver acima), e
        // "sucesso que nao fez nada" e o unico desfecho inaceitavel aqui.
        if (alertaNum === null || estresseNum === null) return;
        salvar.mutate({ kpa_alerta: alertaNum, kpa_estresse: estresseNum });
      }}
    >
      <h2 className="painel__titulo">Limites de {talhao.nome}</h2>
      <p className="painel__ajuda secundario">
        A escala vai de {textoKpa(TOPO_KPA)} (solo encharcado) a {textoKpa(FUNDO_KPA)} (solo seco).
        Quanto mais negativo, mais seco.
      </p>

      {/* Os rotulos saem de aparencia(), nao de texto solto: sao as MESMAS
          palavras que o indicador e a legenda da regua usam. Quem configura
          "Secando" aqui reconhece "Secando" na tela do no; escrever a
          terceira copia da palavra seria abrir a porta para tres nomes
          diferentes da mesma faixa. */}
      <div className="campo">
        <label htmlFor={idAlerta}>
          Marcar como “{aparencia('alerta').rotulo.toLowerCase()}” a partir de (kPa)
        </label>
        <input
          id={idAlerta}
          className="numero"
          type="number"
          required
          step={0.1}
          /* Piso = o valor corrente do campo de baixo. E o que torna a
             inversao inexprimivel, sem nenhuma comparacao escrita. */
          min={estresseNum ?? FUNDO_KPA}
          max={TOPO_KPA}
          value={rascunho.alerta}
          onChange={(e) => setRascunho((r) => ({ ...r, alerta: e.target.value }))}
        />
        <span className="dica">{aparencia('alerta').explicacao}</span>
      </div>

      <div className="campo">
        <label htmlFor={idEstresse}>
          Marcar como “{aparencia('estresse').rotulo.toLowerCase()}” a partir de (kPa)
        </label>
        <input
          id={idEstresse}
          className="numero"
          type="number"
          required
          step={0.1}
          min={FUNDO_KPA}
          /* Teto = o valor corrente do campo de cima. O outro lado da mesma
             cerca; nenhum dos dois compara nada. */
          max={alertaNum ?? TOPO_KPA}
          value={rascunho.estresse}
          onChange={(e) => setRascunho((r) => ({ ...r, estresse: e.target.value }))}
        />
        <span className="dica">{aparencia('estresse').explicacao}</span>
      </div>

      {salvar.isError && (
        /* A MENSAGEM DO SERVIDOR VAI PARA A TELA COMO VEIO, embaixo de uma
           manchete fixa.
           Como veio porque ela e a unica que sabe QUAL restricao falhou, e
           reescreve-la aqui criaria uma segunda fonte de verdade sobre a mesma
           regra -- que e o que o `min`/`max` cruzado existe para evitar.
           Embaixo de uma manchete porque, com a inversao e a faixa ja barradas
           pelo input, o erro que de fato chega aqui e o do par igual, e ele
           chega escrito em nome de coluna: "kpa_estresse precisa ser menor...".
           Isso e pior que jargao de agronomia para quem nunca viu o banco.
           A manchete e uma constante -- diz que NAO SALVOU, nao diz por que --,
           entao nao sabe regra nenhuma; o detalhe tecnico fica logo abaixo,
           legivel para quem for diagnosticar. */
        <p className="painel__erro" role="alert">
          <strong>Os limites não foram salvos.</strong>{' '}
          {salvar.error instanceof ErroAPI && salvar.error.status === SEM_REDE ? (
            'Sem conexão com o servidor.'
          ) : salvar.error instanceof ErroAPI ? (
            <span className="secundario">{salvar.error.message}</span>
          ) : (
            'Tente de novo em alguns instantes.'
          )}
        </p>
      )}

      <div className="painel__acoes">
        <button type="submit" className="botao" disabled={salvar.isPending}>
          {salvar.isPending ? 'Salvando…' : 'Salvar'}
        </button>
        <button
          type="button"
          className="botao botao--contorno"
          onClick={() => setAberto(false)}
          disabled={salvar.isPending}
        >
          Cancelar
        </button>
      </div>
    </form>
  );
}
