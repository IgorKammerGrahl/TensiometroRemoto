// Package previsao estima quando a tensao de agua no solo alcanca um limiar.
//
// A SAIDA DESTE PACOTE E UMA FRASE QUE MANDA ALGUEM IRRIGAR OU NAO IRRIGAR.
// Por isso ele foi escrito para se CALAR com facilidade: quase todo o codigo
// abaixo e motivo para nao responder, e so uma pequena parte e a conta.
// Devolver "sem previsao" custa uma linha cinza na tela; devolver um numero
// que a serie nao sustenta custa uma lavoura irrigada tarde.
//
// Pacote separado de `telemetria` de proposito. La o TestMain faz os.Exit(0)
// quando TEST_DATABASE_URL nao esta definida -- o pacote inteiro e pulado,
// e funcao pura junto. Aqui nao ha banco, entao o teste roda sempre, que e a
// diferenca entre ter teste e ter arquivo de teste.
//
// METODO: estimador de Theil-Sen (Theil 1950; Sen 1968), a mediana das
// inclinacoes par a par. A escolha nao e estilistica. Minimos quadrados
// minimiza o QUADRADO do residuo, entao uma unica leitura ruim -- e o ADC do
// no entrega leitura ruim -- entorta a reta inteira e, o que e pior, entorta
// em silencio. A mediana das inclinacoes so se move se MAIS DA METADE dos
// pares estiver errada. O ponto de ruptura e 29%, contra 0% do OLS.
//
// De brinde, Sen (1968) da um intervalo de confianca nao-parametrico a partir
// da propria distribuicao das inclinacoes -- sem supor residuo normal, que
// esta serie nao tem.
//
// AUTOCORRELACAO: a variancia de Sen supoe observacoes INDEPENDENTES, e
// secagem de solo nao e -- a leitura de agora e quase a de quinze minutos
// atras. Sem correcao o intervalo sai absurdamente estreito, e isso nao e
// teoria: na validacao retroativa a faixa nao corrigida continha o valor real
// em 1 de 126 janelas. Uma faixa que erra 99% das vezes e pior que faixa
// nenhuma, porque parece precisao.
//
// A correcao esta em nEfetivo: substitui n pelo tamanho amostral efetivo
// n(1-r1)/(1+r1), com r1 medido nos RESIDUOS em torno da propria reta. E a
// correcao classica de Bayley & Hammersley (1946), a mesma ideia que Hamed &
// Rao (1998) aplicam a Mann-Kendall em serie hidrologica.
//
// E MESMO CORRIGIDA A FAIXA NAO E UM INTERVALO DE PREVISAO. Isto foi medido,
// nao suposto: na validacao retroativa (validacao_test.go) a faixa corrigida
// contem o instante real em 0,8% das janelas, e 99,2% dos erros caem do MESMO
// lado.
//
// Parte desses 0,8% e artefato da fixture, que nao tem ruido de medicao
// nenhum: injetando ruido gaussiano realista a cobertura sobe para 8,7%
// (0,25 kPa) e 36% (1,0 kPa), com o erro central intacto. Ou seja, a faixa e
// estreita demais em parte porque o dado e limpo demais. O resto e vies. Erro de um lado so nao e ruido, e VIES DE MODELO -- a secagem
// desacelera, uma reta ajustada nas ultimas 48 h e mais ingreme que o futuro,
// e a previsao chega cedo. Intervalo sobre a INCLINACAO nao cobre erro do
// MODELO, por mais largo que fique.
//
// O que se pode afirmar hoje, e so isto:
//
//   - Em errou 0,6 h de mediana -- mas o horizonte mediano era 3,2 h (maximo
//     6,5 h), o que da 18% de erro RELATIVO. E ler o absoluto sem o horizonte
//     que faz o metodo parecer melhor do que se mediu.
//   - O vies aponta para o lado SEGURO: previsao cedo faz irrigar cedo.
//
// E ESTE METODO NUNCA FOI EXERCITADO NO REGIME PARA O QUAL EXISTE. A fixture
// vai de -10 a -30 kPa em ~6 h; solo de verdade leva DIAS nesse trecho. Entao
// toda previsao conferida acima foi de 3 h, e o sistema existe para dizer "seu
// talhao chega no limiar em dois dias". A constante de tempo da fixture esta
// fora por uma ordem de grandeza, e o resultado de 18% nao se transporta.
//   - Cedo e Tarde sao dispersao das inclinacoes observadas. Servem de portao
//     ("a faixa alcanca zero? entao nao afirme") e nao de promessa. Chamar
//     isso de "entre X e Y dias" na tela seria a mesma precisao inventada que
//     este projeto ja recusou em outros lugares.
//
// Calibrar a faixa como intervalo de previsao exige serie de CAMPO, que o
// projeto ainda nao tem -- o tensiometro quebrou antes do ensaio final.
// validacao_test.go e o arnes pronto para refazer essa medida quando tiver.
package previsao

