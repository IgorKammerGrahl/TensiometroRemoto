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
