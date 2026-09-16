-- +goose Up

CREATE TABLE usuarios (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email      TEXT NOT NULL,
  -- bcrypt custo 12. Ao contrario do HMAC usado no token de device (alta
  -- entropia, alto volume de conferencia por ingestao), a senha do usuario
  -- tem baixa entropia e baixo volume de login: o custo computacional do
  -- bcrypt e o que torna forca bruta offline inviavel mesmo com a senha
  -- fraca que um humano tende a escolher. HMAC seria rapido demais aqui.
  senha_hash TEXT NOT NULL,
  nome       TEXT NOT NULL,
  ativo      BOOLEAN NOT NULL DEFAULT true,
  criado_em  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX usuarios_email_key ON usuarios (lower(email));

CREATE TABLE usuario_talhoes (
  usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
  talhao_id  UUID NOT NULL REFERENCES talhoes(id)  ON DELETE CASCADE,
  PRIMARY KEY (usuario_id, talhao_id)
);

CREATE TABLE sessoes (
  token_hash TEXT PRIMARY KEY,
  usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
  criada_em  TIMESTAMPTZ NOT NULL DEFAULT now(),
  expira_em  TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessoes_usuario_idx ON sessoes (usuario_id);

ALTER TABLE talhoes
  ADD COLUMN cultura      TEXT,
  ADD COLUMN kpa_alerta   REAL,
  ADD COLUMN kpa_estresse REAL,
  ADD CONSTRAINT talhoes_faixas_ck CHECK (
    (kpa_alerta IS NULL AND kpa_estresse IS NULL)
    OR (kpa_alerta IS NOT NULL AND kpa_estresse IS NOT NULL
        AND kpa_estresse < kpa_alerta
        AND kpa_alerta   BETWEEN -80 AND 0
        AND kpa_estresse BETWEEN -80 AND 0)
  );

-- +goose Down

ALTER TABLE talhoes
  DROP CONSTRAINT talhoes_faixas_ck,
  DROP COLUMN kpa_alerta,
  DROP COLUMN kpa_estresse,
  DROP COLUMN cultura;

DROP TABLE sessoes;
DROP TABLE usuario_talhoes;
DROP TABLE usuarios;
