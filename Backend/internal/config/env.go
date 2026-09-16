// config carrega o .env da raiz de Backend/ e reporta variavel obrigatoria
// ausente de um jeito que diga como resolver.
//
// Nao usa godotenv: o parser cabe aqui, e o README promete tres dependencias
// para quem le o repositorio. Formato aceito: KEY=VALOR por linha, `#` no
// inicio da linha e comentario, aspas em volta do valor sao opcionais.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NomeArquivo e procurado a partir do diretorio corrente subindo ate a raiz
// do modulo, para que `go run ./cmd/api` funcione de dentro de Backend/ ou
// de um subdiretorio.
const NomeArquivo = ".env"

// Carregar le o .env e exporta o que ainda nao estiver no ambiente.
//
// O ambiente vence o arquivo, e nao o contrario: em producao a configuracao
// vem do ambiente, e um .env esquecido no disco nao pode sobrescrever a
// variavel que o orquestrador injetou. Arquivo ausente nao e erro -- e
// exatamente o caso de producao.
func Carregar() {
	caminho, ok := procurar()
	if !ok {
		return
	}
	f, err := os.Open(caminho)
	if err != nil {
		return // ponytail: ilegivel e o mesmo que ausente; a variavel faltando falha depois, com mensagem melhor
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		chave, valor, ok := analisar(sc.Text())
		if !ok {
			continue
		}
		if _, definida := os.LookupEnv(chave); definida {
			continue
		}
		os.Setenv(chave, valor)
	}
}

// analisar quebra uma linha em chave e valor. Devolve ok=false para linha
// vazia, comentario ou linha sem `=`.
func analisar(linha string) (chave, valor string, ok bool) {
	linha = strings.TrimSpace(linha)
	if linha == "" || strings.HasPrefix(linha, "#") {
		return "", "", false
	}
	linha = strings.TrimPrefix(linha, "export ")

	// Corta no primeiro `=` so: DATABASE_URL e o segredo em base64 podem
	// conter `=` no valor, e quebrar em todos truncaria os dois.
	chave, valor, ok = strings.Cut(linha, "=")
	if !ok {
		return "", "", false
	}
	chave = strings.TrimSpace(chave)
	if chave == "" {
		return "", "", false
	}
	valor = strings.TrimSpace(valor)
	if len(valor) >= 2 && (valor[0] == '"' || valor[0] == '\'') && valor[len(valor)-1] == valor[0] {
		valor = valor[1 : len(valor)-1]
	}
	return chave, valor, true
}

// procurar sobe do diretorio corrente ate achar um .env, parando na raiz do
// sistema de arquivos.
func procurar() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		caminho := filepath.Join(dir, NomeArquivo)
		if info, err := os.Stat(caminho); err == nil && !info.IsDir() {
			return caminho, true
		}
		pai := filepath.Dir(dir)
		if pai == dir {
			return "", false
		}
		dir = pai
	}
}

// Obrigatoria devolve o valor da variavel ou encerra o processo explicando
// o que faltou e como resolver.
//
// proposito responde "para que serve" em uma linha; entra na mensagem para
// que quem bate no erro nao precise ler o codigo para entender o que a
// variavel controla.
func Obrigatoria(nome, proposito string) string {
	if v := os.Getenv(nome); v != "" {
		return v
	}
	// Sintoma antes da causa: quem le isso viu o processo morrer, nao viu
	// uma variavel faltar.
	fmt.Fprintf(os.Stderr, `erro: o processo nao subiu.

  A causa e %s ausente -- %s

  Resolva com:
      cp .env.example .env     (a partir da raiz de Backend/)

  O .env.example ja vem com os valores de desenvolvimento local; so os
  segredos precisam ser gerados, e ele diz como. Ver README, secao
  "Configuracao".
`, nome, proposito)
	os.Exit(1)
	return ""
}

// SegredoMinimo e o comprimento minimo aceito para um segredo.
//
// 32 caracteres, e nao "32 bytes de entropia", porque o que chega aqui e uma
// string de um .env e nao ha como medir entropia de uma string: "aaaa...a" e
// base64 de /dev/urandom tem o mesmo comprimento. O piso nao mede forca, ele
// so pega o erro que de fato acontece -- o valor de exemplo que ficou, o
// "troque-me", o "segredo123" digitado com pressa na vespera da apresentacao.
//
// O numero vem do que o README manda gerar: `openssl rand -base64 32` sai com
// 44 caracteres, entao 32 aceita o valor correto com folga e recusa qualquer
// coisa que caiba na memoria de alguem.
const SegredoMinimo = 32

// SegredoObrigatorio e Obrigatoria com piso de comprimento, para variavel que
// e chave criptografica.
//
// O motivo de existir separado: TELEMETRIA_TOKEN_SECRET e a chave do HMAC que
// produz devices.token_hash. Um segredo adivinhavel deixa qualquer um forjar
// o token de um no e inserir leituras em nome dele -- e a falha e silenciosa,
// porque tudo continua funcionando normalmente.
//
// Recusa subir, e nao avisa: o aviso de boot seria lido uma vez, no dia em que
// o valor foi posto, por quem ja sabia que era provisorio.
func SegredoObrigatorio(nome, proposito string) string {
	v := Obrigatoria(nome, proposito)
	if len(v) < SegredoMinimo {
		fmt.Fprintf(os.Stderr, `erro: o processo nao subiu.

  %s tem %d caracteres; o minimo e %d.

  %s

  Um segredo curto o bastante para ser adivinhado deixa forjar credencial
  sem deixar rastro -- e nada para de funcionar quando isso acontece.

  Gere um valor novo com:
      openssl rand -base64 32

  ATENCAO: trocar este valor invalida TODOS os tokens ja emitidos. Cada no
  precisa ser reprovisionado com um token novo (ver Circuito/README.md).
`, nome, len(v), SegredoMinimo, proposito)
		os.Exit(1)
	}
	return v
}
