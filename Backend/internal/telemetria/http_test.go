package telemetria

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/igorkg/tcc/backend/internal/config"
	"github.com/igorkg/tcc/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Os testes rodam contra um Postgres real (TEST_DATABASE_URL). O que esta
// sob teste aqui e justamente ON CONFLICT, chave estrangeira e TIMESTAMPTZ
// -- nenhum mock reproduz isso.
//
//	docker run -d --name tcc-pg -e POSTGRES_PASSWORD=tcc -e POSTGRES_USER=tcc \
//	  -e POSTGRES_DB=telemetria_test -p 55432:5432 postgres:16-alpine
//	export TEST_DATABASE_URL='postgres://tcc:tcc@localhost:55432/telemetria_test?sslmode=disable'

const (
	segredoTeste = "segredo-de-teste"
	tokenOK      = "token-do-tensio-01"
	tokenOutro   = "token-do-tensio-02"
	tokenInativo = "token-do-no-desativado"
	calOK        = "cal-2026-08-20-a"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	config.Carregar()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "TEST_DATABASE_URL nao definida; testes de integracao pulados")
		os.Exit(0)
	}
	if err := migrations.Aplicar(url); err != nil {
		fmt.Fprintln(os.Stderr, "migrations falharam:", err)
		os.Exit(1)
	}
	pool, err := NovoPool(context.Background(), url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "conexao falhou:", err)
		os.Exit(1)
	}
	defer pool.Close()
	testPool = pool
	os.Exit(m.Run())
}

// ------------------------------------------------------------- fixtures

