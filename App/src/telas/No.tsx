import { useQuery } from '@tanstack/react-query';
import { useId, useRef, useState } from 'react';
import { Link, useParams } from 'react-router';
import { api, ErroAPI, SEM_REDE } from '../api';
import { Calibracao, useCalibracoes } from '../componentes/Calibracao';
import { Faixa } from '../componentes/Faixa';
import { Gerenciar } from '../componentes/Gerenciar';
import { Grafico } from '../componentes/Grafico';
import { MarcaVeredito } from '../componentes/MarcaVeredito';
import { Regua } from '../componentes/Regua';
import { SeloSimulado } from '../componentes/Simulado';
import { textoKpa } from '../escala';
import {
  type Ancora,
  diaLocal,
  type Intervalo,
  intervaloInicial,
  intervaloValido,
  JANELAS,
  JANELA_PADRAO,
  parametros,
  parametrosIntervalo,
  tituloIntervalo,
} from '../janela';
import { situacao, textoDeIdade } from '../leitura';
import { useAgora } from '../relogio';
import { frasesDaSituacao } from '../veredito';
import { visualDaSituacao } from '../visual';

// Detalhe do no (UC02). Tres leituras da mesma grandeza, em ordem de esforco
// crescente para quem le: o indicador diz o que fazer, a regua diz onde isso
// cai na escala, e o grafico diz como chegou ate aqui.
//
// O NUMERO SOZINHO NAO INFORMA. -22 kPa nao significa nada para quem nunca
// viu um tensiometro; "Solo umido - nao precisa irrigar agora" significa. Por
// isso a resposta em palavras vem primeiro, e o numero depois, menor.
// Por isso o valor nunca aparece desacompanhado da frase de acao, e por isso
// a regua existe: ela e a unica parte da tela em que a grandeza vira posicao,
// que e como se aprende uma escala nova sem ninguem explicar.


