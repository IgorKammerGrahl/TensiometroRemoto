-- +goose Up

-- Talhao: unidade de manejo onde o no sensor fica instalado.
CREATE TABLE talhoes (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  nome          TEXT NOT NULL,
  criado_em     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE devices (
  id            TEXT PRIMARY KEY,
  descricao     TEXT NOT NULL,
  talhao_id     UUID REFERENCES talhoes(id),
  -- HMAC-SHA256(token, TELEMETRIA_TOKEN_SECRET) em hex. O token em claro
  -- nunca e persistido; e exibido uma unica vez por cmd/devtoken.
  token_hash    TEXT NOT NULL,
  ativo         BOOLEAN NOT NULL DEFAULT true,
  criado_em     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- UNIQUE, e nao so INDEX: o lookup de autenticacao e por token_hash e
-- precisa ser O(1) e inequivoco.
CREATE UNIQUE INDEX devices_token_hash_key ON devices (token_hash);

-- Coeficientes de conversao tensao -> kPa de um ensaio de calibracao.
--
-- RECONSTRUCAO DA LEITURA A PARTIR DE raw_mv
-- ------------------------------------------
-- readings.kpa e o valor que o firmware calculou e reportou. Ele NAO e a
-- fonte de verdade: quando a calibracao experimental contra o vacuometro
-- mecanico for refeita, o historico inteiro pode ser reprocessado a partir
-- de readings.raw_mv + readings.vdd_mv + a calibracao referenciada.
--
-- O transdutor XGZP6847A100KPGN e ratiometrico: a saida e proporcional a
-- tensao de alimentacao. Por isso a leitura carrega vdd_mv, e por isso os
-- coeficientes do ensaio precisam ser corrigidos quando a alimentacao em
-- campo diverge da alimentacao de bancada:
--
--   Vsensor          = raw_mv * fator_divisor / 1000
--   fator_vdd        = vdd_mv / vdd_ensaio_mv
--   v_zero_corrigido = v_zero_kpa  * fator_vdd
--   k_corrigido      = k_v_por_kpa * fator_vdd
--   kPa              = (Vsensor - v_zero_corrigido) / k_corrigido
--
-- fator_divisor mora aqui e nao em devices porque trocar os resistores do
-- divisor invalida o ensaio: e uma calibracao nova, nao um device novo.
-- Quando vdd_mv for NULL, a reconstrucao assume fator_vdd = 1.
CREATE TABLE calibrations (
  id            TEXT PRIMARY KEY,
  device_id     TEXT NOT NULL REFERENCES devices(id),
  v_zero_kpa    REAL NOT NULL,
  k_v_por_kpa   REAL NOT NULL CHECK (k_v_por_kpa <> 0),
  fator_divisor REAL NOT NULL CHECK (fator_divisor > 0),
  vdd_ensaio_mv INTEGER NOT NULL CHECK (vdd_ensaio_mv > 0),
  r2            REAL,
  rmse_kpa      REAL,
  ensaio_em     DATE NOT NULL,
  nota          TEXT
);

CREATE INDEX calibrations_device_id_idx ON calibrations (device_id);

-- PRIMARY KEY (device_id, seq) e o mecanismo de idempotencia: reenvio de
-- lote apos resposta perdida colide e e descartado silenciosamente.
--
-- Os CHECK de faixa duplicam a validacao da aplicacao de proposito. A
-- aplicacao valida para poder rejeitar leitura por leitura e devolver o
-- motivo ao no; o banco valida para que nenhum outro caminho de escrita
-- (backfill manual, carga de CSV) contorne a regra fisica.
CREATE TABLE readings (
  device_id      TEXT NOT NULL REFERENCES devices(id),
  seq            BIGINT NOT NULL,
  measured_at    TIMESTAMPTZ NOT NULL,
  received_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  raw_mv         INTEGER NOT NULL CHECK (raw_mv BETWEEN 0 AND 3300),
  vdd_mv         INTEGER,
  kpa            REAL NOT NULL CHECK (kpa BETWEEN -100 AND 10),
  calibration_id TEXT REFERENCES calibrations(id),
  PRIMARY KEY (device_id, seq)
);

CREATE INDEX readings_device_measured_idx ON readings (device_id, measured_at DESC);

-- +goose Down
DROP TABLE readings;
DROP TABLE calibrations;
DROP TABLE devices;
DROP TABLE talhoes;
