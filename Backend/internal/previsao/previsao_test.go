package previsao

import (
	"math"
	"sort"
	"testing"
	"time"
)

// Sem banco de proposito: e o motivo de este pacote nao morar em `telemetria`,
// onde o TestMain faz os.Exit(0) sem TEST_DATABASE_URL e leva os testes puros
// junto. Aqui `go test ./...` exercita tudo em qualquer maquina.

var t0 = time.Date(2026, 9, 1, 6, 0, 0, 0, time.UTC)

// serie monta uma secagem sintetica: `n` pontos de `passo` em `passo`, saindo
// de `kpa0` e caindo `inclHora` kPa por hora.
//
// `ruido` usa seno e nao rand: a mesma entrada precisa dar o mesmo resultado
// em toda execucao, senao um teste que falha uma vez em cinquenta vira um
// teste que alguem desabilita.
func serie(n int, passo time.Duration, kpa0, inclHora, ruido float64) []Ponto {
	return serieFreq(n, passo, kpa0, inclHora, ruido, 1.7)
}

// serieFreq controla a FREQUENCIA do ruido, que e como se controla a
// autocorrelacao dos residuos: `freq` alta troca de sinal a cada passo e
// deixa r1 negativo; `freq` baixa desenha uma ondulacao lenta, com r1 perto
// de 1, que e o que uma serie de solo real parece.
func serieFreq(n int, passo time.Duration, kpa0, inclHora, ruido, freq float64) []Ponto {
	p := make([]Ponto, n)
	for i := range n {
		h := float64(i) * passo.Hours()
		p[i] = Ponto{
			T:   t0.Add(time.Duration(i) * passo),
			KPa: kpa0 + inclHora*h + ruido*math.Sin(float64(i)*freq),
		}
	}
	return p
}

// desloca soma `d` kPa a todos os pontos a partir de `i` -- uma irrigacao, que
// empurra a tensao EM DIRECAO A ZERO, logo `d` positivo.
func desloca(p []Ponto, i int, d float64) []Ponto {
	for j := i; j < len(p); j++ {
		p[j].KPa += d
	}
	return p
}

// --------------------------------------------------------------- Theil-Sen

func TestTheilSenRecuperaInclinacaoConhecida(t *testing.T) {
	incl, ok := TheilSen(serie(48, time.Hour, -20, -0.35, 0.2))
	if !ok {
		t.Fatal("TheilSen recusou serie valida")
	}
	if math.Abs(incl.Central-(-0.35)) > 0.02 {
		t.Errorf("inclinacao = %.4f kPa/h, esperado ~-0.35", incl.Central)
	}
}

// O TESTE QUE JUSTIFICA O METODO.
//
// Se alguem trocar Theil-Sen por minimos quadrados "porque e mais simples",
// este e o teste que quebra. Uma leitura de -95 kPa no meio de uma serie em
// torno de -25 nao e hipotese: e ADC com contato ruim, e o firmware envia. Sob
// OLS aquele ponto sozinho entorta a reta e a previsao de irrigacao antecipa
// dias. Sob a mediana das inclinacoes ele e um par entre milhares.
func TestTheilSenIgnoraLeituraAbsurda(t *testing.T) {
	limpa := serie(48, time.Hour, -20, -0.35, 0.2)
	sujo := append([]Ponto(nil), limpa...)
	sujo[4].KPa = -95 // contato ruim no divisor resistivo

	// Perto da PONTA, e nao no meio. No centro da serie um outlier quase nao
	// move a inclinacao de OLS -- ele tem alavancagem zero e so desloca o
	// intercepto. A primeira versao deste teste errou por isso e "provou" que
	// OLS estava bem. E na ponta que a alavancagem e maxima, e e na ponta que
	// as leituras ruins de ligar e desligar o no aparecem.

	bom, _ := TheilSen(limpa)
	ruim, _ := TheilSen(sujo)

	if math.Abs(ruim.Central-bom.Central) > 0.02 {
		t.Errorf("uma leitura absurda moveu a inclinacao de %.4f para %.4f",
			bom.Central, ruim.Central)
	}
	// E a comparacao explicita com o que NAO usamos, para o numero ficar no
	// registro em vez de so na afirmacao do comentario.
	if d := math.Abs(minimosQuadrados(sujo) - bom.Central); d < 0.05 {
		t.Fatalf("premissa do teste falhou: OLS so desviou %.4f, o outlier era brando demais", d)
	}
}

