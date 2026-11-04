package previsao

import (
	"encoding/csv"
	"math"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"
)

// VALIDACAO RETROATIVA: joga a previsao contra uma serie ja vivida e mede o
// erro contra o que de fato aconteceu depois.
//
// Nao roda por padrao. Precisa de um CSV "instante,kpa" em SERIE_CSV:
//
//	psql -At -F, -c "select to_char(measured_at,'YYYY-MM-DD\"T\"HH24:MI:SSZ'), kpa
//	   from readings where device_id='X' order by measured_at" > serie.csv
//	SERIE_CSV=serie.csv LIMIAR_KPA=-30 go test ./internal/previsao/ -run Validacao -v
//
// t.Skip e nao os.Exit: o pacote existe para ter teste puro rodando em
// qualquer maquina, e um TestMain que derruba o pacote inteiro por falta de
// dado externo desfaria exatamente isso -- e a armadilha que ja existe em
// internal/telemetria.
//
// O QUE ESTE TESTE NAO E: nao afirma acuracia de campo. A serie que o projeto
// tem hoje e de bancada e de fixture; o tensiometro quebrou antes do ensaio
// final. Ele mede se o metodo se comporta como projetado sobre dado com forma
// realista -- cadencia irregular, lacunas, irrigacoes no meio -- e sobretudo
// se as RECUSAS aparecem onde deveriam. Acuracia so depois do ensaio.
func TestValidacaoRetroativa(t *testing.T) {
	caminho := os.Getenv("SERIE_CSV")
	if caminho == "" {
		t.Skip("SERIE_CSV nao definida; validacao retroativa pulada")
	}
	limiar := -30.0
	if v := os.Getenv("LIMIAR_KPA"); v != "" {
		var err error
		if limiar, err = strconv.ParseFloat(v, 64); err != nil {
			t.Fatalf("LIMIAR_KPA invalida: %v", err)
		}
	}

	serie := lerCSV(t, caminho)
	t.Logf("serie: %d pontos, de %s a %s (%.0f h), limiar %.1f kPa",
		len(serie), serie[0].T.Format(time.DateOnly),
		serie[len(serie)-1].T.Format(time.DateOnly),
		serie[len(serie)-1].T.Sub(serie[0].T).Hours(), limiar)

	// A JANELA E FIXA E NAO ACOMPANHA O GRAFICO. O que a pessoa escolhe olhar
	// nao pode mudar o que o sistema afirma -- do contrario trocar de zoom
	// mudaria a previsao, e nenhuma das duas seria confiavel.
	//
	// JANELA_H existe para VARRER o parametro, nao para configurar producao:
	// o tamanho da janela decide quanto da curvatura da secagem entra no
	// ajuste, e so medindo da para saber onde fica o joelho entre "reta local
	// demais, so ruido" e "reta longa demais, vies de curvatura".
	janela := 48 * time.Hour
	if v := os.Getenv("JANELA_H"); v != "" {
		h, err := strconv.ParseFloat(v, 64)
		if err != nil {
			t.Fatalf("JANELA_H invalida: %v", err)
		}
		janela = time.Duration(h * float64(time.Hour))
	}

	var (
		erros      []float64 // horas, previsto - real
		larguras   []float64 // horas, Tarde - Cedo
		razaoN     []float64 // n efetivo / n bruto
		cedoDemais int       // previu antes do que aconteceu
		horizontes []float64 // horas ate o cruzamento REAL
		relativos  []float64 // erro absoluto / horizonte real
		usados     []int     // pontos que entraram em cada ajuste
		duracoes   []float64 // horas de trecho de secagem usadas
		dentro     int       // real caiu em [Cedo, Tarde]
		irrigado   int       // choveu/irrigou antes de cruzar: verdade indefinida
		semFuturo  int       // serie acabou antes da resposta
		motivos    = map[Motivo]int{}
	)

	for i := range serie {
		jan := recorte(serie, i, janela)
		p, motivo := Prever(jan, limiar)
		if motivo != OK {
			motivos[motivo]++
			continue
		}
		motivos[OK]++
		usados = append(usados, p.Pontos)
		duracoes = append(duracoes, serie[i].T.Sub(p.Desde).Hours())

		real, estado := quandoCruzou(serie, i, limiar)
		switch estado {
		case "irrigado":
			irrigado++
			continue
		case "sem futuro":
			semFuturo++
			continue
		}
		// DIAGNOSTICO: quanto da amostra sobrevive a correcao de
		// autocorrelacao, e o erro tem sinal preferido? Erro sempre do mesmo
		// lado e vies de MODELO (a reta nao descreve a curva), e nenhum
		// intervalo sobre a inclinacao cobre isso.
		trecho := jan[InicioDaSecagem(jan):]
		if incl, ok := TheilSen(trecho); ok {
			razaoN = append(razaoN, nEfetivo(subamostrar(trecho, maxPontosAjuste), incl.Central)/
				float64(min(len(trecho), maxPontosAjuste)))
		}
		if p.Em < real {
			cedoDemais++
		}
		horizontes = append(horizontes, real.Hours())
		relativos = append(relativos, math.Abs(p.Em.Hours()-real.Hours())/real.Hours())
		erros = append(erros, p.Em.Hours()-real.Hours())
		larguras = append(larguras, (p.Tarde - p.Cedo).Hours())
		if p.Cedo <= real && real <= p.Tarde {
			dentro++
		}
	}

	sort.Ints(usados)
	t.Logf("janela %.0f h; pontos por ajuste: mediano %d, max %d; duracao mediana do trecho %.1f h",
		janela.Hours(), usados[len(usados)/2], usados[len(usados)-1], duracoes[len(duracoes)/2])
	t.Log("--- decisoes por janela")
	for _, m := range []Motivo{OK, SemFaixa, PoucosPontos, Desordenada,
		JaUltrapassado, TrechoCurto, NaoSeca, HorizonteLonge} {
		if n := motivos[m]; n > 0 {
			rotulo := string(m)
			if m == OK {
				rotulo = "previu"
			}
			t.Logf("  %-32s %5d  (%4.1f%%)", rotulo, n, 100*float64(n)/float64(len(serie)))
		}
	}

	t.Logf("--- das %d previsoes: %d verificaveis, %d irrigadas antes, %d sem futuro",
		motivos[OK], len(erros), irrigado, semFuturo)
	if len(erros) == 0 {
		t.Log("nenhuma previsao verificavel nesta serie")
		return
	}

	abs := make([]float64, len(erros))
	var vies float64
	for i, e := range erros {
		abs[i], vies = math.Abs(e), vies+e
	}
	sort.Float64s(abs)
	vies /= float64(len(erros))

	sort.Float64s(horizontes)
	sort.Float64s(relativos)
	t.Logf("  horizonte real mediano  %6.1f h  (min %.1f, max %.1f)",
		horizontes[len(horizontes)/2], horizontes[0], horizontes[len(horizontes)-1])
	t.Logf("  ERRO RELATIVO mediano   %6.1f%%  (p90 %.1f%%)",
		100*relativos[len(relativos)/2], 100*relativos[int(0.9*float64(len(relativos)))])
	t.Logf("  erro absoluto mediano   %6.1f h", abs[len(abs)/2])
	t.Logf("  erro absoluto p90       %6.1f h", abs[int(0.9*float64(len(abs)))])
	t.Logf("  pior erro absoluto      %6.1f h", abs[len(abs)-1])
	t.Logf("  vies medio              %+6.1f h  (positivo = previu tarde demais)", vies)
	sort.Float64s(duracoes)
	sort.Float64s(razaoN)
	t.Logf("  n efetivo / n bruto     %6.3f  (mediano; 1,0 = residuo sem autocorrelacao)",
		razaoN[len(razaoN)/2])
	t.Logf("  erros para o lado cedo  %6.1f%%  (%d de %d; 50%% = sem vies de modelo)",
		100*float64(cedoDemais)/float64(len(erros)), cedoDemais, len(erros))
	sort.Float64s(larguras)
	t.Logf("  largura mediana da faixa %5.1f h", larguras[len(larguras)/2])
	t.Logf("  cobertura da faixa      %6.1f%%  (%d de %d dentro de [Cedo, Tarde])",
		100*float64(dentro)/float64(len(erros)), dentro, len(erros))

	// UM UNICO LIMITE, E ELE E FROUXO DE PROPOSITO. Numero apertado aqui so
	// viraria pressao para ajustar constante ate o teste passar, que e o
	// caminho mais curto para um metodo que so funciona na serie que ele viu.
	// O que se afirma e modesto: a previsao nao pode estar fora por mais que
	// a propria janela que ela olhou.
	if mediano := abs[len(abs)/2]; mediano > janela.Hours() {
		t.Errorf("erro mediano de %.1f h excede a janela de %.0f h", mediano, janela.Hours())
	}
}

