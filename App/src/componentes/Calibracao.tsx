import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useId, useState } from 'react';
import { api, type DeviceApp, ErroAPI, SEM_REDE } from '../api';
import {
  corpoDeCalibracao,
  dataDoEnsaio,
  LIMITES,
  NOMINAIS,
  RASCUNHO_VAZIO,
  type Rascunho,
} from '../calibracao';

// Calibracao do no pelo aplicativo.
//
// ESTE PAINEL EXISTE POR CAUSA DE UMA FALHA QUE NAO APARECE EM LUGAR NENHUM.
//
// Sem nenhuma linha em `calibrations` para o no, o ciclo inteiro parece
// funcionar: o aparelho conecta, autentica, `POST /readings` responde 200 --
// e toda leitura volta dentro de `rejected`, por `calibration_id` desconhecido.
// Do lado de ca o no simplesmente nunca reporta. O sintoma tem cara de defeito
// de firmware, de antena, de bateria; o que falta e cadastro, e ate agora o
// cadastro so existia em `admin calibracao`, que pede um terminal. Fechar esse
// ciclo nao pode exigir voltar da lavoura.
//
// Dai as tres decisoes deste arquivo:
//
//   1. O AVISO VEM ANTES DO FORMULARIO, e o texto acusa a causa em vez de
//      pedir um ensaio. Quem abre a tela de um no mudo esta procurando defeito
//      no aparelho; se o painel so dissesse "Calibrar este no", continuaria
//      procurando.
//   2. O ID E DO SERVIDOR e precisa ser transcrito para o portal do no. Ele
//      fica visivel no estado FECHADO do painel, nao so na confirmacao: ao
//      contrario do token de NovoNo.tsx, este valor e recuperavel, e a
//      maneira de garantir isso e nunca escondê-lo.
//   3. `fator_divisor` e `vdd_ensaio_mv` sao PRE-PREENCHIDOS a partir do
//      ensaio anterior; `v_zero_kpa` e `k_v_por_kpa` nao. Os dois primeiros
//      descrevem a placa e raramente mudam entre ensaios; os dois ultimos SAO
//      o resultado do ensaio, e herda-los seria oferecer um formulario
//      pronto para salvar sem que nada tenha sido medido.
//
// Nao ha botao de editar nem de apagar: o servidor nao tem PATCH nem DELETE
// aqui, de proposito. Ensaio novo e id novo -- sobrescrever coeficientes
// reescreveria em silencio o significado de todas as leituras ja gravadas que
// apontam para eles.

/** Uma consulta so, compartilhada por quem precisar do estado de calibracao.
 *
 *  A tela do no usa para corrigir o diagnostico de "nunca reportou" e este
 *  painel usa para se desenhar; com a mesma chave, o react-query resolve as
 *  duas com uma requisicao. Escrever a queryFn duas vezes e que abriria espaco
 *  para as duas divergirem. */
export function useCalibracoes(id: string) {
  return useQuery({
    queryKey: ['calibracoes', id],
    queryFn: ({ signal }) => api.calibracoes(id, signal),
    staleTime: 5 * 60_000,
  });
}