func TestTheilSenFaixaCercaOCentral(t *testing.T) {
	incl, _ := TheilSen(serie(60, 30*time.Minute, -15, -0.4, 0.8))
	if !(incl.Baixa <= incl.Central && incl.Central <= incl.Alta) {
		t.Fatalf("faixa fora de ordem: %.4f <= %.4f <= %.4f", incl.Baixa, incl.Central, incl.Alta)
	}
	if !(incl.Baixa <= -0.4 && -0.4 <= incl.Alta) {
		t.Errorf("faixa [%.4f, %.4f] nao contem a inclinacao verdadeira -0.4", incl.Baixa, incl.Alta)
	}
}

func TestTheilSenRecusaSemIntervaloDeTempo(t *testing.T) {
	// Tres leituras no mesmo carimbo: nenhum par tem dt > 0, e dividir por
	// zero produziria +Inf, que passaria pelo portao `Alta >= 0` como se
	// fosse secagem infinitamente rapida.
	mesmo := []Ponto{{t0, -10}, {t0, -11}, {t0, -12}}
	if _, ok := TheilSen(mesmo); ok {
		t.Error("TheilSen aceitou serie sem intervalo de tempo entre pontos")
	}
}

// SERIE CORRELACIONADA PRECISA DAR FAIXA MAIS LARGA, e este e o unico teste
// que prende a correcao de autocorrelacao.
//
// Duas series identicas em tamanho, inclinacao e amplitude de ruido; so muda
// a frequencia. Mil leituras de uma curva lisa nao sao mil informacoes
// independentes, e sem nEfetivo a faixa sai igual nos dois casos -- foi
// exatamente isso que a validacao retroativa pegou (0,8% de cobertura).
func TestFaixaAlargaComResiduoCorrelacionado(t *testing.T) {
	solta, ok1 := TheilSen(serieFreq(120, 15*time.Minute, -20, -0.4, 0.5, 1.7))
	presa, ok2 := TheilSen(serieFreq(120, 15*time.Minute, -20, -0.4, 0.5, 0.08))
	if !ok1 || !ok2 {
		t.Fatal("TheilSen recusou serie valida")
	}
	if presa.Alta-presa.Baixa <= solta.Alta-solta.Baixa {
		t.Errorf("faixa correlacionada (%.4f) nao e mais larga que a solta (%.4f)",
			presa.Alta-presa.Baixa, solta.Alta-solta.Baixa)
	}

	// A comparacao acima sozinha nao prende a correcao: a serie lenta tambem
	// tem inclinacoes par a par mais espalhadas, entao a faixa sairia maior
	// mesmo sem nEfetivo. O que prende e comparar, na MESMA serie, com a
	// faixa que a formula nao corrigida daria.
	//
	// O fator 2 nao e calibragem: e para a comparacao nao depender do
	// arredondamento. Os indices saem com floor/ceil, sempre para fora, entao
	// a faixa nao corrigida ja fica um fio acima da referencia truncada e um
	// `<=` exato passaria sem correcao nenhuma. A correcao de verdade alarga
	// 24x nesta serie (n efetivo cai de 120 para 2), com folga ate o limite.
	if largura, ref := presa.Alta-presa.Baixa, senSemCorrecao(120, 0.08); largura <= 2*ref {
		t.Errorf("faixa corrigida (%.4f) nao chega ao dobro da sem correcao (%.4f)", largura, ref)
	}

	// E A CORRECAO NUNCA PODE ESTREITAR. Residuo alternante da r1 negativo, e
	// n(1-r1)/(1+r1) fica MAIOR que n -- aproveitar isso encolheria a faixa e
	// tornaria a afirmacao mais forte a partir de ruido que trocou de sinal.
	// O grampo min(.., n) existe para isso; sem ele este caso passa.
	if largura, ref := solta.Alta-solta.Baixa, senSemCorrecao(120, 1.7); largura < ref {
		t.Errorf("faixa (%.4f) ficou mais estreita que a sem correcao (%.4f)", largura, ref)
	}
}

