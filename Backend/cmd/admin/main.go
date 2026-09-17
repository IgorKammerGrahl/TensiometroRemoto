// admin provisiona o que a interface nao cria: usuarios, talhoes,
// concessoes e a associacao de um no fisico a um talhao.
//
// Um binario com subcomandos, e nao um por tarefa: sao quatro operacoes
// raras que compartilham conexao, validacao e mensagens de erro.
//
// Diferente de cmd/devtoken, que imprime SQL para revisao humana, admin
// escreve direto no banco: bcrypt precisa rodar em Go e as concessoes
// dependem de ids que so o banco conhece. Por isso exige DATABASE_URL.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/igorkg/tcc/backend/internal/config"
	"github.com/igorkg/tcc/backend/internal/telemetria"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const uso = `uso: admin <subcomando> [flags]

  usuario  -email=<e-mail> -nome=<nome>
           cria o usuario e imprime a senha gerada uma unica vez

  talhao   -nome=<nome>
           cria o talhao e imprime o uuid; as faixas de atencao sao
           configuradas depois pela interface (UC04)

  conceder -email=<e-mail> -talhao=<uuid>
           da ao usuario acesso ao talhao e a todos os nos instalados nele

  device   -id=<id> -descricao=<texto> [-talhao=<uuid>]
           cadastra o no, imprime o token uma unica vez e ja o associa ao
           talhao se -talhao vier. Exige TELEMETRIA_TOKEN_SECRET.

  calibracao -id=<id> -device=<id> -v-zero=<V> -k=<V/kPa>
           -divisor=<fator> -vdd=<mV> [-r2=] [-rmse=] [-nota=]
           registra o ensaio de bancada. Sem isso o no ate autentica, mas
           toda leitura que ele enviar e recusada por calibration_id ausente.

  associar -device=<id> -talhao=<uuid>
           poe um no ja cadastrado dentro de um talhao. Enquanto o no nao
           estiver em nenhum, ele nao aparece para usuario nenhum.

Variavel de ambiente obrigatoria: DATABASE_URL (ver .env.example)
`

var subcomandos = map[string]func(context.Context, *pgxpool.Pool, []string) error{
	"usuario":    criarUsuario,
	"talhao":     criarTalhao,
	"conceder":   conceder,
	"device":     criarDevice,
	"calibracao": criarCalibracao,
	"associar":   associar,
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, uso)
		os.Exit(2)
	}

	// O subcomando e resolvido ANTES de tocar no banco: `admin -h` tem de
	// imprimir esta ajuda, e nao morrer em DATABASE_URL sem nunca dizer que
	// subcomandos existem.
	rodar, ok := subcomandos[os.Args[1]]
	if !ok {
		saida := os.Stderr
		codigo := 2
		if arg := os.Args[1]; arg == "-h" || arg == "--help" || arg == "help" {
			saida, codigo = os.Stdout, 0
		}
		fmt.Fprint(saida, uso)
		os.Exit(codigo)
	}

	config.Carregar()
	url := config.Obrigatoria("DATABASE_URL",
		"e a string de conexao com o Postgres: admin escreve direto no banco\n  (bcrypt em Go, ids que so o banco conhece).")

	ctx := context.Background()
	pool, err := telemetria.NovoPool(ctx, url)
	if err != nil {
		fatal("conexao com o banco falhou: " + err.Error())
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fatal("ping no banco falhou: " + err.Error())
	}

	if err := rodar(ctx, pool, os.Args[2:]); err != nil {
		fatal(err.Error())
	}
}

// ------------------------------------------------------------- usuario

