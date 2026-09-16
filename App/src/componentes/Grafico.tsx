import { scaleTime } from 'd3-scale';
import { useId, useMemo, useState } from 'react';
import type { PontoSerie } from '../api';
import { bandas, posicao, textoKpa } from '../escala';
import { segmentarSerie } from '../serie';
import { aparencia } from '../zona';

// Serie no tempo. Duas regras de honestidade governam este componente e as
// duas sao invisiveis quando quebram.
//
// 1. LACUNA NAO SE LIGA. Balde sem leitura nao gera ponto (store_app.go), e
//    um <path> unico atravessando o vazio transformaria doze horas de no
//    offline numa interpolacao limpa e plausivel. Por isso a serie e quebrada
//    por segmentarSerie() e cada trecho contiguo vira UM elemento <path>: a
//    verificacao "o grafico nao ligou atraves da lacuna" vira contar paths no
//    DOM, e nao ler uma string `d`.
//
// 2. A MEDIA ESCONDE O PIOR CASO. O servidor deriva a zona do balde de
//    min(kpa) justamente para que uma hora que tocou estresse nao apareca
//    como conforto. Desenhar so kpa_med reintroduziria esse apagamento no
//    cliente -- a linha passaria pela faixa de conforto sem nada indicando a
//    excursao. A faixa min-max atras da linha e a contrapartida visual dessa
//    decisao do servidor, nao enfeite.
//
// ESCALA Y FIXA, igual a da regua (0 a -80 kPa), e o eixo cresce PARA BAIXO:
// linha descendo = solo secando. Autoescala faria uma oscilacao de 2 kPa
// ocupar a tela inteira como se fosse uma seca, faria duas telas do mesmo
// aplicativo nao serem comparaveis, e moveria as linhas de referencia de
// lugar a cada consulta.
//
// ponytail: d3-shape foi removido do projeto. Suas curvas (curveMonotoneX e
// afins) inventam suavidade entre amostras -- o mesmo pecado da lacuna ligada,
// em escala menor -- e a interpolacao linear honesta e uma string montada em
// uma linha. Sobrou d3-scale, que fica pelos ticks de tempo: escolher rotulos
// legiveis num intervalo arbitrario e o unico pedaco aqui que nao vale
// reescrever.

const VB = 1000; // sistema de coordenadas normalizado do viewBox

type Props = {
  pontos: readonly PontoSerie[];
  bucketS: number;
  deMs: number;
  ateMs: number;
  kpaAlerta: number | null;
  kpaEstresse: number | null;
};