export function Calibracao({ dev }: { dev: DeviceApp }) {
  const cliente = useQueryClient();
  const idVZero = useId();
  const idK = useId();
  const idDivisor = useId();
  const idVdd = useId();
  const idR2 = useId();
  const idRmse = useId();
  const idNota = useId();

  const q = useCalibracoes(dev.id);
  const [aberto, setAberto] = useState(false);
  const [rascunho, setRascunho] = useState<Rascunho>(RASCUNHO_VAZIO);

  const atual = q.data?.[0] ?? null;

  const salvar = useMutation({
    mutationFn: (corpo: NonNullable<ReturnType<typeof corpoDeCalibracao>>) =>
      api.criarCalibracao(dev.id, corpo),
    onSuccess: () => {
      // So a lista de ensaios muda. As leituras ja gravadas continuam
      // apontando para os coeficientes antigos -- e essa e a propriedade que
      // a ausencia de PATCH preserva --, entao invalidar ['serie'] aqui
      // sugeriria um recalculo que nao acontece.
      void cliente.invalidateQueries({ queryKey: ['calibracoes', dev.id] });
      setRascunho(RASCUNHO_VAZIO);
      setAberto(false);
    },
  });

  const criada = salvar.data ?? null;

  // Enquanto a consulta nao respondeu, o painel nao afirma nada: dizer "sem
  // calibracao" durante o carregamento acusaria de mudo um no que esta bem.
  if (q.isPending || q.isError) return null;

  if (criada) {
    return (
      <div className="painel">
        <div className="painel__confirma" role="status">
          <p>
            <strong>Ensaio registrado.</strong> Grave este identificador no portal do nó, no campo{' '}
            <span className="numero">CALIBRATION_ID</span>:
          </p>
          {/* Mesma caixa copiavel do token em NovoNo.tsx -- o estilo e de
              "valor para transcrever", nao do token em si. Somente leitura
              porque editar aqui nao mudaria o que o servidor gravou. */}
          <input
            className="numero token__valor"
            type="text"
            readOnly
            value={criada.calibracao.id}
            spellCheck={false}
            autoComplete="off"
            onFocus={(e) => e.currentTarget.select()}
          />
          <p className="secundario">{criada.aviso}</p>
        </div>
        <div className="painel__acoes">
          <button type="button" className="botao" onClick={() => salvar.reset()}>
            Já anotei
          </button>
        </div>
      </div>
    );
  }

  if (!aberto) {
    return (
      <div className={atual ? 'painel painel--fechado' : 'painel'}>
        {atual ? (
          <p className="secundario">
            Calibrado em {dataDoEnsaio(atual.ensaio_em)} · <span className="numero">{atual.id}</span>
            {q.data.length > 1 ? ` · ${q.data.length} ensaios registrados` : ''}
          </p>
        ) : (
          /* A faixa de aviso, e nao a cor de estresse: "a lavoura esta
             secando" e outra coisa. Ver o comentario de .aviso em base.css. */
          <div className="aviso" role="alert">
            <span>
              <strong>Este nó ainda não foi calibrado.</strong> Enquanto não houver um ensaio, o
              servidor recusa todas as leituras que ele enviar — o nó parece mudo, mas o que falta é
              este cadastro.
            </span>
          </div>
        )}
        <div className="painel__acoes">
          <button
            type="button"
            className={atual ? 'botao botao--contorno' : 'botao'}
            onClick={() => {
              /* Herda so o que descreve a placa. Ver a decisao 3 no topo. */
              setRascunho({
                ...RASCUNHO_VAZIO,
                fatorDivisor: atual ? String(atual.fator_divisor) : '',
                vddEnsaio: atual ? String(atual.vdd_ensaio_mv) : '',
              });
              setAberto(true);
            }}
          >
            {atual ? 'Registrar novo ensaio' : 'Calibrar este nó'}
          </button>
        </div>
      </div>
    );
  }

  return (
    <form
      className="painel"
      onSubmit={(e) => {
        e.preventDefault();
        const corpo = corpoDeCalibracao(rascunho);
        // O `required` ja barra o campo vazio; isto pega o que ele nao pega,
        // que e texto nao numerico colado num campo numerico.
        if (corpo) salvar.mutate(corpo);
      }}
    >
      <h2 className="painel__titulo">Ensaio de {dev.descricao || dev.id}</h2>
      <p className="painel__ajuda secundario">
        Os coeficientes saem da reta que liga a tensão lida à sucção de referência.{' '}
        <strong>São em volts</strong> — 4,5 V, e não 4500 mV.
        {atual
          ? ' O ensaio anterior continua valendo para as leituras já gravadas; este passa a valer para as próximas.'
          : ''}
      </p>

      {/* Os `min`/`max` sao a copia declarada das faixas do servidor, como o
          maxLength de NovoNo.tsx. E o teto de v_zero que faz o erro de mil
          vezes deixar de ser digitavel -- nao ha comparacao escrita em lugar
          nenhum deste arquivo. */}
      <div className="campo">
        <label htmlFor={idVZero}>Tensão no ponto de 0 kPa (V)</label>
        <input
          id={idVZero}
          className="numero"
          type="number"
          required
          step="any"
          min={0}
          max={LIMITES.vZeroMaxV}
          placeholder={NOMINAIS.vZero}
          value={rascunho.vZero}
          onChange={(e) => setRascunho((r) => ({ ...r, vZero: e.target.value }))}
        />
        <span className="dica">
          O que o sensor entrega com o tensiômetro em equilíbrio, sem sucção. Em volts.
        </span>
      </div>

      <div className="campo">
        <label htmlFor={idK}>Coeficiente da reta (V/kPa)</label>
        <input
          id={idK}
          className="numero"
          type="number"
          required
          step="any"
          min={-LIMITES.kMaxVPorKPa}
          max={LIMITES.kMaxVPorKPa}
          placeholder={NOMINAIS.k}
          value={rascunho.k}
          onChange={(e) => setRascunho((r) => ({ ...r, k: e.target.value }))}
        />
        <span className="dica">
          Quantos volts a leitura muda por kPa. Negativo se a tensão cai conforme o solo seca. Não
          pode ser zero.
        </span>
      </div>

      <div className="campo">
        <label htmlFor={idDivisor}>Fator do divisor de tensão</label>
        <input
          id={idDivisor}
          className="numero"
          type="number"
          required
          step="any"
          min={0}
          max={LIMITES.fatorDivisorMax}
          placeholder={NOMINAIS.fatorDivisor}
          value={rascunho.fatorDivisor}
          onChange={(e) => setRascunho((r) => ({ ...r, fatorDivisor: e.target.value }))}
        />
        <span className="dica">
          Quanto o divisor da placa reduz a tensão antes do ADC. É o que reconstrói o valor do
          sensor a partir do que o pino leu.
        </span>
      </div>

      <div className="campo">
        <label htmlFor={idVdd}>Alimentação do sensor no ensaio (mV)</label>
        <input
          id={idVdd}
          className="numero"
          type="number"
          required
          step={1}
          min={LIMITES.vddEnsaioMinMV}
          max={LIMITES.vddEnsaioMaxMV}
          placeholder={NOMINAIS.vddEnsaio}
          value={rascunho.vddEnsaio}
          onChange={(e) => setRascunho((r) => ({ ...r, vddEnsaio: e.target.value }))}
        />
        {/* Este e o unico campo em milivolts, e e proposital: e a unidade em
            que o proprio no reporta vdd. E o que permite ao servidor corrigir
            a leitura quando a alimentacao em campo difere da do ensaio. */}
        <span className="dica">
          Em milivolts, ao contrário dos campos acima. Sem ele o servidor não tem como corrigir a
          leitura quando a alimentação em campo for diferente.
        </span>
      </div>

      <div className="campo">
        <label htmlFor={idR2}>R² do ajuste (opcional)</label>
        <input
          id={idR2}
          className="numero"
          type="number"
          step="any"
          min={0}
          max={1}
          value={rascunho.r2}
          onChange={(e) => setRascunho((r) => ({ ...r, r2: e.target.value }))}
        />
        <span className="dica">Deixe em branco se não foi calculado.</span>
      </div>

      <div className="campo">
        <label htmlFor={idRmse}>RMSE do ajuste, em kPa (opcional)</label>
        <input
          id={idRmse}
          className="numero"
          type="number"
          step="any"
          min={0}
          max={LIMITES.rmseMaxKPa}
          value={rascunho.rmse}
          onChange={(e) => setRascunho((r) => ({ ...r, rmse: e.target.value }))}
        />
        <span className="dica">Deixe em branco se não foi calculado.</span>
      </div>

      <div className="campo">
        <label htmlFor={idNota}>Nota (opcional)</label>
        <input
          id={idNota}
          type="text"
          maxLength={LIMITES.notaMax}
          value={rascunho.nota}
          onChange={(e) => setRascunho((r) => ({ ...r, nota: e.target.value }))}
        />
        <span className="dica">
          Onde e como o ensaio foi feito. É o que distingue um ensaio do outro daqui a um ano.
        </span>
      </div>

      {salvar.isError && (
        /* A mensagem do servidor vai para a tela COMO VEIO, embaixo de uma
           manchete fixa -- mesmo arranjo de Faixa.tsx e pela mesma razao. Aqui
           ela importa ainda mais: a recusa mais valiosa do servidor e a que
           projeta o ponto de 0 kPa de volta no pino e diz em quantos mV ele
           caiu. Reescrever isso em "valor invalido" jogaria fora justamente o
           numero que explica o erro de unidade. */
        <p className="painel__erro" role="alert">
          <strong>O ensaio não foi registrado.</strong>{' '}
          {salvar.error instanceof ErroAPI && salvar.error.status === SEM_REDE ? (
            'Sem conexão com o servidor.'
          ) : salvar.error instanceof ErroAPI && salvar.error.status === 404 ? (
            'Este nó pode ter saído do seu acesso. Atualize a página; se o problema continuar, procure quem administra o sistema.'
          ) : salvar.error instanceof ErroAPI && salvar.error.status === 400 ? (
            <span className="secundario">{salvar.error.message}</span>
          ) : (
            'Tente de novo em alguns instantes.'
          )}
        </p>
      )}

      <div className="painel__acoes">
        <button type="submit" className="botao" disabled={salvar.isPending}>
          {salvar.isPending ? 'Registrando…' : 'Registrar ensaio'}
        </button>
        <button
          type="button"
          className="botao botao--contorno"
          disabled={salvar.isPending}
          onClick={() => {
            setRascunho(RASCUNHO_VAZIO);
            setAberto(false);
          }}
        >
          Cancelar
        </button>
      </div>
    </form>
  );
}