func criarUsuario(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("usuario", flag.ExitOnError)
	email := fs.String("email", "", "e-mail de login")
	nome := fs.String("nome", "", "nome do produtor")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" || *nome == "" {
		return errors.New("uso: admin usuario -email=produtor@exemplo.com -nome=\"Nome do Produtor\"")
	}

	// Senha gerada, nao recebida por flag: senha em flag fica no historico do
	// shell e na lista de processos. Aparece uma unica vez, aqui -- o banco
	// guarda so o bcrypt, que e irreversivel. Perdeu, cria outra.
	senha, err := senhaAleatoria()
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(senha), telemetria.CustoBcrypt)
	if err != nil {
		return err
	}

	var id string
	err = pool.QueryRow(ctx, `
		INSERT INTO usuarios (email, senha_hash, nome) VALUES ($1, $2, $3)
		RETURNING id`, *email, string(hash), *nome).Scan(&id)
	if erroDeUnicidade(err) {
		return fmt.Errorf("ja existe usuario com o e-mail %s", *email)
	}
	if err != nil {
		return err
	}

	fmt.Printf(`usuario : %s
email   : %s
senha   : %s
          ^ entregue agora; nao e recuperavel depois
`, id, *email, senha)
	return nil
}

// --------------------------------------------------------------- talhao

func criarTalhao(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("talhao", flag.ExitOnError)
	nome := fs.String("nome", "", "nome do talhao")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *nome == "" {
		return errors.New("uso: admin talhao -nome=\"Talhao Norte\"")
	}

	var id string
	if err := pool.QueryRow(ctx,
		`INSERT INTO talhoes (nome) VALUES ($1) RETURNING id`, *nome).Scan(&id); err != nil {
		return err
	}
	fmt.Printf("talhao  : %s (%s)\n", id, *nome)
	fmt.Println("proximo : admin conceder -email=<e-mail> -talhao=" + id)
	return nil
}

// ------------------------------------------------------------- conceder

func conceder(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("conceder", flag.ExitOnError)
	email := fs.String("email", "", "e-mail do usuario")
	talhao := fs.String("talhao", "", "uuid do talhao")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" || *talhao == "" {
		return errors.New("uso: admin conceder -email=produtor@exemplo.com -talhao=<uuid>")
	}

	var usuarioID string
	err := pool.QueryRow(ctx,
		`SELECT id FROM usuarios WHERE lower(email) = lower($1)`, *email).Scan(&usuarioID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("usuario %s nao existe; crie com: admin usuario -email=%s -nome=...", *email, *email)
	}
	if err != nil {
		return err
	}

	tag, err := pool.Exec(ctx, `
		INSERT INTO usuario_talhoes (usuario_id, talhao_id)
		SELECT $1, id FROM talhoes WHERE id = $2
		ON CONFLICT DO NOTHING`, usuarioID, *talhao)
	if erroDeUUIDInvalido(err) {
		return fmt.Errorf("%q nao e um uuid de talhao", *talhao)
	}
	if err != nil {
		return err
	}
	// Zero linhas com o usuario ja resolvido significa talhao inexistente ou
	// concessao ja existente -- os dois inofensivos, mas o operador precisa
	// saber qual foi.
	if tag.RowsAffected() == 0 {
		var existe bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM talhoes WHERE id = $1)`, *talhao).Scan(&existe); err != nil {
			return err
		}
		if !existe {
			return fmt.Errorf("talhao %s nao existe", *talhao)
		}
		fmt.Printf("concessao: %s ja tinha acesso ao talhao %s\n", *email, *talhao)
		return nil
	}
	fmt.Printf("concessao: %s agora enxerga o talhao %s e todos os nos nele\n", *email, *talhao)
	return nil
}

// --------------------------------------------------------------- device

// criarDevice faz o que cmd/devtoken faz, mas gravando.
//
// devtoken continua existindo para o caso em que quem gera o token nao tem
// -- e nao deve ter -- acesso ao banco: ele imprime o INSERT para outra
// pessoa revisar. Aqui o caso e o oposto, o bootstrap de um ambiente novo,
// e mandar o operador colar SQL no psql no meio da sequencia e onde a
// reproducao quebra.
func criarDevice(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("device", flag.ExitOnError)
	id := fs.String("id", "", "id do device (ex: tensio-01)")
	descricao := fs.String("descricao", "", "descricao do no")
	talhao := fs.String("talhao", "", "uuid do talhao (opcional)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" || *descricao == "" {
		return errors.New("uso: admin device -id=tensio-01 -descricao=\"no do talhao norte\" [-talhao=<uuid>]")
	}

	// O mesmo segredo que a API usa: o hash gravado aqui so casa com o token
	// do firmware se as duas pontas compartilharem esta variavel. Subir com
	// outro valor produz um cadastro que nunca autentica.
	segredo := config.SegredoObrigatorio("TELEMETRIA_TOKEN_SECRET",
		"e a chave do HMAC do token: o hash gravado agora so casa com o token do\n  firmware se as duas pontas compartilharem esta variavel.")

	bruto := make([]byte, 32) // 256 bits: e por isso que o token usa HMAC e nao bcrypt
	if _, err := rand.Read(bruto); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(bruto)

	// talhao_id vira NULL quando -talhao nao vem, e o no nasce invisivel de
	// proposito -- ver a nota do no orfao no README.
	var talhaoID *string
	if *talhao != "" {
		talhaoID = talhao
	}
	tag, err := pool.Exec(ctx, `
		INSERT INTO devices (id, descricao, talhao_id, token_hash)
		SELECT $1, $2, $3::uuid, $4
		 WHERE $3::uuid IS NULL
		    OR EXISTS (SELECT 1 FROM talhoes WHERE id = $3::uuid)`,
		*id, *descricao, talhaoID, telemetria.HashToken(segredo, token))
	if erroDeUnicidade(err) {
		return fmt.Errorf("device %s ja existe; para so move-lo de talhao use: admin associar -device=%s -talhao=...", *id, *id)
	}
	if erroDeUUIDInvalido(err) {
		return fmt.Errorf("%q nao e um uuid de talhao", *talhao)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("talhao %s nao existe", *talhao)
	}

	fmt.Printf(`device  : %s
