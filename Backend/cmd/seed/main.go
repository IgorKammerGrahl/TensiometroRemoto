// seed povoa o banco com uma serie sintetica plausivel: secagem gradual
// interrompida por irrigacoes, ruido de ADC e lacunas de no offline. Existe
// para que a interface tenha o que mostrar antes da validacao em bancada.
//
// tensio-01 (o no fisico) fica de fora de proposito: ver a nota impressa
// abaixo sobre por que semear leituras nele quebraria a validacao.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"math"
	mrand "math/rand/v2"
	neturl "net/url"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/igorkg/tcc/backend/internal/config"
	"github.com/igorkg/tcc/backend/internal/telemetria"
)

const (
	talhaoNome    = "Talhao Demonstracao"
	talhaoCultura = "Soja"
	kpaAlerta     = -30
	kpaEstresse   = -60

	usuarioEmail = "demo@tcc.local"
	usuarioNome  = "Produtor Demonstracao"
	usuarioSenha = "demo1234" // so para a apresentacao; nunca usar em producao

	deviceDemoID        = "tensio-demo"
	deviceDemoDescricao = "no sintetico de demonstracao"
	calibDemoID         = "cal-seed-demo"

	// Mesma referencia usada em http_test.go, para que a serie sintetica e
	// os testes de integracao concordem sobre como e uma calibracao real.
	vZeroKPa     = 4.5
	kVPorKPa     = 0.04
	fatorDivisor = 1.5
	vddEnsaioMV  = 5000

	asymptoteSeca       = -75.0
	taxaSecagem         = 0.01 // fracao do caminho ate a assintota, por amostra
	kpaPosIrrigacao     = -9.0
	ruidoRawMVDesvio    = 3.5  // mV; medido em bancada com o tubo aberto para a atmosfera
	toleranciaRoundTrip = 0.05 // kPa; ver nota no self-check abaixo
)

func main() {
	dias := flag.Int("dias", 14, "dias de historico sintetico a gerar")
	intervalo := flag.Duration("intervalo", 15*time.Minute, "intervalo entre amostras")
	seed := flag.Int64("seed", time.Now().UnixNano(), "seed do gerador (repita o valor para reproduzir a mesma serie)")
	confirmo := flag.Bool("confirmo", false, "confirma gravar no banco apontado por DATABASE_URL")
	flag.Parse()

	config.Carregar()
	url := config.Obrigatoria("DATABASE_URL",
		"e a string de conexao com o Postgres: e nele que a serie sintetica e gravada.")

	// O ALVO VEM DO AMBIENTE, E O AMBIENTE NAO APARECE NA LINHA DE COMANDO.
	//
	// `go run ./cmd/seed` e identico quando aponta para o Postgres do compose e
	// quando aponta para o banco que guarda a bancada -- a unica diferenca esta
	// num .env que ninguem releu. O comando cria uma conta com senha conhecida
	// e ~1300 leituras sinteticas; no banco errado isso e contaminacao dos dados
	// que sustentam o trabalho, e nao ha desfazer.
	//
	// Por isso o alvo e impresso e a execucao exige um segundo gesto. Redacted()
	// tira a senha da string de conexao: este comando costuma rodar com alguem
	// olhando a tela, e o resto da URL e o que identifica o banco.
	alvo, err := neturl.Parse(url)
	if err != nil {
		fatal("DATABASE_URL nao e uma URL valida: " + err.Error())
	}
	fmt.Printf("banco alvo    : %s\n", alvo.Redacted())
	if !*confirmo {
		fatal("este comando GRAVA no banco acima (conta de demonstracao e serie\n" +
			"  sintetica). Confira o alvo e repita com -confirmo.")
	}
	segredo := config.SegredoObrigatorio("TELEMETRIA_TOKEN_SECRET",
		"e a chave do HMAC do token: o device sintetico tambem precisa de um\n  token_hash valido para existir na tabela devices.")

	ctx := context.Background()
	pool, err := telemetria.NovoPool(ctx, url)
	if err != nil {
		fatal("conexao com o banco falhou: " + err.Error())
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fatal("ping no banco falhou: " + err.Error())
	}
	store := telemetria.NewStore(pool)

	talhaoID, err := garantirTalhao(ctx, pool)
	if err != nil {
		fatal("talhao: " + err.Error())
	}
	fmt.Printf("talhao        : %s (%s)\n", talhaoID, talhaoNome)

	usuarioID, criado, err := garantirUsuario(ctx, pool)
	if err != nil {
		fatal("usuario: " + err.Error())
	}
	if criado {
		fmt.Printf("usuario       : %s / senha %s (grave agora; nao aparece de novo)\n", usuarioEmail, usuarioSenha)
	} else {
		fmt.Printf("usuario       : %s (ja existia)\n", usuarioEmail)
	}

	if err := garantirConcessao(ctx, pool, usuarioID, talhaoID); err != nil {
		fatal("concessao: " + err.Error())
	}

	if err := garantirDeviceDemo(ctx, pool, store, talhaoID, segredo); err != nil {
		fatal("device demo: " + err.Error())
	}
	if err := garantirCalibracaoDemo(ctx, pool); err != nil {
		fatal("calibracao demo: " + err.Error())
	}

	if err := tentarAssociarTensio01(ctx, pool, talhaoID); err != nil {
		fatal("associar tensio-01: " + err.Error())
	}

	r := mrand.New(mrand.NewPCG(uint64(*seed), uint64(*seed)))
	leituras := gerarSerie(r, *dias, *intervalo)
	aceitos, err := store.InserirLeituras(ctx, deviceDemoID, leituras)
	if err != nil {
		fatal("insercao das leituras sinteticas: " + err.Error())
	}
	fmt.Printf("serie sintetica: %d leituras geradas, %d novas, %d ja existiam (seed=%d)\n",
		len(leituras), len(aceitos), len(leituras)-len(aceitos), *seed)
}

