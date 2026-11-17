package telemetria

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

// relogio falso: o limitador e sobre passagem de tempo, e um teste que dorme
// 30 s para provar recarga nao e rodado por ninguem.
func comRelogio(l *limitador) (*limitador, func(time.Duration)) {
	t0 := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	agora := t0
	l.agora = func() time.Time { return agora }
	return l, func(d time.Duration) { agora = agora.Add(d) }
}

func TestLimitadorGastaRajadaEDepoisRecusa(t *testing.T) {
	l, _ := comRelogio(novoLimitador(3, 1.0, 16))

	for i := range 3 {
		if _, ok := l.permitir("a"); !ok {
			t.Fatalf("tentativa %d deveria passar dentro da rajada", i+1)
		}
	}
	espera, ok := l.permitir("a")
	if ok {
		t.Fatal("quarta tentativa passou; a rajada e 3")
	}
	if espera <= 0 {
		t.Fatalf("recusa sem espera positiva: %v", espera)
	}
}

func TestLimitadorRecarregaComOTempo(t *testing.T) {
	l, avancar := comRelogio(novoLimitador(3, 1.0, 16))

	for range 3 {
		l.permitir("a")
	}
	if _, ok := l.permitir("a"); ok {
		t.Fatal("balde deveria estar vazio")
	}

	avancar(time.Second) // 1 ficha/s
	if _, ok := l.permitir("a"); !ok {
		t.Fatal("uma ficha deveria ter voltado depois de 1 s")
	}
	if _, ok := l.permitir("a"); ok {
		t.Fatal("voltou mais ficha do que o tempo decorrido paga")
	}
}

func TestLimitadorNaoPassaDaCapacidade(t *testing.T) {
	l, avancar := comRelogio(novoLimitador(3, 1.0, 16))

	l.permitir("a")
	avancar(time.Hour) // muito mais do que o balde comporta

	for i := range 3 {
		if _, ok := l.permitir("a"); !ok {
			t.Fatalf("tentativa %d deveria passar: o balde recarregou cheio", i+1)
		}
	}
	if _, ok := l.permitir("a"); ok {
		t.Fatal("balde acumulou fichas acima da capacidade")
	}
}

func TestLimitadorSeparaChaves(t *testing.T) {
	l, _ := comRelogio(novoLimitador(1, 1.0, 16))

	if _, ok := l.permitir("a"); !ok {
		t.Fatal("primeira chave deveria passar")
	}
	if _, ok := l.permitir("a"); ok {
		t.Fatal("chave a deveria estar esgotada")
	}
	if _, ok := l.permitir("b"); !ok {
		t.Fatal("chave b nao pode herdar o estado de a")
	}
}

// A varredura e o que impede o mapa de crescer sem limite. Ela so pode
// descartar balde CHEIO -- descartar balde pela metade perdoaria justamente
// quem esta gastando fichas.
func TestVarreduraSoDescartaBaldeCheio(t *testing.T) {
	l, avancar := comRelogio(novoLimitador(2, 1.0, 16))

	l.permitir("gasto") // fica com 1 de 2
	l.permitir("cheio")
	avancar(10 * time.Second) // so "cheio" recarrega por completo... e "gasto" tambem

	// Depois de tempo suficiente os dois estao cheios: os dois saem.
	l.mu.Lock()
	l.varrer(l.agora())
	n := len(l.baldes)
	l.mu.Unlock()
	if n != 0 {
		t.Fatalf("baldes cheios deveriam sair da varredura; sobraram %d", n)
	}

	// Agora sem deixar recarregar: o balde gasto precisa sobreviver.
	l.permitir("gasto")
	l.mu.Lock()
	l.varrer(l.agora())
	_, sobreviveu := l.baldes["gasto"]
	l.mu.Unlock()
	if !sobreviveu {
		t.Fatal("balde parcialmente gasto foi descartado; o gasto seria perdoado")
	}
}

// Descartar balde cheio nao pode mudar resposta nenhuma: cheio e
// indistinguivel de inexistente.
func TestVarreduraNaoPerdeInformacao(t *testing.T) {
	l, avancar := comRelogio(novoLimitador(2, 1.0, 16))

	l.permitir("a")
	avancar(10 * time.Second)
	l.mu.Lock()
	l.varrer(l.agora())
	l.mu.Unlock()

	for i := range 2 {
		if _, ok := l.permitir("a"); !ok {
			t.Fatalf("tentativa %d deveria passar apos a varredura", i+1)
		}
	}
	if _, ok := l.permitir("a"); ok {
		t.Fatal("a varredura deu fichas a mais")
	}
}

