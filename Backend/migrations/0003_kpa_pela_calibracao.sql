-- +goose Up

-- O kPa exibido passa a ser calculado aqui, a partir de raw_mv e da
-- calibracao que a leitura referencia. Ate esta migration a tela mostrava
-- readings.kpa, o valor que o FIRMWARE converteu com as constantes compiladas
-- nele: cadastrar uma calibracao pelo app nao mudava nada na tela, e o kPa so
-- refletia um ensaio se alguem regravasse o no com as constantes certas.

-- 1. Erro de unidade gravado antes de bd723cb.
--
-- A ajuda de `admin calibracao` dizia milivolts, e o no de bancada foi
-- cadastrado com 4500 e 40 em vez de 4,5 V e 0,04 V/kPa. Enquanto a tela
-- mostrava o kpa do firmware o erro era latente; com o calculo no servidor,
-- a serie inteira iria para -112 kPa. So se corrige a linha em que os DOIS
-- coeficientes estao em mV -- uma linha mista nao e adivinhada aqui, e o
-- CHECK abaixo derruba a migration inteira em vez de gravar um palpite.
--
-- Nao e o "editar no lugar" que a ausencia de PATCH proibe: aquilo seria
-- trocar um ensaio por outro em silencio. Aqui os numeros sempre quiseram
-- dizer o nominal de catalogo (a nota da linha diz isso); so a unidade
-- estava errada, e a correcao fica registrada nesta migration.
UPDATE calibrations
   SET v_zero_kpa  = v_zero_kpa / 1000,
       k_v_por_kpa = k_v_por_kpa / 1000
 WHERE v_zero_kpa > 10 AND abs(k_v_por_kpa) > 1;

-- 2. O banco recusa por si as mesmas faixas que o endpoint do app recusa
-- (http_app.go: vZeroMaxV, kMaxVPorKPa). cmd/admin, carga de CSV e psql nao
-- passam pelo endpoint.
ALTER TABLE calibrations
  ADD CONSTRAINT calibrations_v_zero_em_volts_ck CHECK (v_zero_kpa > 0 AND v_zero_kpa <= 10),
  ADD CONSTRAINT calibrations_k_em_volts_ck      CHECK (abs(k_v_por_kpa) <= 1);

-- 3. leituras_kpa: a unica definicao do kPa que o sistema exibe.
--
--   Vsensor = raw_mv * fator_divisor / 1000
--   kPa     = (Vsensor - v_zero_kpa) / k_v_por_kpa
--
-- SEM a correcao ratiometrica por vdd_mv / vdd_ensaio_mv que 0001_init.sql
-- descreve. O no nao mede a alimentacao: vdd_mv e a constante VDD_MV_NOMINAL
-- do config.h. Dividir o VDD medido no ensaio por uma constante escalaria a
-- serie inteira (2,7% com um ensaio a 4870 mV) sem nada no dado acusar.
-- A calibracao vale, portanto, para a alimentacao em que o ensaio foi feito.
-- ponytail: quando o no medir o VDD (segundo divisor no GPIO35), a correcao
-- entra aqui, e so aqui.
--
-- Arredondado a 0,01 kPa, a mesma resolucao com que o no reporta: sem isso,
-- ruido de ponto flutuante numa leitura exatamente igual ao limiar poderia
-- trocar a zona (Zona() compara com >=).
--
-- Leitura sem calibracao (so possivel por escrita direta: a ingestao exige
-- calibration_id) mostra o kpa do no, como antes desta migration.
CREATE VIEW leituras_kpa AS
SELECT r.device_id, r.seq, r.measured_at,
       COALESCE(
         round(((r.raw_mv * c.fator_divisor::float8 / 1000 - c.v_zero_kpa)
                / c.k_v_por_kpa)::numeric, 2)::real,
         r.kpa) AS kpa
  FROM readings r
  LEFT JOIN calibrations c ON c.id = r.calibration_id;

-- +goose Down
DROP VIEW leituras_kpa;
ALTER TABLE calibrations
  DROP CONSTRAINT calibrations_v_zero_em_volts_ck,
  DROP CONSTRAINT calibrations_k_em_volts_ck;
-- A correcao de unidade NAO e desfeita: os valores em mV eram erro, nao
-- estado a restaurar.
