import type { UseQueryResult } from '@tanstack/react-query';
import { useState } from 'react';
import { ErroAPI, SEM_REDE } from '../api';
import { formatarDuracao } from '../tempo';

/** Estado da conexao, igual em Talhoes e Talhao. So texto: nada gira, nada
 *  pisca. "Atualizando…" so aparece quando a PESSOA pede; a atualizacao
 *  automatica de 60 s e silenciosa, senao a linha mudaria sozinha a cada
 *  minuto. Com dados em memoria e sem rede, a tela continua: as respostas se
 *  degradam sozinhas pela idade das leituras (leitura.ts), nada e inventado. */
export function LinhaDeEstado({
  q,
  agora,
  carregando,
}: {
  q: UseQueryResult<unknown>;
  agora: number;
  carregando: string;
}) {
  const [pedido, setPedido] = useState(false);
  const atualizar = () => {
    setPedido(true);
    void q.refetch().finally(() => setPedido(false));
  };

  if (q.isPending) return <p className="linha-estado">{carregando}</p>;

  const semRede = q.error instanceof ErroAPI && q.error.status === SEM_REDE;
  const idade = formatarDuracao((agora - q.dataUpdatedAt) / 1000);
  let texto: string;
  let erro = false;
  if (q.isError && q.data === undefined) {
    erro = true;
    texto = semRede
      ? 'Sem conexão com o servidor. Confira a internet do celular.'
      : 'O servidor não respondeu como devia. Tente de novo em alguns minutos.';
  } else if (q.isError) {
    erro = true;
    texto = semRede ? `Sem conexão · dados de há ${idade}` : `Não foi possível atualizar · dados de há ${idade}`;
  } else if (pedido) {
    texto = 'Atualizando…';
  } else {
    texto = `Atualizado há ${idade}`;
  }

  return (
    <div className={`linha-estado${erro ? ' linha-estado--erro' : ''}`} role={erro ? 'alert' : undefined}>
      <p>{texto}</p>
      <button type="button" className="botao botao--contorno" disabled={pedido} onClick={atualizar}>
        {erro ? 'Tentar de novo' : 'Atualizar'}
      </button>
    </div>
  );
}
