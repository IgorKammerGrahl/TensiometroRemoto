package telemetria

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Mesmo padrao de http_test.go: Postgres real via TEST_DATABASE_URL. O que
// esta sob teste aqui e autorizacao expressa em JOIN, e nenhum mock de banco
// reproduz um JOIN que nao casa.
//
// A leitura util deste arquivo e por RISCO, nao por funcao: cada teste
// nomeia o vazamento ou a inversao que ele impede.

const (
	senhaTeste = "senha-de-teste-do-produtor"

	emailA       = "produtora@exemplo.com"
	emailB       = "vizinho@exemplo.com"
	emailInativo = "desativada@exemplo.com"
)

// Gerado uma vez: bcrypt custo 12 leva ~250 ms por chamada.
var hashSenhaTeste string

type cenario struct {
	h       http.Handler
	logs    *bytes.Buffer
	talhaoA string // faixas -30/-60, concedido a A e a inativa
	talhaoB string // faixas -30/-60, concedido so a B
	talhaoC string // SEM faixas, concedido a A
	usuarioA,
	usuarioB,
	usuarioInativo string
}

// novoCenarioApp monta o mundo minimo em que os riscos aparecem: dois
// produtores vizinhos, um talhao sem faixa configurada, uma conta desativada
// e um no orfao.
func novoCenarioApp(t *testing.T) *cenario {
	t.Helper()
	ctx := context.Background()

	if _, err := testPool.Exec(ctx,
		`TRUNCATE readings, calibrations, devices, talhoes, usuarios, sessoes CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	c := &cenario{}
	talhao := func(nome string, alerta, estresse any) string {
		var id string
		if err := testPool.QueryRow(ctx, `
			INSERT INTO talhoes (nome, cultura, kpa_alerta, kpa_estresse)
			VALUES ($1, 'Soja', $2, $3) RETURNING id`, nome, alerta, estresse).Scan(&id); err != nil {
			t.Fatalf("seed talhao %s: %v", nome, err)
		}
		return id
	}
	c.talhaoA = talhao("Talhao A", -30, -60)
	c.talhaoB = talhao("Talhao B", -30, -60)
	c.talhaoC = talhao("Talhao C sem faixa", nil, nil)

	usuario := func(email string, ativo bool) string {
		var id string
		if err := testPool.QueryRow(ctx, `
			INSERT INTO usuarios (email, senha_hash, nome, ativo)
			VALUES ($1, $2, $3, $4) RETURNING id`,
			email, hashSenhaTeste, "Nome "+email, ativo).Scan(&id); err != nil {
			t.Fatalf("seed usuario %s: %v", email, err)
		}
		return id
	}
	c.usuarioA = usuario(emailA, true)
	c.usuarioB = usuario(emailB, true)
	c.usuarioInativo = usuario(emailInativo, false)

	conceder := func(usuarioID, talhaoID string) {
		if _, err := testPool.Exec(ctx,
			`INSERT INTO usuario_talhoes (usuario_id, talhao_id) VALUES ($1, $2)`,
			usuarioID, talhaoID); err != nil {
			t.Fatalf("seed concessao: %v", err)
		}
	}
	conceder(c.usuarioA, c.talhaoA)
	conceder(c.usuarioA, c.talhaoC)
	conceder(c.usuarioB, c.talhaoB)
	conceder(c.usuarioInativo, c.talhaoA)

	if _, err := testPool.Exec(ctx, `
		INSERT INTO devices (id, descricao, talhao_id, token_hash, ativo) VALUES
		  ('dev-a1',    'no do talhao A',        $1,   $4, true),
		  ('dev-a2',    'segundo no do A',       $1,   $5, true),
		  ('dev-b1',    'no do vizinho',         $2,   $6, true),
		  ('dev-c1',    'no do talhao sem faixa',$3,   $7, true),
		  ('dev-orfao', 'no de bancada',         NULL, $8, true)`,
		c.talhaoA, c.talhaoB, c.talhaoC,
		HashToken(segredoTeste, "tok-a1"), HashToken(segredoTeste, "tok-a2"),
		HashToken(segredoTeste, "tok-b1"), HashToken(segredoTeste, "tok-c1"),
		HashToken(segredoTeste, "tok-orfao")); err != nil {
		t.Fatalf("seed devices: %v", err)
	}

	var logs bytes.Buffer
	api := NewAPI(NewStore(testPool), segredoTeste, slog.New(slog.NewTextHandler(&logs, nil)))
	c.h, c.logs = api.Router(), &logs
	return c
}

// semearKPa grava leituras direto no banco (calibration_id nulo): o que se
// testa aqui e a leitura autorizada, nao o caminho de ingestao.
func semearKPa(t *testing.T, deviceID string, quando time.Time, kpas ...float32) {
	t.Helper()
	// seq continua de onde a chamada anterior parou: a PK e (device_id, seq),
	// e um teste que semeia duas ilhas de leituras colidiria comecando do 1.
	//
	// Este helper ja nasceu com esse bug -- reiniciava seq em 1 a cada
	// chamada. E a TERCEIRA aparicao do mesmo risco em tres camadas: o
	// firmware reinicia seq quando perde o contador, o seed sintetico
	// reinicia seq quando roda duas vezes, e o helper de teste reiniciava
	// aqui. A defesa em cada camada e diferente (contador persistido, aviso
	// no seed, este SELECT max), mas o alarme de colisao do backend existe
	// justamente porque nenhuma delas e confiavel sozinha: a unica camada
	// que enxerga a colisao e a que guarda a chave.
	var base int64
	if err := testPool.QueryRow(context.Background(),
		`SELECT COALESCE(max(seq), 0) FROM readings WHERE device_id = $1`, deviceID).Scan(&base); err != nil {
		t.Fatalf("seed leitura: %v", err)
	}
	for i, kpa := range kpas {
		if _, err := testPool.Exec(context.Background(), `
			INSERT INTO readings (device_id, seq, measured_at, raw_mv, kpa)
			VALUES ($1, $2, $3, 3000, $4)`,
			deviceID, base+int64(i+1), quando.Add(time.Duration(i)*time.Minute), kpa); err != nil {
			t.Fatalf("seed leitura: %v", err)
		}
	}
}

// ----------------------------------------------------------- requisicoes

func req(t *testing.T, h http.Handler, metodo, caminho, cookie, corpo string) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if corpo == "" {
		body = strings.NewReader("")
	} else {
		body = strings.NewReader(corpo)
	}
	r := httptest.NewRequest(metodo, caminho, body)
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: nomeCookieSessao, Value: cookie})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// entrar faz login de verdade e devolve o valor do cookie. Os testes de
// autorizacao passam pelo login inteiro de proposito: e o unico jeito de
// garantir que o que o middleware valida e o que o login emite.
func entrar(t *testing.T, h http.Handler, email string) string {
	t.Helper()
	rec := req(t, h, http.MethodPost, "/api/v1/app/login", "",
		fmt.Sprintf(`{"email":%q,"senha":%q}`, email, senhaTeste))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("login de %s: status = %d; corpo: %s", email, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == nomeCookieSessao {
			return c.Value
		}
	}
	t.Fatalf("login de %s nao devolveu cookie %q", email, nomeCookieSessao)
	return ""
}

func decodificarResp(t *testing.T, rec *httptest.ResponseRecorder, destino any) {
	t.Helper()
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("status = %d; corpo: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), destino); err != nil {
		t.Fatalf("decode: %v; corpo: %s", err, rec.Body.String())
	}
}

func expiraEm(t *testing.T, cookie string) time.Time {
	t.Helper()
	var e time.Time
	if err := testPool.QueryRow(context.Background(),
		`SELECT expira_em FROM sessoes WHERE token_hash = $1`,
		HashToken(segredoTeste, cookie)).Scan(&e); err != nil {
		t.Fatalf("expira_em: %v", err)
	}
	return e
}

func contarSessoes(t *testing.T, cookie string) int {
	t.Helper()
	var n int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM sessoes WHERE token_hash = $1`,
		HashToken(segredoTeste, cookie)).Scan(&n); err != nil {
		t.Fatalf("contar sessoes: %v", err)
	}
	return n
}

