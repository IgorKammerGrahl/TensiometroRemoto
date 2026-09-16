package telemetria

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// PLANO DE AUTENTICACAO DE USUARIO
//
// O backend tem dois planos de autenticacao, separados por prefixo de rota:
//
//	dispositivo | Bearer token | POST /api/v1/readings + os dois GET de device
//	usuario     | cookie       | /api/v1/app/*
//
// A separacao e estrutural, e nao uma checagem replicada dentro de cada
// handler. "Token de dispositivo nao abre a tela do produtor" e propriedade
// do roteamento: as rotas /app/* so existem embrulhadas em a.sessao, que
// nao olha para o cabecalho Authorization. Um token de dispositivo enviado a
// /app/* nao apresenta cookie e leva 401; um cookie de sessao enviado a
// POST /readings nao apresenta Bearer e leva 401. Nenhum dos dois casos
// depende de alguem lembrar de checar.
//
// Este arquivo e separado de http.go pelo mesmo motivo.

const (
	nomeCookieSessao = "sessao"
	maxCorpoApp      = 16 << 10 // 16 KiB: os corpos daqui sao formularios

	// Alvo e teto de pontos por serie. O alvo guia a escolha automatica de
	// bucket; o teto impede que um bucket explicito e pequeno sobre um
	// intervalo longo peca meio milhao de linhas.
	pontosAlvoSerie = 1500
	pontosMaxSerie  = 5000
)

// hashDeReferencia e um bcrypt custo 12 de uma senha que nenhuma conta usa.
//
// Existe para que o login gaste o mesmo tempo quando o e-mail nao existe e
// quando existe. Sem isso, a diferenca de latencia (~250 ms de bcrypt contra
// um retorno imediato) responde "esse e-mail tem conta aqui?" a quem
// perguntar em volume -- e o unico oraculo que sobra depois que a mensagem
// de erro ja e a mesma nos dois casos.
const hashDeReferencia = "$2a$12$Ysdlzl9kIf98ZAYZTqDhIu0MJoHwMv6iPJlQjLuC60wjxPQuI.f5y"

// ctxUsuario e um tipo distinto de ctxChave (device) de proposito: nao ha
// como uma requisicao autenticada por token de dispositivo popular a chave
// de usuario, nem o contrario.
type ctxUsuario struct{}

func usuarioDoCtx(ctx context.Context) string {
	id, _ := ctx.Value(ctxUsuario{}).(string)
	return id
}

func (a *API) registrarApp(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/app/login", a.appLogin)
	mux.Handle("POST /api/v1/app/logout", a.sessao(a.appLogout))
	mux.Handle("GET /api/v1/app/me", a.sessao(a.appMe))

	mux.Handle("GET /api/v1/app/devices", a.sessao(a.appDevices))
	mux.Handle("POST /api/v1/app/devices", a.sessao(a.appCriarDevice))
	mux.Handle("GET /api/v1/app/devices/{id}", a.sessao(a.appDevice))
	mux.Handle("PATCH /api/v1/app/devices/{id}", a.sessao(a.appPatchDevice))
	mux.Handle("GET /api/v1/app/devices/{id}/series", a.sessao(a.appSerie))

	mux.Handle("GET /api/v1/app/talhoes", a.sessao(a.appTalhoes))
	mux.Handle("PATCH /api/v1/app/talhoes/{id}", a.sessao(a.appPatchTalhao))
}

// ---------------------------------------------------------- middleware