// senSemCorrecao devolve a largura que a faixa de Sen teria com o n BRUTO, na
// mesma serie -- a referencia contra a qual se mede se a correcao fez efeito.
// A formula esta escrita aqui de novo de proposito: e o comportamento antigo
// servindo de baliza, e nao um espelho do codigo em teste, que usa n efetivo.
func senSemCorrecao(n int, freq float64) float64 {
	v := inclinacoesDe(serieFreq(n, 15*time.Minute, -20, -0.4, 0.5, freq))
	nf := float64(n)
	meia := z975 * math.Sqrt(nf*(nf-1)*(2*nf+5)/18) / (2 * (nf * (nf - 1) / 2))
	return quantil(v, 0.5+meia) - quantil(v, 0.5-meia)
}

// Sem o piso de 2 em nEfetivo, n(1-r1)/(1+r1) com r1 perto de 1 cai abaixo de
// 1, `nf*(nf-1)` fica NEGATIVO, e a raiz vira NaN. NaN atravessa toda
// comparacao como falso -- inclusive o portao `Alta >= 0`, que e o que impede
// afirmar secagem onde nao ha. Uma faixa NaN nao recusa nada.
func TestFaixaNaoViraNaNEmSerieMuitoCorrelacionada(t *testing.T) {
	incl, ok := TheilSen(serieFreq(200, 15*time.Minute, -20, -0.4, 1.5, 0.01))
	if !ok {
		t.Fatal("TheilSen recusou serie valida")
	}
	if math.IsNaN(incl.Baixa) || math.IsNaN(incl.Central) || math.IsNaN(incl.Alta) {
		t.Fatalf("faixa com NaN: [%v, %v, %v]", incl.Baixa, incl.Central, incl.Alta)
	}
	if !(incl.Baixa <= incl.Central && incl.Central <= incl.Alta) {
		t.Errorf("faixa fora de ordem: %.4f <= %.4f <= %.4f", incl.Baixa, incl.Central, incl.Alta)
	}
}

// ------------------------------------------------------- escala e segmentacao

// MAD e nao desvio padrao, e o teste mede as duas escalas na MESMA serie.
//
// A JANELA E DE 48 H COM DUAS IRRIGACOES, que e o cenario de producao: quem
// irriga de manha e de tarde tem dois eventos na janela que o servidor olha.
// Com os eventos RAROS as duas escalas funcionam -- foi assim que a primeira
// versao deste teste falhou, e a honestidade aqui e dizer onde o ganho
// realmente esta. Com dois eventos em 48 pontos o desvio padrao ja foi
// inflado pelos proprios saltos e o limite subiu acima deles: o detector
// ficaria cego exatamente para o que existe para ver. O MAD nao se move
// enquanto os eventos forem minoria das diferencas.
func TestEscalaDoRuidoNaoInflaComOEvento(t *testing.T) {
	p := desloca(desloca(serie(48, time.Hour, -20, -0.3, 0.2), 16, 14), 32, 14)

	mad := EscalaDoRuido(p)
	dp := desvioPadraoDasDiferencas(p)
	salto := p[32].KPa - p[31].KPa

	if kEvento*mad >= salto {
		t.Errorf("limite por MAD (%.3f) nao enxerga o salto de %.3f", kEvento*mad, salto)
	}
	if kEvento*dp < salto {
		t.Fatalf("premissa do teste falhou: o desvio padrao (%.3f) tambem enxergaria o salto de %.3f",
			kEvento*dp, salto)
	}
	// E o detector achando o segundo evento apesar do primeiro estar na
	// janela -- que e o que a escala robusta compra na pratica.
	if i := InicioDaSecagem(p); i != 32 {
		t.Errorf("inicio = %d, esperado 32", i)
	}
}