import (
	"math"
	"slices"
	"sort"
	"time"
)

// Ponto e uma leitura ja validada. KPa e negativo, e MAIS NEGATIVO E MAIS
// SECO -- o sinal e a origem da maior parte dos erros possiveis aqui, entao
// cada comparacao abaixo esta comentada com o que ela significa fisicamente.
type Ponto struct {
	T   time.Time
	KPa float64
}

// Motivo diz por que nao ha previsao. Vazio significa que ha.
//
// E tipo proprio, e nao bool, porque "nao da para prever" tem causas que a
// interface trata de formas diferentes: um talhao sem faixa pede configurar,
// um solo umedecendo nao pede nada, e poucos pontos pede esperar. Um bool
// colapsaria os tres na mesma linha cinza.
type Motivo string

const (
	OK Motivo = ""

	// SemFaixa: o talhao nao tem limiar configurado. Nao ha para onde prever.
	SemFaixa Motivo = "talhao sem faixa configurada"
	// PoucosPontos: serie curta demais para uma mediana significar algo.
	PoucosPontos Motivo = "leituras insuficientes"
	// Desordenada: o chamador entregou pontos fora de ordem cronologica.
	Desordenada Motivo = "serie fora de ordem"
	// JaUltrapassado: o limiar ja foi cruzado. Isso e estado atual, nao
	// previsao, e quem mostra o estado atual e outro caminho da interface.
	JaUltrapassado Motivo = "limiar ja atingido"
	// TrechoCurto: houve irrigacao ou chuva recente e o trecho de secagem
	// depois dela ainda nao tem extensao para sustentar uma reta.
	TrechoCurto Motivo = "trecho de secagem muito curto"
	// NaoSeca: a faixa da inclinacao inclui zero. Pode estar secando, pode
	// nao estar -- e afirmar qual dos dois seria inventar o sinal.
	NaoSeca Motivo = "sem secagem sustentada"
	// HorizonteLonge: a extrapolacao iria muito alem do que foi observado.
	// Prever dez dias a partir de seis horas nao e previsao, e ficcao.
	HorizonteLonge Motivo = "horizonte alem do observado"
)

// Previsao carrega a estimativa central e a dispersao em volta dela.
//
// LEIA A NOTA DE VIES NO CABECALHO ANTES DE MANDAR Cedo e Tarde PARA A TELA.
// A faixa nao foi validada como intervalo de previsao; ela e dispersao das
// inclinacoes observadas, e existe sobretudo para o portao de recusa.
// Apresentar "entre X e Y dias" a partir dela afirmaria uma precisao que a
// serie nao sustenta -- o mesmo motivo que tirou uma contagem da interface
// ("contar seria afirmar uma precisao que o dado nao tem", App/README.md).
type Previsao struct {
	// Em, Cedo e Tarde sao contados A PARTIR DA ULTIMA LEITURA, nao de agora.
	// A diferenca importa quando o no esta mudo ha horas: o envelhecimento e
	// do chamador, que ja sabe classificar leitura velha. Somar "agora" aqui
	// dentro duplicaria essa regra num segundo lugar.
	Em    time.Duration
	Cedo  time.Duration
	Tarde time.Duration

	// Inclinacao central, em kPa por hora. Negativa: o solo seca.
	Inclinacao float64
	// Desde marca o inicio do trecho de secagem usado -- ou seja, a ultima
	// vez que agua chegou. Serve para a tela dizer em cima de que a conta foi
	// feita, em vez de apresentar um numero sem procedencia.
	Desde  time.Time
	Pontos int
}

