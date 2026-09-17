import { useQuery } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { Link, useParams } from 'react-router';
import { api, ErroAPI, SEM_REDE } from '../api';
import { Calibracao, useCalibracoes } from '../componentes/Calibracao';
import { MarcaSituacao } from '../componentes/Estado';
import { Faixa } from '../componentes/Faixa';
import { Gerenciar } from '../componentes/Gerenciar';
import { Grafico } from '../componentes/Grafico';
import { Regua } from '../componentes/Regua';
import { textoKpa } from '../escala';
import { type Ancora, JANELAS, JANELA_PADRAO, parametros } from '../janela';
import { situacao, textoDeIdade } from '../leitura';
import { useAgora } from '../relogio';
import { aparencia } from '../zona';

// Detalhe do no (UC02). Tres leituras da mesma grandeza, em ordem de esforco
// crescente para quem le: o indicador diz o que fazer, a regua diz onde isso
// cai na escala, e o grafico diz como chegou ate aqui.
//
// O NUMERO SOZINHO NAO INFORMA. -22 kPa nao significa nada para quem nunca
// viu um tensiometro; "Umidade boa - a planta tem agua disponivel" significa.
// Por isso o valor nunca aparece desacompanhado da frase de acao, e por isso
// a regua existe: ela e a unica parte da tela em que a grandeza vira posicao,
// que e como se aprende uma escala nova sem ninguem explicar.


export function No() {
  const { id = '' } = useParams();
  const agora = useAgora();
  const [janela, setJanela] = useState(JANELA_PADRAO);

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
    queryKey: ['serie', id, janela.chave],
    queryFn: async ({ signal }) => {
      const r = await api.serie(id, parametros(janela, ancora.current), signal);
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
  const zonaAtual = s?.tipo === 'atual' ? aparencia(s.zona) : null;

  return (
    <div className="pagina">
      <header className="cabecalho cabecalho--detalhe">
        <Link to="/" className="voltar">
          ← Nós
        </Link>
      </header>

      {qDev.isPending && <p className="secundario">Carregando…</p>}

      {qDev.isError && (
        <div className="aviso" role="alert">
          {qDev.error instanceof ErroAPI && qDev.error.status === 404
            ? 'Este nó não existe ou não está em um talhão seu.'
            : qDev.error instanceof ErroAPI && qDev.error.status === SEM_REDE
              ? 'Sem conexão com o servidor.'
              : 'Não foi possível carregar este nó agora.'}
        </div>
      )}

      {dev && s && (
        <div className="pilha conteudo">
          <div>
            <h1>{dev.descricao || dev.id}</h1>
            <p className="secundario">
              {dev.talhao.nome}
              {dev.talhao.cultura ? ` · ${dev.talhao.cultura}` : ''}
            </p>
          </div>

          <section className="agora">
            <MarcaSituacao s={s} />
            {s.tipo === 'atual' && zonaAtual && (
              <>
                <p className="agora__valor">
                  <span className="numero">{textoKpa(s.kpa)}</span>
                  <span className="agora__unidade"> kPa</span>
                </p>
                <p className="agora__acao">{zonaAtual.explicacao}</p>
                <p className="secundario">Medido {textoDeIdade(s)}.</p>
              </>
            )}
            {s.tipo === 'velha' && (
              <p className="secundario">
                O nó parou de reportar. O histórico abaixo mostra até onde ele chegou; o estado
                atual do solo é desconhecido.
              </p>
            )}
            {s.tipo === 'nunca' &&
              (semCalibracao ? (
                /* A causa provavel primeiro, e ela nao e o aparelho. Sem
                   ensaio o servidor recusa tudo que o no envia, entao mandar
                   conferir bateria e sinal aqui e mandar procurar defeito
                   onde nao ha. O conserto esta no painel logo abaixo. */
                <p className="secundario">
                  O nó está cadastrado neste talhão, mas nenhuma leitura entrou — e ele ainda não
                  tem calibração. Sem ela o servidor recusa tudo que o nó envia, mesmo com o
                  aparelho ligado e com sinal.
                </p>
              ) : (
                <p className="secundario">
                  O nó está cadastrado neste talhão, mas nenhuma leitura chegou até agora. Verifique
                  se ele está ligado e com sinal.
                </p>
              ))}
            {s.tipo === 'desativado' && (
              <p className="secundario">
                Enquanto estiver desativado, o servidor recusa as leituras deste nó.
              </p>
            )}
          </section>

          <Regua
            kpa={s.tipo === 'atual' ? s.kpa : null}
            kpaAlerta={dev.talhao.kpa_alerta}
            kpaEstresse={dev.talhao.kpa_estresse}
          />

          {/* O formulario fica logo abaixo da regua porque e ali que a falta
              aparece: quem acabou de ler "Faixa nao configurada" tem o
              conserto na linha seguinte, em vez de ter que procurar outra
              tela para consertar o que a regua reclamou. */}
          <Faixa talhao={dev.talhao} />

          {/* Ao lado da faixa, e pelo mesmo motivo: sao os dois painels de
              "isto aqui ainda nao esta configurado", e ficam acima do
              historico porque um no sem calibracao nao TEM historico -- o
              servidor recusou tudo. Quem acabou de ler a frase da secao de
              cima encontra o conserto sem trocar de tela. */}
          <Calibracao dev={dev} />

          <section>
            <h2>{janela.titulo}</h2>

            <div className="janelas" role="group" aria-label="Período do histórico">
              {JANELAS.map((j) => (
                <button
                  key={j.chave}
                  type="button"
                  className={`janelas__opcao${j.chave === janela.chave ? ' janelas__opcao--ativa' : ''}`}
                  aria-pressed={j.chave === janela.chave}
                  onClick={() => setJanela(j)}
                >
                  {j.rotulo}
                </button>
              ))}
            </div>

            {qSerie.isPending && <p className="secundario">Carregando o histórico…</p>}
            {qSerie.isError && (
              <p className="secundario">Não foi possível carregar o histórico agora.</p>
            )}
            {qSerie.data && (
              /* A chave remonta o grafico a cada troca de janela. Sem ela o
                 ponto em foco do desenho anterior sobreviveria a troca,
                 apontando para um instante que ja nao esta no eixo. */
              <Grafico
                key={janela.chave}
                pontos={qSerie.data.pontos}
                bucketS={qSerie.data.bucket_s}
                deMs={Date.parse(qSerie.data.from)}
                ateMs={Date.parse(qSerie.data.to)}
                kpaAlerta={dev.talhao.kpa_alerta}
                kpaEstresse={dev.talhao.kpa_estresse}
              />
            )}
          </section>

          {/* A gestao do no fica no fim, depois do estado e do historico:
              quem abre esta tela quer saber como esta a lavoura. Editar o
              cadastro e a excecao, e excecao nao disputa o topo da tela. */}
          <p className="nota secundario">
            Nó <span className="numero">{dev.id}</span>
          </p>
          <Gerenciar dev={dev} />
        </div>
      )}
    </div>
  );
}