// ------------------------------------------------------------- bootstrap

func garantirTalhao(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `SELECT id FROM talhoes WHERE nome = $1`, talhaoNome).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO talhoes (nome, cultura, kpa_alerta, kpa_estresse)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, talhaoNome, talhaoCultura, kpaAlerta, kpaEstresse).Scan(&id)
	return id, err
}

func garantirUsuario(ctx context.Context, pool *pgxpool.Pool) (id string, criado bool, err error) {
	err = pool.QueryRow(ctx, `SELECT id FROM usuarios WHERE lower(email) = lower($1)`, usuarioEmail).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, err
	}
	// telemetria.CustoBcrypt e nao bcrypt.DefaultCost: o padrao da biblioteca
	// e 10, e o comentario de usuarios.senha_hash diz 12.
	hash, err := bcrypt.GenerateFromPassword([]byte(usuarioSenha), telemetria.CustoBcrypt)
	if err != nil {
		return "", false, err
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO usuarios (email, senha_hash, nome)
		VALUES ($1, $2, $3)
		RETURNING id`, usuarioEmail, string(hash), usuarioNome).Scan(&id)
	return id, true, err
}

func garantirConcessao(ctx context.Context, pool *pgxpool.Pool, usuarioID, talhaoID string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO usuario_talhoes (usuario_id, talhao_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, usuarioID, talhaoID)
	return err
}

func garantirDeviceDemo(ctx context.Context, pool *pgxpool.Pool, store *telemetria.Store, talhaoID, segredo string) error {
	existe, err := store.DeviceExiste(ctx, deviceDemoID)
	if err != nil || existe {
		return err
	}
	bruto := make([]byte, 32)
	if _, err := rand.Read(bruto); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(bruto)
	_, err = pool.Exec(ctx, `
		INSERT INTO devices (id, descricao, talhao_id, token_hash)
		VALUES ($1, $2, $3, $4)`,
		deviceDemoID, deviceDemoDescricao, talhaoID, telemetria.HashToken(segredo, token))
	return err
}

func garantirCalibracaoDemo(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO calibrations (id, device_id, v_zero_kpa, k_v_por_kpa, fator_divisor, vdd_ensaio_mv, ensaio_em, nota)
		VALUES ($1, $2, $3, $4, $5, $6, now()::date, $7)
		ON CONFLICT (id) DO NOTHING`,
		calibDemoID, deviceDemoID, vZeroKPa, kVPorKPa, fatorDivisor, vddEnsaioMV,
		"calibracao sintetica gerada por cmd/seed")
	return err
}

// tentarAssociarTensio01 so atualiza o talhao_id do no fisico; nunca insere
// leitura nele. Ver o comentario de pacote para o motivo.
func tentarAssociarTensio01(ctx context.Context, pool *pgxpool.Pool, talhaoID string) error {
	var talhaoAtual *string
	err := pool.QueryRow(ctx, `SELECT talhao_id FROM devices WHERE id = 'tensio-01'`).Scan(&talhaoAtual)
	if errors.Is(err, pgx.ErrNoRows) {
		fmt.Println("nota: tensio-01 ainda nao provisionado. Quando provisionar, associe direto:")
		fmt.Printf("      devtoken -id tensio-01 -descricao \"no de bancada\" -talhao %s\n", talhaoID)
		return nil
	}
	if err != nil {
		return err
	}
	if talhaoAtual != nil {
		if *talhaoAtual == talhaoID {
			fmt.Println("tensio-01     : ja associado ao talhao de demonstracao, sem leituras semeadas")
		} else {
			fmt.Printf("nota: tensio-01 ja associado a outro talhao (%s); nada foi alterado\n", *talhaoAtual)
		}
		return nil
	}
	if _, err := pool.Exec(ctx, `UPDATE devices SET talhao_id = $1 WHERE id = 'tensio-01'`, talhaoID); err != nil {
		return err
	}
	fmt.Println("tensio-01     : associado ao talhao de demonstracao, sem leituras semeadas")
	return nil
}

