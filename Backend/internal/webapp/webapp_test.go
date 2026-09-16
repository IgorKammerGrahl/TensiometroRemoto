package webapp

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
)

// apiFalsa nao registra rota nenhuma: e a API mais vazia possivel. Se mesmo
// assim uma requisicao sob /api/ chegar ate ela em vez de virar index.html, a
// separacao de webapp.Servir esta de pe -- e essa e a unica coisa que estes
// testes precisam provar sobre o prefixo.
func apiFalsa(chamada *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*chamada = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"rota nao encontrada"}`))
	})
}

func pedir(t *testing.T, h http.Handler, metodo, caminho string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(metodo, caminho, nil))
	return rec
}

// O modo de falha da etapa 6, e o motivo de este arquivo existir: endpoint
// digitado errado responder 200 com index.html. O cliente faria parse de HTML
// como JSON e o sintoma -- erro de sintaxe -- nao apontaria para a causa.
// Pior: 200 e sucesso para qualquer camada acima.
func TestRotaDesconhecidaSobAPINuncaViraHTML(t *testing.T) {
	for _, caminho := range []string{
		"/api/",
		"/api/v1/naoexiste",
		"/api/v1/app/devicess",        // erro de digitacao plausivel
		"/api/v1/app/devices/x/serie", // singular no lugar de "series"
	} {
		t.Run(caminho, func(t *testing.T) {
			var chegou bool
			rec := pedir(t, Servir(apiFalsa(&chegou)), http.MethodGet, caminho)

			if !chegou {
				t.Fatalf("%s nao chegou na API: o fallback da SPA engoliu a rota", caminho)
			}
			if rec.Code == http.StatusOK {
				t.Errorf("%s respondeu 200; rota de API desconhecida nao pode parecer sucesso", caminho)
			}
			if tipo := rec.Header().Get("Content-Type"); strings.Contains(tipo, "text/html") {
				t.Errorf("%s respondeu %s; o cliente espera JSON", caminho, tipo)
			}
			if strings.Contains(rec.Body.String(), "<!doctype") {
				t.Errorf("%s devolveu index.html", caminho)
			}
		})
	}
}

// `..` no caminho nao e forma de sair do prefixo: o ServeMux normaliza antes
// de despachar, entao a tentativa vira um redirecionamento para o caminho
// limpo -- que continua sob /api/ e continua sem chegar na PWA.
func TestTraversalNaoEscapaDoPrefixoDaAPI(t *testing.T) {
	var chegou bool
	rec := pedir(t, Servir(apiFalsa(&chegou)), http.MethodGet, "/api/v1/app/devices/../../no")

	destino := rec.Header().Get("Location")
	if destino == "" {
		destino = "(sem redirecionamento)"
	}
	if !strings.HasPrefix(destino, "/api/") {
		t.Errorf("caminho normalizado foi para %s; deveria continuar sob /api/", destino)
	}
	if strings.Contains(rec.Body.String(), "<!doctype") {
		t.Error("traversal alcancou o index.html")
	}
}

func TestRotaDaAplicacaoRecebeIndex(t *testing.T) {
	var chegou bool
	h := Servir(apiFalsa(&chegou))

	// Rotas que so existem no react-router: nao ha arquivo com esses nomes.
	for _, caminho := range []string{"/", "/no/tensio-01", "/nos/novo"} {
		rec := pedir(t, h, http.MethodGet, caminho)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d, esperado 200", caminho, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "<!doctype html>") {
			t.Errorf("%s: corpo nao e o index.html", caminho)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
			t.Errorf("%s: Cache-Control %q; index com nome fixo precisa revalidar", caminho, cc)
		}
	}
	if chegou {
		t.Error("rota da aplicacao vazou para a API")
	}
}

// O bundle e imutavel porque o nome tem hash de conteudo; o resto nao tem
// hash nenhum e cachear seria servir interface velha depois do proximo build.
func TestCacheSeparaHashDeNomeFixo(t *testing.T) {
	var chegou bool
	h := Servir(apiFalsa(&chegou))

	entradas, err := fs.ReadDir(embutido, "dist/assets")
	if err != nil {
		t.Fatalf("dist/assets ausente; rode `npm run build` em App/: %v", err)
	}
	if len(entradas) == 0 {
		t.Fatal("dist/assets vazio")
	}
	asset := path.Join("/assets", entradas[0].Name())

	rec := pedir(t, h, http.MethodGet, asset)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: status %d", asset, rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("%s: Cache-Control %q; nome com hash pode ser imutavel", asset, cc)
	}

	// sw.js e o caso em que cachear se perpetua: um service worker velho e
	// quem decide o que buscar depois, inclusive a propria atualizacao.
	rec = pedir(t, h, http.MethodGet, "/sw.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("/sw.js: status %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("/sw.js: Cache-Control %q, esperado no-cache", cc)
	}
}

// Manifesto em text/plain e o defeito que nao da erro: o navegador
// simplesmente nao oferece instalar, e nada na tela diz por que.
func TestManifestoSaiComOTipoQueTornaInstalavel(t *testing.T) {
	var chegou bool
	rec := pedir(t, Servir(apiFalsa(&chegou)), http.MethodGet, "/manifest.webmanifest")

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if tipo := rec.Header().Get("Content-Type"); tipo != "application/manifest+json" {
		t.Errorf("Content-Type = %q, quero application/manifest+json", tipo)
	}
}

func TestEscritaNaPWANaoEConfundidaComAPI(t *testing.T) {
	var chegou bool
	rec := pedir(t, Servir(apiFalsa(&chegou)), http.MethodPost, "/nos/novo")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST em rota de PWA: status %d, esperado 405", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "<!doctype") {
		t.Error("POST em rota de PWA devolveu index.html")
	}
}