const (
	// Minimo de pontos para uma mediana de inclinacoes significar algo. Abaixo
	// disso a "robustez" de Theil-Sen e ilusoria: com 6 pontos, 15 pares, tres
	// leituras ruins ja sao 20% e deslocam a mediana.
	minPontos = 12

	// Trecho curto ajusta reta em cima de ruido. Tres horas e o minimo em que
	// a secagem real supera a resolucao do sensor no pior caso da bancada.
	minDuracao = 3 * time.Hour

	// Extrapolar no maximo o DOBRO do que se observou. Nao ha teoria por tras
	// do 2: ha a recusa a afirmar muito alem da evidencia. O numero e o botao
	// para girar quando houver series longas de campo para calibra-lo.
	fatorHorizonte = 2.0

	// Salto acima de 5 desvios em relacao a deriva corrente e agua chegando,
	// nao ruido. Com residuo gaussiano, |d| > 5s tem probabilidade ~6e-7 por
	// amostra; mesmo em milhares de amostras continua raro. O valor e alto de
	// proposito: falso POSITIVO aqui descarta historico bom e encurta o
	// trecho, o que leva a recusar por TrechoCurto -- o lado seguro.
	kEvento = 5.0

	// Theil-Sen e O(n^2) em pares. Com 48 h a 60 s sao 2880 pontos, 4,1 M
	// pares -- ~33 MB so para ordenar, dentro do caminho de uma requisicao.
	//
	// ponytail: subamostragem uniforme ate 400 pontos (80 mil pares, trivial).
	// Uniforme e nao aleatoria para a mesma serie dar sempre a mesma resposta;
	// previsao que muda sozinha entre dois F5 nao e crivel. Se um dia a
	// precisao da subamostra incomodar, o caminho e o estimador de Theil-Sen
	// randomizado em O(n log n), nao subir este teto.
	maxPontosAjuste = 400

	// z de 97,5% para a faixa de Sen. Ver a ressalva de autocorrelacao no
	// cabecalho do pacote antes de chamar isto de "95% de confianca".
	z975 = 1.959964
)

// Prever devolve a previsao, ou o motivo de nao haver uma.
//
// A ORDEM DAS VERIFICACOES E A PROPRIA DEFESA, e nao arrumacao.
//
// Cada porta abaixo so e atravessada depois que a anterior garantiu o que ela
// pressupoe: nao adianta medir inclinacao de uma serie fora de ordem, nem
// segmentar uma serie curta demais, nem extrapolar antes de saber que ha
// secagem. Inverter duas delas nao quebra teste nenhum de imediato -- devolve
// numero plausivel a partir de premissa falsa, que e o modo de falha que este
// pacote inteiro existe para evitar.
//
// `limiar` e o kPa de destino (tipicamente kpa_alerta do talhao), negativo.
// `pontos` precisa vir em ordem cronologica crescente.
func Prever(pontos []Ponto, limiar float64) (Previsao, Motivo) {
	// 1. Ha destino? Faixa nao configurada e NULL no banco, e o chamador
	//    traduz NULL para NaN ou zero. Os dois caem aqui.
	if math.IsNaN(limiar) || limiar >= 0 {
		return Previsao{}, SemFaixa
	}

	// 2. Ha material? Antes de olhar a ordem, ver se ha o que ordenar.
	if len(pontos) < minPontos {
		return Previsao{}, PoucosPontos
	}

	// 3. A serie e crivel? Ordem fora do lugar significa erro do chamador --
	//    e ordenar aqui dentro o esconderia. Uma diferenca de tempo negativa
	//    vira inclinacao com o sinal trocado, ou seja, "secando" lido em cima
	//    de solo que umedece.
	for i := 1; i < len(pontos); i++ {
		if pontos[i].T.Before(pontos[i-1].T) {
			return Previsao{}, Desordenada
		}
	}

	// 4. O limiar ja foi cruzado? MAIS NEGATIVO E MAIS SECO, entao "atingiu"
	//    e atual <= limiar. Nao ha previsao a fazer sobre o que ja aconteceu.
	atual := pontos[len(pontos)-1]
	if atual.KPa <= limiar {
		return Previsao{}, JaUltrapassado
	}

	// 5. Sobre QUE trecho ajustar. Reta atravessando uma irrigacao mede a
	//    media entre encher e secar, que nao descreve nenhum dos dois.
	inicio := InicioDaSecagem(pontos)
	trecho := pontos[inicio:]
	if len(trecho) < minPontos {
		return Previsao{}, TrechoCurto
	}
	duracao := trecho[len(trecho)-1].T.Sub(trecho[0].T)
	if duracao < minDuracao {
		return Previsao{}, TrechoCurto
	}

	// 6. Qual a inclinacao, e ela e distinguivel de zero?
	incl, ok := TheilSen(trecho)
	if !ok {
		return Previsao{}, PoucosPontos
	}
	// Alta e o limite da faixa mais proximo de zero. Se ele alcanca zero, a
	// faixa contem "nao esta secando" -- e escolher o lado negativo seria
	// escolher a conclusao. Esta linha e o portao mais importante do arquivo:
	// sem ele, ruido puro vira previsao de irrigacao.
	if incl.Alta >= 0 {
		return Previsao{}, NaoSeca
	}

	// 7. A conta. `falta` e negativo (o limiar esta abaixo do atual) e a
	//    inclinacao tambem, entao o quociente e um tempo positivo.
	falta := limiar - atual.KPa
	horas := func(inclinacao float64) time.Duration {
		return time.Duration(falta / inclinacao * float64(time.Hour))
	}
	// Inclinacao mais negativa = seca mais rapido = chega antes. Baixa e a
	// mais negativa das duas, entao Baixa da Cedo.
	p := Previsao{
		Em:         horas(incl.Central),
		Cedo:       horas(incl.Baixa),
		Tarde:      horas(incl.Alta),
		Inclinacao: incl.Central,
		Desde:      trecho[0].T,
		Pontos:     len(trecho),
	}

	// 8. A extrapolacao cabe na evidencia? Por ultimo porque so agora existe
	//    um horizonte para comparar com a duracao observada.
	if p.Em > time.Duration(fatorHorizonte*float64(duracao)) {
		return Previsao{}, HorizonteLonge
	}
	return p, OK
}

