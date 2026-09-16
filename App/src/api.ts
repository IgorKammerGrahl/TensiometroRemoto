// Contrato e transporte do plano de app (/api/v1/app/*).
//
// Os tipos abaixo espelham as structs de Backend/internal/telemetria/store_app.go.
// Quando uma delas mudar, este arquivo muda junto -- nao ha geracao automatica,
// e a divergencia aparece como erro de tipo no primeiro uso.

/** Zona de atencao, JA CLASSIFICADA PELO SERVIDOR.
 *
 * NAO RE-DERIVAR NO CLIENTE. kPa e negativo e mais negativo e pior, entao a
 * comparacao correta e `>=` -- o inverso da intuicao "maior e pior". A regra
 * mora num unico lugar, Zona() em store_app.go, e duplicar aqui duplicaria a
 * chance de inverter o operador. Ver zona.ts: a funcao de apresentacao nao
 * aceita limiares na assinatura, entao a comparacao nem chega a ser
 * expressavel deste lado.
 *
 * null = talhao sem faixa configurada. NULL NAO E CONFORTO.
 */
export type Zona = 'conforto' | 'alerta' | 'estresse' | null;

export type Usuario = {
  id: string;
  email: string;
  nome: string;
};

export type Talhao = {
  id: string;
  nome: string;
  cultura: string | null;
  /** Limiares brutos. Servem para UMA coisa: desenhar as linhas de
   *  referencia do grafico (RF08). Nunca do lado direito de um comparador. */
  kpa_alerta: number | null;
  kpa_estresse: number | null;
};

export type UltimaLeitura = {
  measured_at: string;
  kpa: number;
  zona: Zona;
  /** Idade da leitura em segundos, calculada pelo servidor no instante da
   *  resposta. Envelhece na mao do cliente: ver idadeSegundos() em tempo.ts. */
  medido_ha_s: number;
};

export type DeviceApp = {
  id: string;
  descricao: string;
  ativo: boolean;
  talhao: Talhao;
  /** null = no cadastrado que nunca reportou. Terceiro estado, distinto de
   *  "leitura velha" e de "leitura atual". */
  ultima: UltimaLeitura | null;
};

/** Resposta do cadastro de no (RF10). O `token` so existe aqui.
 *
 *  `devices.token_hash` e irreversivel por construcao e nao ha endpoint que
 *  recupere um token existente -- nem no plano de app, nem no de dispositivo.
 *  Este campo e a unica aparicao do valor em claro na vida inteira do no. */
export type DeviceCriado = {
  device: DeviceApp;
  token: string;
  /** Aviso escrito pelo servidor. Vai para a tela como veio: quem definiu que
   *  o token nao e recuperavel e ele. */
  aviso: string;
};

export type PontoSerie = {
  t: string;
  kpa_med: number;
  kpa_min: number;
  kpa_max: number;
  n: number;
  /** Vem de kpa_min, o pior caso do balde: uma hora que tocou estresse nao
   *  pode aparecer como conforto por causa da media. */
  zona: Zona;
};

export type Serie = {
  device_id: string;
  from: string;
  to: string;
  /** Largura do balde em segundos: a resolucao com que o servidor AGREGA.
   *  NAO e o periodo de amostragem do no, e a diferenca entre os dois ja
   *  custou uma tela errada -- ver o cabecalho de serie.ts. */
  bucket_s: number;
  talhao: Talhao;
  count: number;
  pontos: PontoSerie[];
};

export type ListaDevices = { count: number; devices: DeviceApp[] };

export type RespostaMe = { usuario: Usuario; talhoes: Talhao[] };

// ------------------------------------------------------------- transporte

const BASE = '/api/v1/app';

/** SEM_REDE: nao ha status HTTP 0, entao o valor e inequivoco. Distingue
 *  "o servidor respondeu um erro" de "nao houve resposta" -- o segundo caso
 *  nao deve derrubar a sessao nem mandar o usuario para o login. */
export const SEM_REDE = 0;

export class ErroAPI extends Error {
  constructor(
    readonly status: number,
    readonly caminho: string,
    mensagem: string,
  ) {
    super(mensagem);
    this.name = 'ErroAPI';
  }
}