type respDevices struct {
	Count   int         `json:"count"`
	Devices []DeviceApp `json:"devices"`
}

// ------------------------------------------------------- RISCO: login

func TestLoginCredenciais(t *testing.T) {
	c := novoCenarioApp(t)
	for _, caso := range []struct {
		nome, corpo string
		querido     int
	}{
		{"senha correta", fmt.Sprintf(`{"email":%q,"senha":%q}`, emailA, senhaTeste), http.StatusNoContent},
		{"senha errada", fmt.Sprintf(`{"email":%q,"senha":"outra"}`, emailA), http.StatusUnauthorized},
		{"e-mail inexistente", `{"email":"ninguem@exemplo.com","senha":"x"}`, http.StatusUnauthorized},
		{"e-mail em outra caixa", fmt.Sprintf(`{"email":%q,"senha":%q}`, strings.ToUpper(emailA), senhaTeste), http.StatusNoContent},
		// A conta desativada nao recebe sessao: 403, e nao 401, porque a
		// credencial vale -- reautenticar nao resolveria.
		{"conta desativada", fmt.Sprintf(`{"email":%q,"senha":%q}`, emailInativo, senhaTeste), http.StatusForbidden},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			rec := req(t, c.h, http.MethodPost, "/api/v1/app/login", "", caso.corpo)
			if rec.Code != caso.querido {
				t.Errorf("status = %d, quero %d; corpo: %s", rec.Code, caso.querido, rec.Body.String())
			}
		})
	}
}

// A mensagem de erro nao pode distinguir senha errada de conta inexistente:
// distinguir enumera contas.
func TestLoginNaoEnumeraContas(t *testing.T) {
	c := novoCenarioApp(t)
	errada := req(t, c.h, http.MethodPost, "/api/v1/app/login", "",
		fmt.Sprintf(`{"email":%q,"senha":"outra"}`, emailA)).Body.String()
	inexistente := req(t, c.h, http.MethodPost, "/api/v1/app/login", "",
		`{"email":"ninguem@exemplo.com","senha":"outra"}`).Body.String()
	if errada != inexistente {
		t.Errorf("respostas distinguem os casos:\n  senha errada: %s\n  inexistente : %s", errada, inexistente)
	}
}

