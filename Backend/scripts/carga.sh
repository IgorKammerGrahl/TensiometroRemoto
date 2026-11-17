#!/usr/bin/env bash
# Ensaio de carga: N nos simulados contra a API real, num banco DESCARTAVEL.
#
#   make carga ENGINE=docker            (ou ./scripts/carga.sh de dentro de Backend/)
#
# O que faz, nesta ordem:
#   1. recria o banco telemetria_carga no mesmo container do dev (tcc-pg)
#      e aplica as migrations nele;
#   2. cria por cmd/admin um usuario e dois talhoes so para a carga;
#   3. sobe a API compilada na porta 8081, apontada para esse banco;
#   4. roda cmd/simulador em varredura (10, 100 e 1000 nos na cadencia do
#      firmware, depois 1000 nos com intervalos cada vez menores);
#   5. derruba a API e deixa os relatorios em JSON no diretorio de saida.
#
# POR QUE OUTRO BANCO. telemetria guarda as leituras de bancada, que so
# existem no volume. Mil nos e centenas de milhares de leituras ali so
# atrapalhariam a lista do produtor de demonstracao, e a varredura precisa
# comecar de um banco vazio para ser comparavel entre execucoes. O nome do
# banco e CONSTANTE neste arquivo, e nao vem do .env, de proposito: o DROP
# abaixo nao pode alcancar o banco da bancada por engano de configuracao.
#
# O .env continua sendo lido pela API e pelo admin, mas so para o segredo do
# HMAC: DATABASE_URL e PORT sao sobrescritos aqui pelo ambiente, que vence o
# arquivo (ver internal/config).
set -euo pipefail

ENGINE=${ENGINE:-podman}
BANCO=telemetria_carga                       # constante; ver acima
PG=${CARGA_PG:-tcc:tcc@localhost:55432}
URL_BANCO="postgres://${PG}/${BANCO}?sslmode=disable"
PORTA=${CARGA_PORTA:-8081}
ALVO="http://localhost:${PORTA}"

NOS_CADENCIA=${NOS_CADENCIA:-"10 100 1000"}  # na cadencia do firmware
INTERVALO=${INTERVALO:-60s}
DURACAO=${DURACAO:-5m}
NOS_SATURACAO=${NOS_SATURACAO:-1000}
INTERVALOS_SATURACAO=${INTERVALOS_SATURACAO:-"5s 1s 500ms 250ms"}
DURACAO_SATURACAO=${DURACAO_SATURACAO:-1m}
SEMENTE=${SEMENTE:-20261112}

cd "$(dirname "$0")/.."                      # Backend/
SAIDA=${SAIDA:-../Registros/carga-$(date +%F-%H%M)}
mkdir -p "$SAIDA"
SAIDA=$(cd "$SAIDA" && pwd)
BIN=$(mktemp -d)
trap 'kill "${PID_API:-}" 2>/dev/null || true; rm -rf "$BIN"' EXIT

echo "# saida: $SAIDA"
[ -f .env ] || { echo 'ERRO: falta o .env (o segredo do HMAC vem dele)'; exit 1; }
$ENGINE exec tcc-pg pg_isready -U tcc -q || {
  echo "ERRO: o Postgres (tcc-pg) nao responde via $ENGINE. Suba com make up ENGINE=$ENGINE"; exit 1; }

echo "# 1. banco $BANCO (recriado)"
$ENGINE exec tcc-pg psql -U tcc -d postgres -v ON_ERROR_STOP=1 -q \
  -c "DROP DATABASE IF EXISTS ${BANCO} WITH (FORCE)" \
  -c "CREATE DATABASE ${BANCO}"
go build -o "$BIN/api" ./cmd/api
go build -o "$BIN/admin" ./cmd/admin
go build -o "$BIN/simulador" ./cmd/simulador
DATABASE_URL=$URL_BANCO "$BIN/api" -migrate >/dev/null

echo "# 2. usuario e talhoes da carga"
admin() { DATABASE_URL=$URL_BANCO "$BIN/admin" "$@"; }
SENHA=$(admin usuario -email=carga@tcc.local -nome="Carga Simulada" | awk '/^senha/ {print $3}')
T1=$(admin talhao -nome="Fazenda Simulada - Talhao 1" | awk '/^talhao/ {print $3}')
T2=$(admin talhao -nome="Fazenda Simulada - Talhao 2" | awk '/^talhao/ {print $3}')
admin conceder -email=carga@tcc.local -talhao="$T1" >/dev/null
admin conceder -email=carga@tcc.local -talhao="$T2" >/dev/null

echo "# 3. API em :$PORTA"
DATABASE_URL=$URL_BANCO PORT=$PORTA "$BIN/api" >"$SAIDA/api.log" 2>&1 &
PID_API=$!
for _ in $(seq 1 50); do
  curl -s -o /dev/null "$ALVO/api/v1/app/me" && break
  sleep 0.1
done

{
  echo "data        : $(date -Is)"
  echo "kernel      : $(uname -sr)"
  echo "cpu         : $(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2- | sed 's/^ //') ($(nproc) threads)"
  echo "memoria     : $(awk '/MemTotal/ {printf "%.1f GiB", $2/1048576}' /proc/meminfo)"
  echo "go          : $(go version | cut -d' ' -f3-)"
  echo "postgres    : $($ENGINE exec tcc-pg postgres --version)"
  echo "engine      : $ENGINE"
  echo "rede        : loopback, HTTP sem TLS (o no real fala HTTPS com o Caddy)"
  echo "commit      : $(git rev-parse --short HEAD)$([ -z "$(git status --porcelain .)" ] || echo ' + alteracoes nao commitadas')"
} >"$SAIDA/maquina.txt"
cat "$SAIDA/maquina.txt"

simular() {
  SIMULADOR_SENHA=$SENHA "$BIN/simulador" -url="$ALVO" -email=carga@tcc.local \
    -talhao="$T1,$T2" -estado="$BIN/estado.json" -semente="$SEMENTE" "$@"
}

echo "# 4a. cadencia do firmware: um envio a cada $INTERVALO por no, $DURACAO"
for n in $NOS_CADENCIA; do
  simular -nos="$n" -intervalo="$INTERVALO" -duracao="$DURACAO" \
    -relatorio="$SAIDA/cadencia-${n}nos.json"
done

echo "# 4b. saturacao: $NOS_SATURACAO nos, intervalo cada vez menor, $DURACAO_SATURACAO cada"
for iv in $INTERVALOS_SATURACAO; do
  simular -nos="$NOS_SATURACAO" -intervalo="$iv" -duracao="$DURACAO_SATURACAO" \
    -relatorio="$SAIDA/saturacao-${NOS_SATURACAO}nos-${iv}.json"
done

echo "# 5. conferencia no banco"
$ENGINE exec tcc-pg psql -U tcc -d "$BANCO" -c \
  "SELECT count(*) AS nos, count(*) FILTER (WHERE simulado) AS simulados,
          (SELECT count(*) FROM readings) AS leituras
     FROM devices" | tee "$SAIDA/conferencia.txt"
echo "# feito: $SAIDA"