token   : %s
          ^ grave no firmware agora; nao e recuperavel depois
`, *id, token)
	if talhaoID == nil {
		fmt.Println("talhao  : nenhum -- o no NAO aparece para usuario nenhum ate ser associado")
		fmt.Println("proximo : admin associar -device=" + *id + " -talhao=<uuid>")
	} else {
		fmt.Printf("talhao  : %s\n", *talhao)
	}
	return nil
}

// ----------------------------------------------------------- calibracao

// criarCalibracao registra o ensaio que converte tensao em kPa.
//
// O sintoma de faltar: o no autentica, o POST /readings responde 200, e
// TODAS as leituras vem em "rejected" com calibration_id ausente. Parece
// erro do firmware. E cadastro faltando no backend.
//
// Os coeficientes nao tem padrao: vem do ensaio daquele sensor com aquele
// divisor. Um valor chutado aqui produz uma serie inteira de kPa plausiveis
// e errados, que e pior que serie nenhuma -- por isso todos sao obrigatorios.
func criarCalibracao(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("calibracao", flag.ExitOnError)
	id := fs.String("id", "", "id da calibracao (ex: cal-2026-08-25-a)")
	device := fs.String("device", "", "id do device ensaiado")
	// VOLTS, e nao milivolts. A ajuda dizia mV e a coluna sempre foi V: a
	// formula de 0001_init.sql compara v_zero_kpa com
	// `raw_mv * fator_divisor / 1000`, que ja converteu. Seguir a ajuda
	// antiga (4500 em vez de 4.5) passa por todos os CHECK do banco e grava
	// uma serie inteira mil vezes errada.
	vZero := fs.Float64("v-zero", 0, "tensao em VOLTS a 0 kPa (ex: 4.5)")
	k := fs.Float64("k", 0, "coeficiente em V por kPa, nao pode ser zero (ex: 0.04)")
	divisor := fs.Float64("divisor", 0, "fator do divisor resistivo (> 0)")
	vdd := fs.Int("vdd", 0, "Vdd do ensaio em mV (> 0)")
	r2 := fs.Float64("r2", 0, "R2 do ajuste (opcional, 0 = nao informado)")
	rmse := fs.Float64("rmse", 0, "RMSE em kPa (opcional, 0 = nao informado)")
	nota := fs.String("nota", "", "observacao do ensaio (opcional)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Zero e ausencia aqui: k e divisor nao podem ser zero pelo CHECK, e
	// v-zero zero significaria um sensor que le 0 mV a 0 kPa.
	if *id == "" || *device == "" || *vZero == 0 || *k == 0 || *divisor <= 0 || *vdd <= 0 {
		return errors.New("uso: admin calibracao -id=cal-2026-08-25-a -device=tensio-01 " +
			"-v-zero=<V> -k=<V/kPa> -divisor=<fator> -vdd=<mV> [-r2=] [-rmse=] [-nota=]")
	}

	opcional := func(v float64) *float64 {
		if v == 0 {
			return nil
		}
		return &v
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO calibrations
		  (id, device_id, v_zero_kpa, k_v_por_kpa, fator_divisor, vdd_ensaio_mv, r2, rmse_kpa, ensaio_em, nota)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now()::date, $9)`,
		*id, *device, *vZero, *k, *divisor, *vdd, opcional(*r2), opcional(*rmse), *nota)
	if erroDeUnicidade(err) {
		return fmt.Errorf("ja existe calibracao %s; um ensaio novo e um id novo, nao uma sobrescrita", *id)
	}
	// 23503 = foreign_key_violation: o device precisa existir antes do ensaio.
	if codigoPG(err) == "23503" {
		return fmt.Errorf("device %s nao existe; cadastre com: admin device -id=%s -descricao=...", *device, *device)
	}
	if err != nil {
		return err
	}
	fmt.Printf("calibracao: %s registrada para %s\n", *id, *device)
	fmt.Println("            o firmware deve enviar calibration_id=" + *id)
	return nil
}

