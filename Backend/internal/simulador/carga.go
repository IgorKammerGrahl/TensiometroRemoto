package simulador

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Config descreve uma execucao. Os padroes do cmd/simulador sao os do
// firmware (Circuito/sketch/config.h.example): uma leitura a cada 60 s,
// timeout de 10 s e conexao nova a cada envio.
type Config struct {
	Base      string
	Intervalo time.Duration
	Duracao   time.Duration
	Timeout   time.Duration
	// ReusarConexao mantem a conexao TCP aberta entre envios do mesmo
	// processo. O firmware NAO faz isso (http.end() a cada volta do loop),
	// entao o padrao e false; ligar serve para separar, numa carga alta, o
	// custo de abrir conexao do custo de ingerir.
	ReusarConexao bool
	// Progresso recebe uma linha a cada 10 s; nil desliga.
	Progresso io.Writer
}

// EnviosPorNo e o maximo de envios de um no numa execucao, e portanto o
// tamanho do bloco de seq que Estado.Reservar separa para ele.
func EnviosPorNo(intervalo, duracao time.Duration) int64 {
	return int64((duracao + intervalo - 1) / intervalo)
}

// Distribuicao resume uma amostra de duracoes, em milissegundos.
type Distribuicao struct {
	N     int     `json:"n"`
	Media float64 `json:"media"`
	P50   float64 `json:"p50"`
	P90   float64 `json:"p90"`
	P99   float64 `json:"p99"`
	Max   float64 `json:"max"`
}

type Leituras struct {
	Aceitas    int `json:"aceitas"`
	Duplicadas int `json:"duplicadas"`
	Rejeitadas int `json:"rejeitadas"`
	// Perdidas: envios cuja leitura nao entrou no banco por nenhum motivo --
	// rejeitada, status diferente de 200 ou sem resposta. No firmware, cada
	// uma vira um buraco na sequencia.
	Perdidas int `json:"perdidas"`
}

type Ambiente struct {
	GoVersion  string `json:"go_version"`
	GOOS       string `json:"goos"`
	GOARCH     string `json:"goarch"`
	NumCPU     int    `json:"num_cpu"`
	GOMAXPROCS int    `json:"gomaxprocs"`
}

// Relatorio e o resultado de uma execucao, na forma em que vai para o JSON.
type Relatorio struct {
	Inicio        time.Time `json:"inicio"`
	Fim           time.Time `json:"fim"`
	Alvo          string    `json:"alvo"`
	Nos           int       `json:"nos"`
	IntervaloS    float64   `json:"intervalo_s"`
	DuracaoS      float64   `json:"duracao_s"`
	TimeoutS      float64   `json:"timeout_s"`
	ReusarConexao bool      `json:"reusar_conexao"`

	// TaxaPlanejadaRPS e o que a agenda pede (nos / intervalo);
	// TaxaObtidaRPS e o que de fato saiu. Se a obtida ficar abaixo da
	// planejada, o cliente nao deu conta, e a medida e do cliente.
	TaxaPlanejadaRPS float64 `json:"taxa_planejada_rps"`
	TaxaObtidaRPS    float64 `json:"taxa_obtida_rps"`
	// IngestaoLPS: leituras aceitas por segundo, a taxa de ingestao.
	IngestaoLPS float64 `json:"ingestao_lps"`

	Envios            int            `json:"envios"`
	PorStatus         map[string]int `json:"por_status"`
	ErrosDeTransporte map[string]int `json:"erros_de_transporte"`
	ExemplosDeErro    []string       `json:"exemplos_de_erro,omitempty"`
	Leituras          Leituras       `json:"leituras"`

	// LatenciaMS so conta envios que tiveram resposta HTTP; os que nao
	// tiveram estao em ErrosDeTransporte. Misturar o tempo de um timeout na
	// distribuicao poria 10 000 ms no p99 sem dizer de onde veio.
	LatenciaMS Distribuicao `json:"latencia_ms"`
	// AtrasoAgendaMS: quanto depois do horario agendado o envio saiu. Perto
	// de zero, a carga foi a pedida; crescendo, o proprio simulador virou
	// gargalo e a latencia medida deixa de valer como medida do servidor.
	AtrasoAgendaMS Distribuicao `json:"atraso_agenda_ms"`

	Ambiente Ambiente `json:"ambiente"`
}

