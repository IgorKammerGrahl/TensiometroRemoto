package telemetria

import (
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LIMITE DE TENTATIVAS DE LOGIN
//
// O login e o unico endpoint da API que roda bcrypt custo 12: ~250 ms de CPU
// por requisicao, de proposito (ver hashDeReferencia). Isso o torna caro para
// o servidor e barato para quem ataca -- e o mesmo custo e pago quando o
// e-mail nem existe, porque a defesa de timing exige isso. Sem limite, um
// laco simples ocupa a maquina inteira e ainda vai testando senhas.
//
// O algoritmo e token bucket, e nao "N tentativas por janela fixa": janela
// fixa permite o dobro das tentativas na virada (N no fim de uma, N no inicio
// da seguinte) e ainda zera o contador em horario previsivel. O balde recarrega
// continuamente, entao nao ha instante privilegiado.
//
// Sem dependencia nova de proposito: o README promete tres dependencias diretas,
// e golang.org/x/time/rate seria a quarta por ~70 linhas de codigo.
//
// FORA DO LIMITE, EXPLICITAMENTE: POST /api/v1/readings. O no nao tem buffer
// -- uma leitura recusada com 429 esta perdida para sempre, e o que se protege
// custa um SELECT por indice unico, nao um bcrypt. Limitar a ingestao trocaria
// um risco que nao existe por perda de dado que e o objeto do trabalho.

const (
	// Por IP: 10 de rajada, uma ficha a cada 6 s (10/min em regime).
	// Cabe um humano que erra a senha varias vezes e cabe a banca inteira
	// atras do mesmo NAT; nao cabe forca bruta.
	loginRajadaIP  = 10
	loginRecargaIP = 1.0 / 6.0

	// Por e-mail: mais apertado, porque aqui a chave e a conta alvo. Sem isso,
	// uma botnet distribui as tentativas por muitos IPs e cada um fica
	// folgadamente abaixo do limite acima enquanto todos atacam a MESMA conta.
	loginRajadaEmail  = 5
	loginRecargaEmail = 1.0 / 30.0

	// Teto de chaves vivas por limitador. O mapa e a superficie de memoria que
	// o atacante controla: sem teto, variar o e-mail a cada tentativa faz o
	// processo crescer ate o OOM -- uma negacao de servico construida com a
	// propria defesa.
	loginTetoChaves = 4096
)

type balde struct {
	fichas float64
	visto  time.Time
}

type limitador struct {
	mu     sync.Mutex
	baldes map[string]*balde

	max     float64 // capacidade do balde, em fichas
	recarga float64 // fichas por segundo
	teto    int     // maximo de chaves vivas

	// agora e injetavel para o teste conseguir avancar o tempo sem dormir.
	agora func() time.Time
}

func novoLimitador(rajada int, recarga float64, teto int) *limitador {
	return &limitador{
		baldes:  make(map[string]*balde),
		max:     float64(rajada),
		recarga: recarga,
		teto:    teto,
		agora:   time.Now,
	}
}

// permitir consome uma ficha da chave. Devolve quanto falta esperar quando
// recusa; a espera nao tem significado quando devolve true.
func (l *limitador) permitir(chave string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	agora := l.agora()
	b, existe := l.baldes[chave]
	if !existe {
		if len(l.baldes) >= l.teto {
			l.varrer(agora)
		}
		if len(l.baldes) >= l.teto {
			// FALHA FECHADA. Nao ha espaco para contabilizar esta chave, e
			// contabilizar e a unica forma de saber se ela passou do limite.
			// Deixar passar sem contar entregaria o limitador inteiro a quem
			// soubesse enche-lo primeiro: bastaria ocupar o mapa com chaves
			// descartaveis para que toda tentativa seguinte ficasse livre.
			// Recusar aqui e visivel e temporario; a alternativa e silenciosa.
			return l.esperaPorFicha(), false
		}
		b = &balde{fichas: l.max, visto: agora}
		l.baldes[chave] = b
	}

	// Recarga preguicosa: nao ha varredura periodica, cada balde se atualiza
	// quando e consultado. Balde que ninguem consulta nao custa CPU nenhuma.
	if decorrido := agora.Sub(b.visto).Seconds(); decorrido > 0 {
		b.fichas += decorrido * l.recarga
		if b.fichas > l.max {
			b.fichas = l.max
		}
	}
	b.visto = agora

	if b.fichas < 1 {
		return time.Duration((1 - b.fichas) / l.recarga * float64(time.Second)), false
	}
	b.fichas--
	return 0, true
}

// varrer descarta os baldes que ja recarregaram por completo.
//
// A remocao e SEM PERDA DE INFORMACAO, e nao uma heuristica de despejo: um
// balde cheio e indistinguivel de um balde que nunca existiu -- a proxima
// consulta daquela chave recria com fichas == max, exatamente o estado que
// acabou de ser apagado. Por isso nao ha despejo de balde parcialmente gasto,
// que seria justamente perdoar quem esta atacando.
func (l *limitador) varrer(agora time.Time) {
	for chave, b := range l.baldes {
		if b.fichas+agora.Sub(b.visto).Seconds()*l.recarga >= l.max {
			delete(l.baldes, chave)
		}
	}
}

func (l *limitador) esperaPorFicha() time.Duration {
	return time.Duration(float64(time.Second) / l.recarga)
}

// ConfiarNoProxy passa a ler o X-Forwarded-For das conexoes que vem de ip --
// e so delas. Ver chaveOrigem.
//
// Sem isto, atras do Caddy o RemoteAddr de toda requisicao e o proprio Caddy:
// a internet inteira divide um balde de 10 tentativas, e quem errar a senha
// dez vezes tranca o login de todo mundo.
func (a *API) ConfiarNoProxy(ip netip.Addr) {
	a.proxyConfiavel = ip.Unmap()
	a.log.Info("limite de login lendo X-Forwarded-For do proxy", "proxy", a.proxyConfiavel.String())
}

// chaveOrigem identifica o cliente pelo endereco da conexao TCP -- ou, quando
// a conexao vem do proxy configurado em ConfiarNoProxy, pelo IP que ELE anotou.
//
// X-Forwarded-For de qualquer outro peer NAO e lido, e isso e deliberado: o
// cabecalho vem do cliente e pode dizer qualquer coisa. Confiar nele entrega
// um balde novo por tentativa -- o limitador some, e ainda por cima enche o
// mapa.
//
// Do proxy, so vale o item MAIS A DIREITA do ultimo cabecalho: e o que o
// proprio proxy escreveu, o peer que ele viu. Tudo a esquerda veio do cliente.
// O Caddy sem trusted_proxies ja descarta o cabecalho recebido, mas esta regra
// nao depende disso. E um salto so: nao ha CDN na frente do Caddy.
//
// proxy e um IP, e nao uma faixa: na rede do compose o gateway do host cai
// dentro da faixa, e ele nao e o Caddy.
func chaveOrigem(r *http.Request, proxy netip.Addr) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Formato inesperado (RemoteAddr vazio em teste, socket unix): a
		// string crua ainda serve de chave. Agrupar demais aqui aperta o
		// limite, nunca o afrouxa.
		return r.RemoteAddr
	}
	if !proxy.IsValid() {
		return ip
	}
	if peer, err := netip.ParseAddr(ip); err != nil || peer.Unmap() != proxy {
		return ip
	}

	// Cabecalho ausente ou ilegivel cai no IP do proxy: todos dividem um
	// balde. Mesma regra de cima -- aperta, nunca afrouxa.
	xff := r.Header.Values("X-Forwarded-For")
	if len(xff) == 0 {
		return ip
	}
	ultimo := xff[len(xff)-1]
	cliente, err := netip.ParseAddr(strings.TrimSpace(ultimo[strings.LastIndexByte(ultimo, ',')+1:]))
	if err != nil {
		return ip
	}
	return cliente.Unmap().String()
}

// recusarPorLimite responde 429 com a MESMA mensagem para os dois limitadores.
//
// Distinguir "seu IP estourou" de "esta conta estourou" responderia "essa conta
// existe e esta sob ataque" a quem so precisa contar as respostas -- o mesmo
// oraculo que hashDeReferencia existe para fechar.
func recusarPorLimite(w http.ResponseWriter, espera time.Duration) {
	// Piso de 1: Retry-After: 0 convida a repetir na hora.
	w.Header().Set("Retry-After", strconv.Itoa(max(1, int(espera.Seconds()))))
	erroJSON(w, http.StatusTooManyRequests, "tentativas demais; tente de novo em instantes")
}