type Opcoes = {
  metodo?: 'GET' | 'POST' | 'PATCH';
  corpo?: unknown;
  /** Respostas 204 nao tem corpo. Marcar explicitamente evita tentar
   *  desserializar vazio e transformar sucesso em erro de parse. */
  semCorpo?: boolean;
  sinal?: AbortSignal;
};

async function requisitar<T>(caminho: string, opcoes: Opcoes = {}): Promise<T> {
  const { metodo = 'GET', corpo, semCorpo = false, sinal } = opcoes;

  const cabecalhos: Record<string, string> = { Accept: 'application/json' };
  if (corpo !== undefined) cabecalhos['Content-Type'] = 'application/json';

  let resposta: Response;
  try {
    resposta = await fetch(BASE + caminho, {
      method: metodo,
      headers: cabecalhos,
      // Explicito embora seja o padrao para URL de mesma origem: o cookie de
      // sessao e HttpOnly e o unico mecanismo de autenticacao daqui. Deixar
      // implicito o que a autenticacao inteira depende economiza uma palavra
      // e custa uma sessao de depuracao.
      credentials: 'same-origin',
      ...(corpo !== undefined ? { body: JSON.stringify(corpo) } : {}),
      ...(sinal ? { signal: sinal } : {}),
    });
  } catch (causa) {
    if (causa instanceof DOMException && causa.name === 'AbortError') throw causa;
    throw new ErroAPI(SEM_REDE, caminho, 'sem conexao com o servidor');
  }

  if (!resposta.ok) {
    throw new ErroAPI(resposta.status, caminho, await mensagemDeErro(resposta));
  }

  if (semCorpo || resposta.status === 204) return undefined as T;

  // Uma resposta 200 que nao e JSON so acontece de um jeito: a rota caiu no
  // fallback de SPA e voltou o index.html. E o modo de falha que a etapa 6
  // previne no servidor (rota desconhecida sob /api/ devolve 404 JSON); aqui
  // ele para de ser silencioso do lado do cliente tambem. Sem esta checagem,
  // JSON.parse de HTML viraria um erro de sintaxe sem relacao aparente com a
  // causa.
  const tipo = resposta.headers.get('content-type') ?? '';
  if (!tipo.includes('application/json')) {
    throw new ErroAPI(
      resposta.status,
      caminho,
      `resposta ${resposta.status} nao e JSON (content-type: ${tipo || 'ausente'})`,
    );
  }
  return (await resposta.json()) as T;
}

async function mensagemDeErro(resposta: Response): Promise<string> {
  try {
    const corpo: unknown = await resposta.json();
    if (
      typeof corpo === 'object' &&
      corpo !== null &&
      'error' in corpo &&
      typeof corpo.error === 'string'
    ) {
      return corpo.error;
    }
  } catch {
    // Corpo ausente ou nao-JSON: cai no texto de status.
  }
  return resposta.statusText || `erro ${resposta.status}`;
}

