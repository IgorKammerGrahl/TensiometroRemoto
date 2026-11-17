#!/usr/bin/env bash
# T2 sessao entre replicas, T3 idempotencia entre replicas, T4 limite de login.
set -uo pipefail
S=$(cd "$(dirname "$0")" && pwd); cd "$S"
U=https://localhost:8444; C=(--cacert tcc-a-root.crt -s)
SENHA=$(awk '/^senha/ {print $3}' tcc-a-usuario.txt)

echo "== T2: login uma vez, 30 x GET /me (rodizio entre 3 replicas)"
rm -f jar.txt
curl "${C[@]}" -c jar.txt -o /dev/null -w 'login %{http_code}\n' -H 'Content-Type: application/json' \
  -d "{\"email\":\"teste@tcc.local\",\"senha\":\"$SENHA\"}" $U/api/v1/app/login
for i in $(seq 30); do curl "${C[@]}" -b jar.txt -o /dev/null -w '%{http_code}\n' $U/api/v1/app/me; done | sort | uniq -c

echo "== T3: a mesma leitura (seq novo) em 6 POSTs simultaneos"
read -r ID TOKEN CAL < <(python3 -c "
import json; n=json.load(open('t1-estado.json'))['nos'][0]; print(n['id'], n['token'], n['calibracao_id'])")
AGORA=$(date -u +%Y-%m-%dT%H:%M:%SZ)
CORPO="{\"device_id\":\"$ID\",\"readings\":[{\"seq\":100000,\"measured_at\":\"$AGORA\",\"raw_mv\":2500,\"vdd_mv\":5000,\"kpa\":-18.75,\"calibration_id\":\"$CAL\"}]}"
for i in $(seq 6); do
  curl "${C[@]}" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "$CORPO" $U/api/v1/readings > t3-$i.json &
done; wait
for i in $(seq 6); do cat t3-$i.json; echo; done
./dc a exec -T db psql -U tcc telemetria -tAc "select count(*) from readings where device_id='$ID' and seq=100000"

echo "== T4: 12 logins errados, e-mails diferentes, mesmo IP"
for i in $(seq 12); do
  curl "${C[@]}" -o /dev/null -w '%{http_code} ' -H 'Content-Type: application/json' \
    -d "{\"email\":\"naoexiste$i@tcc.local\",\"senha\":\"errada\"}" $U/api/v1/app/login
done; echo