// Rodar dispara os nos e espera todos terminarem. inicios[i] e o primeiro
// seq reservado para nos[i]; usados[i], na volta, quantos dele se gastaram.
//
// A AGENDA E FIXA, e nao "espera o intervalo depois de cada envio" como o
// loop() do firmware. Com espera depois do envio, um servidor lento faria os
// nos enviarem menos, e a carga cairia justamente quando deveria pressionar
// -- a lentidao esconderia a si mesma na medida. Com agenda fixa a chegada e
// a pedida, haja o que houver do outro lado. O no real envia a cada
// intervalo + tempo do envio, entao a carga daqui e um teto da dele.
//
// Os nos comecam espalhados por igual ao longo do primeiro intervalo: no
// campo eles nao ligam no mesmo segundo, e ligar todos juntos mediria uma
// rajada que nao existe.
//
// Cancelar ctx para a agenda; envio em voo termina (limitado por Timeout),
// para que o seq usado seja contado e devolvido certo.
func Rodar(ctx context.Context, cfg Config, nos []No, inicios []int64, solos []*Solo) (*Relatorio, []int64) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil // a medida e do servidor, nunca de um proxy do ambiente
	tr.DisableKeepAlives = !cfg.ReusarConexao
	tr.MaxIdleConns = len(nos)
	tr.MaxIdleConnsPerHost = len(nos)
	hc := &http.Client{Timeout: cfg.Timeout, Transport: tr}
	defer tr.CloseIdleConnections()

	parciais := make([]parcial, len(nos))
	var contEnvios, contOK, contFalhas atomic.Int64

	t0 := time.Now()
	fim := t0.Add(cfg.Duracao)
	var wg sync.WaitGroup
	for i := range nos {
		wg.Go(func() {
			p := &parciais[i]
			offset := time.Duration(int64(cfg.Intervalo) * int64(i) / int64(len(nos)))
			for k := 0; ; k++ {
				agendado := t0.Add(offset + time.Duration(k)*cfg.Intervalo)
				if !agendado.Before(fim) || !dormirAte(ctx, agendado) {
					return
				}
				agora := time.Now()
				seq := inicios[i] + p.usados
				p.usados++ // gasto antes do envio, como no firmware
				e := Enviar(context.Background(), hc, cfg.Base, nos[i], seq, agora, solos[i].Medir(agora))
				p.registrar(agora.Sub(agendado), e)

				contEnvios.Add(1)
				if e.Status == http.StatusOK && e.Aceitas+e.Duplicadas == 1 {
					contOK.Add(1)
				} else {
					contFalhas.Add(1)
				}
			}
		})
	}

	if cfg.Progresso != nil {
		pararProgresso := make(chan struct{})
		defer close(pararProgresso)
		go func() {
			tk := time.NewTicker(10 * time.Second)
			defer tk.Stop()
			for {
				select {
				case <-pararProgresso:
					return
				case t := <-tk.C:
					fmt.Fprintf(cfg.Progresso, "  %4.0fs  envios=%d  ok=%d  falhas=%d\n",
						t.Sub(t0).Seconds(), contEnvios.Load(), contOK.Load(), contFalhas.Load())
				}
			}
		}()
	}
	wg.Wait()
	t1 := time.Now()

	usados := make([]int64, len(nos))
	for i := range parciais {
		usados[i] = parciais[i].usados
	}
	return consolidar(cfg, len(nos), t0, t1, parciais), usados
}

// dormirAte devolve false se ctx for cancelado antes de t.
func dormirAte(ctx context.Context, t time.Time) bool {
	d := time.Until(t)
	if d <= 0 {
		return ctx.Err() == nil
	}
	tm := time.NewTimer(d)
	defer tm.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-tm.C:
		return true
	}
}

// parcial e o que um no acumula. Cada goroutine escreve so no seu, e a
// consolidacao acontece depois do wg.Wait: nenhuma trava no caminho quente.
type parcial struct {
	usados             int64
	latencias, atrasos []time.Duration
	porStatus          map[int]int
	erros              map[string]int
	exemplos           []string
	leituras           Leituras
}

