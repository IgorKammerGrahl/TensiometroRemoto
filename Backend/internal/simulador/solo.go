// Package simulador faz N nos sensores falarem com a API real, do jeito que
// o firmware fala, para medir o backend com mais nos do que existem em
// bancada. Ver cmd/simulador para o uso e migrations/0004_simulado.sql para
// como o dado gerado aqui fica marcado.
package simulador

import (
	"math"
	"math/rand/v2"
	"time"
)

// Calibracao sintetica. Mesma referencia do cmd/seed e de http_test.go: o
// nominal de catalogo do XGZP6847A com o divisor 20k/10k do circuito.
// Passa pelas checagens de appCriarCalibracao (0 kPa cai em 3000 mV no pino).
const (
	VZeroV       = 4.5
	KVPorKPa     = 0.04
	FatorDivisor = 1.5
	VddMV        = 5000 // VDD_MV_NOMINAL do config.h: o no nao mede a alimentacao
)

// Dinamica do solo. Os mesmos numeros do cmd/seed, que avanca 1% do
// caminho ate a assintota a cada amostra de 15 min; aqui a mesma taxa vira
// constante de tempo (-15 min / ln 0,99 ~ 25 h), porque o passo entre
// medidas nao e fixo: depende do -intervalo da execucao.
//
// Isto NAO e modelo de solo validado. E um sinal plausivel para a tela e
// para a carga -- o banco nao tem curva de secagem real para calibrar nada
// (o no fisico leu ar e a outra serie e fixture).
const (
	assintotaSecaKPa = -75.0
	tauSecagem       = 25 * time.Hour
	kpaPosIrrigacao  = -9.0
	ruidoRawMV       = 3.5 // desvio medido em bancada com o tubo aberto
	kpaMinFisico     = -80 // cavitacao do tensiometro
	kpaMaxFisico     = 0
	rawMVMax         = 3300 // teto do ADC; mesmo limite da validacao da API
)

// Solo e o estado de um no simulado. Cada no tem o seu: secam em ritmos
// diferentes e sao irrigados em dias diferentes.
type Solo struct {
	r                *rand.Rand
	kpa              float64
	ultima           time.Time
	proximaIrrigacao time.Time
}

// NovoSolo sorteia um ponto de partida ja no meio do ciclo. Comecar todos
// em -9 kPa poria todos os nos na mesma zona por um dia inteiro; espalhados
// entre -9 e -65, a lista mostra as tres zonas desde o primeiro envio.
func NovoSolo(r *rand.Rand, agora time.Time) *Solo {
	return &Solo{
		r:                r,
		kpa:              kpaPosIrrigacao + r.Float64()*(-65-kpaPosIrrigacao),
		ultima:           agora,
		proximaIrrigacao: agora.Add(time.Duration(6+r.IntN(5*24-6)) * time.Hour),
	}
}

// Medida e o que o firmware poe no corpo: o mV lido no pino e o kPa que ele
// calculou com a calibracao compilada.
type Medida struct {
	RawMV int32
	KPa   float32
}

// Medir avanca o solo ate t e devolve a leitura com ruido de ADC.
func (s *Solo) Medir(t time.Time) Medida {
	if !t.Before(s.proximaIrrigacao) {
		s.kpa = kpaPosIrrigacao + s.r.NormFloat64()*1.5
		s.proximaIrrigacao = t.Add(time.Duration(2+s.r.IntN(4)) * 24 * time.Hour)
	} else if dt := t.Sub(s.ultima); dt > 0 {
		s.kpa += (assintotaSecaKPa - s.kpa) * (1 - math.Exp(-float64(dt)/float64(tauSecagem)))
	}
	s.ultima = t
	s.kpa = limitar(s.kpa, kpaMinFisico, kpaMaxFisico)

	raw := math.Round(limitar(MVDoKPa(s.kpa)+s.r.NormFloat64()*ruidoRawMV, 0, rawMVMax))
	// O firmware formata kPa com %.2f; o servidor recalcula a partir de
	// raw_mv de qualquer jeito (leituras_kpa), mas o campo e obrigatorio e
	// precisa concordar com o mV enviado.
	kpa := math.Round(KPaDoMV(raw)*100) / 100
	return Medida{RawMV: int32(raw), KPa: float32(kpa)}
}

// MVDoKPa e KPaDoMV sao a formula de leituras_kpa (migration 0003) e a sua
// inversa, com a calibracao sintetica acima.
func MVDoKPa(kpa float64) float64 {
	return (kpa*KVPorKPa + VZeroV) * 1000 / FatorDivisor
}

func KPaDoMV(mv float64) float64 {
	return (mv*FatorDivisor/1000 - VZeroV) / KVPorKPa
}

func limitar(v, lo, hi float64) float64 {
	return max(lo, min(hi, v))
}