func novaAPI(t *testing.T) (http.Handler, *bytes.Buffer) {
	t.Helper()
	ctx := context.Background()

	if _, err := testPool.Exec(ctx,
		`TRUNCATE readings, calibrations, devices, talhoes CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	var talhaoID string
	if err := testPool.QueryRow(ctx,
		`INSERT INTO talhoes (nome) VALUES ('Talhao Norte') RETURNING id`).Scan(&talhaoID); err != nil {
		t.Fatalf("seed talhao: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO devices (id, descricao, talhao_id, token_hash, ativo) VALUES
		  ('tensio-01', 'no do talhao norte', $1, $2, true),
		  ('tensio-02', 'no do talhao sul',   NULL, $3, true),
		  ('tensio-off','no desativado',      NULL, $4, false)`,
		talhaoID, HashToken(segredoTeste, tokenOK),
		HashToken(segredoTeste, tokenOutro), HashToken(segredoTeste, tokenInativo)); err != nil {
		t.Fatalf("seed devices: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO calibrations
		  (id, device_id, v_zero_kpa, k_v_por_kpa, fator_divisor, vdd_ensaio_mv, ensaio_em)
		VALUES ($1, 'tensio-01', 4.5, 0.04, 1.5, 5000, '2026-08-20')`, calOK); err != nil {
		t.Fatalf("seed calibration: %v", err)
	}

	var logs bytes.Buffer
	api := NewAPI(NewStore(testPool), segredoTeste, slog.New(slog.NewTextHandler(&logs, nil)))
	return api.Router(), &logs
}

// medido devolve um instante dentro da janela plausivel, deslocado para
// que os testes nao dependam da data real do relogio.
func medido(deslocamento time.Duration) string {
	return time.Now().UTC().Add(-time.Hour + deslocamento).Format(time.RFC3339)
}

func leitura(seq int, quando string, rawMV int, kpa float64) string {
	return fmt.Sprintf(
		`{"seq":%d,"measured_at":%q,"raw_mv":%d,"vdd_mv":4980,"kpa":%g,"calibration_id":%q}`,
		seq, quando, rawMV, kpa, calOK)
}

func corpo(deviceID string, leituras ...string) string {
	return fmt.Sprintf(`{"device_id":%q,"readings":[%s]}`, deviceID, strings.Join(leituras, ","))
}

func post(t *testing.T, h http.Handler, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/readings", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func get(t *testing.T, h http.Handler, token, caminho string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, caminho, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func lerLote(t *testing.T, rec *httptest.ResponseRecorder) loteResposta {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quero 200; corpo: %s", rec.Code, rec.Body.String())
	}
	var resp loteResposta
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

func conferir(t *testing.T, resp loteResposta, aceitas, duplicadas, rejeitadas int) {
	t.Helper()
	if resp.Accepted != aceitas || resp.Duplicates != duplicadas || resp.Rejected != rejeitadas {
		t.Errorf("accepted/duplicates/rejected = %d/%d/%d, quero %d/%d/%d (rejeicoes: %+v)",
			resp.Accepted, resp.Duplicates, resp.Rejected, aceitas, duplicadas, rejeitadas, resp.Rejections)
	}
	if resp.Rejected != len(resp.Rejections) {
		t.Errorf("Rejected=%d nao bate com len(Rejections)=%d", resp.Rejected, len(resp.Rejections))
	}
}

// ------------------------------------------------------------- ingestao

func TestLoteNormal(t *testing.T) {
	h, _ := novaAPI(t)
	resp := lerLote(t, post(t, h, tokenOK, corpo("tensio-01",
		leitura(1043, medido(0), 3016, -12.4),
		leitura(1044, medido(time.Minute), 3010, -12.9))))
	conferir(t, resp, 2, 0, 0)

	if resp.ServerTime.IsZero() {
		t.Error("server_time ausente")
	}
	var n int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM readings WHERE device_id='tensio-01'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("gravadas %d leituras, quero 2", n)
	}
}

func TestDuplicataCompleta(t *testing.T) {
	h, _ := novaAPI(t)
	lote := corpo("tensio-01",
		leitura(1043, medido(0), 3016, -12.4),
		leitura(1044, medido(time.Minute), 3010, -12.9))

	conferir(t, lerLote(t, post(t, h, tokenOK, lote)), 2, 0, 0)
	// Reenvio apos resposta perdida: descartado em silencio, nao e erro.
	conferir(t, lerLote(t, post(t, h, tokenOK, lote)), 0, 2, 0)
}

func TestDuplicataParcial(t *testing.T) {
	h, _ := novaAPI(t)
	conferir(t, lerLote(t, post(t, h, tokenOK, corpo("tensio-01",
		leitura(1043, medido(0), 3016, -12.4)))), 1, 0, 0)

	// O no reenvia 1043 junto com duas leituras novas.
	conferir(t, lerLote(t, post(t, h, tokenOK, corpo("tensio-01",
		leitura(1043, medido(0), 3016, -12.4),
		leitura(1044, medido(time.Minute), 3010, -12.9),
		leitura(1045, medido(2*time.Minute), 3005, -13.2)))), 2, 1, 0)
}

func TestForaDeFaixaNoMeioDoLote(t *testing.T) {
	casos := []struct {
		nome   string
		ruim   string
		trecho string
	}{
		{"kpa abaixo do minimo", leitura(1044, medido(time.Minute), 3016, -140.2), "kpa fora da faixa"},
		{"kpa acima do maximo", leitura(1044, medido(time.Minute), 3016, 55), "kpa fora da faixa"},
		{"raw_mv acima do maximo", leitura(1044, medido(time.Minute), 4200, -12.4), "raw_mv fora da faixa"},
		{"raw_mv negativo", leitura(1044, medido(time.Minute), -5, -12.4), "raw_mv fora da faixa"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			h, _ := novaAPI(t)
			resp := lerLote(t, post(t, h, tokenOK, corpo("tensio-01",
				leitura(1043, medido(0), 3016, -12.4),
				c.ruim,
				leitura(1045, medido(2*time.Minute), 3005, -13.2))))

			// A leitura ruim cai sozinha; as duas boas entram.
			conferir(t, resp, 2, 0, 1)
			if got := resp.Rejections[0].Seq; got == nil || *got != 1044 {
				t.Errorf("seq rejeitado = %v, quero 1044", got)
			}
			if resp.Rejections[0].Index != 1 {
				t.Errorf("index rejeitado = %d, quero 1", resp.Rejections[0].Index)
			}
			if !strings.Contains(resp.Rejections[0].Reason, c.trecho) {
				t.Errorf("motivo = %q, quero conter %q", resp.Rejections[0].Reason, c.trecho)
			}
		})
	}
}

func TestMeasuredAtEm1970(t *testing.T) {
	h, _ := novaAPI(t)
	resp := lerLote(t, post(t, h, tokenOK, corpo("tensio-01",
		leitura(1, "1970-01-01T00:00:00Z", 3016, -12.4),
		leitura(2, medido(0), 3010, -12.9))))

	conferir(t, resp, 1, 0, 1)
	if !strings.Contains(resp.Rejections[0].Reason, "measured_at fora da janela") {
		t.Errorf("motivo = %q", resp.Rejections[0].Reason)
	}
}

func TestMeasuredAtNoFuturo(t *testing.T) {
	h, _ := novaAPI(t)
	futuro := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
	resp := lerLote(t, post(t, h, tokenOK, corpo("tensio-01", leitura(1, futuro, 3016, -12.4))))
	conferir(t, resp, 0, 0, 1)
}

func TestLoteAcimaDoLimite(t *testing.T) {
	h, _ := novaAPI(t)
	leituras := make([]string, MaxLeiturasPorLote+1)
	for i := range leituras {
		leituras[i] = leitura(i, medido(time.Duration(i)*time.Second), 3016, -12.4)
	}
	rec := post(t, h, tokenOK, corpo("tensio-01", leituras...))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, quero 413; corpo: %s", rec.Code, rec.Body.String())
	}
	// O firmware precisa do numero para fatiar o buffer.
	if !strings.Contains(rec.Body.String(), fmt.Sprint(MaxLeiturasPorLote)) {
		t.Errorf("mensagem de 413 nao informa o limite: %s", rec.Body.String())
	}
}

func TestLoteNoLimiteExato(t *testing.T) {
	h, _ := novaAPI(t)
	leituras := make([]string, MaxLeiturasPorLote)
	for i := range leituras {
		leituras[i] = leitura(i, medido(time.Duration(i)*time.Second), 3016, -12.4)
	}
	conferir(t, lerLote(t, post(t, h, tokenOK, corpo("tensio-01", leituras...))), MaxLeiturasPorLote, 0, 0)
}

func TestTokenInvalido(t *testing.T) {
	h, _ := novaAPI(t)
	lote := corpo("tensio-01", leitura(1043, medido(0), 3016, -12.4))

	for _, c := range []struct {
		nome, token string
	}{
		{"token desconhecido", "token-que-nao-existe"},
		{"sem authorization", ""},
	} {
		t.Run(c.nome, func(t *testing.T) {
			if rec := post(t, h, c.token, lote); rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, quero 401; corpo: %s", rec.Code, rec.Body.String())
			}
		})
	}

	var n int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM readings`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("gravou %d leituras com token invalido", n)
	}
}

func TestDeviceIDDiferenteDoToken(t *testing.T) {
	h, _ := novaAPI(t)
	// Token do tensio-02 tentando escrever na serie do tensio-01.
	rec := post(t, h, tokenOutro, corpo("tensio-01", leitura(1043, medido(0), 3016, -12.4)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, quero 403; corpo: %s", rec.Code, rec.Body.String())
	}

	var n int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM readings WHERE device_id='tensio-01'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("gravou %d leituras na serie alheia", n)
	}
}

func TestCalibrationIDInexistente(t *testing.T) {
	h, _ := novaAPI(t)
	desconhecida := strings.Replace(
		leitura(1044, medido(time.Minute), 3010, -12.9), calOK, "cal-que-nao-existe", 1)

	resp := lerLote(t, post(t, h, tokenOK, corpo("tensio-01",
		leitura(1043, medido(0), 3016, -12.4),
		desconhecida,
		leitura(1045, medido(2*time.Minute), 3005, -13.2))))

	// FK invalida derrubaria o INSERT inteiro; aqui cai so a leitura ruim.
	conferir(t, resp, 2, 0, 1)
	if got := resp.Rejections[0].Seq; got == nil || *got != 1044 {
		t.Errorf("seq rejeitado = %v, quero 1044", got)
	}
	if !strings.Contains(resp.Rejections[0].Reason, "calibration_id desconhecido") {
		t.Errorf("motivo = %q", resp.Rejections[0].Reason)
	}
}

func TestCalibracaoDeOutroDevice(t *testing.T) {
	h, _ := novaAPI(t)
	// calOK pertence ao tensio-01; o tensio-02 nao pode referencia-la.
	resp := lerLote(t, post(t, h, tokenOutro, corpo("tensio-02",
		leitura(1, medido(0), 3016, -12.4))))
	conferir(t, resp, 0, 0, 1)
}

func TestCalibrationIDAusente(t *testing.T) {
	h, _ := novaAPI(t)
	semCal := fmt.Sprintf(`{"seq":1,"measured_at":%q,"raw_mv":3016,"vdd_mv":4980,"kpa":-12.4}`, medido(0))
	resp := lerLote(t, post(t, h, tokenOK, corpo("tensio-01", semCal)))

	// Sem calibracao a leitura nao pode ser reprocessada a partir de raw_mv.
	conferir(t, resp, 0, 0, 1)
	if !strings.Contains(resp.Rejections[0].Reason, "calibration_id ausente") {
		t.Errorf("motivo = %q", resp.Rejections[0].Reason)
	}
}

func TestDeviceInativo(t *testing.T) {
	h, _ := novaAPI(t)
	rec := post(t, h, tokenInativo, corpo("tensio-off", leitura(1, medido(0), 3016, -12.4)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, quero 403; corpo: %s", rec.Code, rec.Body.String())
	}
}

func TestJSONInvalido(t *testing.T) {
	h, _ := novaAPI(t)
	if rec := post(t, h, tokenOK, `{"device_id":`); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, quero 400", rec.Code)
	}
}

// TestAlertaResetDeSeq: reenvio legitimo nao alarma; seq repetido com
// outro measured_at alarma.
func TestAlertaResetDeSeq(t *testing.T) {
	h, logs := novaAPI(t)
	original := corpo("tensio-01", leitura(1, medido(0), 3016, -12.4))
	conferir(t, lerLote(t, post(t, h, tokenOK, original)), 1, 0, 0)

	conferir(t, lerLote(t, post(t, h, tokenOK, original)), 0, 1, 0)
	if strings.Contains(logs.String(), "possivel reset") {
		t.Errorf("reenvio identico nao deveria alarmar: %s", logs.String())
	}

	// Apos reflash o contador volta a 1, mas a leitura e outra.
	conferir(t, lerLote(t, post(t, h, tokenOK,
		corpo("tensio-01", leitura(1, medido(-6*time.Hour), 2900, -25.0)))), 0, 1, 0)
	if !strings.Contains(logs.String(), "possivel reset") {
		t.Errorf("seq colidindo com measured_at diferente deveria alarmar: %s", logs.String())
	}
}

// -------------------------------------------------------------- consulta

func semearLeituras(t *testing.T, h http.Handler) {
	t.Helper()
	conferir(t, lerLote(t, post(t, h, tokenOK, corpo("tensio-01",
		leitura(1, medido(0), 3016, -12.4),
		leitura(2, medido(time.Minute), 3010, -12.9),
		leitura(3, medido(2*time.Minute), 3005, -13.2)))), 3, 0, 0)
}

type respostaLista struct {
	DeviceID string    `json:"device_id"`
	Count    int       `json:"count"`
	Readings []Reading `json:"readings"`
}

func TestListarLeituras(t *testing.T) {
	h, _ := novaAPI(t)
	semearLeituras(t, h)

	rec := get(t, h, tokenOK, "/api/v1/devices/tensio-01/readings")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; corpo: %s", rec.Code, rec.Body.String())
	}
	var resp respostaLista
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 || len(resp.Readings) != 3 {
		t.Fatalf("count = %d, len = %d, quero 3", resp.Count, len(resp.Readings))
	}
	// Mais recente primeiro.
	if resp.Readings[0].Seq != 3 || resp.Readings[2].Seq != 1 {
		t.Errorf("ordem = %d,%d,%d, quero 3,2,1",
			resp.Readings[0].Seq, resp.Readings[1].Seq, resp.Readings[2].Seq)
	}
	if resp.Readings[0].MeasuredAt.Location() != time.UTC {
		t.Errorf("measured_at nao veio em UTC: %v", resp.Readings[0].MeasuredAt.Location())
	}
	if resp.Readings[0].VddMV == nil || *resp.Readings[0].VddMV != 4980 {
		t.Errorf("vdd_mv = %v, quero 4980", resp.Readings[0].VddMV)
	}
	if resp.Readings[0].CalibrationID == nil || *resp.Readings[0].CalibrationID != calOK {
		t.Errorf("calibration_id = %v", resp.Readings[0].CalibrationID)
	}
}

