-- +goose Up

-- Marca o no cuja serie inteira e gerada por software, e nao medida.
--
-- POR QUE UMA COLUNA, E NAO UMA CONVENCAO DE NOME
-- -----------------------------------------------
-- O trabalho aceita dado simulado desde que declarado (orientacao de
-- 12/11/2026). "Declarado" so no texto nao basta: a tela de demonstracao, um
-- print no capitulo ou uma consulta de volume de dados teriam de saber, cada
-- um por conta propria, que `sim-0042` nao e um sensor. Um prefixo no id e
-- uma promessa que nada no banco cobra. A coluna e consultavel
-- (`WHERE NOT simulado`), vai junto na resposta da API e chega a interface.
--
-- A marca e do NO, e nao da leitura: um no simulado so tem leitura simulada,
-- e o simulador (cmd/simulador) so envia com tokens de nos que ele mesmo
-- cadastrou com simulado = true -- confere isso pela API antes de cada
-- execucao e aborta se algum nao estiver marcado. Leitura simulada na serie
-- de um no real e o caso que esta separacao existe para impedir.
--
-- Fixada no cadastro e sem caminho de escrita depois: o PATCH de device nao
-- aceita o campo. Um no que deixasse de ser simulado reescreveria em silencio
-- a procedencia de toda a serie que ja gravou.
ALTER TABLE devices ADD COLUMN simulado BOOLEAN NOT NULL DEFAULT false;

-- tensio-demo e o no do cmd/seed: a serie inteira dele e sintetica, gerada
-- direto no banco. Nos ambientes em que o seed nunca rodou (a VM de deploy)
-- esta linha nao casa nada.
UPDATE devices SET simulado = true WHERE id = 'tensio-demo';

-- +goose Down
ALTER TABLE devices DROP COLUMN simulado;
