import type { Zona } from './api';

// Apresentacao da zona de atencao.
//
// A ASSINATURA E O CONTRATO: aparencia() recebe a zona ja classificada e
// NADA MAIS. Nao ha parametro de limiar, entao nao existe forma de escrever
// aqui a comparacao `kpa >= kpa_alerta` -- nem certa nem invertida. E o mesmo
// padrao do usuarioID obrigatorio no backend: o erro deixa de ser proibido
// por convencao e passa a ser inexpressavel.
//
// Os limiares brutos existem no contrato (Talhao.kpa_alerta / kpa_estresse) e
// tem um unico consumidor legitimo: as linhas de referencia do grafico.

/** Chave visual. Nao e a zona: 'nao-configurado' cobre tanto null quanto um
 *  valor que o servidor passe a mandar e este cliente ainda nao conheca. */
export type ChaveVisual = 'conforto' | 'alerta' | 'estresse' | 'nao-configurado';

/** Hachura do preenchimento na regua de escala e no grafico.
 *
 *  Existe porque cor sozinha nao sobrevive a tela de celular sob sol forte
 *  nem a daltonismo (RNF05). Cada zona carrega tres canais redundantes --
 *  posicao na regua, hachura e rotulo textual -- e a cor e o quarto. */
export type Hachura = 'solido' | 'diagonal' | 'cruzado' | 'vazado';

/** enfase separa "chamar atencao" de "e ruim".
 *
 *  'pendencia' nao e um grau intermediario entre 'nenhuma' e 'atencao': e
 *  outro eixo. Talhao sem faixa configurada nao e uma leitura pior nem
 *  melhor, e a ausencia da regra que diria qual das duas. Colapsar isso num
 *  booleano `destacar` reintroduziria a chance de "nao destacado" ser lido
 *  como "esta tudo bem". */
export type Enfase = 'nenhuma' | 'atencao' | 'grave' | 'pendencia';

export type Aparencia = {
  readonly chave: ChaveVisual;
  /** Rotulo curto. E o canal primario, nao um complemento da cor. */
  readonly rotulo: string;
  /** Frase sem jargao, no imperativo do que fazer. O produtor le isto, nao
   *  "potencial matricial". */
  readonly explicacao: string;
  readonly hachura: Hachura;
  readonly enfase: Enfase;
};

const CONFORTO: Aparencia = {
  chave: 'conforto',
  rotulo: 'Umidade boa',
  explicacao: 'A planta tem água disponível. Não precisa irrigar agora.',
  hachura: 'solido',
  enfase: 'nenhuma',
};

const ALERTA: Aparencia = {
  chave: 'alerta',
  rotulo: 'Secando',
  explicacao: 'O solo está perdendo água. Prepare a irrigação.',
  hachura: 'diagonal',
  enfase: 'atencao',
};

const ESTRESSE: Aparencia = {
  chave: 'estresse',
  rotulo: 'Seco',
  explicacao: 'A planta está sofrendo por falta de água. Irrigue.',
  hachura: 'cruzado',
  enfase: 'grave',
};

const NAO_CONFIGURADO: Aparencia = {
  chave: 'nao-configurado',
  rotulo: 'Faixa não configurada',
  explicacao:
    'Este talhão ainda não tem os limites definidos. Sem eles não dá para dizer se a umidade está boa ou ruim.',
  hachura: 'vazado',
  enfase: 'pendencia',
};

/** Mesma aparencia visual de NAO_CONFIGURADO -- cinza, vazado, "nao confie
 *  neste indicador" -- com rotulo honesto. Cobre o caso de o servidor ganhar
 *  uma quarta zona antes deste cliente ser reconstruido. */
const DESCONHECIDA: Aparencia = {
  ...NAO_CONFIGURADO,
  rotulo: 'Estado desconhecido',
  explicacao:
    'O servidor informou um estado que esta versão do aplicativo não reconhece. Atualize o aplicativo.',
};

export function aparencia(zona: Zona): Aparencia {
  switch (zona) {
    case 'conforto':
      return CONFORTO;
    case 'alerta':
      return ALERTA;
    case 'estresse':
      return ESTRESSE;
    case null:
      return NAO_CONFIGURADO;
    default: {
      // Duas travas para o mesmo risco, em tempos diferentes.
      //
      // Compilacao: a atribuicao a `never` quebra o build se a uniao Zona
      // ganhar um membro e este switch nao ganhar o case correspondente.
      //
      // Execucao: os tipos nao valem nada sobre JSON que chegou pela rede. Um
      // valor fora da uniao cai aqui e vira DESCONHECIDA -- cinza e vazado,
      // nunca o verde de conforto. Fail-closed: o custo de mostrar "estado
      // desconhecido" quando esta tudo bem e um susto; o de mostrar "umidade
      // boa" sobre um estado que nao sabemos ler e uma lavoura nao irrigada.
      const _exaustivo: never = zona;
      void _exaustivo;
      return DESCONHECIDA;
    }
  }
}