// O limite existe porque o login roda bcrypt custo 12 nos dois caminhos --
// tambem quando a conta nao existe, por causa de hashDeReferencia. Sem ele,
// um laco simples come a CPU da maquina enquanto testa senhas.
//
// Os e-mails sao distintos de proposito: com o mesmo e-mail o limitador por
// conta (rajada 5) dispararia antes e o teste nao provaria nada sobre o de IP.
func TestLoginLimitaTentativasPorIP(t *testing.T) {
	c := novoCenarioApp(t)

	for i := range loginRajadaIP {
		corpo := fmt.Sprintf(`{"email":"ninguem%d@exemplo.com","senha":"x"}`, i)
		rec := req(t, c.h, http.MethodPost, "/api/v1/app/login", "", corpo)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("tentativa %d: status = %d, quero 401 dentro da rajada", i+1, rec.Code)
		}
	}

	rec := req(t, c.h, http.MethodPost, "/api/v1/app/login", "",
		`{"email":"ninguem-extra@exemplo.com","senha":"x"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, quero 429 depois de esgotar a rajada", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("429 sem Retry-After: o cliente nao sabe quando voltar")
	}
}

func TestLoginLimitaTentativasPorEmail(t *testing.T) {
	c := novoCenarioApp(t)

	corpo := fmt.Sprintf(`{"email":%q,"senha":"outra"}`, emailA)
	for i := range loginRajadaEmail {
		if rec := req(t, c.h, http.MethodPost, "/api/v1/app/login", "", corpo); rec.Code != http.StatusUnauthorized {
			t.Fatalf("tentativa %d: status = %d, quero 401 dentro da rajada", i+1, rec.Code)
		}
	}
	// A rajada por conta (5) e menor que a por IP (10), entao quem chega aqui
	// foi barrado pelo e-mail -- que e o caso do ataque distribuido por muitos
	// IPs contra uma conta so.
	if rec := req(t, c.h, http.MethodPost, "/api/v1/app/login", "", corpo); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, quero 429 apos %d tentativas na mesma conta", rec.Code, loginRajadaEmail)
	}

	// A senha CERTA tambem e barrada: o limite protege a conta, nao pune o
	// erro. Se a senha certa passasse, bastaria tentar ate acertar.
	if rec := req(t, c.h, http.MethodPost, "/api/v1/app/login", "",
		fmt.Sprintf(`{"email":%q,"senha":%q}`, emailA, senhaTeste)); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d; o limite nao pode ceder para a senha correta", rec.Code)
	}

	// Outra conta, mesmo IP, continua passando: o balde e por chave.
	if rec := req(t, c.h, http.MethodPost, "/api/v1/app/login", "",
		fmt.Sprintf(`{"email":%q,"senha":%q}`, emailB, senhaTeste)); rec.Code == http.StatusTooManyRequests {
		t.Error("o limite de uma conta vazou para outra")
	}
}

// As duas recusas precisam ser indistinguiveis. "Seu IP estourou" contra
// "esta conta estourou" responde "essa conta existe" a quem contar respostas
// -- o mesmo oraculo que TestLoginNaoEnumeraContas fecha no 401.
func TestLoginRecusaPorLimiteNaoEnumeraContas(t *testing.T) {
	porIP := novoCenarioApp(t)
	for range loginRajadaIP {
		req(t, porIP.h, http.MethodPost, "/api/v1/app/login", "",
			`{"email":"ninguem@exemplo.com","senha":"x"}`)
	}
	deIP := req(t, porIP.h, http.MethodPost, "/api/v1/app/login", "",
		`{"email":"ninguem@exemplo.com","senha":"x"}`).Body.String()

	porEmail := novoCenarioApp(t)
	corpo := fmt.Sprintf(`{"email":%q,"senha":"outra"}`, emailA)
	for range loginRajadaEmail {
		req(t, porEmail.h, http.MethodPost, "/api/v1/app/login", "", corpo)
	}
	deEmail := req(t, porEmail.h, http.MethodPost, "/api/v1/app/login", "", corpo).Body.String()

	if deIP != deEmail {
		t.Errorf("as recusas se distinguem:\n  por IP   : %s\n  por e-mail: %s", deIP, deEmail)
	}
}

func TestCookieDeSessaoTemAtributosDeSeguranca(t *testing.T) {
	c := novoCenarioApp(t)
	rec := req(t, c.h, http.MethodPost, "/api/v1/app/login", "",
		fmt.Sprintf(`{"email":%q,"senha":%q}`, emailA, senhaTeste))
	var cookie *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == nomeCookieSessao {
			cookie = ck
		}
	}
	if cookie == nil {
		t.Fatal("sem cookie de sessao")
	}
	if !cookie.HttpOnly {
		t.Error("cookie sem HttpOnly: XSS leria o token")
	}
	if !cookie.Secure {
		t.Error("cookie sem Secure fora do modo de demonstracao")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, quero Lax", cookie.SameSite)
	}
	// O cookie carrega o token em claro; o banco, so o HMAC.
	var n int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM sessoes WHERE token_hash = $1`, cookie.Value).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("token em claro persistido em sessoes.token_hash")
	}
}

func TestLogoutInvalidaSessao(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	if rec := req(t, c.h, http.MethodPost, "/api/v1/app/logout", cookie, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("logout: status = %d; corpo: %s", rec.Code, rec.Body.String())
	}
	if n := contarSessoes(t, cookie); n != 0 {
		t.Errorf("sessao sobreviveu ao logout (%d linhas)", n)
	}
	if rec := req(t, c.h, http.MethodGet, "/api/v1/app/devices", cookie, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("cookie reutilizado apos logout: status = %d, quero 401", rec.Code)
	}
}

// ----------------------------------------------- RISCO: sessao e ordem

func TestSemCookie401(t *testing.T) {
	c := novoCenarioApp(t)
	for _, caminho := range []string{
		"/api/v1/app/me", "/api/v1/app/devices", "/api/v1/app/devices/dev-a1",
		"/api/v1/app/devices/dev-a1/series", "/api/v1/app/talhoes",
	} {
		if rec := req(t, c.h, http.MethodGet, caminho, "", ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, quero 401", caminho, rec.Code)
		}
	}
}

