import { Link } from 'react-router';
import type { DeviceApp } from '../api';
import { textoKpa } from '../escala';
import { type Situacao, situacao, textoDeIdade } from '../leitura';
import type { GrupoTalhao } from '../talhoes';
import { formatarDuracao } from '../tempo';
import { frasesDoVeredito, idadeMaisRecente, textoSemLeitura, type Veredito } from '../veredito';
import { visualDoVeredito } from '../visual';
import { MarcaSituacao } from './Estado';
import { BarraProporcao } from './BarraProporcao';
import { classesDoVisual, MarcaVeredito } from './MarcaVeredito';

/** Ate quantos sensores ligados o cartao os lista um por um; acima, a barra.
 *  O caso de campo e de 1 a 4 sensores por talhao: a barra so serve a quem
 *  tem muitos, e um por um diz mais quando sao poucos. */
const LINHAS_ATE = 3;

const sensores = (n: number) => (n === 1 ? 'sensor' : 'sensores');

type Props = { g: GrupoTalhao; v: Veredito; fetchadoEm: number; agora: number };

/** O cartao de um talhao na tela inicial: a resposta em palavras primeiro.
 *
 *  <article> com o link no TITULO, esticado sobre o cartao inteiro (guia
 *  "card" do modern-web-guidance): o alvo de toque e o cartao todo, sem
 *  aninhar links, e o leitor de tela ouve o nome do talhao como nome do link.
 *  O "Ver o sensor ->" do rodape e so texto -- um segundo link para o mesmo
 *  destino seria ruido na lista de links. */
export function CartaoTalhao({ g, v, fetchadoEm, agora }: Props) {
  const frases = frasesDoVeredito(v);
  const visual = visualDoVeredito(v);
  const total = g.nos.length;
  const ligados = g.nos.filter((d) => d.ativo);
  const idade = idadeMaisRecente(g.nos, fetchadoEm, agora);
  const todosSimulados = g.simulados > 0 && g.simulados === total;
  const sit = (d: DeviceApp) => situacao(d.ativo, d.ultima, fetchadoEm, agora);
  const aviso = v.tipo === 'zona' && v.complemento !== null;

  // Com um sensor so, a pagina do talhao so repetiria a dele: vai direto.
  const direto = total === 1 ? g.nos[0]! : null;
  const destino = direto
    ? `/no/${encodeURIComponent(direto.id)}`
    : `/talhao/${encodeURIComponent(g.talhao.id)}`;

  const rodape = [
    idade !== null ? `Última leitura há ${formatarDuracao(idade)}` : null,
    todosSimulados ? 'Dados simulados' : null,
  ]
    .filter(Boolean)
    .join(' · ');

  return (
    <article className="cartao-talhao">
      <span className={`cartao-talhao__faixa ${classesDoVisual(visual)}`} aria-hidden="true" />

      <header className="cartao-talhao__topo">
        <h2>
          <Link className="cartao-talhao__link" to={destino}>
            {g.talhao.nome}
          </Link>
        </h2>
        {g.talhao.cultura && <span className="secundario">{g.talhao.cultura}</span>}
      </header>

      <div className="resposta">
        <MarcaVeredito visual={visual} />
        <div className="resposta__texto">
          <p className="resposta__principal">{frases.principal}</p>
          {frases.linhas.map((l, i) => (
            <p key={l} className={`resposta__linha${aviso && i === 0 ? ' resposta__linha--aviso' : ''}`}>
              {l}
            </p>
          ))}
        </div>
      </div>

      {v.tipo === 'sem-limites' ? (
        <p className="secundario">
          <span className="numero">{v.contagem.semZona}</span> {sensores(v.contagem.semZona)} com leitura, sem
          limites para classificar. Toque para definir os limites.
        </p>
      ) : ligados.length === 1 ? (
        <LinhaUnica d={ligados[0]!} s={sit(ligados[0]!)} />
      ) : ligados.length > 1 && ligados.length <= LINHAS_ATE ? (
        <div className="cartao-talhao__sensores">
          <ul className="linhas-sensor">
            {ligados.map((d) => (
              <LinhaSensor key={d.id} d={d} s={sit(d)} />
            ))}
          </ul>
          {/* Em "poucos dados" a contagem ja e a segunda linha da resposta. */}
          {v.contagem.semLeitura > 0 ? (
            v.tipo === 'zona' && <p className="secundario">{textoSemLeitura(v)}</p>
          ) : (
            ligados.length === 2 &&
            v.tipo === 'zona' && <p className="secundario">Com dois sensores, vale o mais seco.</p>
          )}
        </div>
      ) : ligados.length > LINHAS_ATE ? (
        <BarraProporcao c={v.contagem} />
      ) : null}

      <footer className="cartao-talhao__rodape">
        {rodape && <span>{rodape}</span>}
        <span className="cartao-talhao__ver" aria-hidden="true">
          {direto ? 'Ver o sensor →' : `Ver os ${total} sensores →`}
        </span>
      </footer>

    </article>
  );
}

/** Um sensor so: nome e numero. A idade fica no rodape ("Ultima leitura ha"),
 *  que ja e a dele -- repetir aqui eram duas frases para o mesmo fato. */
function LinhaUnica({ d, s }: { d: DeviceApp; s: Situacao }) {
  return (
    <p className="secundario">
      {d.descricao || d.id} ·{' '}
      {s.tipo === 'atual' ? (
        <>
          <span className="numero">{textoKpa(s.kpa)}</span> kPa
        </>
      ) : (
        textoDeIdade(s)
      )}
    </p>
  );
}

function LinhaSensor({ d, s }: { d: DeviceApp; s: Situacao }) {
  return (
    <li className="linha-sensor">
      <span className="linha-sensor__nome">{d.descricao || d.id}</span>
      <MarcaSituacao s={s} />
      {s.tipo === 'atual' && (
        <span className="linha-sensor__valor">
          <span className="numero">{textoKpa(s.kpa)}</span> kPa
        </span>
      )}
    </li>
  );
}