// recorte devolve os pontos de `janela` terminando em `i`, inclusive.
func recorte(serie []Ponto, i int, janela time.Duration) []Ponto {
	corte := serie[i].T.Add(-janela)
	j := i
	for j > 0 && !serie[j-1].T.Before(corte) {
		j--
	}
	return serie[j : i+1]
}

// quandoCruzou anda para a FRENTE a partir de `i` e devolve quanto tempo
// passou ate a tensao alcancar o limiar de verdade.
//
// "irrigado" quando agua chega antes do cruzamento: a previsao dizia "se nada
// mudar", e algo mudou, entao nao ha o que conferir. Contar esses casos como
// erro puniria o metodo por a pessoa ter irrigado -- que e, afinal, o que se
// espera que ela faca depois de ler a previsao.
func quandoCruzou(serie []Ponto, i int, limiar float64) (time.Duration, string) {
	// Mesma escala de evento do detector, sobre o futuro.
	fut := serie[i:]
	if len(fut) < 3 {
		return 0, "sem futuro"
	}
	limite := max(kEvento*EscalaDoRuido(fut), 1e-6*amplitude(fut))
	difs := diferencas(fut)
	centro := mediana(append([]float64(nil), difs...))

	for k := 1; k < len(fut); k++ {
		if difs[k-1]-centro > limite {
			return 0, "irrigado"
		}
		if fut[k].KPa <= limiar { // mais negativo e mais seco
			return fut[k].T.Sub(serie[i].T), "cruzou"
		}
	}
	return 0, "sem futuro"
}

func lerCSV(t *testing.T, caminho string) []Ponto {
	t.Helper()
	f, err := os.Open(caminho)
	if err != nil {
		t.Fatalf("abrindo %s: %v", caminho, err)
	}
	defer f.Close()

	linhas, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("lendo %s: %v", caminho, err)
	}
	pontos := make([]Ponto, 0, len(linhas))
	for n, l := range linhas {
		if len(l) < 2 {
			continue
		}
		ts, err1 := time.Parse(time.RFC3339, l[0])
		kpa, err2 := strconv.ParseFloat(l[1], 64)
		if err1 != nil || err2 != nil {
			t.Fatalf("linha %d de %s: %v / %v", n+1, caminho, err1, err2)
		}
		pontos = append(pontos, Ponto{T: ts, KPa: kpa})
	}
	if len(pontos) < minPontos {
		t.Fatalf("%s tem so %d pontos", caminho, len(pontos))
	}
	return pontos
}