// RISCO: os dois planos de auth se misturam. A separacao e por prefixo de
// rota, entao nenhuma credencial atravessa para o outro lado.
func TestPlanosDeAuthNaoSeMisturam(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	// Token de dispositivo numa rota de usuario: nao ha cookie, logo 401.
	r := httptest.NewRequest(http.MethodGet, "/api/v1/app/devices", nil)
	r.Header.Set("Authorization", "Bearer tok-a1")
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, r)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("token de dispositivo em /app/*: status = %d, quero 401; corpo: %s", rec.Code, rec.Body.String())
	}

	// Cookie de sessao numa rota de dispositivo: nao ha Bearer, logo 401.
	if rec := req(t, c.h, http.MethodGet, "/api/v1/devices/dev-a1/latest", cookie, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("cookie de sessao em rota de device: status = %d, quero 401", rec.Code)
	}
	if rec := req(t, c.h, http.MethodPost, "/api/v1/readings", cookie,
		`{"device_id":"dev-a1","readings":[]}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("cookie de sessao em POST /readings: status = %d, quero 401", rec.Code)
	}
}

// RISCO: sessao expirada revive. E a linha morta precisa sair do banco.
func TestSessaoExpirada(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	if _, err := testPool.Exec(context.Background(),
		`UPDATE sessoes SET expira_em = now() - interval '1 minute' WHERE token_hash = $1`,
		HashToken(segredoTeste, cookie)); err != nil {
		t.Fatal(err)
	}

	if rec := req(t, c.h, http.MethodGet, "/api/v1/app/devices", cookie, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, quero 401; corpo: %s", rec.Code, rec.Body.String())
	}
	if n := contarSessoes(t, cookie); n != 0 {
		t.Errorf("linha expirada continua no banco (%d)", n)
	}
}

// RISCO: sessao morre no meio do uso. Toda requisicao autenticada empurra
// expira_em para frente (D10).
func TestRenovacaoDeslizante(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	// Encurta a janela para simular uma sessao ja no fim da vida.
	if _, err := testPool.Exec(context.Background(),
		`UPDATE sessoes SET expira_em = now() + interval '1 hour' WHERE token_hash = $1`,
		HashToken(segredoTeste, cookie)); err != nil {
		t.Fatal(err)
	}
	antes := expiraEm(t, cookie)

	if rec := req(t, c.h, http.MethodGet, "/api/v1/app/me", cookie, ""); rec.Code != http.StatusOK {
		t.Fatalf("status = %d; corpo: %s", rec.Code, rec.Body.String())
	}

	depois := expiraEm(t, cookie)
	if !depois.After(antes) {
		t.Fatalf("expira_em nao avancou: antes=%v depois=%v", antes, depois)
	}
	// Voltou para perto da janela cheia, e nao so alguns segundos adiante.
	if faltando := time.Until(depois); faltando < JanelaSessao-time.Hour {
		t.Errorf("renovacao devolveu so %v de prazo, quero perto de %v", faltando, JanelaSessao)
	}
}

// RISCO: conta desativada continua entrando com sessao antiga. 403 e nao
// 401: a credencial vale, a conta e que foi desligada.
func TestUsuarioInativo403(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	if _, err := testPool.Exec(context.Background(),
		`UPDATE usuarios SET ativo = false WHERE id = $1`, c.usuarioA); err != nil {
		t.Fatal(err)
	}
	rec := req(t, c.h, http.MethodGet, "/api/v1/app/devices", cookie, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, quero 403; corpo: %s", rec.Code, rec.Body.String())
	}
}

// ------------------------------------------------------- RISCO: IDOR

// Usuario A pedindo device do talhao de B: 404, nunca 403 e nunca 200. 403
// confirmaria a existencia do recurso.
func TestDeviceDeOutroUsuario404(t *testing.T) {
	c := novoCenarioApp(t)
	semearKPa(t, "dev-b1", time.Now().UTC().Add(-time.Hour), -25)
	cookie := entrar(t, c.h, emailA)

	for _, caminho := range []string{
		"/api/v1/app/devices/dev-b1",
		"/api/v1/app/devices/dev-b1/series",
	} {
		rec := req(t, c.h, http.MethodGet, caminho, cookie, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, quero 404; corpo: %s", caminho, rec.Code, rec.Body.String())
		}
	}

	// E o device do vizinho tambem nao aparece na lista.
	var resp respDevices
	decodificarResp(t, req(t, c.h, http.MethodGet, "/api/v1/app/devices", cookie, ""), &resp)
	for _, d := range resp.Devices {
		if d.ID == "dev-b1" {
			t.Error("device do vizinho apareceu na lista")
		}
	}
}

// Device inexistente e device alheio dao a MESMA resposta -- e o que impede
// varrer ids para mapear os nos da vizinhanca.
func TestDeviceInexistenteEAlheioSaoIndistinguiveis(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	alheio := req(t, c.h, http.MethodGet, "/api/v1/app/devices/dev-b1", cookie, "")
	fantasma := req(t, c.h, http.MethodGet, "/api/v1/app/devices/dev-nao-existe", cookie, "")
	if alheio.Code != fantasma.Code || alheio.Body.String() != fantasma.Body.String() {
		t.Errorf("respostas distinguem os casos:\n  alheio  : %d %s\n  fantasma: %d %s",
			alheio.Code, alheio.Body.String(), fantasma.Code, fantasma.Body.String())
	}
}

// RISCO: no orfao vaza. talhao_id NULL nao casa nenhum JOIN, entao o no de
// bancada e invisivel ATE para quem tem concessao em todos os talhoes.
func TestDeviceOrfaoEInvisivelParaTodos(t *testing.T) {
	c := novoCenarioApp(t)
	semearKPa(t, "dev-orfao", time.Now().UTC().Add(-time.Hour), -70)

	for _, email := range []string{emailA, emailB} {
		cookie := entrar(t, c.h, email)

		var resp respDevices
		decodificarResp(t, req(t, c.h, http.MethodGet, "/api/v1/app/devices", cookie, ""), &resp)
		for _, d := range resp.Devices {
			if d.ID == "dev-orfao" {
				t.Errorf("%s: no orfao apareceu na lista", email)
			}
		}

		for _, caminho := range []string{
			"/api/v1/app/devices/dev-orfao",
			"/api/v1/app/devices/dev-orfao/series",
		} {
			if rec := req(t, c.h, http.MethodGet, caminho, cookie, ""); rec.Code != http.StatusNotFound {
				t.Errorf("%s %s: status = %d, quero 404", email, caminho, rec.Code)
			}
		}

		// E nao ha PATCH que o adote: a concessao de origem nao casa.
		rec := req(t, c.h, http.MethodPatch, "/api/v1/app/devices/dev-orfao", cookie,
			fmt.Sprintf(`{"talhao_id":%q}`, c.talhaoA))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: PATCH no orfao: status = %d, quero 404", email, rec.Code)
		}
	}

	var talhao *string
	if err := testPool.QueryRow(context.Background(),
		`SELECT talhao_id FROM devices WHERE id = 'dev-orfao'`).Scan(&talhao); err != nil {
		t.Fatal(err)
	}
	if talhao != nil {
		t.Errorf("no orfao foi adotado pela API: talhao_id = %s", *talhao)
	}
}

// --------------------------------------------- RISCO: IDOR em escrita

// PATCH movendo o no para um talhao sem concessao: negado. Sem a checagem no
// DESTINO, o dono de um no o empurraria para fora do alcance de quem o
// vigiava.
func TestPatchParaTalhaoSemConcessao(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	rec := req(t, c.h, http.MethodPatch, "/api/v1/app/devices/dev-a1", cookie,
		fmt.Sprintf(`{"talhao_id":%q}`, c.talhaoB))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quero 404; corpo: %s", rec.Code, rec.Body.String())
	}

	var talhao string
	if err := testPool.QueryRow(context.Background(),
		`SELECT talhao_id FROM devices WHERE id = 'dev-a1'`).Scan(&talhao); err != nil {
		t.Fatal(err)
	}
	if talhao != c.talhaoA {
		t.Errorf("device foi movido mesmo assim: talhao_id = %s", talhao)
	}
}

// PATCH sobre um no que esta num talhao sem concessao: negado. Sem a
// checagem na ORIGEM, qualquer um puxaria um no alheio para dentro do
// proprio alcance.
func TestPatchDeTalhaoSemConcessao(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	// A quer trazer o no do vizinho para o proprio talhao. O destino ate e
	// concedido; a origem nao e.
	rec := req(t, c.h, http.MethodPatch, "/api/v1/app/devices/dev-b1", cookie,
		fmt.Sprintf(`{"talhao_id":%q}`, c.talhaoA))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quero 404; corpo: %s", rec.Code, rec.Body.String())
	}

	var talhao string
	if err := testPool.QueryRow(context.Background(),
		`SELECT talhao_id FROM devices WHERE id = 'dev-b1'`).Scan(&talhao); err != nil {
		t.Fatal(err)
	}
	if talhao != c.talhaoB {
		t.Errorf("no do vizinho foi capturado: talhao_id = %s", talhao)
	}

	// Nem descricao, que parece inofensiva, passa.
	if rec := req(t, c.h, http.MethodPatch, "/api/v1/app/devices/dev-b1", cookie,
		`{"descricao":"meu agora"}`); rec.Code != http.StatusNotFound {
		t.Errorf("PATCH de descricao alheia: status = %d, quero 404", rec.Code)
	}
}

func TestPatchDeviceValido(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	// Move entre dois talhoes concedidos ao mesmo usuario, e desativa.
	rec := req(t, c.h, http.MethodPatch, "/api/v1/app/devices/dev-a1", cookie,
		fmt.Sprintf(`{"descricao":"no realocado","talhao_id":%q,"ativo":false}`, c.talhaoC))
	var d DeviceApp
	decodificarResp(t, rec, &d)

	if d.Descricao != "no realocado" || d.Talhao.ID != c.talhaoC || d.Ativo {
		t.Errorf("device apos PATCH = %+v", d)
	}
	// Campo ausente e campo preservado.
	rec = req(t, c.h, http.MethodPatch, "/api/v1/app/devices/dev-a1", cookie, `{"ativo":true}`)
	decodificarResp(t, rec, &d)
	if d.Descricao != "no realocado" || d.Talhao.ID != c.talhaoC || !d.Ativo {
		t.Errorf("PATCH parcial sobrescreveu campo ausente: %+v", d)
	}
}

// POST /devices com talhao nao concedido: 404 e nada inserido.
func TestCadastroEmTalhaoSemConcessao(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	rec := req(t, c.h, http.MethodPost, "/api/v1/app/devices", cookie,
		fmt.Sprintf(`{"id":"dev-invasor","descricao":"no","talhao_id":%q}`, c.talhaoB))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quero 404; corpo: %s", rec.Code, rec.Body.String())
	}

	var n int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM devices WHERE id = 'dev-invasor'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("device foi inserido apesar do 404")
	}
}

func TestCadastroDeDevice(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	rec := req(t, c.h, http.MethodPost, "/api/v1/app/devices", cookie,
		fmt.Sprintf(`{"id":"dev-novo","descricao":"no novo","talhao_id":%q}`, c.talhaoA))
	var resp struct {
		Device DeviceApp `json:"device"`
		Token  string    `json:"token"`
	}
	decodificarResp(t, rec, &resp)
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, quero 201", rec.Code)
	}
	if resp.Token == "" {
		t.Fatal("token nao devolvido; ele nao e recuperavel depois")
	}
	if resp.Device.Ultima != nil {
		t.Error("device recem-cadastrado com leitura")
	}

	// O token so existe em claro nessa resposta; o banco guarda o HMAC.
	var hash string
	if err := testPool.QueryRow(context.Background(),
		`SELECT token_hash FROM devices WHERE id = 'dev-novo'`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash != HashToken(segredoTeste, resp.Token) {
		t.Error("token_hash gravado nao casa com o token devolvido")
	}
	if hash == resp.Token {
		t.Error("token em claro persistido")
	}

	// O no novo ja aparece na lista, mesmo sem serie ainda.
	var lista respDevices
	decodificarResp(t, req(t, c.h, http.MethodGet, "/api/v1/app/devices", cookie, ""), &lista)
	achou := false
	for _, d := range lista.Devices {
		achou = achou || d.ID == "dev-novo"
	}
	if !achou {
		t.Error("device recem-cadastrado nao aparece na lista")
	}

	// Id repetido, ja com a autorizacao satisfeita, e 409 e nao 404.
	rec = req(t, c.h, http.MethodPost, "/api/v1/app/devices", cookie,
		fmt.Sprintf(`{"id":"dev-novo","descricao":"outro","talhao_id":%q}`, c.talhaoA))
	if rec.Code != http.StatusConflict {
		t.Errorf("id duplicado: status = %d, quero 409; corpo: %s", rec.Code, rec.Body.String())
	}
}

// ------------------------------------------------------- RISCO: sinal

// A tabela-verdade da 4.1, incluindo as fronteiras exatas. -30 e -60 caem na
// zona MENOS severa: e consequencia do >=, e e o caractere que muda se
// alguem "corrigir" a comparacao.
func TestZonaTabelaVerdade(t *testing.T) {
	alerta, estresse := float32(-30), float32(-60)
	for _, caso := range []struct {
		kpa  float32
		quer string
	}{
		{-10, ZonaConforto},
		{-30, ZonaConforto}, // fronteira exata
		{-30.1, ZonaAlerta},
		{-40, ZonaAlerta},
		{-60, ZonaAlerta}, // fronteira exata
		{-60.1, ZonaEstresse},
		{-70, ZonaEstresse},
	} {
		if got := Zona(caso.kpa, &alerta, &estresse); got == nil || *got != caso.quer {
			t.Errorf("Zona(%g) = %v, quero %q", caso.kpa, valor(got), caso.quer)
		}
	}
	// Faixa nao configurada nao e conforto.
	if got := Zona(-10, nil, nil); got != nil {
		t.Errorf("Zona sem limiares = %q, quero nulo", *got)
	}
}

// O mesmo pela API, que e onde a inversao teria consequencia.
func TestZonaNaListaDeDevices(t *testing.T) {
	c := novoCenarioApp(t)
	agora := time.Now().UTC().Add(-time.Hour)
	semearKPa(t, "dev-a1", agora, -10)
	semearKPa(t, "dev-a2", agora, -70)
	cookie := entrar(t, c.h, emailA)

	var resp respDevices
	decodificarResp(t, req(t, c.h, http.MethodGet, "/api/v1/app/devices", cookie, ""), &resp)

	zonas := map[string]string{}
	for _, d := range resp.Devices {
		if d.Ultima != nil && d.Ultima.Zona != nil {
			zonas[d.ID] = *d.Ultima.Zona
		}
	}
	if zonas["dev-a1"] != ZonaConforto {
		t.Errorf("dev-a1 (-10 kPa) = %q, quero conforto", zonas["dev-a1"])
	}
	if zonas["dev-a2"] != ZonaEstresse {
		t.Errorf("dev-a2 (-70 kPa) = %q, quero estresse", zonas["dev-a2"])
	}
	// Limiares brutos viajam junto, para as linhas de referencia do grafico.
	for _, d := range resp.Devices {
		if d.Talhao.ID == c.talhaoA && (d.Talhao.KPaAlerta == nil || d.Talhao.KPaEstresse == nil) {
			t.Errorf("limiares brutos ausentes em %s", d.ID)
		}
	}
}

// RISCO: talhao sem faixa vira conforto. Deve vir zona nula, que a interface
// mostra como "nao configurado" -- verde com limiar ausente e pior que
// indicador nenhum.
func TestTalhaoSemFaixaDevolveZonaNula(t *testing.T) {
	c := novoCenarioApp(t)
	semearKPa(t, "dev-c1", time.Now().UTC().Add(-time.Hour), -10)
	cookie := entrar(t, c.h, emailA)

	var d DeviceApp
	decodificarResp(t, req(t, c.h, http.MethodGet, "/api/v1/app/devices/dev-c1", cookie, ""), &d)
	if d.Ultima == nil {
		t.Fatal("sem ultima leitura")
	}
	if d.Ultima.Zona != nil {
		t.Errorf("zona = %q, quero nula (talhao sem faixa configurada)", *d.Ultima.Zona)
	}
	// E o JSON precisa trazer a chave com null, nao omiti-la: a interface
	// distingue "nao configurado" de "campo que eu esqueci de ler".
	bruto := req(t, c.h, http.MethodGet, "/api/v1/app/devices/dev-c1", cookie, "").Body.String()
	if !strings.Contains(bruto, `"zona":null`) {
		t.Errorf("zona nula ausente do JSON: %s", bruto)
	}
}

// RISCO: "pior primeiro" escrito como DESC. Pior e o mais negativo, logo ASC.
func TestOrdenacaoPiorPrimeiro(t *testing.T) {
	c := novoCenarioApp(t)
	agora := time.Now().UTC().Add(-time.Hour)
	semearKPa(t, "dev-a1", agora, -10) // conforto
	semearKPa(t, "dev-a2", agora, -70) // estresse
	semearKPa(t, "dev-c1", agora, -40) // no meio
	// Um no recem-instalado, ainda sem leitura nenhuma.
	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO devices (id, descricao, talhao_id, token_hash)
		VALUES ('dev-mudo', 'sem leitura ainda', $1, $2)`,
		c.talhaoA, HashToken(segredoTeste, "tok-mudo")); err != nil {
		t.Fatal(err)
	}
	cookie := entrar(t, c.h, emailA)

	var resp respDevices
	decodificarResp(t, req(t, c.h, http.MethodGet, "/api/v1/app/devices", cookie, ""), &resp)

	var ordem []string
	for _, d := range resp.Devices {
		if d.Ultima != nil {
			ordem = append(ordem, d.ID)
		}
	}
	quero := []string{"dev-a2", "dev-c1", "dev-a1"}
	if fmt.Sprint(ordem) != fmt.Sprint(quero) {
		t.Errorf("ordem = %v, quero %v (mais negativo primeiro)", ordem, quero)
	}
	// No sem leitura nenhuma vai para o fim: ausencia nao e a pior situacao.
	if ultimo := resp.Devices[len(resp.Devices)-1]; ultimo.Ultima != nil {
		t.Errorf("no sem leitura nao ficou por ultimo; ultimo = %s", ultimo.ID)
	}
}