export function Grafico({ pontos, bucketS, deMs, ateMs, kpaAlerta, kpaEstresse }: Props) {
  const idTitulo = useId();
  const [foco, setFoco] = useState<PontoSerie | null>(null);

  const faixas = bandas(kpaAlerta, kpaEstresse);
  const vao = Math.max(1, ateMs - deMs);
  const x = (ms: number) => ((ms - deMs) / vao) * VB;

  const segmentos = useMemo(() => segmentarSerie(pontos, bucketS), [pontos, bucketS]);

  const marcas = useMemo(() => {
    const ticks = scaleTime().domain([new Date(deMs), new Date(ateMs)]).ticks(4);
    const dias = vao > 2 * 86_400_000;
    const fmt = new Intl.DateTimeFormat('pt-BR', {
      ...(dias ? { day: '2-digit', month: '2-digit' } : { hour: '2-digit', minute: '2-digit' }),
    });
    return ticks.map((d) => ({ pct: (x(d.getTime()) / VB) * 100, texto: fmt.format(d) }));
  }, [deMs, ateMs, vao]);

  // Um ponto grampeado pela escala e desenhado na borda, o que achata uma
  // excursao real. Melhor dizer que aconteceu do que deixar a linha mentir.
  const grampeado = pontos.some((p) => (posicao(p.kpa_min)?.fora ?? null) !== null);

  if (pontos.length === 0) {
    return (
      <p className="grafico__vazio">
        Nenhuma leitura neste período. O nó pode não ter reportado, ou o intervalo pode ser
        anterior à instalação dele.
      </p>
    );
  }

  return (
    <figure className="grafico">
      <p className="grafico__leitura" aria-live="polite">
        {foco ? (
          <>
            <strong className="numero">{textoKpa(foco.kpa_med)} kPa</strong> em{' '}
            {fmtQuando(foco.t)}
            {foco.kpa_min !== foco.kpa_max && (
              <span className="secundario">
                {' '}
                (entre <span className="numero">{textoKpa(foco.kpa_min)}</span> e{' '}
                <span className="numero">{textoKpa(foco.kpa_max)}</span>)
              </span>
            )}
          </>
        ) : (
          <span className="secundario">Toque no gráfico para ler um ponto.</span>
        )}
      </p>

      <div className="grafico__corpo">
        <div className="grafico__eixo-y" aria-hidden="true">
          <span className="numero">0</span>
          <span className="numero">-80</span>
        </div>

        <svg
          className="grafico__plano"
          viewBox={`0 0 ${VB} ${VB}`}
          preserveAspectRatio="none"
          role="img"
          aria-labelledby={idTitulo}
          onPointerMove={(ev) => setFoco(maisProximo(ev, pontos, deMs, vao))}
          onPointerLeave={() => setFoco(null)}
        >
          <title id={idTitulo}>{descrever(pontos, segmentos.length, faixas.length > 0)}</title>

          {faixas.map((b) => (
            <rect
              key={b.chave}
              className={`grafico__banda grafico__banda--${b.chave}`}
              x={0}
              y={b.de * VB}
              width={VB}
              height={(b.ate - b.de) * VB}
            />
          ))}

          {/* Linhas de referencia: o outro consumidor legitimo dos limiares
              brutos. Elas caem exatamente sobre a fronteira das bandas porque
              as duas saem da mesma funcao (escala.ts). */}
          {faixas.slice(0, -1).map((b) => (
            <line
              key={`ref-${b.chave}`}
              className="grafico__referencia"
              x1={0}
              x2={VB}
              y1={b.ate * VB}
              y2={b.ate * VB}
              vectorEffect="non-scaling-stroke"
            />
          ))}

          {segmentos.map((seg, i) => {
            const faixa = areaDe(seg, x);
            return (
              <g key={`seg-${i}`}>
                {faixa && <path className="grafico__faixa" d={faixa} />}
                <path
                  className="grafico__linha"
                  d={linhaDe(seg, x)}
                  vectorEffect="non-scaling-stroke"
                />
              </g>
            );
          })}

          {foco && (
            <line
              className="grafico__cursor"
              x1={x(Date.parse(foco.t))}
              x2={x(Date.parse(foco.t))}
              y1={0}
              y2={VB}
              vectorEffect="non-scaling-stroke"
            />
          )}
        </svg>
      </div>

      <div className="grafico__eixo-x" aria-hidden="true">
        {marcas.map((m) => (
          <span key={m.texto + m.pct} style={{ left: `${m.pct}%` }}>
            {m.texto}
          </span>
        ))}
      </div>

      <figcaption className="grafico__legenda secundario">
        Linha descendo é solo secando. A faixa clara em volta mostra o intervalo entre a menor e a
        maior leitura de cada período.
        {/* A LEGENDA NAO CONTA PARADAS, PORQUE NAO TEM COMO SABER QUANTAS FORAM.
            O numero de segmentos e uma propriedade do desenho; "quantas vezes o
            no parou" e uma afirmacao sobre o mundo, e as duas nao coincidem.
            Um balde vazio por deriva de grade (o no reporta a 61 s contra balde
            de 60 s) e um reporte realmente perdido produzem o mesmo salto na
            serie agregada -- medido no tensio-01: 3 saltos de 120 s em 247
            pontos, nenhum deles parada real. Anunciar "7 trechos sem dados"
            quando no maximo 4 foram paradas e inventar precisao que o dado nao
            tem, que e a mesma familia de erro que a segmentacao existe para
            impedir. Entao a legenda diz o que e verificavel -- houve
            interrupcao, e onde -- e cala sobre a contagem. */}
        {segmentos.length > 1 && (
          <>
            {' '}
            <strong>A linha se interrompe onde leituras consecutivas ficaram distantes</strong> — ali
            o nó pode ter parado de reportar.
          </>
        )}
        {grampeado && ' Alguma leitura passou do fim da escala e foi desenhada no limite.'}
      </figcaption>
    </figure>
  );
}

