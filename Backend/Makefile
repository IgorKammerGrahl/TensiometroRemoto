# Atalhos do ciclo de desenvolvimento.
#
# ENGINE escolhe quem sobe o container. O padrao e podman (rootless: sem
# daemon e sem seu usuario num grupo equivalente a root); `make dev
# ENGINE=docker` roda identico, porque `compose.yaml` e a mesma
# especificacao para os dois.
ENGINE  ?= podman
COMPOSE := $(ENGINE) compose

# DATABASE_URL e TELEMETRIA_TOKEN_SECRET NAO sao definidos aqui de proposito:
# quem os fornece e o .env, que nao e versionado. Definir aqui sobrescreveria
# o arquivo (variavel de ambiente tem precedencia) e esconderia a
# configuracao real do dono da maquina.
TEST_DATABASE_URL ?= postgres://tcc:tcc@localhost:55432/telemetria_test?sslmode=disable

.PHONY: dev up down logs migrate seed psql test dump restore

## dev: sobe o banco, aplica migrations e roda a API em primeiro plano
dev: up migrate
	go run ./cmd/api

## up: sobe o Postgres e espera ele aceitar conexao
up: .env
	$(COMPOSE) up -d
	@printf '# aguardando o Postgres'
	@for i in $$(seq 1 60); do \
	  if $(ENGINE) exec tcc-pg pg_isready -U tcc -d telemetria >/dev/null 2>&1; then \
	    echo ' pronto.'; exit 0; \
	  fi; \
	  printf '.'; sleep 1; \
	done; \
	echo ' TIMEOUT. Veja `make logs`.'; exit 1

## down: para o Postgres. O volume PERMANECE -- nao ha alvo que o apague.
down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f db

migrate:
	go run ./cmd/api -migrate

## seed: serie sintetica para a interface ter o que mostrar (nao toca no tensio-01)
##       `make seed` so IMPRIME o banco alvo e para; `make seed ARGS=-confirmo` grava.
##       O alvo vem do .env, que nao aparece na linha de comando -- ver cmd/seed.
seed:
	go run ./cmd/seed $(ARGS)

psql:
	$(ENGINE) exec -it tcc-pg psql -U tcc -d telemetria

## test: a suite pula os testes de integracao sem TEST_DATABASE_URL
test:
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test ./...

## dump: backup datado. As leituras de bancada so existem no volume.
dump:
	$(ENGINE) exec tcc-pg pg_dump -U tcc --clean --if-exists telemetria > dump-$$(date +%F).sql
	@ls -l dump-$$(date +%F).sql

## restore: make restore FILE=dump-AAAA-MM-DD.sql
restore:
	@test -n "$(FILE)" || { echo 'uso: make restore FILE=dump-AAAA-MM-DD.sql'; exit 1; }
	$(ENGINE) exec -i tcc-pg psql -U tcc -d telemetria < $(FILE)

# Nao ha regra que gere o .env: ele tem o segredo do HMAC e a credencial do
# banco. Criar um automaticamente daria um segredo em branco funcionando por
# acidente, que e pior do que falhar aqui.
.env:
	@echo 'ERRO: falta o .env. Copie de .env.example e preencha'
	@echo '      TELEMETRIA_TOKEN_SECRET antes de continuar.'
	@exit 1
