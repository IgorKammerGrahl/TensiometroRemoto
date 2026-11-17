package telemetria

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/igorkg/tcc/backend/internal/simulador"
)

// O simulador contra o roteador de verdade e o Postgres de teste.
//
// Mora aqui, e nao em internal/simulador, por causa do banco: os dois
// pacotes fariam TRUNCATE no mesmo telemetria_test, e `go test ./...` roda
// pacotes em paralelo. No mesmo pacote os testes ficam em serie.

// O ciclo inteiro, so pela API: login, cadastro, calibracao e envio. Cada
// leitura enviada tem de estar no banco, sob o no certo, com o seq que o
// simulador reservou -- e o no tem de estar marcado.
func TestSimuladorDePontaAPonta(t *testing.T) {
	c := novoCenarioApp(t)
	srv := httptest.NewServer(c.h)
	defer srv.Close()
	ctx := context.Background()

	cli := simulador.NovoCliente(srv.URL)
	if err := cli.Entrar(ctx, emailA, senhaTeste); err != nil {
		t.Fatal(err)
	}

	e := &simulador.Estado{URL: srv.URL}
	salvos := 0
	criados, err := simulador.Provisionar(ctx, cli, e, func() error { salvos++; return nil },
		3, "sim", []string{c.talhaoA, c.talhaoC})
	if err != nil {
		t.Fatal(err)
	}
	// Um salvamento por cadastro e outro por calibracao: o token nao pode
	// esperar a calibracao para ir ao disco.
	if criados != 3 || salvos != 6 {
		t.Errorf("criados=%d salvos=%d; quero 3 e 6", criados, salvos)
	}
	if e.Nos[0].TalhaoID != c.talhaoA || e.Nos[1].TalhaoID != c.talhaoC || e.Nos[2].TalhaoID != c.talhaoA {
		t.Errorf("rodizio de talhoes errado: %+v", e.Nos)
	}
	if err := simulador.Verificar(ctx, cli, e.Nos); err != nil {
		t.Fatal(err)
	}

	cfg := simulador.Config{Base: srv.URL, Intervalo: 50 * time.Millisecond,
		Duracao: 500 * time.Millisecond, Timeout: 5 * time.Second}
	inicios := e.Reservar(3, simulador.EnviosPorNo(cfg.Intervalo, cfg.Duracao))
	solos := make([]*simulador.Solo, 3)
	for i := range solos {
		solos[i] = simulador.NovoSolo(rand.New(rand.NewPCG(7, uint64(i))), time.Now())
	}
	rel, usados := simulador.Rodar(ctx, cfg, e.Nos, inicios, solos)

	if rel.Envios != 30 || rel.PorStatus["200"] != 30 {
		t.Errorf("envios=%d por status=%v; quero 30, todos 200", rel.Envios, rel.PorStatus)
	}
	if l := rel.Leituras; l.Aceitas != 30 || l.Duplicadas != 0 || l.Rejeitadas != 0 || l.Perdidas != 0 {
		t.Errorf("leituras = %+v; quero 30 aceitas e nada mais", l)
	}

	var n, simulados int
	var seqMin, seqMax int64
	if err := testPool.QueryRow(ctx, `
		SELECT count(*), min(r.seq), max(r.seq), count(*) FILTER (WHERE d.simulado)
		  FROM readings r JOIN devices d ON d.id = r.device_id
		 WHERE r.device_id = 'sim-0002'`).Scan(&n, &seqMin, &seqMax, &simulados); err != nil {
		t.Fatal(err)
	}
	if n != 10 || seqMin != 1 || seqMax != 10 || simulados != 10 {
		t.Errorf("sim-0002 no banco: %d leituras, seq %d..%d, %d simuladas; quero 10, 1..10, 10",
			n, seqMin, seqMax, simulados)
	}

	e.Devolver(inicios, usados)
	if e.Nos[1].ProximoSeq != 11 {
		t.Errorf("ProximoSeq depois da execucao = %d; quero 11", e.Nos[1].ProximoSeq)
	}
}

// RISCO: o token de um no real parar no arquivo de estado. O simulador
// mandaria sinal gerado para a serie dele, com cara de medida. Verificar
// precisa recusar ANTES de qualquer envio.
func TestSimuladorRecusaNoReal(t *testing.T) {
	c := novoCenarioApp(t)
	srv := httptest.NewServer(c.h)
	defer srv.Close()
	ctx := context.Background()

	cli := simulador.NovoCliente(srv.URL)
	if err := cli.Entrar(ctx, emailA, senhaTeste); err != nil {
		t.Fatal(err)
	}

	// dev-a1 e um no real visivel para A; o token dele e valido.
	real := simulador.No{ID: "dev-a1", Token: "tok-a1", CalibracaoID: "cal-qualquer", ProximoSeq: 1}
	err := simulador.Verificar(ctx, cli, []simulador.No{real})
	if !errors.Is(err, simulador.ErrNoNaoSimulado) {
		t.Fatalf("Verificar com no real devolveu %v; quero ErrNoNaoSimulado", err)
	}

	// Um no simulado no meio de outros nao salva o lote: um errado aborta
	// todos.
	e := &simulador.Estado{URL: srv.URL}
	if _, err := simulador.Provisionar(ctx, cli, e, func() error { return nil },
		1, "sim", []string{c.talhaoA}); err != nil {
		t.Fatal(err)
	}
	if err := simulador.Verificar(ctx, cli, append(e.Nos, real)); !errors.Is(err, simulador.ErrNoNaoSimulado) {
		t.Errorf("lote com um no real devolveu %v; quero ErrNoNaoSimulado", err)
	}
}
