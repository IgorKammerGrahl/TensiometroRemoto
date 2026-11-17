#!/usr/bin/env bash
# Fixture de campo: talhoes pequenos, como os de verdade, para o redesenho do
# App (spec 2026-11-16) e para o teste com pessoas.
#
#   make campo            (ou ./scripts/campo.sh de dentro de Backend/)
#
# A vitrine tem 4 talhoes de 40 nos: serve para escala, nao para o caso de
# campo, que e de 1 a 4 sensores por talhao. Esta fixture cria, num banco
# DESCARTAVEL (telemetria_campo):
#
#   Horta   1 sensor    limites -30/-60
#   Pomar   2 sensores  limites -30/-60
#   Milho   3 sensores  limites -30/-60
#   Feijao  4 sensores  limites -30/-60
#   Pasto   2 sensores  SEM limites
#   Estufa  2 sensores  cadastrados que nunca enviaram (sem leitura)
#
# e deixa a API em :8092 com o cmd/simulador enviando na cadencia do firmware
# ate o Ctrl-C. Todo sensor que envia e simulado e marcado como tal
# (devices.simulado); os da Estufa nao tem leitura nenhuma a declarar.
#
# Banco constante, como em carga.sh: o DROP nao pode alcancar o telemetria.
# psql do host, e nao `$ENGINE exec`: o exec falha dentro do sandbox
# (oom_score_adj) e o psql do host fala com o mesmo Postgres pela 55432.
set -euo pipefail

BANCO=telemetria_campo                       # constante; ver acima
PG=${CAMPO_PG:-tcc:tcc@localhost:55432}
URL_BANCO="postgres://${PG}/${BANCO}?sslmode=disable"
URL_ADMIN="postgres://${PG}/postgres?sslmode=disable"
PORTA=${CAMPO_PORTA:-8092}
ALVO="http://localhost:${PORTA}"
EMAIL=campo@tcc.local
SEMENTE=${SEMENTE:-20261116}

cd "$(dirname "$0")/.."                      # Backend/
[ -f .env ] || { echo 'ERRO: falta o .env (o segredo do HMAC vem dele)'; exit 1; }
BIN=$(mktemp -d)
trap 'kill $(jobs -p) 2>/dev/null || true; rm -rf "$BIN"' EXIT

echo "# 1. banco $BANCO (recriado)"
psql "$URL_ADMIN" -v ON_ERROR_STOP=1 -q \
  -c "DROP DATABASE IF EXISTS ${BANCO} WITH (FORCE)" \
  -c "CREATE DATABASE ${BANCO}"
go build -o "$BIN/api" ./cmd/api
go build -o "$BIN/admin" ./cmd/admin
go build -o "$BIN/simulador" ./cmd/simulador
DATABASE_URL=$URL_BANCO "$BIN/api" -migrate >/dev/null

echo "# 2. usuario e talhoes"
admin() { DATABASE_URL=$URL_BANCO "$BIN/admin" "$@"; }
SENHA=$(admin usuario -email=$EMAIL -nome="Sítio Boa Esperança" | awk '/^senha/ {print $3}')
declare -A T
for nome in Horta Pomar Milho Feijão Pasto Estufa; do
  T[$nome]=$(admin talhao -nome="Boa Esperança · $nome" | awk '/^talhao/ {print $3}')
  admin conceder -email=$EMAIL -talhao="${T[$nome]}" >/dev/null
done
psql "$URL_BANCO" -v ON_ERROR_STOP=1 -q \
  -c "UPDATE talhoes SET kpa_alerta = -30, kpa_estresse = -60
       WHERE id IN ('${T[Horta]}','${T[Pomar]}','${T[Milho]}','${T[Feijão]}','${T[Estufa]}')" \
  -c "UPDATE talhoes t SET cultura = v.cultura FROM (VALUES
        ('${T[Horta]}'::uuid,'Alface'), ('${T[Pomar]}'::uuid,'Laranja'),
        ('${T[Milho]}'::uuid,'Milho'), ('${T[Feijão]}'::uuid,'Feijão'),
        ('${T[Pasto]}'::uuid,'Capim'), ('${T[Estufa]}'::uuid,'Tomate')
      ) AS v(id, cultura) WHERE t.id = v.id"
admin device -id=estufa-0001 -descricao="Entrada" -talhao="${T[Estufa]}" >/dev/null
admin device -id=estufa-0002 -descricao="Fundo" -talhao="${T[Estufa]}" >/dev/null

echo "# 3. API em :$PORTA"
DATABASE_URL=$URL_BANCO PORT=$PORTA "$BIN/api" >"$BIN/api.log" 2>&1 &
for _ in $(seq 1 50); do
  curl -s -o /dev/null "$ALVO/api/v1/app/me" && break
  sleep 0.1
done

echo "# 4. sensores simulados, um envio por minuto cada"
i=0
simular() { # $1 talhao, $2 quantos, $3 prefixo do id
  i=$((i + 1))
  SIMULADOR_SENHA=$SENHA "$BIN/simulador" -url="$ALVO" -email=$EMAIL \
    -talhao="${T[$1]}" -nos="$2" -prefixo="$3" -estado="$BIN/estado-$3.json" \
    -semente=$((SEMENTE + i)) -intervalo=60s -duracao=12h >"$BIN/sim-$3.log" 2>&1 &
}
simular Horta 1 horta
simular Pomar 2 pomar
simular Milho 3 milho
simular Feijão 4 feijao
simular Pasto 2 pasto

echo "# 5. nomes de campo (o simulador cadastra como 'Nó simulado NNNN')"
for _ in $(seq 1 100); do
  [ "$(psql "$URL_BANCO" -Atc 'SELECT count(*) FROM devices WHERE simulado')" -ge 12 ] && break
  sleep 0.2
done
psql "$URL_BANCO" -v ON_ERROR_STOP=1 -q <<'SQL'
UPDATE devices d SET descricao = v.nome FROM (VALUES
  ('horta-0001','Canteiro'),
  ('pomar-0001','Baixada'), ('pomar-0002','Morro'),
  ('milho-0001','Entrada'), ('milho-0002','Meio'), ('milho-0003','Fundo'),
  ('feijao-0001','Norte'), ('feijao-0002','Sul'), ('feijao-0003','Leste'), ('feijao-0004','Oeste'),
  ('pasto-0001','Açude'), ('pasto-0002','Cerca')
) AS v(id, nome) WHERE d.id = v.id;
SQL

echo "# pronto: $ALVO  login $EMAIL  senha $SENHA  (banco descartavel)"
echo "# Ctrl-C encerra API e simuladores"
wait
