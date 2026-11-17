#!/usr/bin/env bash
# T1: 200 nos a 1 s pelo Caddy (HTTPS), 90 s; replica 2 parada aos 30 s e
# religada aos 60 s. Relatorio em $S/t1-*.json, eventos em $S/t1-eventos.txt.
set -euo pipefail
S=$(cd "$(dirname "$0")" && pwd); cd "$S"
rotulo=${1:-t1}
SENHA=$(awk '/^senha/ {print $3}' tcc-a-usuario.txt)
ev() { echo "$(date +%T.%3N) $*" | tee -a "$S/$rotulo-eventos.txt"; }
: > "$S/$rotulo-eventos.txt"
# Cadastro (fora da medicao): a primeira execucao cadastra os 200 nos.
if [ ! -f "$S/$rotulo-estado.json" ]; then
  SSL_CERT_FILE=tcc-a-root.crt SIMULADOR_SENHA=$SENHA ./simulador -url=https://localhost:8444 \
    -email=teste@tcc.local -talhao="$(cat tcc-a-talhoes.txt)" -estado="$S/$rotulo-estado.json" \
    -nos=200 -intervalo=1s -duracao=2s -semente=20261116 >/dev/null 2>&1
  ev "cadastro feito"
fi
SSL_CERT_FILE=tcc-a-root.crt SIMULADOR_SENHA=$SENHA ./simulador -url=https://localhost:8444 \
  -email=teste@tcc.local -talhao="$(cat tcc-a-talhoes.txt)" -estado="$S/$rotulo-estado.json" \
  -nos=200 -intervalo=1s -duracao=90s -semente=20261116 -relatorio="$S/$rotulo-relatorio.json" \
  > "$S/$rotulo-simulador.log" 2>&1 &
PID=$!
ev "carga iniciada"
sleep 15; docker stats --no-stream --format '{{.Name}} {{.CPUPerc}}' | grep -E 'tcc-a-(api|caddy|db)' | tee -a "$S/$rotulo-eventos.txt"
sleep 15; ev "parando tcc-a-api-2"; docker stop tcc-a-api-2 >/dev/null; ev "tcc-a-api-2 parada"
sleep 30; ev "religando tcc-a-api-2"; docker start tcc-a-api-2 >/dev/null; ev "tcc-a-api-2 religada"
sleep 12; docker stats --no-stream --format '{{.Name}} {{.CPUPerc}}' | grep -E 'tcc-a-(api|caddy|db)' | tee -a "$S/$rotulo-eventos.txt"
wait $PID; ev "carga terminou (exit $?)"