// sessao autentica o usuario. A ORDEM DAS VERIFICACOES E O CONTRATO:
//
//  1. cookie presente                    -> senao 401
//  2. hash casa uma sessao nao expirada  -> senao 401 (e a linha e removida)
//  3. usuarios.ativo                     -> senao 403
//  4. so entao o handler, com usuario_id no contexto
//
// Nada vindo do cliente influencia os passos 1 a 4. O unico dado de entrada
// e o proprio cookie; path, query e corpo nao sao lidos ate o passo 4. Um
// handler que precise do usuario o tira do contexto -- nunca do corpo.
func (a *API) sessao(prox func(http.ResponseWriter, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1.
		c, err := r.Cookie(nomeCookieSessao)
		if err != nil || c.Value == "" {
			erroJSON(w, http.StatusUnauthorized, "sessao ausente")
			return
		}
		hash := HashToken(a.segredo, c.Value)

		// 2. Valida e renova na mesma instrucao (D10). Zero linhas cobre
		// token inexistente e token expirado de forma indistinguivel.
		usuarioID, ativo, err := a.store.RenovarSessao(r.Context(), hash)
		if errors.Is(err, ErrNaoEncontrado) {
			// A linha expirada sai daqui: este e o unico instante em que
			// sabemos que ela nao serve mais para nada. Nao muda a resposta
			// -- e a mesma para token que nunca existiu.
			if err := a.store.DescartarSessao(r.Context(), hash); err != nil {
				a.log.WarnContext(r.Context(), "falha ao descartar sessao expirada", "erro", err)
			}
			erroJSON(w, http.StatusUnauthorized, "sessao invalida ou expirada")
			return
		}
		if err != nil {
			a.erroInterno(w, "renovacao de sessao", err)
			return
		}

		// 3. 403 e nao 401 pelo mesmo motivo do device inativo: a credencial
		// e valida, a conta e que foi desativada. Reautenticar nao resolve, e
		// o cliente nao deve ficar pedindo senha de novo.
		if !ativo {
			erroJSON(w, http.StatusForbidden, "usuario inativo")
			return
		}

		// 4.
		prox(w, r.WithContext(context.WithValue(r.Context(), ctxUsuario{}, usuarioID)))
	})
}

// cookieSessao monta o cookie da 3.7.
//
//   - HttpOnly: XSS nao consegue ler o token. E o motivo de nao guardar a
//     sessao em localStorage.
//   - SameSite=Lax: requisicao POST cross-site nao carrega o cookie, o que
//     cobre CSRF nas rotas de escrita sem token anti-CSRF separado.
//   - Secure: em producao a PWA e servida pelo proprio binario, mesma
//     origem e HTTPS. Ver a.cookieInseguro para o caso da demonstracao em
//     rede local.
func (a *API) cookieSessao(valor string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     nomeCookieSessao,
		Value:    valor,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   !a.cookieInseguro,
		SameSite: http.SameSiteLaxMode,
	}
}

// --------------------------------------------------------------- sessao

