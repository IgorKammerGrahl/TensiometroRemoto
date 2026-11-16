package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/igorkg/tcc/backend/internal/config"
	"github.com/igorkg/tcc/backend/internal/telemetria"
	"github.com/igorkg/tcc/backend/internal/webapp"
	"github.com/igorkg/tcc/backend/migrations"
)

func main() {
	migrar := flag.Bool("migrate", false, "aplica as migrations e encerra")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	config.Carregar()

	url := config.Obrigatoria("DATABASE_URL",
		"e a string de conexao com o Postgres: sem ela nao ha onde ler nem\n  gravar leitura nenhuma.")

	if *migrar {
		if err := migrations.Aplicar(url); err != nil {
			log.Error("migrations falharam", "erro", err)
			os.Exit(1)
		}
		log.Info("migrations aplicadas")
		return
	}

	// Migration no boot fica fora do padrao: com mais de uma replica subindo
	// junto, duas instancias correriam o goose ao mesmo tempo. Em producao
	// use `api -migrate` como passo separado do deploy.
	if os.Getenv("RUN_MIGRATIONS") == "true" {
		if err := migrations.Aplicar(url); err != nil {
			log.Error("migrations falharam", "erro", err)
			os.Exit(1)
		}
		log.Warn("migrations aplicadas no boot (RUN_MIGRATIONS=true): nao use com multiplas replicas")
	}

	// Falha barulhenta: subir sem segredo geraria hashes que nenhum token
	// existente casa, e o cadastro de devices viraria lixo silencioso.
	segredo := config.SegredoObrigatorio("TELEMETRIA_TOKEN_SECRET",
		"e a chave do HMAC que autentica os nos: sem ela o servidor nao consegue\n  conferir token nenhum, e todo POST /readings seria recusado.")

	ctx := context.Background()
	pool, err := telemetria.NovoPool(ctx, url)
	if err != nil {
		log.Error("conexao com o banco falhou", "erro", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Error("ping no banco falhou", "erro", err)
		os.Exit(1)
	}

	porta := os.Getenv("PORT")
	if porta == "" {
		porta = "8080"
	}

	api := telemetria.NewAPI(telemetria.NewStore(pool), segredo, log)
	// Ver PermitirCookieInseguro: so para demonstracao em rede local, onde a
	// origem http://192.168.x.x faz o navegador descartar o cookie Secure.
	if os.Getenv("SESSAO_COOKIE_INSEGURO") == "true" {
		api.PermitirCookieInseguro()
	}
	// Ver ConfiarNoProxy. Valor invalido derruba o boot: subir ignorando-o
	// deixaria o login atras do Caddy com um balde so para todo mundo, calado.
	if v := os.Getenv("PROXY_CONFIAVEL"); v != "" {
		ip, err := netip.ParseAddr(v)
		if err != nil {
			log.Error("PROXY_CONFIAVEL invalido: precisa ser um IP so, sem faixa nem porta", "valor", v, "erro", err)
			os.Exit(1)
		}
		api.ConfiarNoProxy(ip)
	}

	srv := &http.Server{
		Addr: ":" + porta,
		// Um binario so: /api/ e a API, o resto e a PWA embarcada. Ver
		// webapp.Servir -- e o roteamento que impede endpoint errado de
		// responder 200 com index.html.
		Handler:           webapp.Servir(api.Router()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	// ENCERRAMENTO SEM CORTAR LEITURA.
	//
	// O no nao retransmite: uma leitura cuja requisicao e cortada no meio esta
	// perdida. Com replicas atras do Caddy (deploy/compose.yaml), parar uma
	// delas -- deploy novo, `docker compose up --scale`, reinicio da VM --
	// precisa deixar terminar o que ja entrou. Shutdown fecha o listener
	// primeiro, entao conexao nova e recusada e o Caddy tenta outra replica
	// (lb_try_duration no Caddyfile); as requisicoes em andamento terminam.
	//
	// 8 s, e nao mais: o Docker manda SIGKILL 10 s depois do SIGTERM. Um
	// POST /readings leva milissegundos; o prazo so estoura com o banco
	// travado, e ai nao ha o que esperar.
	ctxSinal, pararSinal := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer pararSinal()

	falhou := make(chan error, 1)
	go func() {
		log.Info("escutando", "porta", porta)
		falhou <- srv.ListenAndServe()
	}()

	select {
	case err := <-falhou:
		// Antes de Shutdown, ListenAndServe so volta com erro (porta ocupada).
		log.Error("servidor encerrou", "erro", err)
		os.Exit(1)
	case <-ctxSinal.Done():
	}
	// Um segundo Ctrl-C mata na hora, como sem o NotifyContext.
	pararSinal()

	log.Info("sinal recebido: esperando as requisicoes em andamento")
	ctxFim, cancelar := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancelar()
	if err := srv.Shutdown(ctxFim); err != nil {
		log.Error("prazo de encerramento estourou com requisicoes abertas", "erro", err)
		return
	}
	log.Info("encerrado sem requisicoes pendentes")
}