export const api = {
  /** 200 autenticado · 401 sem sessao · 403 conta desativada.
   *  Os tres sao respostas diferentes e a interface trata os tres. */
  me: (sinal?: AbortSignal) =>
    requisitar<RespostaMe>('/me', sinal ? { sinal } : {}),

  /** 204 + Set-Cookie · 401 credencial invalida · 403 conta desativada.
   *  O 401 e o mesmo para senha errada e conta inexistente, por desenho do
   *  servidor. Nao inventar distincao aqui. */
  login: (email: string, senha: string) =>
    requisitar<void>('/login', {
      metodo: 'POST',
      corpo: { email, senha },
      semCorpo: true,
    }),

  logout: () => requisitar<void>('/logout', { metodo: 'POST', semCorpo: true }),

  /** Nos visiveis ao usuario, JA ORDENADOS PELO SERVIDOR.
   *
   *  ORDER BY kpa ASC -- mais negativo primeiro, ou seja, pior primeiro; nos
   *  sem leitura por ultimo (store_app.go). Nao e engano de digitacao e nao e
   *  aleatorio: e a resposta a "de qual talhao eu cuido agora?", que e a
   *  pergunta da UC05.
   *
   *  NUNCA REORDENAR NO CLIENTE. Um `.sort()` aqui ou na tela seria a segunda
   *  implementacao da mesma regra de sinal invertido, e a que ninguem testa.
   *  Ordenar por kpa DESC pareceria certo em qualquer revisao rapida e poria
   *  o talhao mais seco no fim da lista. */
  devices: (sinal?: AbortSignal) =>
    requisitar<ListaDevices>('/devices', sinal ? { sinal } : {}),

  /** 404 tambem quando o no existe mas nao e deste usuario. O servidor nao
   *  distingue de proposito -- 403 confirmaria a existencia do recurso. */
  device: (id: string, sinal?: AbortSignal) =>
    requisitar<DeviceApp>(`/devices/${encodeURIComponent(id)}`, sinal ? { sinal } : {}),

  /** Serie agregada. Sem `bucket`, o servidor escolhe a largura de balde.
   *
   *  `bucket_s` volta na resposta e nao e informativo: e o que permite ao
   *  cliente distinguir "balde sem leitura" de "leitura contigua". Ver
   *  serie.ts. */
  /** Cadastro de no (RF10). O 201 traz o TOKEN EM CLARO, uma unica vez.
   *
   *  Se a tela perder esse valor antes de a pessoa guarda-lo, o no nao
   *  autentica e a unica saida e cadastrar outro. Quem chama isto, portanto,
   *  nao navega, nao fecha e nao descarta a resposta por conta propria --
   *  ver telas/NovoNo.tsx, onde a ordem das operacoes e o assunto.
   *
   *  404 = talhao inexistente OU nao concedido, indistinguiveis de proposito.
   *  409 = ja existe no com esse identificador. */
  criarDevice: (
    corpo: { id: string; descricao: string; talhao_id: string },
    sinal?: AbortSignal,
  ) =>
    requisitar<DeviceCriado>('/devices', {
      metodo: 'POST',
      corpo,
      ...(sinal ? { sinal } : {}),
    }),

  /** Edicao, mudanca de talhao e desativacao do no (RF10).
   *
   *  Campo ausente = campo preservado (COALESCE no servidor). Mandar so o que
   *  mudou nao e economia de bytes: e o que impede de sobrescrever o resto.
   *
   *  Nao ha como zerar `talhao_id` por aqui, e isso e do servidor: um no sem
   *  talhao fica invisivel para todo mundo, e orfanar e operacao de bancada.
   *
   *  MOVER DE TALHAO EXIGE DUAS CONCESSOES -- a de origem e a de destino -- e
   *  o 404 nao diz qual faltou, nem se o problema era o proprio no. Ver
   *  erros.ts: a interface trata o erro sem adivinhar. */
  patchDevice: (
    id: string,
    corpo: { descricao?: string; talhao_id?: string; ativo?: boolean },
    sinal?: AbortSignal,
  ) =>
    requisitar<DeviceApp>(`/devices/${encodeURIComponent(id)}`, {
      metodo: 'PATCH',
      corpo,
      ...(sinal ? { sinal } : {}),
    }),

  /** Limiares do talhao (RF07/UC04).
   *
   * Os dois campos vao SEMPRE juntos, inclusive para limpar (os dois `null`).
   * Nao e escolha da interface: `talhoes_faixas_ck` exige ambos nulos ou ambos
   * preenchidos, e `validarFaixa` recusa o par incompleto com 400. Deixar a
   * assinatura mandar um so seria oferecer um estado que o banco nao aceita. */
  patchTalhao: (
    id: string,
    corpo: { kpa_alerta: number | null; kpa_estresse: number | null },
    sinal?: AbortSignal,
  ) =>
    requisitar<Talhao>(`/talhoes/${encodeURIComponent(id)}`, {
      metodo: 'PATCH',
      corpo,
      ...(sinal ? { sinal } : {}),
    }),

  serie: (
    id: string,
    params: { from?: string; to?: string; bucket?: number } = {},
    sinal?: AbortSignal,
  ) => {
    const q = new URLSearchParams();
    if (params.from) q.set('from', params.from);
    if (params.to) q.set('to', params.to);
    if (params.bucket !== undefined) q.set('bucket', String(params.bucket));
    const busca = q.toString();
    return requisitar<Serie>(
      `/devices/${encodeURIComponent(id)}/series${busca ? `?${busca}` : ''}`,
      sinal ? { sinal } : {},
    );
  },
};
