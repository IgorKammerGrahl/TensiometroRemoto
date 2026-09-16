package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalisar(t *testing.T) {
	casos := []struct {
		linha, chave, valor string
		ok                  bool
	}{
		{linha: "PORT=8080", chave: "PORT", valor: "8080", ok: true},
		{linha: "  PORT = 8080  ", chave: "PORT", valor: "8080", ok: true},
		{linha: "export PORT=8080", chave: "PORT", valor: "8080", ok: true},
		{linha: `PORT="8080"`, chave: "PORT", valor: "8080", ok: true},
		{linha: "# comentario", ok: false},
		{linha: "", ok: false},
		{linha: "sem_igual", ok: false},
		{linha: "=sem_chave", ok: false},

		// Os dois valores reais do projeto. `=` de padding do base64 e o
		// `?sslmode=` da URL sao o motivo do Cut no primeiro `=` so.
		{
			linha: "TELEMETRIA_TOKEN_SECRET=Zm9vYmFyYmF6cXV1eA==",
			chave: "TELEMETRIA_TOKEN_SECRET", valor: "Zm9vYmFyYmF6cXV1eA==", ok: true,
		},
		{
			linha: "DATABASE_URL=postgres://tcc:tcc@localhost:55432/telemetria?sslmode=disable",
			chave: "DATABASE_URL", valor: "postgres://tcc:tcc@localhost:55432/telemetria?sslmode=disable", ok: true,
		},
	}
	for _, c := range casos {
		chave, valor, ok := analisar(c.linha)
		if ok != c.ok || chave != c.chave || valor != c.valor {
			t.Errorf("analisar(%q) = (%q, %q, %v); quer (%q, %q, %v)",
				c.linha, chave, valor, ok, c.chave, c.valor, c.ok)
		}
	}
}

// O comportamento que a producao depende: variavel ja no ambiente nao e
// sobrescrita pelo arquivo em disco.
func TestCarregarNaoSobrescreveAmbiente(t *testing.T) {
	dir := t.TempDir()
	conteudo := "DO_AMBIENTE=valor_do_arquivo\nSO_DO_ARQUIVO=valor_do_arquivo\n"
	if err := os.WriteFile(filepath.Join(dir, NomeArquivo), []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("DO_AMBIENTE", "valor_do_ambiente")

	Carregar()

	if got := os.Getenv("DO_AMBIENTE"); got != "valor_do_ambiente" {
		t.Errorf("o arquivo sobrescreveu o ambiente: DO_AMBIENTE = %q", got)
	}
	if got := os.Getenv("SO_DO_ARQUIVO"); got != "valor_do_arquivo" {
		t.Errorf("o arquivo nao foi carregado: SO_DO_ARQUIVO = %q", got)
	}
	os.Unsetenv("SO_DO_ARQUIVO")
}

// Rodar de um subdiretorio de Backend/ tem de achar o mesmo .env.
func TestCarregarSobeDiretorios(t *testing.T) {
	raiz := t.TempDir()
	if err := os.WriteFile(filepath.Join(raiz, NomeArquivo), []byte("ACHADO_SUBINDO=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(raiz, "cmd", "api")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	Carregar()

	if got := os.Getenv("ACHADO_SUBINDO"); got != "1" {
		t.Errorf("nao achou o .env subindo de %s: %q", sub, got)
	}
	os.Unsetenv("ACHADO_SUBINDO")
}

// Producao nao tem .env, e isso nao pode ser erro.
func TestCarregarSemArquivo(t *testing.T) {
	t.Chdir(t.TempDir())
	Carregar() // nao deve entrar em panico
}