// ------------------------------------------------------------- serie sintetica

type gap struct{ inicio, fim time.Time }

// gerarSerie simula secagem gradual do solo interrompida por irrigacoes,
// com ruido de ADC e lacunas de no offline. O alvo fisico (kpa) e sempre
// mantido em [-80, 0] -- o mesmo teto da cavitacao do tensiometro e do
// CHECK de faixas em talhoes.
func gerarSerie(r *mrand.Rand, dias int, intervalo time.Duration) []telemetria.Reading {
	fim := time.Now().UTC().Truncate(time.Minute)
	inicio := fim.Add(-time.Duration(dias) * 24 * time.Hour)

	nGaps := dias / 4
	if nGaps < 2 {
		nGaps = 2
	}
	gaps := gerarGaps(r, inicio, fim, nGaps)

	leituras := make([]telemetria.Reading, 0, int(fim.Sub(inicio)/intervalo))
	kpa := kpaPosIrrigacao
	proximaIrrigacao := inicio.Add(duracaoAleatoriaDias(r, 2, 5))
	var seq int64 = 1

	for t := inicio; t.Before(fim); t = t.Add(intervalo) {
		if !t.Before(proximaIrrigacao) {
			kpa = kpaPosIrrigacao + r.NormFloat64()*1.5
			proximaIrrigacao = t.Add(duracaoAleatoriaDias(r, 2, 5))
		} else {
			kpa += (asymptoteSeca - kpa) * taxaSecagem
		}
		kpa = clamp(kpa, -80, 0)

		if emGap(t, gaps) {
			continue
		}

		rawExato := inverterKPa(kpa)
		// Round-trip: se a formula ou o sinal da inversao estiverem
		// errados, o desvio aqui salta ordens de grandeza acima da
		// tolerancia -- 1 mV de arredondamento equivale a ~0.0375 kPa,
		// entao 0.05 kPa absorve isso sem mascarar um erro real.
		if reconstruido := reconstruirKPa(rawExato); math.Abs(reconstruido-kpa) > toleranciaRoundTrip {
			fatal(fmt.Sprintf("round-trip falhou em kpa=%.4f: reconstruido=%.4f (formula de calibracao incorreta)", kpa, reconstruido))
		}

		rawFinal := int32(math.Round(clamp(rawExato+r.NormFloat64()*ruidoRawMVDesvio, 0, 3300)))
		vdd := int32(vddEnsaioMV)
		calID := calibDemoID
		leituras = append(leituras, telemetria.Reading{
			Seq:           seq,
			MeasuredAt:    t,
			RawMV:         rawFinal,
			VddMV:         &vdd,
			KPa:           float32(reconstruirKPa(float64(rawFinal))),
			CalibrationID: &calID,
		})
		seq++
	}
	return leituras
}

func gerarGaps(r *mrand.Rand, inicio, fim time.Time, n int) []gap {
	span := fim.Sub(inicio)
	gaps := make([]gap, 0, n)
	for i := 0; i < n; i++ {
		offset := time.Duration(r.Int64N(int64(span)))
		duracao := time.Duration(2+r.IntN(16)) * time.Hour // 2 a 18h offline
		g := gap{inicio.Add(offset), inicio.Add(offset).Add(duracao)}
		gaps = append(gaps, g)
	}
	return gaps
}

func emGap(t time.Time, gaps []gap) bool {
	for _, g := range gaps {
		if !t.Before(g.inicio) && t.Before(g.fim) {
			return true
		}
	}
	return false
}

func duracaoAleatoriaDias(r *mrand.Rand, min, max int) time.Duration {
	dias := min + r.IntN(max-min+1)
	return time.Duration(dias) * 24 * time.Hour
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// inverterKPa / reconstruirKPa implementam a formula de 0001_init.sql com
// fator_vdd = 1 (vdd_mv = vdd_ensaio_mv): a calibracao sintetica e usada na
// sua propria referencia, sem correcao de tensao de alimentacao.
func inverterKPa(kpa float64) float64 {
	vsensor := kpa*kVPorKPa + vZeroKPa
	return vsensor * 1000 / fatorDivisor
}

func reconstruirKPa(rawMV float64) float64 {
	vsensor := rawMV * fatorDivisor / 1000
	return (vsensor - vZeroKPa) / kVPorKPa
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "erro: "+msg)
	os.Exit(1)
}