func TestInicioDaSecagemAchaAIrrigacao(t *testing.T) {
	p := desloca(serie(60, time.Hour, -20, -0.3, 0.2), 30, 14)
	if i := InicioDaSecagem(p); i != 30 {
		t.Errorf("inicio = %d, esperado 30 (primeiro ponto depois da irrigacao)", i)
	}
}

func TestInicioDaSecagemPegaAUltimaIrrigacao(t *testing.T) {
	// Duas chegadas de agua. O trecho corrente comeca na SEGUNDA -- usar a
	// primeira misturaria uma secagem inteira ja encerrada na conta.
	p := desloca(desloca(serie(80, time.Hour, -20, -0.3, 0.2), 20, 12), 55, 12)
	if i := InicioDaSecagem(p); i != 55 {
		t.Errorf("inicio = %d, esperado 55", i)
	}
}

func TestInicioDaSecagemNaoInventaEventoEmSerieLimpa(t *testing.T) {
	if i := InicioDaSecagem(serie(60, time.Hour, -20, -0.3, 0.2)); i != 0 {
		t.Errorf("inicio = %d numa serie sem irrigacao; esperado 0", i)
	}
}

// Serie sem ruido nenhum tem MAD zero. A versao anterior saia cedo nesse caso
// e engolia a irrigacao inteira -- o MAD e tao bom em ignorar um valor unico
// que ignorava o unico valor que interessava.
func TestInicioDaSecagemAchaOSaltoMesmoSemRuido(t *testing.T) {
	p := desloca(serie(60, time.Hour, -20, -0.3, 0), 30, 14)
	if i := InicioDaSecagem(p); i != 30 {
		t.Errorf("inicio = %d em serie sem ruido; esperado 30", i)
	}
}

// -------------------------------------------------------------------- Prever

func TestPreverAcertaOInstanteConhecido(t *testing.T) {
	// Sem ruido a conta e fechada: ultimo ponto em t0+47h vale
	// -20 - 0.2*47 = -29.4; faltam 10.6 kPa a 0.2 kPa/h = 53 h.
	p, err := Prever(serie(48, time.Hour, -20, -0.2, 0), -40)
	if err != OK {
		t.Fatalf("recusou serie boa: %q", err)
	}
	if d := math.Abs(p.Em.Hours() - 53); d > 0.1 {
		t.Errorf("Em = %v, esperado ~53h", p.Em)
	}
	if !(p.Cedo <= p.Em && p.Em <= p.Tarde) {
		t.Errorf("faixa fora de ordem: %v <= %v <= %v", p.Cedo, p.Em, p.Tarde)
	}
	if p.Inclinacao >= 0 {
		t.Errorf("inclinacao %.4f devia ser negativa", p.Inclinacao)
	}
}

// A FAIXA VAI DO MAIS RAPIDO AO MAIS LENTO, e nesta ordem.
//
// Inclinacao mais negativa e secagem mais rapida, logo chegada mais CEDO ao
// limiar. Trocar Baixa por Alta aqui inverteria a frase da tela -- "entre 4 e
// 2 dias" -- e, pior que o texto estranho, a ponta otimista viraria pessimista
// e quem programa a irrigacao pelo limite inferior programaria pelo superior.
//
// Precisa de serie COM ruido: sem ruido as tres inclinacoes coincidem e a
// troca fica invisivel, que foi como este furo passou pela primeira rodada.
func TestPreverFaixaVaiDoMaisRapidoAoMaisLento(t *testing.T) {
	p, err := Prever(serie(60, 30*time.Minute, -15, -0.4, 0.8), -40)
	if err != OK {
		t.Fatalf("recusou serie boa: %q", err)
	}
	if !(p.Cedo < p.Em && p.Em < p.Tarde) {
		t.Errorf("faixa fora de ordem: Cedo=%v Em=%v Tarde=%v", p.Cedo, p.Em, p.Tarde)
	}
}

