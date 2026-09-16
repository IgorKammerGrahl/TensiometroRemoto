// Package webapp serve a PWA embarcada no proprio binario.
//
// O `dist/` versionado mora aqui dentro porque `//go:embed` NAO ACEITA
// caminho com `..`: o destino do build do Vite precisa morar sob o modulo Go
// ou nao existe forma de embarca-lo. Por isso o `outDir` do Vite aponta para
// ca (ver App/README.md) e nao ha etapa de copia depois.
package webapp

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// all: e nao so `dist` para nao perder arquivo iniciado por `_` ou `.` que o
// Vite venha a emitir -- o embed comum ignora esses em silencio.
//
//go:embed all:dist
var embutido embed.FS

// Servir devolve o handler raiz da aplicacao: /api/ vai para a API, todo o
// resto vai para a PWA.
//
// A SEPARACAO E ESTRUTURAL, NAO UMA REGRA A LEMBRAR.
//
// O modo de falha que este arranjo torna impossivel: endpoint digitado errado
// cair no fallback da SPA e responder 200 com index.html. O cliente entao
// tentaria fazer parse de HTML como JSON e o sintoma seria um erro de sintaxe
// sem relacao aparente com a causa -- endereco errado. Pior: um 200 e, para
// qualquer camada acima, sucesso.
//
// Aqui o prefixo /api/ e roteado para a API ANTES de a PWA existir na conta:
// o ServeMux casa o padrao mais especifico, e "/api/" e mais especifico que
// "/". Uma rota desconhecida sob /api/ nao chega a este pacote -- morre no
// 404 em JSON registrado no proprio Router da API. Nao ha caminho de codigo
// por onde a SPA responda a uma requisicao de API, nem por descuido futuro.
func Servir(api http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/", api)
	mux.Handle("/", pwa())
	return seguranca(mux)
}

// politicaCSP restringe de onde a pagina pode carregar e para onde pode falar.
//
// O build do Vite nao emite script nem estilo embutido (ver dist/index.html:
// um <script type="module" src> e um <link rel="stylesheet">), entao
// `script-src 'self'` nao custa nada hoje e e a diretiva que importa: com ela,
// conteudo injetado na pagina nao vira codigo executado.
//
// `style-src` carrega 'unsafe-inline' de proposito, como valvula: estilo
// injetado em runtime nao executa codigo, e uma tela que quebra por causa de
// CSP e um defeito visivel na banca em troca de risco quase nenhum. Se um dia
// isso incomodar, o caminho e nonce por resposta.
//
// `connect-src 'self'` e o par do fato de a PWA ser servida pelo mesmo binario
// que a API (App/src/api.ts fala com /api/v1/app, caminho relativo). Nenhuma
// origem externa aparece no bundle -- as unicas URLs absolutas la sao o
// namespace SVG e links de mensagem de erro do React.
//
// `frame-ancestors 'none'` e o X-Frame-Options moderno; os dois vao juntos
// porque nem todo navegador em um celular de produtor e recente.
const politicaCSP = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'none'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

// seguranca embrulha TUDO -- PWA e API. Nao so a PWA de proposito: `nosniff`
// tambem vale para as respostas JSON, onde impede que um navegador resolva
// interpretar como HTML um corpo que comeca com "<".
//
// Os cabecalhos sao escritos ANTES de o handler rodar, porque depois do
// primeiro WriteHeader o mapa de cabecalhos ja foi enviado e escrever nele
// nao tem efeito nenhum -- silenciosamente.
func seguranca(prox http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", politicaCSP)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		// same-origin: o Referer nao acompanha a saida para outro site. A URL
		// da PWA carrega id de no no caminho (/no/tensio-01).
		h.Set("Referrer-Policy", "same-origin")
		prox.ServeHTTP(w, r)
	})
}

func pwa() http.Handler {
	raiz, err := fs.Sub(embutido, "dist")
	if err != nil {
		// Impossivel na pratica: `dist` e literal do go:embed acima, entao
		// um erro aqui significa binario corrompido, nao entrada do usuario.
		panic("webapp: dist embarcado inacessivel: " + err.Error())
	}
	indice, err := fs.ReadFile(raiz, "index.html")
	if err != nil {
		// Falha barulhenta e no boot: um binario que serve 404 para a raiz
		// parece "app fora do ar" e manda procurar defeito no lugar errado.
		// A causa quase sempre e a mesma -- compilou sem rodar `npm run
		// build` (ver App/README.md).
		panic("webapp: dist/index.html ausente; rode `npm run build` em App/: " + err.Error())
	}
	arquivos := http.FileServerFS(raiz)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			// Nao ha nada para escrever aqui: escrita e /api/. Um POST que
			// chega neste ponto errou o endereco, e 405 diz isso.
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "metodo nao permitido", http.StatusMethodNotAllowed)
			return
		}

		caminho := strings.TrimPrefix(r.URL.Path, "/")
		if caminho == "" {
			servirIndice(w, r, indice)
			return
		}

		if _, err := fs.Stat(raiz, caminho); err != nil {
			// Nao e arquivo: e rota da aplicacao (/no/tensio-01, /nos/novo).
			// O roteamento e do lado do cliente, entao o servidor devolve o
			// mesmo index.html e o React resolve o resto.
			servirIndice(w, r, indice)
			return
		}

		// A tabela de mime do Go nao conhece .webmanifest e cairia em
		// text/plain, que e como um manifesto deixa de ser lido e a PWA deixa
		// de ser instalavel -- sem erro nenhum, so o botao de instalar que
		// nunca aparece. ServeContent respeita Content-Type ja definido.
		if strings.HasSuffix(caminho, ".webmanifest") {
			w.Header().Set("Content-Type", "application/manifest+json")
		}

		cache(w, caminho)
		arquivos.ServeHTTP(w, r)
	})
}

// cache separa o que tem nome com hash do que nao tem.
//
// `assets/` e a saida do Vite com hash de conteudo no nome: mudar o arquivo
// muda o nome, entao a versao antiga nunca precisa ser revalidada e
// `immutable` e literalmente verdade.
//
// Todo o resto -- index.html, sw.js, manifest, icones -- tem nome fixo. Cachear
// esses e a forma classica de servir interface velha depois de um deploy:
// index.html cacheado continua apontando para o bundle anterior, e um sw.js
// cacheado se perpetua sozinho, porque o proprio service worker desatualizado
// e quem decide o que buscar. `no-cache` nao proibe guardar; obriga
// revalidar antes de usar, que e o que se quer.
func cache(w http.ResponseWriter, caminho string) {
	if strings.HasPrefix(caminho, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}

func servirIndice(w http.ResponseWriter, r *http.Request, indice []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	cache(w, "index.html")
	// http.ServeContent e nao w.Write: cuida de HEAD, de Range e do
	// Content-Length sem que este arquivo precise saber de nenhum dos tres.
	// Sem ModTime porque embed.FS nao tem: data zero suprime o Last-Modified,
	// e o `no-cache` acima ja e o que governa a revalidacao.
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(indice))
}