// Inclinacao em kPa por hora. Negativa quando o solo seca.
type Inclinacao struct {
	Central float64
	// Baixa e Alta sao a faixa de Sen (1968), pelas estatisticas de ordem da
	// propria distribuicao de inclinacoes. Baixa <= Central <= Alta, e Baixa
	// e a mais negativa -- ou seja, a secagem mais rapida da faixa.
	Baixa, Alta float64
	Pares       int
}

// TheilSen devolve a mediana das inclinacoes par a par e a faixa de Sen.
//
// `ok` falso quando nao ha par com intervalo de tempo positivo -- serie de um
// ponto so, ou varias leituras com o mesmo carimbo.
func TheilSen(pontos []Ponto) (Inclinacao, bool) {
	amostra := subamostrar(pontos, maxPontosAjuste)
	n := len(amostra)
	if n < 2 {
		return Inclinacao{}, false
	}

	inclinacoes := make([]float64, 0, n*(n-1)/2)
	for i := range n {
		for j := i + 1; j < n; j++ {
			dt := amostra[j].T.Sub(amostra[i].T).Hours()
			// Carimbos iguais dariam divisao por zero, e um par sem tempo
			// entre as pontas nao informa inclinacao nenhuma.
			if dt <= 0 {
				continue
			}
			inclinacoes = append(inclinacoes, (amostra[j].KPa-amostra[i].KPa)/dt)
		}
	}
	if len(inclinacoes) == 0 {
		return Inclinacao{}, false
	}
	sort.Float64s(inclinacoes)

	central := mediana(slices.Clone(inclinacoes))

	// A FAIXA DE SEN EM QUANTIL, E NAO EM POSICAO ABSOLUTA.
	//
	// Sen poe os limites nas posicoes (N +- C)/2 da lista ordenada, com N o
	// numero de pares e C = z*sqrt(Var(S)). Escrito assim, os dois vem do
	// MESMO n, e trocar so o n de Var(S) pelo efetivo colapsa os indices em
	// cima da mediana -- faixa de largura zero, o oposto do que se queria.
	// Foi o primeiro jeito que tentei, e a validacao mostrou 0,0 h de largura.
	//
	// Dividindo por N, o limite vira um quantil: 0,5 +- C/(2N). Nessa forma o
	// n efetivo entra nos dois termos e a faixa ALARGA quando ha menos
	// informacao independente, que e a direcao certa. Com n bruto a expressao
	// reproduz a formula original.
	nf := nEfetivo(amostra, central)
	paresEf := nf * (nf - 1) / 2
	c := z975 * math.Sqrt(nf*(nf-1)*(2*nf+5)/18)

	// Sem teto em `meia`: quando a serie e tao correlacionada que a meia
	// largura passa de 0,5, os indices saem da lista e indiceValido grampeia
	// nas pontas -- a faixa inteira das inclinacoes observadas, que e o
	// desfecho certo. Ela entao alcanca zero e Prever recusa. Um `min(..,0.5)`
	// aqui daria exatamente o mesmo resultado e seria um ramo a mais sem
	// como testar.
	meia := c / (2 * paresEf)

	// Arredondamento PARA FORA nos dois lados: faixa igual ou mais larga que
	// a exata. Mais larga alcanca zero e faz recusar; mais estreita afirma.
	N := float64(len(inclinacoes))
	lo := indiceValido(int(math.Floor((0.5-meia)*N)), len(inclinacoes))
	hi := indiceValido(int(math.Ceil((0.5+meia)*N)), len(inclinacoes))

	return Inclinacao{
		Central: central,
		Baixa:   inclinacoes[lo],
		Alta:    inclinacoes[hi],
		Pares:   len(inclinacoes),
	}, true
}