// A previsao nao pode enxergar ANTES da ultima irrigacao: la a tensao estava
// mais negativa, e uma reta atravessando o salto mede a media entre encher e
// secar, que nao descreve nenhum dos dois.
func TestPreverNaoOlhaAlemDaIrrigacao(t *testing.T) {
	p := desloca(serie(80, time.Hour, -20, -0.3, 0.2), 40, 16)
	r, err := Prever(p, -45)
	if err != OK {
		t.Fatalf("recusou serie boa: %q", err)
	}
	if !r.Desde.Equal(p[40].T) {
		t.Errorf("Desde = %v, esperado o ponto apos a irrigacao (%v)", r.Desde, p[40].T)
	}
	if r.Pontos != 40 {
		t.Errorf("usou %d pontos; o trecho de secagem tem 40", r.Pontos)
	}
	// Se o salto entrasse no ajuste, a mistura de +16 kPa com a queda deixaria
	// a inclinacao bem longe de -0.3.
	if math.Abs(r.Inclinacao-(-0.3)) > 0.03 {
		t.Errorf("inclinacao = %.4f; a irrigacao vazou para dentro do ajuste", r.Inclinacao)
	}
}

func TestPreverIgnoraLeituraAbsurda(t *testing.T) {
	limpa := serie(48, time.Hour, -20, -0.3, 0.2)
	sujo := append([]Ponto(nil), limpa...)
	sujo[10].KPa = -95

	bom, err1 := Prever(limpa, -45)
	ruim, err2 := Prever(sujo, -45)
	if err1 != OK || err2 != OK {
		t.Fatalf("recusas inesperadas: %q / %q", err1, err2)
	}
	if d := math.Abs(ruim.Em.Hours() - bom.Em.Hours()); d > 2 {
		t.Errorf("uma leitura absurda moveu a previsao em %.1f h (%v -> %v)", d, bom.Em, ruim.Em)
	}
}

// AS RECUSAS SAO A MAIOR PARTE DO VALOR DO PACOTE, entao sao a maior parte do
// teste. Cada linha e um jeito de a serie nao sustentar uma previsao.
func TestPreverRecusa(t *testing.T) {
	casos := []struct {
		nome   string
		pontos []Ponto
		limiar float64
		quer   Motivo
	}{
		{
			"talhao sem faixa configurada (NULL vira NaN)",
			serie(48, time.Hour, -20, -0.3, 0.2), math.NaN(), SemFaixa,
		},
		{
			"limiar zero ou positivo nao e tensao de solo",
			serie(48, time.Hour, -20, -0.3, 0.2), 0, SemFaixa,
		},
		{
			"serie curta demais para uma mediana valer algo",
			serie(8, time.Hour, -20, -0.3, 0.2), -45, PoucosPontos,
		},
		{
			"limiar ja atingido e estado atual, nao previsao",
			serie(48, time.Hour, -20, -0.3, 0.2), -10, JaUltrapassado,
		},
		{
			"solo parado: a faixa da inclinacao alcanca zero",
			serie(48, time.Hour, -20, 0, 0.3), -45, NaoSeca,
		},
		{
			"solo umedecendo: inclinacao positiva",
			serie(48, time.Hour, -40, 0.3, 0.2), -60, NaoSeca,
		},
		{
			"irrigacao recente deixou trecho de secagem curto",
			desloca(serie(60, time.Hour, -20, -0.3, 0.2), 54, 14), -45, TrechoCurto,
		},
		{
			"trecho longo em pontos mas curto em tempo",
			serie(40, time.Minute, -20, -0.3, 0.05), -45, TrechoCurto,
		},
		{
			"secagem lenta demais: o limiar fica muito alem do observado",
			serie(48, time.Hour, -20, -0.02, 0.05), -60, HorizonteLonge,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if _, err := Prever(c.pontos, c.limiar); err != c.quer {
				t.Errorf("motivo = %q, esperado %q", err, c.quer)
			}
		})
	}
}

