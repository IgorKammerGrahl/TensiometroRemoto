package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
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

	log.Info("escutando", "porta", porta)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("servidor encerrou", "erro", err)
		os.Exit(1)
	}
}