func (a *API) appLogin(w http.ResponseWriter, r *http.Request) {
	// A ORDEM AQUI E A DEFESA, nao uma preferencia de estilo.
	//
	// O limite por IP vem antes de decodificar: e a unica checagem que nao
	// depende de nada que o corpo diga, entao e a unica que pode custar menos
	// que o ataque. Sair daqui e ler zero byte do corpo.
	if espera, ok := a.loginPorIP.permitir(chaveOrigem(r)); !ok {
		recusarPorLimite(w, espera)
		return
	}

	var in struct {
		Email string `json:"email"`
		Senha string `json:"senha"`
	}
	if !decodificar(w, r, &in) {
		return
	}

	// O limite por e-mail so pode vir depois do corpo -- a chave esta nele --
	// mas vem ANTES do lookup e do bcrypt, que sao a parte cara. A chave e
	// normalizada porque "A@b" e "a@b" sao a mesma conta e nao podem render
	// dois baldes.
	//
	// Consome ficha tambem em login bem-sucedido, sem devolucao: o teto
	// conhecido e que 5 logins da mesma conta em ~2,5 min pedem espera. Nao e
	// um padrao humano, e a recusa se cura sozinha.
	if espera, ok := a.loginPorEmail.permitir(strings.ToLower(in.Email)); !ok {
		recusarPorLimite(w, espera)
		return
	}

	u, hash, ativo, err := a.store.UsuarioPorEmail(r.Context(), in.Email)
	naoExiste := errors.Is(err, ErrNaoEncontrado)
	if err != nil && !naoExiste {
		a.erroInterno(w, "lookup de usuario", err)
		return
	}
	if naoExiste {
		hash = hashDeReferencia // gasta o mesmo tempo; ver a constante
	}
	// bcrypt roda nos dois casos, e so depois o resultado e combinado.
	senhaOK := bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Senha)) == nil
	if naoExiste || !senhaOK {
		// Mensagem unica: "senha errada" e "conta inexistente" nao se
		// distinguem daqui.
		erroJSON(w, http.StatusUnauthorized, "e-mail ou senha invalidos")
		return
	}
	if !ativo {
		erroJSON(w, http.StatusForbidden, "usuario inativo")
		return
	}

	token, err := tokenAleatorio()
	if err != nil {
		a.erroInterno(w, "geracao do token de sessao", err)
		return
	}
	if err := a.store.CriarSessao(r.Context(), HashToken(a.segredo, token), u.ID); err != nil {
		a.erroInterno(w, "criacao de sessao", err)
		return
	}
	// Limpeza oportunista; falhar aqui nao invalida um login bem-sucedido.
	if err := a.store.LimparSessoesExpiradas(r.Context()); err != nil {
		a.log.WarnContext(r.Context(), "falha ao limpar sessoes expiradas", "erro", err)
	}

	http.SetCookie(w, a.cookieSessao(token, int(JanelaSessao.Seconds())))
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) appLogout(w http.ResponseWriter, r *http.Request) {
	// O cookie existe: o middleware nao teria chegado aqui sem ele.
	c, _ := r.Cookie(nomeCookieSessao)
	if err := a.store.DescartarSessao(r.Context(), HashToken(a.segredo, c.Value)); err != nil {
		a.erroInterno(w, "logout", err)
		return
	}
	// MaxAge negativo apaga o cookie no navegador. Zero apenas o tornaria de
	// sessao, o que deixaria o token vivo ate fechar a aba.
	http.SetCookie(w, a.cookieSessao("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) appMe(w http.ResponseWriter, r *http.Request) {
	usuarioID := usuarioDoCtx(r.Context())

	u, err := a.store.Usuario(r.Context(), usuarioID)
	if err != nil {
		a.erroInterno(w, "consulta do usuario", err)
		return
	}
	talhoes, err := a.store.TalhoesDoUsuario(r.Context(), usuarioID)
	if err != nil {
		a.erroInterno(w, "consulta de talhoes", err)
		return
	}
	escreverJSON(w, http.StatusOK, map[string]any{"usuario": u, "talhoes": talhoes})
}

// --------------------------------------------------------------- devices

func (a *API) appDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := a.store.DevicesDoUsuario(r.Context(), usuarioDoCtx(r.Context()))
	if err != nil {
		a.erroInterno(w, "lista de devices", err)
		return
	}
	escreverJSON(w, http.StatusOK, map[string]any{"count": len(devices), "devices": devices})
}

func (a *API) appDevice(w http.ResponseWriter, r *http.Request) {
	d, ok := a.buscarDevice(w, r)
	if !ok {
		return
	}
	escreverJSON(w, http.StatusOK, d)
}

// buscarDevice concentra a traducao "fora do alcance -> 404". Nunca 403:
// 403 confirmaria que o device existe, e um usuario poderia mapear os nos
// dos vizinhos varrendo ids.
func (a *API) buscarDevice(w http.ResponseWriter, r *http.Request) (DeviceApp, bool) {
	d, err := a.store.DeviceDoUsuario(r.Context(), usuarioDoCtx(r.Context()), r.PathValue("id"))
	if errors.Is(err, ErrNaoEncontrado) {
		erroJSON(w, http.StatusNotFound, "device nao encontrado")
		return DeviceApp{}, false
	}
	if err != nil {
		a.erroInterno(w, "consulta de device", err)
		return DeviceApp{}, false
	}
	return d, true
}

func (a *API) appCriarDevice(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID        string `json:"id"`
		Descricao string `json:"descricao"`
		TalhaoID  string `json:"talhao_id"`
	}
	if !decodificar(w, r, &in) {
		return
	}
	switch {
	case in.ID == "" || len(in.ID) > 64:
		erroJSON(w, http.StatusBadRequest, "id ausente ou acima de 64 caracteres")
		return
	case in.Descricao == "":
		erroJSON(w, http.StatusBadRequest, "descricao ausente")
		return
	case !uuidValido(in.TalhaoID):
		erroJSON(w, http.StatusBadRequest, "talhao_id ausente ou malformado")
		return
	}

	token, err := tokenAleatorio()
	if err != nil {
		a.erroInterno(w, "geracao do token do device", err)
		return
	}

	usuarioID := usuarioDoCtx(r.Context())
	err = a.store.CriarDevice(r.Context(), usuarioID, in.ID, in.Descricao, in.TalhaoID,
		HashToken(a.segredo, token))
	switch {
	case errors.Is(err, ErrNaoEncontrado):
		// Talhao inexistente e talhao nao concedido sao a mesma resposta.
		erroJSON(w, http.StatusNotFound, "talhao nao encontrado")
		return
	case errors.Is(err, ErrJaExiste):
		// So chega aqui depois que a autorizacao passou. O id do device e um
		// namespace global escolhido por quem opera, entao a colisao precisa
		// ser dizivel -- 500 aqui viraria "o cadastro nao funciona".
		erroJSON(w, http.StatusConflict, "ja existe device com esse id")
		return
	case err != nil:
		a.erroInterno(w, "cadastro de device", err)
		return
	}

	d, err := a.store.DeviceDoUsuario(r.Context(), usuarioID, in.ID)
	if err != nil {
		a.erroInterno(w, "releitura do device cadastrado", err)
		return
	}
	// O token em claro aparece uma unica vez, aqui, como em cmd/devtoken.
	// devices.token_hash e irreversivel por construcao: nao existe endpoint
	// que recupere um token existente. Perdeu, rotaciona.
	escreverJSON(w, http.StatusCreated, map[string]any{
		"device": d,
		"token":  token,
		"aviso":  "grave o token no firmware agora; ele nao e recuperavel depois",
	})
}