// ------------------------------------------------------------- associar

func associar(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("associar", flag.ExitOnError)
	device := fs.String("device", "", "id do device (ex: tensio-01)")
	talhao := fs.String("talhao", "", "uuid do talhao")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *device == "" || *talhao == "" {
		return errors.New("uso: admin associar -device=tensio-01 -talhao=<uuid>")
	}

	tag, err := pool.Exec(ctx, `
		UPDATE devices SET talhao_id = $2
		 WHERE id = $1 AND EXISTS (SELECT 1 FROM talhoes WHERE id = $2)`, *device, *talhao)
	if erroDeUUIDInvalido(err) {
		return fmt.Errorf("%q nao e um uuid de talhao", *talhao)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var existe bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM devices WHERE id = $1)`, *device).Scan(&existe); err != nil {
			return err
		}
		if !existe {
			return fmt.Errorf("device %s nao existe; cadastre com cmd/devtoken", *device)
		}
		return fmt.Errorf("talhao %s nao existe", *talhao)
	}
	fmt.Printf("associado: %s agora pertence ao talhao %s\n", *device, *talhao)
	fmt.Println("           ele passa a aparecer para quem tem concessao nesse talhao")
	return nil
}

// ------------------------------------------------------------------ util

// senhaAleatoria gera ~72 bits em base32 sem padding, em grupos de 4
// separados por hifen. Base32 e nao base64 porque a senha vai ser digitada
// a mao num celular: sem maiuscula/minuscula ambigua, sem +/ e sem 0/O/1/l.
func senhaAleatoria() (string, error) {
	const alfabeto = "abcdefghijkmnpqrstuvwxyz23456789" // 32 simbolos, sem o/l/0/1
	bruto := make([]byte, 16)                           // 16 x 5 bits = 80 bits
	if _, err := rand.Read(bruto); err != nil {
		return "", err
	}
	// 256 e multiplo de 32, entao o modulo nao introduz vies.
	var sb strings.Builder
	for i, b := range bruto {
		if i > 0 && i%4 == 0 {
			sb.WriteByte('-')
		}
		sb.WriteByte(alfabeto[int(b)%len(alfabeto)])
	}
	return sb.String(), nil
}

func codigoPG(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func erroDeUnicidade(err error) bool { return codigoPG(err) == "23505" }

// 22P02 = invalid_text_representation: o uuid veio malformado.
func erroDeUUIDInvalido(err error) bool {
	if codigoPG(err) == "22P02" {
		return true
	}
	// pgx tambem pode recusar do lado do cliente, antes de enviar.
	return err != nil && strings.Contains(err.Error(), "UUID")
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "erro: "+msg)
	os.Exit(1)
}