func TestMedidoHaS(t *testing.T) {
	c := novoCenarioApp(t)
	semearKPa(t, "dev-a1", time.Now().UTC().Add(-6*time.Hour), -20)
	cookie := entrar(t, c.h, emailA)

	var d DeviceApp
	decodificarResp(t, req(t, c.h, http.MethodGet, "/api/v1/app/devices/dev-a1", cookie, ""), &d)
	if d.Ultima == nil {
		t.Fatal("sem ultima leitura")
	}
	// ~6 h, com folga generosa: o que importa e que a interface consiga
	// dizer "sem dados ha 6 h" em vez de mostrar verde.
	if d.Ultima.MedidoHaS < 5*3600 || d.Ultima.MedidoHaS > 7*3600 {
		t.Errorf("medido_ha_s = %d, quero perto de %d", d.Ultima.MedidoHaS, 6*3600)
	}
}

// ------------------------------------------------------- RISCO: serie

// RISCO: a media esconde o pior caso. Um unico ponto em estresse dentro de
// um bucket majoritariamente confortavel manda o bucket inteiro para
// estresse, porque a zona sai de min(kpa).
func TestBucketUsaOPiorCasoNaoAMedia(t *testing.T) {
	c := novoCenarioApp(t)
	// Onze leituras num intervalo de 11 min: dez em conforto, uma em
	// estresse. Media ~ -14 kPa (conforto), minimo -70 kPa (estresse).
	base := time.Now().UTC().Add(-30 * time.Minute)
	semearKPa(t, "dev-a1", base, -10, -10, -10, -10, -10, -70, -10, -10, -10, -10, -10)
	cookie := entrar(t, c.h, emailA)

	var resp struct {
		BucketS int          `json:"bucket_s"`
		Pontos  []PontoSerie `json:"pontos"`
	}
	// Bucket de 1 h engole as onze leituras.
	decodificarResp(t, req(t, c.h, http.MethodGet,
		"/api/v1/app/devices/dev-a1/series?bucket=3600", cookie, ""), &resp)

	if len(resp.Pontos) != 1 {
		t.Fatalf("pontos = %d, quero 1; %+v", len(resp.Pontos), resp.Pontos)
	}
	p := resp.Pontos[0]
	if p.N != 11 {
		t.Errorf("n = %d, quero 11", p.N)
	}
	if p.KPaMed < -20 {
		t.Errorf("kpa_med = %g; a media deveria estar em conforto, senao o teste nao prova nada", p.KPaMed)
	}
	if p.Zona == nil || *p.Zona != ZonaEstresse {
		t.Errorf("zona do bucket = %v, quero estresse (min = %g, med = %g)",
			valor(p.Zona), p.KPaMin, p.KPaMed)
	}
}