func TestListarComLimite(t *testing.T) {
	h, _ := novaAPI(t)
	semearLeituras(t, h)

	rec := get(t, h, tokenOK, "/api/v1/devices/tensio-01/readings?limit=2")
	var resp respostaLista
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Errorf("count = %d, quero 2", resp.Count)
	}
}

func TestListarComJanela(t *testing.T) {
	h, _ := novaAPI(t)
	semearLeituras(t, h)

	// Janela que exclui a leitura mais antiga (seq 1).
	de := time.Now().UTC().Add(-time.Hour + 30*time.Second).Format(time.RFC3339)
	rec := get(t, h, tokenOK, "/api/v1/devices/tensio-01/readings?from="+de)
	var resp respostaLista
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("count = %d, quero 2 (corpo: %s)", resp.Count, rec.Body.String())
	}
	for _, r := range resp.Readings {
		if r.Seq == 1 {
			t.Error("leitura fora da janela retornada")
		}
	}
}

func TestListarParametrosInvalidos(t *testing.T) {
	h, _ := novaAPI(t)
	for _, q := range []string{"?from=ontem", "?limit=zero", "?limit=-1"} {
		if rec := get(t, h, tokenOK, "/api/v1/devices/tensio-01/readings"+q); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, quero 400", q, rec.Code)
		}
	}
}

