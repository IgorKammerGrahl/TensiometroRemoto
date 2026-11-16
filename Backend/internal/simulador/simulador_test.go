package simulador

import (
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// O que pode dar errado no sinal sem ninguem perceber: leitura fora da
// faixa que a API aceita (o lote volta rejeitado e a carga mede rejeicao em
// vez de ingestao), kPa que nao concorda com o mV enviado, e um solo que
// nunca e irrigado e cola na assintota.
func TestSoloFicaNaFaixaEConcordaComOMV(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	s := NovoSolo(r, t0)

	irrigacoes := 0
	anterior := math.Inf(-1)
	for k := range 30 * 24 * 4 { // 30 dias a cada 15 min
		m := s.Medir(t0.Add(time.Duration(k) * 15 * time.Minute))
		if m.RawMV < 0 || m.RawMV > rawMVMax {
			t.Fatalf("raw_mv fora do ADC: %d", m.RawMV)
		}
		// Faixa fisica [-80, 0] mais a folga do ruido de ADC (3,5 mV ~ 0,13
		// kPa por desvio).
		if m.KPa < kpaMinFisico-1 || m.KPa > kpaMaxFisico+1 {
			t.Fatalf("kpa fora da faixa fisica: %g", m.KPa)
		}
		if d := math.Abs(KPaDoMV(float64(m.RawMV)) - float64(m.KPa)); d > 0.006 {
			t.Fatalf("kpa %g nao concorda com raw_mv %d (diferenca %g)", m.KPa, m.RawMV, d)
		}
		// Irrigacao: salto de mais de 10 kPa para o lado umido.
		if float64(m.KPa)-anterior > 10 {
			irrigacoes++
		}
		anterior = float64(m.KPa)
	}
	// Intervalo de 2 a 5 dias: em 30 dias, entre 5 e 15.
	if irrigacoes < 5 || irrigacoes > 15 {
		t.Errorf("%d irrigacoes em 30 dias; esperado entre 5 e 15", irrigacoes)
	}
}

func TestFormulaIdaEVolta(t *testing.T) {
	for _, kpa := range []float64{-80, -60, -30, -9, 0} {
		if volta := KPaDoMV(MVDoKPa(kpa)); math.Abs(volta-kpa) > 1e-9 {
			t.Errorf("kpa %g -> mV -> %g", kpa, volta)
		}
	}
	// O ponto de 0 kPa cai em 3000 mV no pino: e o que appCriarCalibracao
	// confere, e o que faria o cadastro da calibracao sintetica falhar.
	if mv := MVDoKPa(0); math.Abs(mv-3000) > 1e-9 {
		t.Errorf("0 kPa em %g mV, quero 3000", mv)
	}
}

// RISCO: seq reaproveitado depois de uma execucao interrompida. A leitura
// nova colidiria com a antiga e seria descartada como duplicata, calada.
func TestReservaSobreviveAInterrupcao(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "estado.json")
	e := &Estado{URL: "http://x", Nos: []No{{ID: "sim-0001", ProximoSeq: 1}, {ID: "sim-0002", ProximoSeq: 40}}}

	inicios := e.Reservar(2, 10)
	if inicios[0] != 1 || inicios[1] != 40 {
		t.Fatalf("inicios = %v", inicios)
	}
	if err := e.Salvar(caminho); err != nil {
		t.Fatal(err)
	}

	// Processo morre aqui: Devolver nunca roda. A proxima execucao comeca
	// depois do bloco inteiro.
	e2, err := CarregarEstado(caminho)
	if err != nil {
		t.Fatal(err)
	}
	if got := e2.Reservar(2, 10); got[0] != 11 || got[1] != 50 {
		t.Errorf("depois da interrupcao, inicios = %v; quero [11 50]", got)
	}

	// Execucao que termina inteira devolve o que nao usou.
	e.Devolver(inicios, []int64{3, 0})
	if e.Nos[0].ProximoSeq != 4 || e.Nos[1].ProximoSeq != 40 {
		t.Errorf("depois de Devolver: %d, %d; quero 4, 40", e.Nos[0].ProximoSeq, e.Nos[1].ProximoSeq)
	}
}

// O arquivo guarda tokens: precisa nascer legivel so pelo dono.
func TestEstadoGravadoComPermissao0600(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "estado.json")
	if err := (&Estado{URL: "http://x"}).Salvar(caminho); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(caminho)
	if err != nil {
		t.Fatal(err)
	}
	if p := info.Mode().Perm(); p != 0o600 {
		t.Errorf("permissao %o, quero 600", p)
	}

	vazio, err := CarregarEstado(filepath.Join(t.TempDir(), "nao-existe.json"))
	if err != nil || len(vazio.Nos) != 0 {
		t.Errorf("arquivo ausente: %+v, %v; quero estado vazio sem erro", vazio, err)
	}
}

func TestDistribuirPostoMaisProximo(t *testing.T) {
	var ds []time.Duration
	for i := 100; i >= 1; i-- { // fora de ordem de proposito
		ds = append(ds, time.Duration(i)*time.Millisecond)
	}
	d := Distribuir(ds)
	if d.N != 100 || d.P50 != 50 || d.P90 != 90 || d.P99 != 99 || d.Max != 100 || d.Media != 50.5 {
		t.Errorf("distribuicao de 1..100 ms = %+v", d)
	}
	if um := Distribuir([]time.Duration{7 * time.Millisecond}); um.P50 != 7 || um.P99 != 7 {
		t.Errorf("amostra de um: %+v", um)
	}
	if vazio := Distribuir(nil); vazio != (Distribuicao{}) {
		t.Errorf("amostra vazia: %+v", vazio)
	}
}

func TestEnviosPorNo(t *testing.T) {
	casos := []struct {
		intervalo, duracao time.Duration
		quero              int64
	}{
		{time.Minute, 5 * time.Minute, 5},
		{time.Minute, 5*time.Minute + time.Second, 6},
		{time.Second, time.Second, 1},
	}
	for _, c := range casos {
		if got := EnviosPorNo(c.intervalo, c.duracao); got != c.quero {
			t.Errorf("EnviosPorNo(%s, %s) = %d; quero %d", c.intervalo, c.duracao, got, c.quero)
		}
	}
}