// RISCO: lacuna virou linha reta. Bucket sem leitura nao gera linha; o
// periodo offline continua sendo ausencia de dado.
func TestLacunaSobreviveAAgregacao(t *testing.T) {
	c := novoCenarioApp(t)
	// Truncate(time.Hour) alinha as ilhas ao bucket. semearKPa espaca as
	// leituras de 1 minuto, entao sem isso uma ilha semeada faltando 1 minuto
	// para a hora cai em DOIS buckets e o teste acusa 4 pontos onde quer 2 --
	// falha que so aparece quando o relogio esta no minuto 59.
	agora := time.Now().UTC().Truncate(time.Hour)
	semearKPa(t, "dev-a1", agora.Add(-20*time.Hour), -20, -21)
	semearKPa(t, "dev-a1", agora.Add(-2*time.Hour), -30, -31)
	cookie := entrar(t, c.h, emailA)

	var resp struct {
		BucketS int          `json:"bucket_s"`
		Pontos  []PontoSerie `json:"pontos"`
	}
	decodificarResp(t, req(t, c.h, http.MethodGet,
		"/api/v1/app/devices/dev-a1/series?bucket=3600", cookie, ""), &resp)

	if len(resp.Pontos) != 2 {
		t.Fatalf("pontos = %d, quero 2 (um por ilha de leituras); %+v", len(resp.Pontos), resp.Pontos)
	}
	// bucket_s vem na resposta para que a interface detecte a lacuna sem
	// adivinhar o periodo de amostragem.
	if resp.BucketS != 3600 {
		t.Errorf("bucket_s = %d, quero 3600", resp.BucketS)
	}
	if delta := resp.Pontos[1].T.Sub(resp.Pontos[0].T); delta <= time.Duration(1.5*float64(time.Hour)) {
		t.Errorf("lacuna de %v nao excede 1.5 x bucket_s; a interface nao a detectaria", delta)
	}
}

