// devtoken gera o token de um no sensor e imprime o SQL de cadastro.
//
// O token em claro aparece uma unica vez, aqui. O banco guarda so o HMAC:
// se este valor se perder, gere outro -- nao ha como recupera-lo.
package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/igorkg/tcc/backend/internal/config"
	"github.com/igorkg/tcc/backend/internal/telemetria"
)

func main() {
	id := flag.String("id", "", "id do device (ex: tensio-01)")
	descricao := flag.String("descricao", "", "descricao do no")
	talhao := flag.String("talhao", "", "uuid do talhao (opcional)")
	flag.Parse()

	config.Carregar()
	segredo := config.SegredoObrigatorio("TELEMETRIA_TOKEN_SECRET",
		"e a chave do HMAC do token: sem ela o hash impresso no INSERT nao casaria\n  com nenhum token, e o no nunca autenticaria.")
	if *id == "" || *descricao == "" {
		fatal("uso: devtoken -id tensio-01 -descricao \"no do talhao norte\" [-talhao <uuid>]")
	}

	bruto := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bruto); err != nil {
		fatal("falha ao gerar aleatorio: " + err.Error())
	}
	token := base64.RawURLEncoding.EncodeToString(bruto)

	valorTalhao := "NULL"
	if *talhao != "" {
		valorTalhao = "'" + strings.ReplaceAll(*talhao, "'", "''") + "'"
	}

	// ponytail: imprime o SQL em vez de conectar. Uma pessoa revisa antes de
	// aplicar, e a ferramenta nao precisa de credencial de banco.
	fmt.Printf(`device_id : %s
token     : %s
            ^ grave no firmware agora; nao e recuperavel depois

INSERT INTO devices (id, descricao, talhao_id, token_hash)
VALUES ('%s', '%s', %s, '%s');
`, *id, token, *id, strings.ReplaceAll(*descricao, "'", "''"), valorTalhao,
		telemetria.HashToken(segredo, token))
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "erro: "+msg)
	os.Exit(1)
}
