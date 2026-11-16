#!/usr/bin/env bash
# T5: duas instalacoes lado a lado nao enxergam uma a outra.
set -uo pipefail
S=$(cd "$(dirname "$0")" && pwd); cd "$S"
A=https://localhost:8444; B=https://localhost:8445
SA=$(awk '/^senha/ {print $3}' tcc-a-usuario.txt); SB=$(awk '/^senha/ {print $3}' tcc-b-usuario.txt)
read -r ID TOKEN CAL < <(python3 -c "
import json; n=json.load(open('t1-estado.json'))['nos'][1]; print(n['id'], n['token'], n['calibracao_id'])")
AGORA=$(date -u +%Y-%m-%dT%H:%M:%SZ)
CORPO="{\"device_id\":\"$ID\",\"readings\":[{\"seq\":100001,\"measured_at\":\"$AGORA\",\"raw_mv\":2500,\"vdd_mv\":5000,\"kpa\":-18.75,\"calibration_id\":\"$CAL\"}]}"
echo "token de no da A enviado a B:   $(curl -s --cacert tcc-b-root.crt -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "$CORPO" $B/api/v1/readings)"
echo "usuario da A fazendo login em B: $(curl -s --cacert tcc-b-root.crt -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' -d "{\"email\":\"teste@tcc.local\",\"senha\":\"$SA\"}" $B/api/v1/app/login)"
echo "usuario da B fazendo login em B: $(curl -s --cacert tcc-b-root.crt -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' -d "{\"email\":\"outra@tcc.local\",\"senha\":\"$SB\"}" $B/api/v1/app/login)"
echo "certificado da A aceito por B?   $(curl -s --cacert tcc-a-root.crt -o /dev/null -w '%{http_code}' $B/ || echo ' (recusado: cada instalacao tem a propria CA local)')"
echo "== carga na B (10 nos, 5 s, 60 s), A intocada"
antes=$(./dc a exec -T db psql -U tcc telemetria -tAc "select count(*) from readings")
SSL_CERT_FILE=tcc-b-root.crt SIMULADOR_SENHA=$SB ./simulador -url=$B -email=outra@tcc.local \
  -talhao="$(cat tcc-b-talhoes.txt)" -estado="$S/t5-estado.json" -nos=10 -intervalo=5s -duracao=60s \
  -semente=20261116 -relatorio="$S/t5-relatorio.json" 2>&1 | grep -E "^latencia|perdid|leituras" 
depois=$(./dc a exec -T db psql -U tcc telemetria -tAc "select count(*) from readings")
echo "leituras na A antes/depois da carga na B: $antes / $depois"
echo "leituras na B: $(./dc b exec -T db psql -U tcc telemetria -tAc "select count(*) from readings")"
python3 -c "import json;r=json.load(open('t5-relatorio.json'));print('B:', r['envios'],'envios', r['leituras'])"