func TestUltimaLeitura(t *testing.T) {
	h, _ := novaAPI(t)
	semearLeituras(t, h)

	rec := get(t, h, tokenOK, "/api/v1/devices/tensio-01/latest")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; corpo: %s", rec.Code, rec.Body.String())
	}
	var r Reading
	if err := json.NewDecoder(rec.Body).Decode(&r); err != nil {
		t.Fatal(err)
	}
	if r.Seq != 3 {
		t.Errorf("seq = %d, quero 3", r.Seq)
	}
}

func TestUltimaLeituraSemDados(t *testing.T) {
	h, _ := novaAPI(t)
	if rec := get(t, h, tokenOK, "/api/v1/devices/tensio-01/latest"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, quero 404", rec.Code)
	}
}

func TestConsultaDeOutroDevice(t *testing.T) {
	h, _ := novaAPI(t)
	semearLeituras(t, h)

	for _, caminho := range []string{
		"/api/v1/devices/tensio-01/readings",
		"/api/v1/devices/tensio-01/latest",
	} {
		if rec := get(t, h, tokenOutro, caminho); rec.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, quero 403", caminho, rec.Code)
		}
	}
}

// A outra metade da separacao testada em internal/webapp: la se prova que a
// rota desconhecida sob /api/ chega na API em vez de virar index.html; aqui,
// que o que a API responde nesse caso tem a forma que o cliente espera.
//
// Sem o fundo de poco explicito, o ServeMux responderia o 404 padrao em
// text/plain -- que tambem nao e HTML, mas faria o cliente reclamar de
// content-type em vez de dizer que o endereco nao existe.
func TestRotaDesconhecidaSobAPIRespondeJSON(t *testing.T) {
	h, _ := novaAPI(t)

	for _, caminho := range []string{
		"/api/v1/naoexiste",
		"/api/v1/app/devicess",
		"/api/v1/devices/tensio-01/leituras",
	} {
		rec := get(t, h, tokenOK, caminho)

		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, quero 404", caminho, rec.Code)
		}
		if tipo := rec.Header().Get("Content-Type"); !strings.HasPrefix(tipo, "application/json") {
			t.Errorf("%s: content-type = %q, quero application/json", caminho, tipo)
		}
		var corpo struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
			t.Errorf("%s: corpo nao e JSON: %v", caminho, err)
		}
		if corpo.Error == "" {
			t.Errorf("%s: JSON sem campo error", caminho)
		}
	}
}
