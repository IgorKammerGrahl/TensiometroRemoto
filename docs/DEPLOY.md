# Deploy na VM

Da VM Ubuntu limpa ao sistema no ar em `https://DOMINIO`. Sobem cinco
serviços (`deploy/compose.yaml`): Postgres, `migrate` (aplica as migrations e
sai), API, Caddy (HTTPS automático pelo Let's Encrypt) e `backup` (dump
diário, 14 dias de retenção).

Os comandos `docker compose` rodam de dentro de `deploy/`.

## Antes da VM

- **Arquive o banco de bancada.** A VM começa vazia e com segredo novo. No
  notebook: `cd Backend && make dump ENGINE=docker`, e guarde o
  `dump-AAAA-MM-DD.sql` fora do repositório (Drive, pendrive). Ele tem
  e-mails e hashes de senha e de token: é privado.
- **Anote a calibração do nó de bancada**, que será recadastrada na VM:
  `docker exec tcc-pg psql -U tcc telemetria -c 'select * from calibrations'`.
- **DNS:** registro `A` de `DOMINIO` para o IP público da VM.
- **Rede do IFC:** portas **80 e 443 TCP** abertas da internet até a VM, e a
  VM alcançando a internet (Docker Hub, GitHub, Let's Encrypt).

## 1. Docker

```sh
curl -fsSL https://get.docker.com | sudo sh
sudo docker compose version
```

## 2. Código e configuração

```sh
git clone https://github.com/IgorKammerGrahl/TensiometroRemoto.git
cd TensiometroRemoto/deploy
cp .env.example .env && chmod 600 .env
openssl rand -hex 24      # -> POSTGRES_PASSWORD
openssl rand -base64 32   # -> TELEMETRIA_TOKEN_SECRET
nano .env                 # cole os dois e preencha DOMINIO
```

Guarde uma cópia do `.env` fora da VM (gerenciador de senhas). Sem o
`TELEMETRIA_TOKEN_SECRET`, nenhum token de nó volta a valer.

## 3. Subir

```sh
sudo docker volume create tensiometro-pgdados
sudo docker volume create tensiometro-backups
sudo docker compose up -d --build
sudo docker compose ps
```

Esperado: `db`, `api`, `caddy` e `backup` como `healthy`; `migrate` como
`Exited (0)`.

Os dois volumes são **externos** de propósito: `docker compose down -v` não
os apaga. Sem eles o `up` recusa subir (`external volume ... not found`).

## 4. Conferir o HTTPS e a cadeia

```sh
curl -sI https://DOMINIO/ | head -1        # HTTP/2 200
sudo docker compose logs caddy | grep -E 'certificate obtained|error'
openssl s_client -connect DOMINIO:443 -servername DOMINIO -showcerts </dev/null 2>/dev/null \
  | grep -E '^ *[0-9] s:|^ *i:|Verify return code'
```

A cadeia precisa terminar numa das quatro raízes do `CA_RAIZ_PEM` do
firmware: **ISRG Root X1**, **ISRG Root X2**, **Root YR** ou **Root YE**. Em
novembro de 2026, um certificado ECDSA (o padrão do Caddy) vinha assim:

```
0 s:CN=DOMINIO       i:CN=YE1
1 s:CN=YE1           i:CN=Root YE
2 s:CN=Root YE       i:CN=ISRG Root X2
3 s:CN=ISRG Root X2  i:CN=ISRG Root X1
```

Emissor fora dessas quatro e o ESP32 recusa a conexão. Não afrouxe o
firmware: veja o que mudou na cadeia do Let's Encrypt e acrescente a raiz
nova ao `CA_RAIZ_PEM`.

## 5. Cadastro inicial

A senha e o token aparecem **uma vez cada**.

```sh
sudo docker compose exec api admin usuario  -email=produtor@exemplo.com -nome="Produtor"
sudo docker compose exec api admin talhao   -nome="Talhão de Bancada"      # imprime o uuid
sudo docker compose exec api admin conceder -email=produtor@exemplo.com -talhao=<uuid>
sudo docker compose exec api admin device   -id=tensio-01 -descricao="nó de bancada"
sudo docker compose exec api admin associar -device=tensio-01 -talhao=<uuid>
sudo docker compose exec api admin calibracao -id=<id> -device=tensio-01 \
  -v-zero=<V> -k=<V/kPa> -divisor=<fator> -vdd=<mV>
```

Na calibração, use os coeficientes anotados antes da VM. Com `-id=` igual ao
id já gravado no nó, o campo de calibração do portal não muda. As faixas do
talhão se configuram pelo app.

## 6. Teste com curl

Lote vazio: passa por TLS, Caddy, API e conferência do token sem gravar nada.

```sh
curl -sS https://DOMINIO/api/v1/readings \
  -H "Authorization: Bearer <token>" -H 'Content-Type: application/json' \
  -d '{"device_id":"tensio-01","readings":[]}'
# {"accepted":0,"duplicates":0,"rejected":0,...}     401 = token errado
```

Depois, abra `https://DOMINIO/` no celular e entre com o usuário do passo 5.

## 7. ESP32 de bancada contra a VM, antes de qualquer nó em campo

1. No `Circuito/sketch/config.h`: comente `#define DEV_SEM_TLS` e troque o
   bloco `CA_RAIZ_PEM` pelo do `config.h.example` (as quatro raízes).
   `MODO_CALIBRACAO` continua comentado.
2. Compile e grave:
   ```sh
   arduino-cli compile --fqbn esp32:esp32:esp32doit-devkit-v1 Circuito/sketch
   arduino-cli upload  --fqbn esp32:esp32:esp32doit-devkit-v1 -p /dev/ttyUSB0 Circuito/sketch
   ```
3. Abra o portal (solte o RESET e só **depois** segure o BOOT por ~3 s),
   conecte-se a `tensio-setup-XXXX` (senha `tensiometro`) e grave: URL
   `https://DOMINIO/api/v1/readings`, `tensio-01`, o token do passo 5 e a
   calibração. Mudar a URL exige colar o token de novo.
4. No monitor serial (`arduino-cli monitor -p /dev/ttyUSB0 -c baudrate=115200`):
   ```
   # NTP sincronizado. Agora (UTC): ...
   # HTTP 200  accepted=1 duplicates=0 rejected=0
   ```
   Código negativo no POST é conexão ou TLS: confira a cadeia (passo 4).
   `rejected=1` com `calibration_id` é cadastro (passo 5).
5. Com a leitura aparecendo no app, reprovisione os nós de campo, um token
   novo para cada (`admin device` ou a tela "Novo nó" do app).

## 8. Backup

`backup` grava `telemetria-AAAA-MM-DDTHHMMSSZ.dump` uma vez por dia (e a cada
vez que o container sobe) e apaga os de 14 dias ou mais, só depois de um dump
bem-sucedido. Fica `unhealthy` quando o mais novo passa de 26 h.

```sh
sudo docker compose ps backup
sudo docker compose exec backup ls -lh /backups
sudo docker compose restart backup        # dump extra agora
```

### Cópia semanal para fora da VM

O volume está no disco da VM: cobre um `DROP` ou um banco corrompido, não a
perda da VM. E um problema que passe 14 dias sem ser notado já não tem backup
limpo nela. Toda semana:

```sh
# na VM
sudo docker compose cp backup:/backups /tmp/backups-copia && sudo chown -R "$USER" /tmp/backups-copia
# no notebook
scp -r usuario@DOMINIO:/tmp/backups-copia ./backups-$(date +%F)
# na VM
rm -r /tmp/backups-copia
```

### Restaurar um backup

```sh
sudo docker compose exec backup ls -lh /backups     # escolha o arquivo
sudo docker compose stop api caddy                  # nada grava durante a restauração
sudo docker compose exec backup pg_restore --clean --if-exists --no-owner \
  --single-transaction -d telemetria /backups/telemetria-AAAA-MM-DDTHHMMSSZ.dump
sudo docker compose start api caddy
```

Com `--single-transaction`, uma restauração que falha deixa o banco como
estava. O que chegou depois do dump se perde: o nó não guarda leitura para
reenviar.

Para restaurar uma cópia do notebook, leve-a antes para o volume: `scp` para
a VM e `sudo docker compose cp arquivo.dump backup:/backups/`.

Ensaio sem tocar no banco, que vale fazer depois da primeira cópia semanal:

```sh
sudo docker compose exec backup createdb restore_teste
sudo docker compose exec backup pg_restore --no-owner -d restore_teste /backups/<arquivo>.dump
sudo docker compose exec backup psql -d restore_teste -c 'select count(*) from readings'
sudo docker compose exec backup dropdb restore_teste
```

## 9. Se o certificado não sair

`sudo docker compose logs caddy | grep -iE 'error|challenge'`, e pela mensagem:

| No log | Causa provável | O que conferir |
|---|---|---|
| `Timeout during connect`, `connection refused` | porta 80/443 fechada de fora | firewall do IFC ou NAT. De fora da rede (celular no 4G), `curl -I http://DOMINIO/` precisa responder `308` |
| `NXDOMAIN`, `no valid A records` | DNS | `dig +short DOMINIO` igual ao IP público da VM (`curl -4 ifconfig.me` na VM) |
| `rateLimited`, `too many failed authorizations` | limite do Let's Encrypt depois de falhas | corrija a causa e espere 1 h |
| nenhuma tentativa | Caddy não alcança o Let's Encrypt | saída da VM para `acme-v02.api.letsencrypt.org:443` |

- O Caddy valida pela porta 80 (HTTP-01) ou pela 443 (TLS-ALPN-01); basta uma,
  mas a 80 também faz o redirecionamento para HTTPS. Com as duas bloqueadas, a
  saída é validação por DNS (Caddy com plugin do provedor): fale com o IFC
  antes.
- O `ufw` normalmente não é o bloqueio: o Docker publica as portas com regras
  próprias no iptables, antes das do `ufw`. Porta fechada de fora com o
  container de pé aponta para antes da VM.
- O emissor está fixo no Let's Encrypt (`cert_issuer acme` no `Caddyfile`):
  sem ele, o Caddy poderia cair em outra CA, e o ESP32 recusaria.
- Depois de corrigir, `sudo docker compose restart caddy` tenta na hora.
- `up` falhando com `Pool overlaps`: a faixa `10.250.250.0/28` da rede `borda`
  colide com a da VM. Troque `REDE_OCTETO` no `.env` (250 → outro valor de 0
  a 255): a faixa, o IP do Caddy e o `PROXY_CONFIAVEL` mudam juntos.

## Atualizar

```sh
git pull
sudo docker compose restart backup     # dump antes de mexer
sudo docker compose up -d --build      # migrate roda de novo; o goose só aplica o que falta
```

## Réplicas da API

A API não guarda estado entre requisições: sessão, leituras e idempotência
estão no banco. Mais de uma cópia atrás do Caddy serve o mesmo sistema, e a
parada de uma delas não derruba o serviço.

```sh
# no .env: API_REPLICAS=3   (até 7: é o que cabe na faixa .8–.14 da rede borda)
sudo docker compose up -d
sudo docker compose ps api           # api-1, api-2, api-3 healthy
```

O Caddy acha as réplicas pelo DNS do Docker a cada 5 s (`dynamic a` no
`Caddyfile`) e reparte em rodízio. Réplica que sai do ar: a conexão recusada
é tentada em outra por até 5 s, o que é seguro para o `POST /readings`, porque
o Caddy só repete o que não chegou a ser enviado. Ao receber `SIGTERM`, a API
fecha a porta e termina o que já entrou (até 8 s) antes de sair.

Duas coisas mudam com N > 1:

- **Login:** vai sempre para a mesma réplica por IP (hash), para o limite de
  tentativas por IP continuar valendo. O limite por e-mail, contra ataque de
  muitos IPs, fica N vezes mais folgado. Ver `Backend/README.md`, "Limite de
  tentativas no login".
- **`exec`:** `docker compose exec api ...` cai na réplica 1. Para o `admin`
  tanto faz, porque ele fala com o banco.

Conferido em 16/11/2026, modo local, 3 réplicas (registro em
`Registros/replicas-2026-11-16/`): 200 nós simulados a 1 envio/s por HTTPS,
uma réplica parada e religada no meio da carga, 18 000 envios e nenhuma perda;
sessão válida nas três; a mesma leitura em 6 POSTs simultâneos gravada uma
vez. **Não testado:** atualização sem interrupção (`up -d --build` recria as
réplicas e pode cortar alguns segundos).

## Mais de uma instalação na mesma máquina

Cada instalação (uma propriedade, uma cooperativa) tem banco, segredo,
usuários e nós próprios. O natural é **uma por VM**, com o próprio domínio:
basta repetir este guia. Na mesma máquina, só uma pode ocupar as portas 80 e
443, que o Let's Encrypt exige; a segunda serve para teste em modo local ou
precisa de um proxy na frente das duas, que este compose não traz.

Para pôr uma segunda ao lado da primeira, um `.env` próprio com quatro
diferenças, além de segredos novos:

```sh
cp .env .env.fazenda-b && chmod 600 .env.fazenda-b
# no .env.fazenda-b:
#   INSTANCIA=fazenda-b        -> projeto e volumes fazenda-b-*
#   REDE_OCTETO=251            -> rede 10.250.251.0/28
#   PORTA_HTTP / PORTA_HTTPS   -> outras portas
#   POSTGRES_PASSWORD e TELEMETRIA_TOKEN_SECRET novos (passo 2)
docker volume create fazenda-b-pgdados
docker volume create fazenda-b-backups
docker compose --env-file .env.fazenda-b -f compose.yaml -f compose.local.yaml up -d
```

Todo comando dessa instalação leva o mesmo `--env-file`; sem ele, o compose
fala com a primeira.

Conferido em 16/11/2026 com duas instalações lado a lado: o token de um nó
da primeira e o login de um usuário da primeira são recusados (`401`) pela
segunda, cada uma tem a própria CA local, e 120 leituras enviadas à segunda
não alteraram a contagem da primeira.

## Modo local (sem domínio)

O mesmo sistema na própria máquina, com a CA interna do Caddy
(`compose.local.yaml`). Testado com Docker. Com Podman rootless (não testado)
as portas precisam ser `PORTA_HTTP=8080` e `PORTA_HTTPS=8443`: ele não abre
porta abaixo de 1024.

```sh
cd deploy
cp .env.example .env      # DOMINIO=localhost, portas 8080/8443, segredos como no passo 2
docker volume create tensiometro-pgdados
docker volume create tensiometro-backups
docker compose -f compose.yaml -f compose.local.yaml up -d --build
docker compose cp caddy:/data/caddy/pki/authorities/local/root.crt /tmp/caddy-local.crt
curl --cacert /tmp/caddy-local.crt https://localhost:8443/      # 200
```

Abra `https://localhost:8443` direto (o redirecionamento de HTTP aponta para a
porta 443). O navegador avisa que a CA é desconhecida: aceite, ou importe
`/tmp/caddy-local.crt`. O login funciona porque `localhost` é HTTPS de verdade
e o cookie `Secure` passa. Cadastro e curl como nos passos 5 e 6, sem `sudo`.

O ESP32 não entra neste modo: na bancada ele segue com `DEV_SEM_TLS` contra o
`make dev`.

Desligar: `docker compose down`. Os volumes externos ficam;
`docker volume rm tensiometro-pgdados tensiometro-backups` apaga o banco local.
