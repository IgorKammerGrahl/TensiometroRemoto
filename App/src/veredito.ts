import type { DeviceApp, Talhao } from './api';
import { type Situacao, situacao } from './leitura';
import { formatarDuracao } from './tempo';

// A resposta do talhao: "preciso irrigar?". Spec de 16/11/2026, secao "Regra".
//
// A ORDEM DAS VERIFICACOES E A REGRA. Nao reordenar sem ler isto inteiro.
//
//   1. nenhum sensor ligado  -> nao ha a quem perguntar
//   2. talhao sem limites    -> nenhuma quantidade de dados permite afirmar
//   3. poucos dados          -> menos da metade dos ligados fala agora
//   4. leitura sem zona      -> o servidor nao classificou; nulo NAO e conforto
//   5. mediana superior      -> so aqui uma zona chega a tela
//
// O CLIENTE NAO COMPARA kPa COM LIMITE (ver api.ts e zona.ts). A mediana e
// tirada das ZONAS, que o servidor ja classificou: como a classificacao e
// monotona em kPa, a zona da leitura mediana e a mediana das zonas. Ordenar
// por categoria nao e a segunda implementacao da regra de sinal invertido.
//
// O passo 2 le `kpa_alerta` so como PRESENCA (ha limite ou nao ha), nunca do
// lado de um comparador.

export type ZonaConhecida = 'conforto' | 'alerta' | 'estresse';

/** Contagem dos sensores de um talhao. Diferente de ContagemTalhao
 *  (talhoes.ts): aqui desligado e sem leitura ficam separados, porque
 *  desligado nao entra em conta nenhuma e sem leitura entra no denominador. */
export type Contagem = {
  readonly conforto: number;
  readonly alerta: number;
  readonly estresse: number;
  /** Leitura atual sem zona reconhecida: nula, ou um valor que este cliente
   *  nao conhece. Nunca e somada a conforto. */
  readonly semZona: number;
  /** Ligado, mas sem leitura atual: velha ou nunca reportou. */
  readonly semLeitura: number;
  readonly desligados: number;
};

export type Complemento = { readonly zona: 'alerta' | 'estresse'; readonly n: number };

export type Veredito = { readonly contagem: Contagem; readonly ligados: number } & (
  | { readonly tipo: 'sem-sensor' }
  | { readonly tipo: 'sem-limites' }
  | { readonly tipo: 'poucos-dados' }
  | { readonly tipo: 'indeterminado' }
  | { readonly tipo: 'zona'; readonly zona: ZonaConhecida; readonly complemento: Complemento | null }
);

/** Checagem em tempo de execucao: os tipos nao valem nada sobre JSON que
 *  chegou pela rede. Zona fora da lista nao e reconhecida (fail-closed). */
const conhecida = (z: unknown): z is ZonaConhecida =>
  z === 'conforto' || z === 'alerta' || z === 'estresse';

export function contar(nos: readonly DeviceApp[], fetchadoEmMs: number, agoraMs: number): Contagem {
  const c = { conforto: 0, alerta: 0, estresse: 0, semZona: 0, semLeitura: 0, desligados: 0 };
  for (const d of nos) {
    const s = situacao(d.ativo, d.ultima, fetchadoEmMs, agoraMs);
    if (s.tipo === 'desativado') c.desligados++;
    else if (s.tipo !== 'atual') c.semLeitura++;
    else if (conhecida(s.zona)) c[s.zona]++;
    else c.semZona++;
  }
  return c;
}

export function veredito(
  talhao: Talhao,
  nos: readonly DeviceApp[],
  fetchadoEmMs: number,
  agoraMs: number,
): Veredito {
  const contagem = contar(nos, fetchadoEmMs, agoraMs);
  const ligados = nos.length - contagem.desligados;
  const base = { contagem, ligados };

  if (ligados === 0) return { ...base, tipo: 'sem-sensor' };
  if (talhao.kpa_alerta === null || talhao.kpa_estresse === null) return { ...base, tipo: 'sem-limites' };

  const atuais = contagem.conforto + contagem.alerta + contagem.estresse + contagem.semZona;
  if (atuais * 2 < ligados) return { ...base, tipo: 'poucos-dados' };
  if (contagem.semZona > 0) return { ...base, tipo: 'indeterminado' };

  // Elemento de indice floor(n/2) das zonas em ordem umido -> seco. Com n
  // impar e a mediana; com n par e a mais seca das duas do meio (o lado
  // conservador). Pela contagem, sem ordenar nada.
  const k = Math.floor(atuais / 2);
  const zona: ZonaConhecida =
    k < contagem.conforto ? 'conforto' : k < contagem.conforto + contagem.alerta ? 'alerta' : 'estresse';
  return { ...base, tipo: 'zona', zona, complemento: complemento(zona, contagem, atuais) };
}

