# Réplicas da API e instalações lado a lado — 16/11/2026

Item 2 do plano pós-orientação de 12/11: várias instâncias do servidor, nos
dois sentidos.

- **Réplicas (escala horizontal):** três cópias da API atrás do Caddy, no
  mesmo banco (`API_REPLICAS=3`).
- **Instalações lado a lado:** duas instalações completas e independentes na
  mesma máquina (`INSTANCIA`, `REDE_OCTETO`, portas).

Tudo em modo local (`compose.local.yaml`, CA interna do Caddy), cliente e
servidor na mesma máquina (`maquina.txt`). **Todo dado aqui é simulado.**

## Montagem

Duas instalações de teste, cada uma com o seu `.env` (fora do git, com
segredos novos gerados por `openssl rand`):

| | `tcc-a` | `tcc-b` |
|---|---|---|
| `API_REPLICAS` | 3 | 1 |
| `REDE_OCTETO` | 251 (10.250.251.0/28) | 252 (10.250.252.0/28) |
| Portas HTTP/HTTPS | 8081/8444 | 8082/8445 |
| Usuário | `teste@tcc.local`, 2 talhões | `outra@tcc.local`, 1 talhão |

`dc` é um atalho para `docker compose --env-file tcc-<a|b>.env -f compose.yaml
-f compose.local.yaml`. Os roteiros esperam ao lado deles os `.env`, o
`root.crt` de cada Caddy (`dc a exec caddy cat
/data/caddy/pki/authorities/local/root.crt`), a saída do `admin usuario` e o
binário do `cmd/simulador`. O simulador confia na CA local por
`SSL_CERT_FILE`.

## T1 — carga com uma réplica parada no meio (`t1-parada.sh`)

200 nós simulados, 1 envio por segundo cada (200 req/s), 90 s, **por HTTPS e
com conexão nova a cada envio**, como o firmware. Aos 30 s, `docker stop
tcc-a-api-2`; aos 60 s, `docker start`. Eventos em `t1-eventos.txt`,
relatório em `t1-relatorio.json`.

| Envios | Perdas | p50 | p90 | p99 | Máx. |
|---|---|---|---|---|---|
| 18 000 | 0 | 7,3 ms | 10,6 ms | 16,8 ms | 257,5 ms |

- Todas as 18 000 com `200` e aceitas; nenhum erro de transporte.
- A réplica 2 recebeu o sinal e saiu limpa
  (`sinal recebido` → `encerrado sem requisicoes pendentes`, em 0,6 s).
- CPU aos 15 s: api-1 4,0 %, api-2 3,5 %, api-3 3,9 %, banco 19 %, **Caddy
  47 %**. Depois de religada, a api-2 voltou ao rodízio (5,1 %, junto das
  outras duas). O componente mais caro é o Caddy: um aperto de mão TLS por
  envio.
- O atraso do cliente sobre a agenda ficou em até 4,7 ms: o cliente não foi o
  gargalo.

O ensaio **não isola** a contribuição do encerramento gracioso: com ~7 ms
por requisição e ~0,5 requisição em voo por réplica, um corte abrupto
perderia de zero a poucas leituras, e uma execução não separa isso do acaso.
O encerramento sem corte é garantido pela construção (`http.Server.Shutdown`)
e confirmado pelo log.

## T2 — sessão entre réplicas (`t2-t4.sh`)

Um login, depois 30 × `GET /api/v1/app/me` em rodízio pelas três réplicas:

```
login 204
     30 200
```

## T3 — idempotência entre réplicas (`t2-t4.sh`)

A mesma leitura (mesmo nó, `seq` 100000, nunca usado) em 6 POSTs simultâneos:
1 `accepted`, 5 `duplicates`, e **uma** linha no banco
(`select count(*) ... where seq=100000` → `1`). A unicidade de
`(device_id, seq)` está no banco, então vale com qualquer número de réplicas.

## T4 — limite de login por IP com réplicas (`t2-t4.sh`)

12 logins errados, e-mails diferentes (para não cair no limite por e-mail),
mesmo IP de origem:

```
401 401 401 401 401 401 401 401 401 429 429 429
```

O `429` veio na 11ª tentativa contando o login certo do T2: rajada de 10, a
de **uma** réplica. Em rodízio puro seriam ~30 (10 por réplica). É o hash do
IP no `Caddyfile` que segura. O limite por e-mail contra muitos IPs fica N
vezes mais folgado; ver `Backend/README.md`.

## T5 — isolamento entre as duas instalações (`t5-isolamento.sh`)

```
token de no da A enviado a B:   401
usuario da A fazendo login em B: 401
usuario da B fazendo login em B: 204
certificado da A aceito por B?   000 (recusado: cada instalacao tem a propria CA local)
== carga na B (10 nos, 5 s, 60 s), A intocada
leituras    : 120 aceitas, 0 duplicadas, 0 rejeitadas, 0 perdidas
latencia ms : p50 9.1   p90 11.3   p99 29.9   max 482.7   (n=120)
leituras na A antes/depois da carga na B: 18401 / 18401
leituras na B: 120
```

As 18 401 da A são 18 000 do T1, 400 do cadastro (2 s a 1 envio/s antes do
T1) e 1 do T3.

## O que este registro não cobre

- Atualização sem interrupção: `up -d --build` recria as réplicas; não foi
  medido se corta.
- Réplicas em máquinas diferentes: as três dividem a CPU do mesmo host, então
  o ensaio mostra correção e tolerância à parada, não ganho de vazão.
- O nó físico: o ESP32 não fala com a CA local do Caddy (ver `docs/DEPLOY.md`).
