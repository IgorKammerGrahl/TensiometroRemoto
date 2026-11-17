// simulador faz N nos sensores enviarem leituras pela API real, como o
// firmware envia, e mede o servidor enquanto isso.
//
// Tudo passa pela API, inclusive o cadastro: login, POST /app/devices (com
// simulado=true), POST de calibracao e, por fim, POST /readings com o token
// de cada no. Ao contrario do cmd/seed, que grava direto no banco, aqui o
// que se exercita e o caminho que um no e um produtor usam.
//
// O DADO GERADO AQUI E SIMULADO E FICA MARCADO COMO TAL: os nos nascem com
// devices.simulado = true, e a tela mostra a marca. Ver
// migrations/0004_simulado.sql e simulador.Verificar.
//
// Uso tipico (a senha vem do ambiente, nunca de flag -- ver admin usuario):
//
//	SIMULADOR_SENHA=... go run ./cmd/simulador -email=carga@tcc.local \
//	    -talhao=<uuid> -nos=100 -duracao=5m -relatorio=carga-100.json
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/igorkg/tcc/backend/internal/simulador"
)

func main() {
	base := flag.String("url", "http://localhost:8080", "URL base da API")
	email := flag.String("email", "", "e-mail do usuario dono dos talhoes (senha em SIMULADOR_SENHA)")
	nos := flag.Int("nos", 0, "quantos nos enviam nesta execucao")
	talhoes := flag.String("talhao", "", "uuid(s) de talhao, separados por virgula, para os nos que ainda nao existem")
	prefixo := flag.String("prefixo", "sim", "prefixo do id dos nos cadastrados (sim-0001, sim-0002, ...)")
	intervalo := flag.Duration("intervalo", 60*time.Second, "intervalo entre envios de cada no (firmware: 60s)")
	duracao := flag.Duration("duracao", 5*time.Minute, "duracao da execucao")
	timeout := flag.Duration("timeout", 10*time.Second, "timeout de cada envio (firmware: 10s)")
	reusar := flag.Bool("reusar-conexao", false, "mantem a conexao entre envios (o firmware NAO mantem)")
	caminhoEstado := flag.String("estado", "simulador-estado.json", "arquivo com tokens e seq dos nos (0600, fora do git)")
	caminhoRelatorio := flag.String("relatorio", "", "grava o relatorio em JSON neste arquivo")
	semente := flag.Uint64("semente", uint64(time.Now().UnixNano()), "semente do sinal (repita para reproduzir)")
	flag.Parse()

	senha := os.Getenv("SIMULADOR_SENHA")
	switch {
	case *email == "" || senha == "":
		fatal("informe -email e a senha em SIMULADOR_SENHA (senha em flag fica no historico do shell)")
	case *nos <= 0:
		fatal("-nos precisa ser maior que zero")
	case *intervalo <= 0 || *duracao < *intervalo:
		fatal("-intervalo precisa ser positivo e -duracao, ao menos um intervalo")
	case *timeout <= 0:
		fatal("-timeout precisa ser positivo")
	}
	*base = strings.TrimRight(*base, "/")

	estado, err := simulador.CarregarEstado(*caminhoEstado)
	if err != nil {
		fatal(err.Error())
	}
	if estado.URL == "" {
		estado.URL = *base
	}
	if estado.URL != *base {
		fatal(fmt.Sprintf("o estado %s e da instancia %s, nao de %s: token de uma instancia nao "+
			"autentica em outra. Use outro -estado", *caminhoEstado, estado.URL, *base))
	}
	salvar := func() error { return estado.Salvar(*caminhoEstado) }

	// Quem esta do outro lado aparece antes de qualquer escrita: os nos e as
	// leituras vao para o banco DAQUELE servidor.
	fmt.Printf("alvo     : %s (usuario %s)\n", *base, *email)

	ctx, cancelar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelar()

	cli := simulador.NovoCliente(*base)
	if err := cli.Entrar(ctx, *email, senha); err != nil {
		fatal(err.Error())
	}
	defer func() {
		if err := cli.Sair(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, "aviso: logout falhou:", err)
		}
	}()

	var listaTalhoes []string
	for t := range strings.SplitSeq(*talhoes, ",") {
		if t = strings.TrimSpace(t); t != "" {
			listaTalhoes = append(listaTalhoes, t)
		}
	}
	inicioCadastro := time.Now()
	criados, err := simulador.Provisionar(ctx, cli, estado, salvar, *nos, *prefixo, listaTalhoes)
	if err != nil {
		sair(cli, err.Error())
	}
	if criados > 0 {
		fmt.Printf("cadastro : %d no(s) novo(s) em %s\n", criados, time.Since(inicioCadastro).Round(time.Millisecond))
	}

	ativos := estado.Nos[:*nos]
	if err := simulador.Verificar(ctx, cli, ativos); err != nil {
		sair(cli, err.Error())
	}

	// Reserva gravada ANTES do primeiro envio. Ver Estado.Reservar.
	inicios := estado.Reservar(*nos, simulador.EnviosPorNo(*intervalo, *duracao))
	if err := salvar(); err != nil {
		sair(cli, "reserva de seq nao gravada; nada foi enviado: "+err.Error())
	}

	solos := make([]*simulador.Solo, *nos)
	agora := time.Now()
	for i := range solos {
		// Um gerador por no: cada goroutine mexe so no seu, e a serie de
		// cada no fica reproduzivel pela semente e pela posicao.
		solos[i] = simulador.NovoSolo(rand.New(rand.NewPCG(*semente, uint64(i))), agora)
	}

	cfg := simulador.Config{
		Base:          *base,
		Intervalo:     *intervalo,
		Duracao:       *duracao,
		Timeout:       *timeout,
		ReusarConexao: *reusar,
		Progresso:     os.Stdout,
	}
	fmt.Printf("carga    : %d no(s), um envio a cada %s por no (%.2f req/s pedidos), por %s\n",
		*nos, *intervalo, float64(*nos)/intervalo.Seconds(), *duracao)
	rel, usados := simulador.Rodar(ctx, cfg, ativos, inicios, solos)

	estado.Devolver(inicios, usados)
	if err := salvar(); err != nil {
		// Nao e perda de dado: a reserva inteira ja estava gravada, entao a
		// proxima execucao so comeca mais adiante na sequencia.
		fmt.Fprintln(os.Stderr, "aviso: estado final nao gravado; a proxima execucao pula o bloco reservado:", err)
	}

	imprimir(rel)
	if *caminhoRelatorio != "" {
		bruto, _ := json.MarshalIndent(struct {
			*simulador.Relatorio
			Semente uint64 `json:"semente"`
		}{rel, *semente}, "", "  ")
		if err := os.WriteFile(*caminhoRelatorio, append(bruto, '\n'), 0o644); err != nil {
			sair(cli, "relatorio nao gravado: "+err.Error())
		}
		fmt.Println("relatorio:", *caminhoRelatorio)
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		fmt.Println("interrompido: o relatorio cobre so o que rodou")
	}
}

