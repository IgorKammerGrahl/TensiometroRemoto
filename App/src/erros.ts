import { ErroAPI, SEM_REDE } from './api';

// Mensagem de falha das operacoes de no (RF10).
//
// O SERVIDOR JUNTA CASOS DE PROPOSITO, E A INTERFACE NAO OS SEPARA DE VOLTA.
//
// `AtualizarDevice` exige DUAS concessoes quando o PATCH move o no de talhao:
// a de ORIGEM, sem a qual qualquer um puxaria um no alheio para dentro do
// proprio alcance, e a de DESTINO, sem a qual o dono de um no o empurraria
// para fora do alcance de quem o vigiava. Sao dois riscos distintos, e por
// isso sao dois EXISTS. Mas zero linhas afetadas vira um 404 unico, que cobre
// tres situacoes: no inexistente, origem ausente, destino ausente. O cadastro
// tem o mesmo desenho com duas: talhao inexistente e talhao nao concedido.
//
// Escolher uma delas para mostrar seria inventar informacao que a resposta nao
// contem -- e a escolhida estaria errada nos outros casos. Entao a mensagem
// NOMEIA AS POSSIBILIDADES E NAO AFIRMA NENHUMA.
//
// A garantia nao esta na disciplina de quem escreve o texto: esta na
// assinatura. Esta funcao recebe o erro e a operacao, e nada mais -- nao ha
// parametro por onde "qual concessao faltou" pudesse entrar, porque esse dado
// nao existe do lado de ca. Do mesmo jeito que a inversao de limiares e
// inexprimivel no formulario da faixa, o palpite e inexprimivel aqui.

/** `cadastro` = POST /devices (404 fala so de talhao, porque o no ainda nao
 *  existe). `edicao` = PATCH /devices/{id} (404 fala de no E de talhao). */
export type Operacao = 'cadastro' | 'edicao';

export function explicarFalhaDeNo(erro: unknown, operacao: Operacao): string {
  if (!(erro instanceof ErroAPI)) return 'Tente de novo em alguns instantes.';

  switch (erro.status) {
    case SEM_REDE:
      // Nao e falha da operacao: e ausencia de resposta. Dizer "nao foi
      // possivel cadastrar" aqui afirmaria que o servidor recusou, e ele pode
      // nem ter sido alcancado.
      return 'Sem conexão com o servidor. Verifique a rede do aparelho.';

    case 404:
      return operacao === 'cadastro'
        ? 'O talhão escolhido não está mais disponível para você. Atualize a página; se ele continuar fora da lista, procure quem administra o sistema.'
        : 'O nó ou o talhão escolhido pode ter saído do seu acesso. Atualize a página; se o problema continuar, procure quem administra o sistema.';

    case 409:
      // O servidor diz "ja existe device com esse id". "Device" e "id" sao
      // palavras do banco; a regra e a mesma, so o vocabulario muda.
      return 'Já existe um nó com esse identificador. Escolha outro.';

    case 400:
      // Campo recusado. O servidor nomeia QUAL, e nomear de novo aqui seria
      // manter duas listas de campos que divergem na primeira mudanca.
      return erro.message;

    default:
      // Inclui o 500. A mensagem de erro interno e para o log do servidor,
      // nao para a tela de quem esta no meio da lavoura.
      return 'Tente de novo em alguns instantes.';
  }
}