// RISCO: historico truncado. Intervalo longo precisa devolver pontos
// distribuidos no intervalo, e nao so o ultimo dia.
func TestIntervaloLongoNaoTrunca(t *testing.T) {
	c := novoCenarioApp(t)
	agora := time.Now().UTC()
	// Uma leitura por dia ao longo de 180 dias.
	for i := 0; i < 180; i++ {
		if _, err := testPool.Exec(context.Background(), `
			INSERT INTO readings (device_id, seq, measured_at, raw_mv, kpa)
			VALUES ('dev-a1', $1, $2, 3000, -20)`,
			int64(i+1), agora.AddDate(0, 0, -i)); err != nil {
			t.Fatal(err)
		}
	}
	cookie := entrar(t, c.h, emailA)

	de := agora.AddDate(0, 0, -180).Format(time.RFC3339)
	var resp struct {
		BucketS int          `json:"bucket_s"`
		Pontos  []PontoSerie `json:"pontos"`
	}
	decodificarResp(t, req(t, c.h, http.MethodGet,
		"/api/v1/app/devices/dev-a1/series?from="+de, cookie, ""), &resp)

	if len(resp.Pontos) < 100 {
		t.Fatalf("pontos = %d; 180 dias deveriam render a serie inteira, nao so o fim", len(resp.Pontos))
	}
	// O primeiro ponto precisa estar perto do inicio do intervalo pedido.
	if idade := agora.Sub(resp.Pontos[0].T); idade < 170*24*time.Hour {
		t.Errorf("primeiro ponto tem %v de idade; a serie foi truncada no fim do intervalo", idade)
	}
	// E o bucket automatico precisa ter subido de 60 s.
	if resp.BucketS <= 3600 {
		t.Errorf("bucket_s = %d; para 180 dias o bucket automatico deveria ser maior", resp.BucketS)
	}
}

func TestBucketAutomatico(t *testing.T) {
	for _, caso := range []struct {
		janela time.Duration
		quer   int
	}{
		{24 * time.Hour, 60},
		{14 * 24 * time.Hour, 900},
		{180 * 24 * time.Hour, 10800},
		{5 * 365 * 24 * time.Hour, 86400},
	} {
		if got := bucketAutomatico(caso.janela); got != caso.quer {
			t.Errorf("bucketAutomatico(%v) = %d, quero %d", caso.janela, got, caso.quer)
		}
	}
}

func TestSerieParametrosInvalidos(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)
	for _, q := range []string{"?from=ontem", "?bucket=zero", "?bucket=-1", "?bucket=0"} {
		if rec := req(t, c.h, http.MethodGet, "/api/v1/app/devices/dev-a1/series"+q, cookie, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, quero 400", q, rec.Code)
		}
	}
	// Bucket minusculo sobre intervalo longo: recusado antes de virar meio
	// milhao de linhas.
	de := time.Now().UTC().AddDate(0, 0, -180).Format(time.RFC3339)
	if rec := req(t, c.h, http.MethodGet,
		"/api/v1/app/devices/dev-a1/series?bucket=1&from="+de, cookie, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bucket=1 em 180 dias: status = %d, quero 400", rec.Code)
	}
}

// ------------------------------------------------------ RISCO: talhoes