// O mapa e a superficie de memoria que o atacante controla variando a chave.
func TestLimitadorRecusaAoEncherOMapa(t *testing.T) {
	const teto = 8
	l, _ := comRelogio(novoLimitador(1, 1.0/3600.0, teto)) // recarga lentissima

	for i := range teto {
		if _, ok := l.permitir(string(rune('a' + i))); !ok {
			t.Fatalf("chave %d deveria caber no teto", i)
		}
	}
	// Todos os baldes estao gastos, entao a varredura nao libera nada.
	if _, ok := l.permitir("chave-nova"); ok {
		t.Fatal("mapa cheio deveria falhar FECHADO, e nao deixar passar")
	}
	l.mu.Lock()
	n := len(l.baldes)
	l.mu.Unlock()
	if n > teto {
		t.Fatalf("mapa passou do teto: %d chaves", n)
	}
}

func TestChaveOrigemIgnoraForwardedFor(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/app/login", nil)
	r.RemoteAddr = "203.0.113.7:54321"
	r.Header.Set("X-Forwarded-For", "198.51.100.1")

	// Sem proxy configurado, confiar no cabecalho daria um balde novo por
	// tentativa.
	if got := chaveOrigem(r, netip.Addr{}); got != "203.0.113.7" {
		t.Fatalf("chaveOrigem = %q; queria o IP da conexao, nao o cabecalho", got)
	}
}

func TestChaveOrigemSemPorta(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/app/login", nil)
	r.RemoteAddr = "socket-unix"
	if got := chaveOrigem(r, netip.MustParseAddr("10.250.250.2")); got != "socket-unix" {
		t.Fatalf("chaveOrigem = %q; endereco sem porta ainda precisa virar chave", got)
	}
}

// Com o Caddy configurado, o cabecalho vale SO na conexao que vem dele, e so
// o item que ele mesmo escreveu (o mais a direita).
func TestChaveOrigemAtrasDoProxy(t *testing.T) {
	caddy := netip.MustParseAddr("10.250.250.2")
	casos := []struct {
		nome   string
		remote string
		xff    []string
		quero  string
	}{
		{"cliente direto forjando o cabecalho", "203.0.113.7:54321",
			[]string{"198.51.100.1"}, "203.0.113.7"},
		{"cliente via Caddy", "10.250.250.2:40000",
			[]string{"203.0.113.7"}, "203.0.113.7"},
		{"cliente forjando via Caddy: vale o que o Caddy anotou", "10.250.250.2:40000",
			[]string{"198.51.100.1, 203.0.113.7"}, "203.0.113.7"},
		{"cabecalho repetido: vale o ultimo", "10.250.250.2:40000",
			[]string{"198.51.100.1", "203.0.113.7"}, "203.0.113.7"},
		{"Caddy em IPv4 mapeado em IPv6", "[::ffff:10.250.250.2]:40000",
			[]string{"203.0.113.7"}, "203.0.113.7"},
		{"via Caddy sem cabecalho: balde do Caddy", "10.250.250.2:40000",
			nil, "10.250.250.2"},
		{"via Caddy com lixo no item final: balde do Caddy", "10.250.250.2:40000",
			[]string{"203.0.113.7, nao-e-ip"}, "10.250.250.2"},
		{"gateway na mesma rede nao e o Caddy", "10.250.250.1:40000",
			[]string{"198.51.100.1"}, "10.250.250.1"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/v1/app/login", nil)
			r.RemoteAddr = c.remote
			for _, v := range c.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if got := chaveOrigem(r, caddy); got != c.quero {
				t.Fatalf("chaveOrigem = %q; quero %q", got, c.quero)
			}
		})
	}
}

// O handler de login usa a chave do proxy, e nao so a funcao. Corpo vazio
// para no 400 do decode, depois do limite por IP e antes de qualquer acesso
// ao banco -- por isso o store pode ser nil.
func TestLoginAtrasDoProxyLimitaPorCliente(t *testing.T) {
	a := NewAPI(nil, segredoTeste, slog.New(slog.DiscardHandler))
	a.ConfiarNoProxy(netip.MustParseAddr("10.250.250.2"))
	h := a.Router()

	login := func(cliente string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/app/login", nil)
		r.RemoteAddr = "10.250.250.2:40000"
		r.Header.Set("X-Forwarded-For", cliente)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}

	for i := range loginRajadaIP {
		if got := login("203.0.113.7"); got != http.StatusBadRequest {
			t.Fatalf("tentativa %d: status = %d, quero 400 dentro da rajada", i+1, got)
		}
	}
	if got := login("203.0.113.7"); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, quero 429 para o cliente que esgotou a rajada", got)
	}
	// O ponto da mudanca: atras do mesmo Caddy, outro cliente tem balde proprio.
	if got := login("198.51.100.1"); got != http.StatusBadRequest {
		t.Fatalf("status = %d; o balde de um cliente vazou para outro atras do Caddy", got)
	}
}