func (a *API) appPatchDevice(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Descricao *string `json:"descricao"`
		TalhaoID  *string `json:"talhao_id"`
		Ativo     *bool   `json:"ativo"`
	}
	if !decodificar(w, r, &in) {
		return
	}
	if in.Descricao != nil && *in.Descricao == "" {
		erroJSON(w, http.StatusBadRequest, "descricao vazia")
		return
	}
	if in.TalhaoID != nil && !uuidValido(*in.TalhaoID) {
		erroJSON(w, http.StatusBadRequest, "talhao_id malformado")
		return
	}

	usuarioID := usuarioDoCtx(r.Context())
	deviceID := r.PathValue("id")
	err := a.store.AtualizarDevice(r.Context(), usuarioID, deviceID,
		DevicePatch{Descricao: in.Descricao, TalhaoID: in.TalhaoID, Ativo: in.Ativo})
	if errors.Is(err, ErrNaoEncontrado) {
		// Uma resposta para tres situacoes distintas, de proposito: device
		// inexistente, sem concessao na origem, sem concessao no destino. O
		// cliente nao aprende qual delas era.
		erroJSON(w, http.StatusNotFound, "device nao encontrado")
		return
	}
	if err != nil {
		a.erroInterno(w, "atualizacao de device", err)
		return
	}

	d, err := a.store.DeviceDoUsuario(r.Context(), usuarioID, deviceID)
	if err != nil {
		a.erroInterno(w, "releitura do device atualizado", err)
		return
	}
	escreverJSON(w, http.StatusOK, d)
}