func imprimir(r *simulador.Relatorio) {
	d := r.Fim.Sub(r.Inicio).Seconds()
	fmt.Printf(`
---- resultado ----------------------------------------------------------
nos         : %d   intervalo %gs   duracao real %.1fs   conexao %s
taxa        : %.2f req/s pedidos, %.2f req/s obtidos
ingestao    : %.2f leituras/s aceitas
envios      : %d   por status %v
transporte  : %v
leituras    : %d aceitas, %d duplicadas, %d rejeitadas, %d perdidas
latencia ms : p50 %.1f   p90 %.1f   p99 %.1f   max %.1f   (n=%d)
atraso ms   : p50 %.1f   p99 %.1f   max %.1f   (atraso do cliente sobre a agenda)
`,
		r.Nos, r.IntervaloS, d, map[bool]string{true: "reaproveitada", false: "nova a cada envio"}[r.ReusarConexao],
		r.TaxaPlanejadaRPS, r.TaxaObtidaRPS, r.IngestaoLPS,
		r.Envios, r.PorStatus, r.ErrosDeTransporte,
		r.Leituras.Aceitas, r.Leituras.Duplicadas, r.Leituras.Rejeitadas, r.Leituras.Perdidas,
		r.LatenciaMS.P50, r.LatenciaMS.P90, r.LatenciaMS.P99, r.LatenciaMS.Max, r.LatenciaMS.N,
		r.AtrasoAgendaMS.P50, r.AtrasoAgendaMS.P99, r.AtrasoAgendaMS.Max)
	for _, ex := range r.ExemplosDeErro {
		fmt.Println("exemplo de erro:", ex)
	}
}

// sair encerra a sessao antes de morrer: os.Exit nao roda defer.
func sair(cli *simulador.Cliente, msg string) {
	_ = cli.Sair(context.Background())
	fatal(msg)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "erro: "+msg)
	os.Exit(1)
}