func (p *parcial) registrar(atraso time.Duration, e Envio) {
	if p.porStatus == nil {
		p.porStatus = map[int]int{}
		p.erros = map[string]int{}
	}
	p.atrasos = append(p.atrasos, atraso)
	p.porStatus[e.Status]++
	if e.Status != 0 {
		p.latencias = append(p.latencias, e.Latencia)
	}
	if e.Erro != "" {
		p.erros[e.Erro]++
		if len(p.exemplos) < 3 {
			p.exemplos = append(p.exemplos, e.ErroBruto)
		}
	}
	p.leituras.Aceitas += e.Aceitas
	p.leituras.Duplicadas += e.Duplicadas
	p.leituras.Rejeitadas += e.Rejeitadas
	if e.Status != http.StatusOK || e.Aceitas+e.Duplicadas != 1 {
		p.leituras.Perdidas++
	}
}

func consolidar(cfg Config, nos int, t0, t1 time.Time, parciais []parcial) *Relatorio {
	r := &Relatorio{
		Inicio:            t0.UTC(),
		Fim:               t1.UTC(),
		Alvo:              cfg.Base,
		Nos:               nos,
		IntervaloS:        cfg.Intervalo.Seconds(),
		DuracaoS:          cfg.Duracao.Seconds(),
		TimeoutS:          cfg.Timeout.Seconds(),
		ReusarConexao:     cfg.ReusarConexao,
		TaxaPlanejadaRPS:  float64(nos) / cfg.Intervalo.Seconds(),
		PorStatus:         map[string]int{},
		ErrosDeTransporte: map[string]int{},
		Ambiente: Ambiente{
			GoVersion:  runtime.Version(),
			GOOS:       runtime.GOOS,
			GOARCH:     runtime.GOARCH,
			NumCPU:     runtime.NumCPU(),
			GOMAXPROCS: runtime.GOMAXPROCS(0),
		},
	}
	var latencias, atrasos []time.Duration
	for _, p := range parciais {
		latencias = append(latencias, p.latencias...)
		atrasos = append(atrasos, p.atrasos...)
		for st, n := range p.porStatus {
			chave := strconv.Itoa(st)
			if st == 0 {
				chave = "sem_resposta"
			}
			r.PorStatus[chave] += n
			r.Envios += n
		}
		for cat, n := range p.erros {
			r.ErrosDeTransporte[cat] += n
		}
		for _, ex := range p.exemplos {
			if len(r.ExemplosDeErro) < 5 {
				r.ExemplosDeErro = append(r.ExemplosDeErro, ex)
			}
		}
		r.Leituras.Aceitas += p.leituras.Aceitas
		r.Leituras.Duplicadas += p.leituras.Duplicadas
		r.Leituras.Rejeitadas += p.leituras.Rejeitadas
		r.Leituras.Perdidas += p.leituras.Perdidas
	}
	r.LatenciaMS = Distribuir(latencias)
	r.AtrasoAgendaMS = Distribuir(atrasos)
	if s := t1.Sub(t0).Seconds(); s > 0 {
		r.TaxaObtidaRPS = float64(r.Envios) / s
		r.IngestaoLPS = float64(r.Leituras.Aceitas) / s
	}
	return r
}

// Distribuir calcula media e percentis pelo metodo do posto mais proximo
// (o menor valor com pelo menos p% da amostra ate ele), sem interpolacao:
// todo percentil relatado e uma latencia que de fato aconteceu.
func Distribuir(ds []time.Duration) Distribuicao {
	if len(ds) == 0 {
		return Distribuicao{}
	}
	ord := slices.Clone(ds)
	slices.Sort(ord)
	var soma time.Duration
	for _, d := range ord {
		soma += d
	}
	ms := func(d time.Duration) float64 { return math.Round(float64(d)/1e3) / 1e3 }
	posto := func(p float64) time.Duration {
		i := int(math.Ceil(p*float64(len(ord)))) - 1
		return ord[max(0, min(i, len(ord)-1))]
	}
	return Distribuicao{
		N:     len(ord),
		Media: ms(soma / time.Duration(len(ord))),
		P50:   ms(posto(0.50)),
		P90:   ms(posto(0.90)),
		P99:   ms(posto(0.99)),
		Max:   ms(ord[len(ord)-1]),
	}
}