// nEfetivo devolve o tamanho amostral corrigido pela autocorrelacao.
//
// Duas mil leituras de quinze em quinze minutos nao sao duas mil informacoes
// independentes sobre a inclinacao: sao uma curva lisa amostrada de perto.
// Tratar como se fossem e o que produz intervalo de confianca que nao contem
// nada. n(1-r1)/(1+r1) desconta isso, com r1 medido nos residuos em torno da
// reta ja estimada -- nos residuos e nao na serie crua, porque a serie crua
// tem r1 perto de 1 por causa da tendencia, que e justamente o que se quer
// estimar e nao descontar.
//
// ponytail: r1 de defasagem 1 em NUMERO de amostras, nao em tempo. Com
// cadencia irregular ou lacuna longa a defasagem deixa de ser constante e a
// correcao fica aproximada. O limite inferior de 2 e o que impede uma serie
// muito correlacionada de virar variancia zero -- e faixa de largura zero, que
// seria a afirmacao mais forte possivel saindo do dado mais fraco possivel.
func nEfetivo(pontos []Ponto, incl float64) float64 {
	n := len(pontos)
	if n < 3 {
		return float64(n)
	}
	// Intercepto de Theil-Sen: mediana dos residuos com a inclinacao fixada.
	t0 := pontos[0].T
	res := make([]float64, n)
	for i, p := range pontos {
		res[i] = p.KPa - incl*p.T.Sub(t0).Hours()
	}
	b := mediana(slices.Clone(res))
	var num, den float64
	for i := range res {
		res[i] -= b
		den += res[i] * res[i]
	}
	if den == 0 {
		return float64(n) // reta perfeita: nao ha residuo para correlacionar
	}
	for i := 1; i < n; i++ {
		num += res[i-1] * res[i]
	}
	// Os dois grampos fazem todo o trabalho de guarda, e por isso nao ha
	// nenhum `if` acima deles:
	//
	//   min(.., n) -- r1 negativo (residuo alternante) daria n efetivo MAIOR
	//     que n, o que estreitaria a faixa. Nao aproveitamos essa "sorte":
	//     estreitar e o lado perigoso.
	//   max(.., 2) -- r1 perto de 1 derruba o resultado abaixo de 1, e ai
	//     `nf*(nf-1)` fica negativo e a raiz de Var(S) vira NaN. NaN atravessa
	//     toda comparacao como falso, inclusive o portao `Alta >= 0`, e uma
	//     faixa NaN nao recusa nada. Tambem cobre r1 fora de [-1, 1], que este
	//     estimador nao garante.
	return min(max(float64(n)*(1-num/den)/(1+num/den), 2), float64(n))
}