// ----------------------------------------------------------------- serie

func (a *API) appSerie(w http.ResponseWriter, r *http.Request) {
	// A autorizacao acontece antes de qualquer parametro de query ser lido:
	// device fora do alcance devolve 404 sem que from/to/bucket sequer
	// tenham sido examinados. Nenhuma mensagem de 400 sobre parametro pode
	// entao revelar que o device existe.
	d, ok := a.buscarDevice(w, r)
	if !ok {
		return
	}

	agora := time.Now().UTC()
	ate, err := horaParam(r, "to", agora)
	if err != nil {
		erroJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	de, err := horaParam(r, "from", ate.Add(-janelaConsultaPadrao))
	if err != nil {
		erroJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	if de.After(ate) {
		erroJSON(w, http.StatusBadRequest, "from posterior a to")
		return
	}

	janela := ate.Sub(de)
	bucket := bucketAutomatico(janela)
	if bruto := r.URL.Query().Get("bucket"); bruto != "" {
		bucket, err = strconv.Atoi(bruto)
		if err != nil || bucket <= 0 {
			erroJSON(w, http.StatusBadRequest, "bucket deve ser inteiro positivo de segundos")
			return
		}
	}
	if int(janela.Seconds())/bucket > pontosMaxSerie {
		erroJSON(w, http.StatusBadRequest,
			fmt.Sprintf("bucket=%ds gera mais de %d pontos nesse intervalo; aumente o bucket ou reduza a janela",
				bucket, pontosMaxSerie))
		return
	}

	pontos, err := a.store.SerieDoUsuario(r.Context(), usuarioDoCtx(r.Context()), d.ID, de, ate, bucket)
	if err != nil {
		a.erroInterno(w, "serie agregada", err)
		return
	}
	// Zona do bucket a partir de kpa_min: o pior caso do intervalo. Uma hora
	// que tocou estresse nao pode aparecer como conforto por causa da media.
	for i := range pontos {
		pontos[i].Zona = Zona(pontos[i].KPaMin, d.Talhao.KPaAlerta, d.Talhao.KPaEstresse)
	}

	escreverJSON(w, http.StatusOK, map[string]any{
		"device_id": d.ID,
		"from":      de,
		"to":        ate,
		// bucket_s volta para que a interface detecte lacuna por
		// dt > 1.5 * bucket_s -- uma regra, um numero, vindo do servidor.
		// Sem isso o cliente teria de adivinhar o periodo de amostragem.
		"bucket_s": bucket,
		// Limiares brutos para as linhas de referencia do grafico (RF08).
		"talhao": d.Talhao,
		"count":  len(pontos),
		"pontos": pontos,
	})
}

// bucketsRedondos: valores escolhidos para que os rotulos do eixo caiam em
// horarios redondos (1 min, 5 min, 15 min, 1 h, 3 h, 6 h, 1 dia). Um bucket
// de, digamos, 437 s produziria marcas em horarios arbitrarios.
var bucketsRedondos = []int{60, 300, 900, 3600, 10800, 21600, 86400}

// bucketAutomatico escolhe o menor bucket redondo que mantem a contagem de
// pontos dentro do alvo. Sem isso, "ultimas 24 h" e "ultimos 6 meses"
// usariam o mesmo bucket e um dos dois graficos ficaria inutil.
func bucketAutomatico(janela time.Duration) int {
	for _, b := range bucketsRedondos {
		if int(janela.Seconds())/b <= pontosAlvoSerie {
			return b
		}
	}
	return bucketsRedondos[len(bucketsRedondos)-1]
}

// --------------------------------------------------------------- talhoes

func (a *API) appTalhoes(w http.ResponseWriter, r *http.Request) {
	talhoes, err := a.store.TalhoesDoUsuario(r.Context(), usuarioDoCtx(r.Context()))
	if err != nil {
		a.erroInterno(w, "lista de talhoes", err)
		return
	}
	escreverJSON(w, http.StatusOK, map[string]any{"count": len(talhoes), "talhoes": talhoes})
}

func (a *API) appPatchTalhao(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Cultura     *string  `json:"cultura"`
		KPaAlerta   *float32 `json:"kpa_alerta"`
		KPaEstresse *float32 `json:"kpa_estresse"`
	}
	if !decodificar(w, r, &in) {
		return
	}
	if motivo, ok := validarFaixa(in.KPaAlerta, in.KPaEstresse); !ok {
		erroJSON(w, http.StatusBadRequest, motivo)
		return
	}

	// Id malformado e id inexistente sao a mesma resposta: 404. Nao ha o que
	// ganhar em dizer ao cliente que o formato estava errado.
	talhaoID := r.PathValue("id")
	if !uuidValido(talhaoID) {
		erroJSON(w, http.StatusNotFound, "talhao nao encontrado")
		return
	}

	t, err := a.store.AtualizarTalhao(r.Context(), usuarioDoCtx(r.Context()), talhaoID,
		TalhaoPatch{Cultura: in.Cultura, KPaAlerta: in.KPaAlerta, KPaEstresse: in.KPaEstresse})
	if errors.Is(err, ErrNaoEncontrado) {
		erroJSON(w, http.StatusNotFound, "talhao nao encontrado")
		return
	}
	if err != nil {
		a.erroInterno(w, "atualizacao de talhao", err)
		return
	}
	escreverJSON(w, http.StatusOK, t)
}

// validarFaixa repete em Go o CHECK talhoes_faixas_ck.
//
// A duplicacao e a mesma ja justificada em 0001: a aplicacao valida para
// devolver o MOTIVO ao usuario; o banco valida para que nenhum outro caminho
// de escrita contorne a regra fisica. O banco continua sendo a ultima
// palavra -- se estas condicoes divergirem do CHECK, o UPDATE falha.
//
// Os dois limiares andam juntos: mandar so um sobre um talhao ainda nao
// configurado deixaria o outro nulo e violaria o CHECK. Exigir o par aqui
// transforma isso em 400 com motivo, em vez de 500 vindo do driver.
func validarFaixa(alerta, estresse *float32) (string, bool) {
	if alerta == nil && estresse == nil {
		return "", true
	}
	if alerta == nil || estresse == nil {
		return "kpa_alerta e kpa_estresse precisam vir juntos", false
	}
	// kpa_estresse e o MAIS NEGATIVO: e o limiar mais severo, e mais seco e
	// mais negativo. "estresse < alerta" e o que parece invertido e nao esta.
	if *estresse >= *alerta {
		return "kpa_estresse precisa ser menor (mais negativo) que kpa_alerta", false
	}
	// -80 kPa e o limite de cavitacao do tensiometro: limiar alem disso e
	// limiar que nunca dispara com leitura valida.
	for _, v := range []float32{*alerta, *estresse} {
		if v < -80 || v > 0 {
			return "limiares precisam estar entre -80 e 0 kPa", false
		}
	}
	return "", true
}

// ------------------------------------------------------------------ util

func decodificar(w http.ResponseWriter, r *http.Request, destino any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxCorpoApp)
	if err := json.NewDecoder(r.Body).Decode(destino); err != nil {
		erroJSON(w, http.StatusBadRequest, "JSON invalido: "+err.Error())
		return false
	}
	return true
}

func tokenAleatorio() (string, error) {
	bruto := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bruto); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bruto), nil
}

// uuidValido checa apenas o formato, e existe para transformar "uuid
// malformado" em 400/404 com motivo em vez de um erro de codificacao do
// driver virando 500. Quem decide a validade de verdade e o Postgres.
func uuidValido(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		hex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !hex {
			return false
		}
	}
	return true
}