func TestTalhoesConcedidos(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	var resp struct {
		Talhoes []Talhao `json:"talhoes"`
	}
	decodificarResp(t, req(t, c.h, http.MethodGet, "/api/v1/app/talhoes", cookie, ""), &resp)
	if len(resp.Talhoes) != 2 {
		t.Fatalf("talhoes = %d, quero 2 (A e C); %+v", len(resp.Talhoes), resp.Talhoes)
	}
	for _, tl := range resp.Talhoes {
		if tl.ID == c.talhaoB {
			t.Error("talhao do vizinho na lista")
		}
	}
}

func TestPatchTalhaoConfiguraFaixa(t *testing.T) {
	c := novoCenarioApp(t)
	semearKPa(t, "dev-c1", time.Now().UTC().Add(-time.Hour), -40)
	cookie := entrar(t, c.h, emailA)

	// Antes: talhao C nao tem faixa, entao a zona e nula.
	var d DeviceApp
	decodificarResp(t, req(t, c.h, http.MethodGet, "/api/v1/app/devices/dev-c1", cookie, ""), &d)
	if d.Ultima.Zona != nil {
		t.Fatalf("zona = %q antes de configurar a faixa", *d.Ultima.Zona)
	}

	var tl Talhao
	decodificarResp(t, req(t, c.h, http.MethodPatch, "/api/v1/app/talhoes/"+c.talhaoC, cookie,
		`{"cultura":"Milho","kpa_alerta":-30,"kpa_estresse":-60}`), &tl)
	if tl.Cultura == nil || *tl.Cultura != "Milho" || tl.KPaAlerta == nil || *tl.KPaAlerta != -30 {
		t.Fatalf("talhao apos PATCH = %+v", tl)
	}

	// Depois: -40 kPa com limiares -30/-60 e alerta.
	decodificarResp(t, req(t, c.h, http.MethodGet, "/api/v1/app/devices/dev-c1", cookie, ""), &d)
	if d.Ultima.Zona == nil || *d.Ultima.Zona != ZonaAlerta {
		t.Errorf("zona = %v, quero alerta", valor(d.Ultima.Zona))
	}
}

func TestPatchTalhaoFaixaInvalida(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)
	for _, caso := range []struct{ nome, corpo string }{
		// kpa_estresse e o MAIS NEGATIVO; invertido nao passa.
		{"limiares invertidos", `{"kpa_alerta":-60,"kpa_estresse":-30}`},
		{"limiares iguais", `{"kpa_alerta":-30,"kpa_estresse":-30}`},
		{"so um limiar", `{"kpa_alerta":-30}`},
		{"alem da cavitacao", `{"kpa_alerta":-30,"kpa_estresse":-95}`},
		{"positivo", `{"kpa_alerta":10,"kpa_estresse":-30}`},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			rec := req(t, c.h, http.MethodPatch, "/api/v1/app/talhoes/"+c.talhaoA, cookie, caso.corpo)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, quero 400; corpo: %s", rec.Code, rec.Body.String())
			}
		})
	}
	// E nada foi gravado.
	var alerta float32
	if err := testPool.QueryRow(context.Background(),
		`SELECT kpa_alerta FROM talhoes WHERE id = $1`, c.talhaoA).Scan(&alerta); err != nil {
		t.Fatal(err)
	}
	if alerta != -30 {
		t.Errorf("kpa_alerta = %g, quero -30 intacto", alerta)
	}
}

// Talhao do vizinho: 404, e o mesmo 404 de um uuid que nao existe.
func TestPatchTalhaoDeOutroUsuario(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	rec := req(t, c.h, http.MethodPatch, "/api/v1/app/talhoes/"+c.talhaoB, cookie,
		`{"cultura":"Invadida"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quero 404; corpo: %s", rec.Code, rec.Body.String())
	}

	var cultura *string
	if err := testPool.QueryRow(context.Background(),
		`SELECT cultura FROM talhoes WHERE id = $1`, c.talhaoB).Scan(&cultura); err != nil {
		t.Fatal(err)
	}
	if cultura != nil && *cultura == "Invadida" {
		t.Error("talhao do vizinho foi alterado")
	}

	fantasma := req(t, c.h, http.MethodPatch,
		"/api/v1/app/talhoes/00000000-0000-0000-0000-000000000000", cookie, `{"cultura":"x"}`)
	if fantasma.Body.String() != rec.Body.String() {
		t.Errorf("talhao alheio e inexistente se distinguem: %s / %s", rec.Body.String(), fantasma.Body.String())
	}
}

// ------------------------------------------------------------------ me

func TestMe(t *testing.T) {
	c := novoCenarioApp(t)
	cookie := entrar(t, c.h, emailA)

	var resp struct {
		Usuario Usuario  `json:"usuario"`
		Talhoes []Talhao `json:"talhoes"`
	}
	decodificarResp(t, req(t, c.h, http.MethodGet, "/api/v1/app/me", cookie, ""), &resp)
	if resp.Usuario.Email != emailA || len(resp.Talhoes) != 2 {
		t.Errorf("me = %+v", resp)
	}
	// O hash da senha nao pode vazar em nenhum campo.
	if bruto := req(t, c.h, http.MethodGet, "/api/v1/app/me", cookie, "").Body.String(); strings.Contains(bruto, "$2a$") {
		t.Errorf("hash de senha no corpo de /me: %s", bruto)
	}
}

func TestUUIDValido(t *testing.T) {
	if !uuidValido("00000000-0000-0000-0000-000000000000") {
		t.Error("uuid nulo recusado")
	}
	for _, s := range []string{
		"", "nao-e-uuid",
		"00000000-0000-0000-0000-00000000000",   // curto
		"00000000-0000-0000-0000-0000000000000", // longo
		"00000000_0000-0000-0000-000000000000",  // separador errado
		"0000000g-0000-0000-0000-000000000000",  // nao-hex
	} {
		if uuidValido(s) {
			t.Errorf("uuidValido(%q) = true", s)
		}
	}
}

func valor(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte(senhaTeste), CustoBcrypt)
	if err != nil {
		panic(err)
	}
	hashSenhaTeste = string(h)
}