// InicioDaSecagem devolve o indice do primeiro ponto depois da ultima vez que
// agua chegou -- irrigacao ou chuva.
//
// O evento e um salto de kPa EM DIRECAO A ZERO grande demais para ser ruido.
// A escala do ruido sai da propria serie (ver EscalaDoRuido), pelo mesmo
// motivo que serie.ts tira o ritmo da mediana dos intervalos observados em
// vez de supor bucket_s: constante escolhida a mao ja custou uma tela errada
// neste projeto.
//
// Compara-se a DIFERENCA de nivel, nao a taxa por hora. A pergunta e "chegou
// agua entre estas duas leituras?", que e sobre quanto o nivel mudou -- uma
// irrigacao durante uma lacuna de doze horas continua sendo uma irrigacao.
//
// ponytail: so o salto de uma amostra para a proxima. Um molhamento lento,
// espalhado por muitas amostras sem que nenhuma passe do limite, nao e
// detectado. O caminho, se aparecer, e soma acumulada com reinicio -- nao
// baixar kEvento, que so trocaria o caso perdido por falso positivo.
func InicioDaSecagem(pontos []Ponto) int {
	if len(pontos) < 3 {
		return 0
	}
	// MAD ZERO NAO PODE VIRAR ATALHO PARA "NAO HA EVENTO".
	//
	// MAD zero significa que mais da metade das diferencas e identica --
	// sensor quantizado, ou serie sem ruido. Sair cedo aqui, como a primeira
	// versao fazia, engoliria uma irrigacao inteira: o MAD e tao bom em
	// ignorar um valor unico que ignorava justamente o unico que interessa.
	// Entao o caminho continua, com o piso logo abaixo no lugar do atalho.
	difs := diferencas(pontos)
	centro := mediana(slices.Clone(difs))

	// O PISO EXISTE POR CAUSA DE PO DE PONTO FLUTUANTE, e nao por cautela
	// vaga. Numa serie perfeitamente linear as diferencas sao iguais a menos
	// de um ulp, o MAD da exatamente zero, e o teste vira "d > 0" -- ai o
	// arredondamento da ultima subtracao, na ordem de 1e-15, e promovido a
	// irrigacao. Foi assim que a primeira versao recusou uma serie limpa por
	// TrechoCurto.
	//
	// O piso e relativo a amplitude observada, nao um epsilon fixo: 1e-6 da
	// faixa da serie esta quinze ordens de grandeza acima do po e varias
	// ordens abaixo de qualquer chegada de agua real. E o `max` garante que
	// em serie com ruido de verdade quem manda continua sendo o MAD.
	limite := max(kEvento*EscalaDoRuido(pontos), 1e-6*amplitude(pontos))

	// De tras para frente: interessa a ULTIMA chegada de agua, porque e ela
	// que abre o trecho de secagem corrente.
	for i, d := range slices.Backward(difs) {
		if d-centro > limite {
			return i + 1 // primeiro ponto DEPOIS do salto
		}
	}
	return 0
}

// EscalaDoRuido estima o desvio das primeiras diferencas por MAD.
//
// MAD e nao desvio padrao: o desvio padrao seria inflado pelo proprio evento
// de irrigacao que queremos detectar, e o limite subiria acima do salto --
// o detector deixaria de ver exatamente o que existe para ver. E o mesmo
// argumento de serie.ts para mediana em vez de media.
//
// O 1,4826 torna o MAD estimador consistente do desvio sob residuo gaussiano,
// para que kEvento possa ser lido em "desvios".
func EscalaDoRuido(pontos []Ponto) float64 {
	if len(pontos) < 3 {
		return 0
	}
	difs := diferencas(pontos)
	centro := mediana(slices.Clone(difs))
	desvios := make([]float64, len(difs))
	for i, d := range difs {
		desvios[i] = math.Abs(d - centro)
	}
	return 1.4826 * mediana(desvios)
}

// ------------------------------------------------------------------ auxiliares

func amplitude(pontos []Ponto) float64 {
	menor, maior := pontos[0].KPa, pontos[0].KPa
	for _, p := range pontos[1:] {
		menor, maior = min(menor, p.KPa), max(maior, p.KPa)
	}
	return maior - menor
}

func diferencas(pontos []Ponto) []float64 {
	difs := make([]float64, len(pontos)-1)
	for i := 1; i < len(pontos); i++ {
		difs[i-1] = pontos[i].KPa - pontos[i-1].KPa
	}
	return difs
}

// mediana ORDENA o slice recebido. Quem precisa preservar a ordem copia antes.
func mediana(v []float64) float64 {
	sort.Float64s(v)
	n := len(v)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}

// subamostrar reduz a no maximo `max` pontos, uniformemente e preservando as
// duas pontas. Uniforme, nao aleatorio: a mesma serie precisa dar a mesma
// resposta em duas consultas seguidas.
func subamostrar(pontos []Ponto, max int) []Ponto {
	if len(pontos) <= max || max < 2 {
		return pontos
	}
	fora := make([]Ponto, 0, max)
	passo := float64(len(pontos)-1) / float64(max-1)
	for i := range max {
		fora = append(fora, pontos[int(math.Round(float64(i)*passo))])
	}
	return fora
}

func indiceValido(i, n int) int {
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}
