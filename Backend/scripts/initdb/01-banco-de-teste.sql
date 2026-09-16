-- A suite de integracao roda contra um Postgres real (ON CONFLICT, chave
-- estrangeira e TIMESTAMPTZ nao sobrevivem a mock), num banco separado para
-- que `go test` possa truncar tabelas sem levar junto os dados de bancada.
--
-- Roda uma unica vez, na inicializacao de um volume vazio.
CREATE DATABASE telemetria_test OWNER tcc;
