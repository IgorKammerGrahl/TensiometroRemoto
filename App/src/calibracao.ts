// Rascunho do formulario de calibracao -> corpo da requisicao.
//
// O PERIGO DESTE FORMULARIO NAO E CAMPO VAZIO, E CAMPO COM A UNIDADE ERRADA.
//
// `v_zero_kpa` e `k_v_por_kpa` sao VOLTS e V/kPa, apesar do nome. Quem le o
// sensor num multimetro em milivolts digita 4500 onde vai 4,5, e o resultado
// passa por todos os CHECK do banco: o servidor grava, o no reporta, o grafico
// desenha, e a serie inteira fica mil vezes errada. Serie errada e pior que
// serie nenhuma, porque ninguem desconfia dela.
//
// Como a defesa esta dividida entre os dois lados:
//
//   VALIDACAO -- decidir se os coeficientes sao aceitaveis. E do servidor
//     (appCriarCalibracao), inclusive a checagem fisica que projeta o ponto de
//     0 kPa de volta no pino e exige que ele caiba na faixa do ADC. A mensagem
//     dele e a autoridade e vai para a tela como veio.
//   AFFORDANCE -- impedir que o formulario CONSTRUA o valor errado. E daqui, e
//     sai do `min`/`max` nativo do input: com teto 10 V, 4500 nem chega a ser
//     digitavel. Mesma divisao de Faixa.tsx.
//
// Os limites abaixo sao copia declarada das constantes do servidor, como o
// `maxLength={64}` de NovoNo.tsx -- nao sao palpite, e se o servidor afrouxar,
// afrouxam aqui. O que eles NAO fazem e substituir a validacao: `min`/`max` sao
// inclusivos e o servidor recusa `v_zero` exatamente zero, entao a igualdade
// continua sendo recusada la, nao aqui.

/** Faixas de appCriarCalibracao (Backend/internal/telemetria/http_app.go). */
export const LIMITES = {
  vZeroMaxV: 10,
  kMaxVPorKPa: 1,
  fatorDivisorMax: 100,
  vddEnsaioMinMV: 1000,
  vddEnsaioMaxMV: 15000,
  rmseMaxKPa: 100,
  notaMax: 500,
} as const;

/** Valores nominais do XGZP6847A com o divisor da placa. Vao para o
 *  `placeholder`, NUNCA para o `value`.
 *
 *  Placeholder porque servem de escala -- quem ve "4.5" cinza no campo nao
 *  digita 4500 -- e valor preenchido porque um formulario que ja vem com
 *  coeficiente plausivel convida a salvar sem ter feito ensaio nenhum, e o
 *  registro resultante seria indistinguivel de um ensaio de verdade. */
export const NOMINAIS = {
  vZero: '4.5',
  k: '0.04',
  fatorDivisor: '1.5',
  vddEnsaio: '5000',
} as const;

export type Rascunho = {
  vZero: string;
  k: string;
  fatorDivisor: string;
  vddEnsaio: string;
  r2: string;
  rmse: string;
  nota: string;
};

export type CorpoCalibracao = {
  v_zero_kpa: number;
  k_v_por_kpa: number;
  fator_divisor: number;
  vdd_ensaio_mv: number;
  r2?: number;
  rmse_kpa?: number;
  nota?: string;
};

export const RASCUNHO_VAZIO: Rascunho = {
  vZero: '',
  k: '',
  fatorDivisor: '',
  vddEnsaio: '',
  r2: '',
  rmse: '',
  nota: '',
};

const numero = (v: string): number | null => {
  const t = v.trim();
  if (t === '') return null;
  const n = Number(t);
  return Number.isFinite(n) ? n : null;
};

/** Monta o corpo, ou devolve null quando falta obrigatorio.
 *
 *  OPCIONAL AUSENTE E OMITIDO, NAO ZERADO. `Number('')` e 0, e um `r2: 0`
 *  escapado daqui seria o servidor gravando "ajuste pessimo" onde a pessoa
 *  quis dizer "nao medi" -- uma afirmacao inventada pela interface, que e
 *  exatamente o que erros.ts existe para nao fazer. Vale igual para `rmse_kpa`
 *  e para a nota em branco.
 *
 *  Nao valida faixa: isso e do servidor. Aqui so se separa preenchido de
 *  vazio. */
export function corpoDeCalibracao(r: Rascunho): CorpoCalibracao | null {
  const vZero = numero(r.vZero);
  const k = numero(r.k);
  const fatorDivisor = numero(r.fatorDivisor);
  const vddEnsaio = numero(r.vddEnsaio);
  if (vZero === null || k === null || fatorDivisor === null || vddEnsaio === null) {
    return null;
  }

  const corpo: CorpoCalibracao = {
    v_zero_kpa: vZero,
    k_v_por_kpa: k,
    fator_divisor: fatorDivisor,
    vdd_ensaio_mv: vddEnsaio,
  };

  const r2 = numero(r.r2);
  if (r2 !== null) corpo.r2 = r2;
  const rmse = numero(r.rmse);
  if (rmse !== null) corpo.rmse_kpa = rmse;
  const nota = r.nota.trim();
  if (nota !== '') corpo.nota = nota;

  return corpo;
}

/** Data do ensaio ("AAAA-MM-DD") em pt-BR, SEM passar por Date.
 *
 *  `new Date('2026-09-17')` e lido como meia-noite UTC; em UTC-3 o dia
 *  renderizado volta para 16. O servidor ja mandou o dia pronto justamente
 *  para nao haver essa conversao -- entao aqui so se trocam os separadores. */
export function dataDoEnsaio(ensaioEm: string): string {
  const [ano, mes, dia] = ensaioEm.split('-');
  return ano && mes && dia ? `${dia}/${mes}/${ano}` : ensaioEm;
}