// Ordem invertida vira dt negativo e inclinacao com o sinal trocado -- "solo
// secando" lido em cima de solo que umedece. Ordenar aqui dentro esconderia o
// defeito do chamador; recusar o expoe.
func TestPreverRecusaSerieForaDeOrdem(t *testing.T) {
	p := serie(48, time.Hour, -20, -0.3, 0.2)
	p[20], p[21] = p[21], p[20]
	if _, err := Prever(p, -45); err != Desordenada {
		t.Errorf("motivo = %q, esperado %q", err, Desordenada)
	}
}

// ------------------------------------------------------------- subamostragem

func TestSubamostrarPreservaPontasEEDeterministico(t *testing.T) {
	p := serie(3000, time.Minute, -20, -0.3, 0.2)
	a := subamostrar(p, maxPontosAjuste)

	if len(a) != maxPontosAjuste {
		t.Errorf("len = %d, esperado %d", len(a), maxPontosAjuste)
	}
	// A ponta direita e o "agora" de onde a extrapolacao parte; perde-la
	// deslocaria toda a previsao.
	if !a[0].T.Equal(p[0].T) || !a[len(a)-1].T.Equal(p[len(p)-1].T) {
		t.Error("subamostragem perdeu uma das pontas")
	}
	for i, q := range subamostrar(p, maxPontosAjuste) {
		if !q.T.Equal(a[i].T) {
			t.Fatalf("subamostragem nao e deterministica no indice %d", i)
		}
	}
}

// Serie longa continua dando a mesma resposta que a curta equivalente: a
// subamostragem e um teto de custo, nao uma troca de metodo.
func TestSubamostragemNaoMudaAResposta(t *testing.T) {
	longa, _ := TheilSen(serie(2000, time.Minute, -20, -0.3, 0.1))
	curta, _ := TheilSen(serie(200, 10*time.Minute, -20, -0.3, 0.1))
	if math.Abs(longa.Central-curta.Central) > 0.01 {
		t.Errorf("subamostrada %.4f vs curta %.4f", longa.Central, curta.Central)
	}
}

// ------------------------------------------------------ so para os testes

// inclinacoesDe e quantil reconstroem a distribuicao par a par no teste, para
// poder medir a faixa em quantil sem a correcao de autocorrelacao.
func inclinacoesDe(p []Ponto) []float64 {
	var v []float64
	for i := range p {
		for j := i + 1; j < len(p); j++ {
			if dt := p[j].T.Sub(p[i].T).Hours(); dt > 0 {
				v = append(v, (p[j].KPa-p[i].KPa)/dt)
			}
		}
	}
	sort.Float64s(v)
	return v
}

func quantil(ordenado []float64, q float64) float64 {
	i := int(q * float64(len(ordenado)))
	return ordenado[min(max(i, 0), len(ordenado)-1)]
}

// minimosQuadrados existe para o teste poder MEDIR o que se ganhou ao nao usar
// OLS, em vez de so afirmar em comentario.
func minimosQuadrados(p []Ponto) float64 {
	n := float64(len(p))
	var sx, sy, sxy, sxx float64
	for _, q := range p {
		x := q.T.Sub(p[0].T).Hours()
		sx, sy, sxy, sxx = sx+x, sy+q.KPa, sxy+x*q.KPa, sxx+x*x
	}
	return (n*sxy - sx*sy) / (n*sxx - sx*sx)
}

func desvioPadraoDasDiferencas(p []Ponto) float64 {
	d := diferencas(p)
	var m float64
	for _, v := range d {
		m += v
	}
	m /= float64(len(d))
	var s float64
	for _, v := range d {
		s += (v - m) * (v - m)
	}
	return math.Sqrt(s / float64(len(d)))
}