export function No() {
  const { id = '' } = useParams();
  const agora = useAgora();
  const [janela, setJanela] = useState(JANELA_PADRAO);
  /* RF07: com intervalo, o historico e o das datas escolhidas, e as janelas
   * relativas ficam de lado; sem, vale a janela. Um estado so de "modo" em vez
   * de dois evitaria nada: a janela continua lembrada para quando o produtor
   * voltar a ela. */
  const [intervalo, setIntervalo] = useState<Intervalo | null>(null);
  const hoje = diaLocal(agora);
  const idDe = useId();
  const idAte = useId();

  const qDev = useQuery({
    queryKey: ['device', id],
    queryFn: ({ signal }) => api.device(id, signal),
    staleTime: 30_000,
  });

  /* A ANCORA E O RELOGIO DO SERVIDOR, ANOTADO NA ULTIMA RESPOSTA (UC03).
   *
   * Trocar de janela nao pode virar "recalcule tudo a partir do meu relogio":
   * o do celular e o do servidor sao relogios distintos, e neste projeto o
   * desvio foi medido, nao suposto -- o ensaio de 17 h de 27/08/2026 registrou
   * latencia MINIMA de -0,123 s, isto e, `received_at` anterior a
   * `measured_at`. Entao o `to` que voltou vira o apoio do proximo `from`, e
   * `to` continua nao sendo enviado nunca (ver janela.ts).
   *
   * Ref e nao estado porque anotar a ancora nao pinta nada: ela e lida no
   * instante do fetch e so. E como nao entra na chave, anotar tambem nao
   * dispara consulta -- do contrario responder mudaria a pergunta. */
  const ancora = useRef<Ancora>(null);

  const qSerie = useQuery({
    queryKey: intervalo
      ? ['serie', id, 'datas', intervalo.de, intervalo.ate]
      : ['serie', id, janela.chave],
    queryFn: async ({ signal }) => {
      const q = intervalo
        ? parametrosIntervalo(intervalo, hoje)
        : parametros(janela, ancora.current);
      const r = await api.serie(id, q, signal);
      const t = Date.parse(r.to);
      // Carimbo malformado nao vira ancora: um NaN aqui viraria RangeError na
      // proxima troca de janela, la dentro do toISOString().
      if (Number.isFinite(t)) ancora.current = t;
      return r;
    },
    staleTime: 5 * 60_000,
  });

  /* A MESMA CONSULTA QUE O PAINEL DE CALIBRACAO FAZ, pela chave -- uma
   * requisicao so. Ela sobe ate aqui por um motivo estreito: um no sem
   * nenhum ensaio tem TODA leitura recusada pelo servidor, e do lado de ca
   * isso e indistinguivel de um no que nunca ligou. Sem este dado, a frase
   * de "nunca reportou" mandaria conferir bateria e sinal de um aparelho que
   * esta funcionando. */
  const qCal = useCalibracoes(id);
  const semCalibracao = qCal.data?.length === 0;

  const dev = qDev.data;
  const s = dev ? situacao(dev.ativo, dev.ultima, qDev.dataUpdatedAt, agora) : null;

  /* Onde a escolha de datas abre: na semana da ultima leitura. Para o no que
   * reporta, e a semana de hoje; para o que parou ha meses, e a ultima semana
   * em que ainda havia dado -- que nenhuma das janelas relativas alcanca. */
  const abrirDatas = () =>
    setIntervalo(intervaloInicial(dev?.ultima ? Date.parse(dev.ultima.measured_at) : agora));
  const mudarIntervalo = (novo: Intervalo) => {
    // O min/max dos campos ja impede o par invertido no seletor; isto cobre o
    // que for digitado. Valor invalido nao vira consulta: o servidor
    // responderia 400 e o grafico sumiria por um erro de digitacao.
    if (intervaloValido(novo, hoje)) setIntervalo(novo);
  };
  const frases = s ? frasesDaSituacao(s) : null;

  return (
    <div className="pagina">
      <header className="cabecalho cabecalho--detalhe">
        <Link to="/" className="voltar">
          ← Talhões
        </Link>
      </header>

      {qDev.isPending && <p className="secundario">Carregando…</p>}

      {qDev.isError && (
        <div className="aviso" role="alert">
          {qDev.error instanceof ErroAPI && qDev.error.status === 404
            ? 'Este sensor não existe ou não está em um talhão seu.'
            : qDev.error instanceof ErroAPI && qDev.error.status === SEM_REDE
              ? 'Sem conexão com o servidor.'
              : 'Não foi possível carregar este sensor agora.'}
        </div>
      )}

      {dev && s && frases && (
        <div className="pilha conteudo">
          <div>
            <h1>
              {dev.descricao || dev.id}
              {dev.simulado && <SeloSimulado />}
            </h1>
            <p className="secundario">
              {dev.talhao.nome}
              {dev.talhao.cultura ? ` · ${dev.talhao.cultura}` : ''}
            </p>
            {dev.simulado && (
              <p className="secundario">
                As leituras deste sensor são geradas por software, não medidas em campo.
              </p>
            )}
          </div>

          <section className="agora" aria-label="Como está o solo agora">
            <div className="resposta">
              <MarcaVeredito visual={visualDaSituacao(s)} />
              <div className="resposta__texto">
                <p className="resposta__principal">{frases.principal}</p>
                {frases.linhas.map((l) => (
                  <p key={l} className="resposta__linha">
                    {l}
                  </p>
                ))}
              </div>
            </div>

            {s.tipo === 'atual' && (
              <>
                {/* O numero depois da resposta e menor que ela: e detalhe para
                    quem quer conferir, nao a informacao principal. */}
                <p className="agora__medida">
                  <span className="agora__rotulo">Tensão da água no solo</span>
                  <span className="agora__valor numero">{textoKpa(s.kpa)}</span>
                  <span className="agora__unidade"> kPa</span>
                </p>
                <p className="secundario">Medido {textoDeIdade(s)}.</p>
              </>
            )}
            {s.tipo === 'velha' && (
              /* Sem este atalho, um sensor parado ha mais de 30 dias abre com o
                 grafico vazio nas tres janelas, e "a ultima leitura" seria uma
                 promessa que a tela nao cumpre. */
              <button type="button" className="botao botao--contorno" onClick={abrirDatas}>
                Ver a semana da última leitura
              </button>
            )}
            {s.tipo === 'nunca' && (
              /* A causa provavel primeiro, e ela nao e o aparelho. Sem ensaio o
                 servidor recusa tudo que o sensor envia, entao mandar conferir
                 bateria e sinal aqui e mandar procurar defeito onde nao ha. O
                 conserto esta em "Para o tecnico", que abre sozinho neste caso. */
              <p className="secundario">
                {semCalibracao
                  ? 'Ele ainda não tem calibração, e sem ela o servidor recusa tudo o que o sensor envia, mesmo ligado e com sinal. Veja em “Para o técnico”, abaixo.'
                  : 'Verifique se ele está ligado e com sinal.'}
              </p>
            )}
            {s.tipo === 'desativado' && (
              <p className="secundario">Enquanto estiver desligado, o servidor recusa as leituras dele.</p>
            )}
          </section>

          <Regua
            kpa={s.tipo === 'atual' ? s.kpa : null}
            kpaAlerta={dev.talhao.kpa_alerta}
            kpaEstresse={dev.talhao.kpa_estresse}
          />

          {/* Limites logo abaixo da regua: e onde "Limites nao definidos"
              aparece, e o conserto fica na linha seguinte. Aberto quando
              faltam. */}
          <details className="dobra" open={dev.talhao.kpa_alerta === null}>
            <summary>Limites de irrigação</summary>
            <Faixa talhao={dev.talhao} />
          </details>

          <section>
            <h2>{intervalo ? tituloIntervalo(intervalo) : janela.titulo}</h2>

            <div className="janelas" role="group" aria-label="Período do histórico">
              {JANELAS.map((j) => {
                const ativa = !intervalo && j.chave === janela.chave;
                return (
                  <button
                    key={j.chave}
                    type="button"
                    className={`janelas__opcao${ativa ? ' janelas__opcao--ativa' : ''}`}
                    aria-pressed={ativa}
                    onClick={() => {
                      setIntervalo(null);
                      setJanela(j);
                    }}
                  >
                    {j.rotulo}
                  </button>
                );
              })}
              <button
                type="button"
                className={`janelas__opcao${intervalo ? ' janelas__opcao--ativa' : ''}`}
                aria-pressed={intervalo !== null}
                onClick={() => (intervalo ? undefined : abrirDatas())}
              >
                Escolher datas
              </button>
            </div>

            {intervalo && (
              /* min/max cruzados, como os limiares em Faixa.tsx: o fim nao
                 escolhe dia antes do inicio, o inicio nao escolhe dia depois
                 do fim, e nenhum dos dois escolhe o futuro. */
              <div className="intervalo">
                <div className="campo">
                  <label htmlFor={idDe}>De</label>
                  <input
                    id={idDe}
                    type="date"
                    required
                    value={intervalo.de}
                    max={intervalo.ate}
                    onChange={(e) => mudarIntervalo({ ...intervalo, de: e.target.value })}
                  />
                </div>
                <div className="campo">
                  <label htmlFor={idAte}>Até</label>
                  <input
                    id={idAte}
                    type="date"
                    required
                    value={intervalo.ate}
                    min={intervalo.de}
                    max={hoje}
                    onChange={(e) => mudarIntervalo({ ...intervalo, ate: e.target.value })}
                  />
                </div>
              </div>
            )}

            {qSerie.isPending && <p className="secundario">Carregando o histórico…</p>}
            {qSerie.isError && (
              <p className="secundario">Não foi possível carregar o histórico agora.</p>
            )}
            {qSerie.data && (
              /* A chave remonta o grafico a cada troca de janela. Sem ela o
                 ponto em foco do desenho anterior sobreviveria a troca,
                 apontando para um instante que ja nao esta no eixo. */
              <Grafico
                key={intervalo ? `${intervalo.de}_${intervalo.ate}` : janela.chave}
                pontos={qSerie.data.pontos}
                bucketS={qSerie.data.bucket_s}
                deMs={Date.parse(qSerie.data.from)}
                ateMs={Date.parse(qSerie.data.to)}
                kpaAlerta={dev.talhao.kpa_alerta}
                kpaEstresse={dev.talhao.kpa_estresse}
              />
            )}
          </section>

          {/* Para o tecnico: calibracao, identificador e cadastro. Fechado e
              no fim: quem abre esta tela quer saber da lavoura, e editar o
              cadastro e a excecao. Aberto quando falta calibracao, porque ai o
              conserto e la. */}
          <details className="dobra" open={semCalibracao}>
            <summary>Para o técnico</summary>
            <div className="pilha conteudo">
              <Calibracao dev={dev} />
              <p className="secundario">
                Sensor <span className="numero">{dev.id}</span>
              </p>
              <Gerenciar dev={dev} />
            </div>
          </details>
        </div>
      )}
    </div>
  );
}