/** Minoria mais seca que a resposta, quando passa de um quarto dos sensores
 *  que estao falando. O seco tem prioridade: e a acao mais urgente. Avisa, nao
 *  muda a resposta. Com 1 ou 2 sensores nao acontece por construcao. */
function complemento(zona: ZonaConhecida, c: Contagem, n: number): Complemento | null {
  const limiar = Math.ceil(n / 4);
  if (zona !== 'estresse' && c.estresse >= limiar) return { zona: 'estresse', n: c.estresse };
  if (zona === 'conforto' && c.alerta >= limiar) return { zona: 'alerta', n: c.alerta };
  return null;
}

// ------------------------------------------------------------------ frases
//
// UMA REDACAO POR ESTADO, como textoDeIdade() em leitura.ts. Duas redacoes do
// mesmo estado sao duas chances de uma delas soar como "tudo bem".

export type Frases = { readonly principal: string; readonly linhas: readonly string[] };

const DA_ZONA: Readonly<Record<ZonaConhecida, { principal: string; acao: string }>> = {
  conforto: { principal: 'Solo úmido', acao: 'Não precisa irrigar agora' },
  alerta: { principal: 'Solo secando', acao: 'Irrigar nos próximos dias' },
  estresse: { principal: 'Solo seco', acao: 'Irrigar agora' },
};

const SEM_LIMITES: Frases = {
  principal: 'Limites não definidos',
  linhas: ['Não dá para dizer se precisa irrigar'],
};

const sensores = (n: number) => (n === 1 ? 'sensor' : 'sensores');

export function frasesDaZona(z: ZonaConhecida): { principal: string; acao: string } {
  return DA_ZONA[z];
}

export function textoComplemento(c: Complemento): string {
  if (c.zona === 'estresse') return c.n === 1 ? 'mas 1 sensor seco' : `mas ${c.n} sensores secos`;
  return c.n === 1 ? 'mas 1 sensor secando' : `mas ${c.n} sensores secando`;
}

export function textoSemLeitura(v: Veredito): string | null {
  const n = v.contagem.semLeitura;
  return n === 0 ? null : `${n} de ${v.ligados} ${sensores(v.ligados)} sem leitura recente`;
}

export function frasesDoVeredito(v: Veredito): Frases {
  switch (v.tipo) {
    case 'zona': {
      const { principal, acao } = DA_ZONA[v.zona];
      return { principal, linhas: v.complemento ? [textoComplemento(v.complemento), acao] : [acao] };
    }
    case 'sem-limites':
      return SEM_LIMITES;
    case 'poucos-dados':
      // Aqui sempre ha ao menos um sem leitura: e o que trouxe ate o passo 3.
      return { principal: 'Sem leitura recente', linhas: [textoSemLeitura(v) ?? ''] };
    case 'indeterminado':
      return { principal: 'Não dá para dizer', linhas: ['Leitura sem classificação'] };
    case 'sem-sensor': {
      const n = v.contagem.desligados;
      return {
        principal: 'Nenhum sensor em uso',
        linhas: [
          n === 1 ? 'O sensor deste talhão está desligado' : `Os ${n} sensores deste talhão estão desligados`,
        ],
      };
    }
  }
}

/** As frases de UM sensor, na tela dele. Mesmas palavras do talhao. */
export function frasesDaSituacao(s: Situacao): Frases {
  switch (s.tipo) {
    case 'desativado':
      return { principal: 'Desligado', linhas: ['Este sensor foi desligado e não envia leituras'] };
    case 'nunca':
      return { principal: 'Ainda sem leitura', linhas: ['Este sensor ainda não enviou nenhuma leitura'] };
    case 'velha':
      return {
        principal: 'Sem leitura recente',
        linhas: [`Última leitura há ${formatarDuracao(s.idadeS)} · não dá para dizer como está agora`],
      };
    case 'atual': {
      if (conhecida(s.zona)) {
        const { principal, acao } = DA_ZONA[s.zona];
        return { principal, linhas: [acao] };
      }
      if (s.zona === null) return SEM_LIMITES;
      return {
        principal: 'Não dá para dizer',
        linhas: ['O servidor mandou um estado que esta versão do aplicativo não conhece. Atualize o aplicativo.'],
      };
    }
  }
}

