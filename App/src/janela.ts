// Janela de consulta do historico.
//
// O CLIENTE ESCOLHE A DURACAO. O SERVIDOR DECIDE QUANDO E "AGORA".
//
// O handler de serie usa `to` = time.Now() do servidor quando o parametro nao
// vem, e `from` = to - 24 h quando esse tambem falta (http_app.go). Entao a
// interface manda SO `from` e nunca `to`: o fim da janela e sempre o relogio do
// servidor, o mesmo que carimbou `received_at` e o mesmo que respondeu a lista.
//
// O desvio de relogio nao e hipotetico neste projeto. O ensaio de 17 h de
// 27/08/2026 (Registros/registro-2026-08-27.md) mediu latencia MINIMA de
// -0,123 s -- `received_at` anterior a `measured_at`, o que so e possivel
// porque os dois carimbos vem de relogios diferentes: o do ESP32 via NTP e o do
// servidor. Sao ~0,12 s, irrelevantes para uma janela de dias; o que importa e
// que a interface nao tem um terceiro relogio melhor que esses dois. O celular
// do produtor, na lavoura, pode estar muito pior que 0,12 s.
//
// Mesma razao de `medido_ha_s` existir em vez de a interface subtrair datas.

/** Fim da janela anterior, em ms, como o servidor o declarou. */
export type Ancora = number | null;

export type ChaveJanela = '24h' | '7d' | '30d';

export type Janela = {
  readonly chave: ChaveJanela;
  readonly rotulo: string;
  /** Titulo do grafico. Sem jargao: o produtor le "Ultimos 7 dias". */
  readonly titulo: string;
  readonly duracaoS: number;
};

const HORA = 3600;
const DIA = 24 * HORA;

export const JANELAS: readonly Janela[] = [
  { chave: '24h', rotulo: '24 horas', titulo: 'Últimas 24 horas', duracaoS: DIA },
  { chave: '7d', rotulo: '7 dias', titulo: 'Últimos 7 dias', duracaoS: 7 * DIA },
  { chave: '30d', rotulo: '30 dias', titulo: 'Últimos 30 dias', duracaoS: 30 * DIA },
];

export const JANELA_PADRAO: Janela = JANELAS[0]!;

export function janelaDe(chave: string | null | undefined): Janela {
  return JANELAS.find((j) => j.chave === chave) ?? JANELA_PADRAO;
}

/** Parametros de consulta da janela. `to` fica de fora DE PROPOSITO.
 *
 * Sem ancora -- primeira carga, antes de qualquer resposta -- devolve `{}`, e o
 * servidor aplica o padrao dele de 24 h e informa `from`/`to` na resposta. E
 * dai que sai a primeira ancora. Trocar de janela antes disso e impossivel: os
 * botoes so existem depois que o grafico existe.
 *
 * Para 24 h o `{}` tambem serve, e e melhor que mandar `from`: uma requisicao a
 * menos de diferenca entre a interface e o padrao do servidor. */
export function parametros(j: Janela, ancora: Ancora): { from?: string } {
  if (ancora === null) return {};
  return { from: new Date(ancora - j.duracaoS * 1000).toISOString() };
}

// ------------------------------------------------- intervalo de datas (RF07)

/** Intervalo escolhido pelo produtor: dias inteiros do calendario dele, no
 *  fuso do aparelho, em 'AAAA-MM-DD' -- o formato do <input type="date">. */
export type Intervalo = { readonly de: string; readonly ate: string };

const DIA_ISO = /^\d{4}-\d{2}-\d{2}$/;

/** Data local 'AAAA-MM-DD' somada de `n` dias. O construtor de Date atravessa
 *  mes e ano sozinho, e acompanharia horario de verao se ele voltasse. */
function deslocar(dia: string, n: number): Date {
  const [a, m, d] = dia.split('-').map(Number);
  return new Date(a!, m! - 1, d! + n);
}

/** 'AAAA-MM-DD' do instante, no fuso do aparelho. */
export function diaLocal(ms: number): string {
  const d = new Date(ms);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

/** de <= ate <= hoje, as duas no formato do input. A comparacao de texto vale
 *  porque o formato e de largura fixa e vai do ano para o dia. */
export function intervaloValido(iv: Intervalo, hoje: string): boolean {
  return DIA_ISO.test(iv.de) && DIA_ISO.test(iv.ate) && iv.de <= iv.ate && iv.ate <= hoje;
}

/** Parametros de consulta de um intervalo de datas.
 *
 * AQUI O CLIENTE MANDA `to`, ao contrario das janelas relativas. As datas sao
 * escolha do produtor, no calendario dele -- "de 1 a 16 de setembro" --, e
 * nao uma duracao contada do agora; quem sabe onde comeca o dia 1 em Rio do
 * Sul e o aparelho, nao o servidor (que fala UTC). Fim exclusivo: meia-noite
 * do dia seguinte ao ultimo.
 *
 * A excecao e o intervalo que termina hoje. Ai o fim e o agora do servidor, e
 * `to` fica de fora pelo motivo do cabecalho deste arquivo. `hoje` e o unico
 * uso do relogio do aparelho aqui, e um erro dele so muda a decisao perto da
 * meia-noite. */
export function parametrosIntervalo(iv: Intervalo, hoje: string): { from: string; to?: string } {
  const from = deslocar(iv.de, 0).toISOString();
  if (iv.ate >= hoje) return { from };
  return { from, to: deslocar(iv.ate, 1).toISOString() };
}

/** Os 7 dias que terminam no dia do instante dado. Usado ao abrir a escolha
 *  de datas: com o instante da ultima leitura, um no que parou de reportar ha
 *  meses abre direto na semana em que ainda havia dado. */
export function intervaloInicial(fimMs: number): Intervalo {
  const ate = diaLocal(fimMs);
  return { de: diaLocal(deslocar(ate, -6).getTime()), ate };
}

export function tituloIntervalo(iv: Intervalo): string {
  const br = (dia: string) => dia.split('-').reverse().join('/');
  return iv.de === iv.ate ? `Dia ${br(iv.de)}` : `De ${br(iv.de)} a ${br(iv.ate)}`;
}