// ------------------------------------------------------------------ geometria

/** Interpolacao linear entre amostras reais, e nada mais. Pontos cujo kPa nao
 *  e finito somem: posicao() devolve null e eles nao entram na string. */
function linhaDe(seg: readonly PontoSerie[], x: (ms: number) => number): string {
  const partes: string[] = [];
  for (const p of seg) {
    const y = posicao(p.kpa_med);
    if (!y) continue;
    partes.push(`${x(Date.parse(p.t)).toFixed(1)},${(y.fracao * VB).toFixed(1)}`);
  }
  return partes.length === 0 ? '' : `M${partes.join('L')}`;
}

/** Faixa min-max do trecho: por cima os minimos, de volta pelos maximos. */
function areaDe(seg: readonly PontoSerie[], x: (ms: number) => number): string | null {
  const cima: string[] = [];
  const baixo: string[] = [];
  for (const p of seg) {
    const yMin = posicao(p.kpa_min);
    const yMax = posicao(p.kpa_max);
    if (!yMin || !yMax) continue;
    const px = x(Date.parse(p.t)).toFixed(1);
    cima.push(`${px},${(yMax.fracao * VB).toFixed(1)}`);
    baixo.unshift(`${px},${(yMin.fracao * VB).toFixed(1)}`);
  }
  if (cima.length < 2) return null;
  return `M${cima.join('L')}L${baixo.join('L')}Z`;
}

function maisProximo(
  ev: React.PointerEvent<SVGSVGElement>,
  pontos: readonly PontoSerie[],
  deMs: number,
  vao: number,
): PontoSerie | null {
  const r = ev.currentTarget.getBoundingClientRect();
  if (r.width === 0) return null;
  const alvo = deMs + ((ev.clientX - r.left) / r.width) * vao;

  let melhor: PontoSerie | null = null;
  let dist = Number.POSITIVE_INFINITY;
  for (const p of pontos) {
    const d = Math.abs(Date.parse(p.t) - alvo);
    if (d < dist) {
      dist = d;
      melhor = p;
    }
  }
  return melhor;
}

// ------------------------------------------------------------------ texto


const fmtQuando = (t: string) =>
  new Intl.DateTimeFormat('pt-BR', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(t));

function descrever(
  pontos: readonly PontoSerie[],
  segmentos: number,
  temFaixa: boolean,
): string {
  const pior = pontos.reduce((a, b) => (b.kpa_min < a.kpa_min ? b : a));
  const zonaPior = aparencia(pior.zona).rotulo.toLowerCase();
  const lacunas =
    segmentos > 1 ? ` A série tem ${segmentos - 1} interrupção sem dados.` : '';
  const faixa = temFaixa ? '' : ' As faixas de umidade não estão configuradas.';
  return (
    `Gráfico da umidade do solo ao longo do tempo, com ${pontos.length} pontos. ` +
    `Quanto mais baixa a linha, mais seco o solo. Pior momento em ${fmtQuando(pior.t)}, ` +
    `${textoKpa(pior.kpa_min)} quilopascal, ${zonaPior}.${lacunas}${faixa}`
  );
}