// --------------------------------------------------------------- contagem

export type ItemContagem = {
  readonly chave: ZonaConhecida | 'sem-leitura';
  readonly n: number;
  readonly texto: string;
};

/** Da esquerda para a direita como a escala: umido -> seco, depois sem
 *  leitura. So o que existe. */
export function itensDaContagem(c: Contagem): ItemContagem[] {
  const itens: ItemContagem[] = [
    { chave: 'conforto', n: c.conforto, texto: c.conforto === 1 ? 'úmido' : 'úmidos' },
    { chave: 'alerta', n: c.alerta, texto: 'secando' },
    { chave: 'estresse', n: c.estresse, texto: c.estresse === 1 ? 'seco' : 'secos' },
    { chave: 'sem-leitura', n: c.semLeitura, texto: 'sem leitura recente' },
  ];
  return itens.filter((i) => i.n > 0);
}

/** Idade, em segundos, da leitura atual mais nova do talhao. */
export function idadeMaisRecente(
  nos: readonly DeviceApp[],
  fetchadoEmMs: number,
  agoraMs: number,
): number | null {
  let menor: number | null = null;
  for (const d of nos) {
    const s = situacao(d.ativo, d.ultima, fetchadoEmMs, agoraMs);
    if (s.tipo === 'atual' && (menor === null || s.idadeS < menor)) menor = s.idadeS;
  }
  return menor;
}

// ---------------------------------------------------------------- urgencia

const URGENCIA = {
  estresse: 0,
  alerta: 1,
  'poucos-dados': 2,
  indeterminado: 3,
  'sem-limites': 4,
  conforto: 5,
  'sem-sensor': 6,
} as const;

export function urgencia(v: Veredito): number {
  return v.tipo === 'zona' ? URGENCIA[v.zona] : URGENCIA[v.tipo];
}

/** Por categoria de resposta; empate mantem a ordem recebida (a do servidor:
 *  talhao com o sensor mais seco primeiro). O indice no desempate deixa a
 *  estabilidade explicita, sem depender da do .sort(). */
export function ordenarPorUrgencia<T>(itens: readonly T[], vereditoDe: (t: T) => Veredito): T[] {
  return itens
    .map((t, i) => ({ t, i, u: urgencia(vereditoDe(t)) }))
    .sort((a, b) => a.u - b.u || a.i - b.i)
    .map((x) => x.t);
}

// ----------------------------------------------------------------- sintese

const talhoes = (n: number) => (n === 1 ? '1 talhão' : `${n} talhões`);

/** A frase do topo da tela inicial: a propriedade inteira numa linha. */
export function sintese(vs: readonly Veredito[]): string {
  return partesDaSintese(vs).join(' · ');
}

/** A mesma sintese em partes, uma por situacao: a tela mostra uma por linha,
 *  porque com seis talhoes a frase corrida passa de cinco linhas e deixa de
 *  ser lida de relance. */
export function partesDaSintese(vs: readonly Veredito[]): string[] {
  if (vs.length === 0) return ['Nenhum sensor instalado nos seus talhões ainda'];
  const n = { agora: 0, logo: 0, semLeitura: 0, semLimites: 0, umidos: 0, semSensor: 0 };
  for (const v of vs) {
    if (v.tipo === 'zona') {
      if (v.zona === 'estresse') n.agora++;
      else if (v.zona === 'alerta') n.logo++;
      else n.umidos++;
    } else if (v.tipo === 'poucos-dados' || v.tipo === 'indeterminado') n.semLeitura++;
    else if (v.tipo === 'sem-limites') n.semLimites++;
    else n.semSensor++;
  }
  if (n.umidos === vs.length) return ['Nenhum talhão precisa de água agora'];
  const partes: Array<[number, string]> = [
    [n.agora, 'para irrigar agora'],
    [n.logo, 'para irrigar nos próximos dias'],
    [n.semLeitura, 'sem leitura recente'],
    [n.semLimites, 'sem limites'],
    [n.umidos, 'com solo úmido'],
    [n.semSensor, 'sem sensor em uso'],
  ];
  return partes.filter(([q]) => q > 0).map(([q, t]) => `${talhoes(q)} ${t}`);
}
