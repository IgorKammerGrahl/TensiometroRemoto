package telemetria

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// CONTRATO DA API DE INGESTAO
//
// POST /api/v1/readings aceita no maximo MaxLeiturasPorLote leituras por
// requisicao. O firmware precisa fatiar o buffer local em multiplos POSTs
// ao respeitar esse teto; lote maior recebe 413 com o limite explicito na
// mensagem, para que o no possa reagir em vez de reenviar em loop.
const (
	MaxLeiturasPorLote = 500
	maxCorpoBytes      = 1 << 20 // 1 MiB

	rawMVMin, rawMVMax = 0, 3300
	kPaMin, kPaMax     = -100, 10

	limiteConsultaPadrao = 100
	limiteConsultaMax    = 1000
	janelaConsultaPadrao = 24 * time.Hour
)

// Janela de plausibilidade de measured_at. O ESP32 nao tem RTC: sem NTP
// ele boota em 1970 e envenenaria a serie temporal. O teto de +2h absorve
// deriva de relogio sem aceitar data futura absurda.
var (
	medidoMin       = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	medidoFuturoMax = 2 * time.Hour
)

// HashToken deriva o hash persistido em devices.token_hash.
//
// HMAC-SHA256 com segredo do servidor, e nao SHA-256 puro: o segredo
// impede que um vazamento do banco permita casar hashes offline contra
// tokens gerados, caso a entropia real seja menor que a nominal. Continua
// deterministico, entao o lookup segue O(1) pelo indice unico -- que e o
// motivo de nao usarmos bcrypt/argon2 aqui: token e 256 bits aleatorios,
// nao senha humana, e um KDF lento obrigaria varrer a tabela por requisicao.
func HashToken(segredo, token string) string {
	m := hmac.New(sha256.New, []byte(segredo))
	m.Write([]byte(token))
	return hex.EncodeToString(m.Sum(nil))
}

type API struct {
	store   *Store
	segredo string
	log     *slog.Logger

	// cookieInseguro omite o atributo Secure do cookie de sessao. Ver
	// PermitirCookieInseguro, em http_app.go.
	cookieInseguro bool

	// Dois limitadores para o login, por chaves diferentes. Ver limite.go.
	loginPorIP    *limitador
	loginPorEmail *limitador
}

// PermitirCookieInseguro desliga o atributo Secure do cookie de sessao.
//
// Existe para um caso so: abrir a PWA num celular apontando para
// http://192.168.x.x durante uma demonstracao. Origem de rede local nao e
// "trustworthy" para o navegador, entao o cookie Secure e descartado em
// silencio e o login parece simplesmente nao funcionar -- sem mensagem que
// aponte a causa.
//
// Em qualquer outro cenario isso e um retrocesso: sem Secure, o cookie
// acompanha uma requisicao HTTP em claro e o token da sessao viaja legivel.
// Por isso e opt-in explicito e anuncia isso no boot, no mesmo espirito do
// DEV_SEM_TLS do firmware.
//
// O aviso vai para o stderr direto, e nao so pelo slog: o log estruturado
// passa por um filtro de nivel, e quem sobe com LevelError nao veria um
// Warn. E a mesma armadilha do `#warning` do firmware, que o -w padrao do
// arduino-cli engolia -- o build inseguro passava calado. Aviso que um
// ajuste de configuracao consegue silenciar nao e aviso.
func (a *API) PermitirCookieInseguro() {
	a.cookieInseguro = true
	const aviso = `
!!! ========================================================================
!!! SESSAO_COOKIE_INSEGURO=true
!!! O cookie de sessao esta SEM o atributo Secure. O token acompanha
!!! requisicoes HTTP em claro e pode ser lido por quem estiver na rede.
!!! Aceitavel apenas em demonstracao local. NUNCA em producao.
!!! ========================================================================
`
	fmt.Fprint(os.Stderr, aviso)
	// E tambem no log estruturado, para quem agrega logs em vez de ler o
	// terminal.
	a.log.Warn("cookie de sessao sem o atributo Secure", "origem", "SESSAO_COOKIE_INSEGURO")
}

func NewAPI(store *Store, segredo string, log *slog.Logger) *API {
	if log == nil {
		log = slog.Default()
	}
	return &API{
		store:         store,
		segredo:       segredo,
		log:           log,
		loginPorIP:    novoLimitador(loginRajadaIP, loginRecargaIP, loginTetoChaves),
		loginPorEmail: novoLimitador(loginRajadaEmail, loginRecargaEmail, loginTetoChaves),
	}
}

func (a *API) Router() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/readings", a.autenticar(a.ingerir))
	mux.Handle("GET /api/v1/devices/{id}/readings", a.autenticar(a.listar))
	mux.Handle("GET /api/v1/devices/{id}/latest", a.autenticar(a.ultima))
	// Plano de usuario, em http_app.go. As rotas /api/v1/app/* nascem
	// embrulhadas em outro middleware; nenhuma delas olha para Authorization.
	a.registrarApp(mux)

	// FUNDO DE POCO DO PREFIXO /api/: rota nao registrada acima morre aqui,
	// em JSON, e nunca no fallback de SPA do pacote webapp.
	//
	// O 404 padrao do ServeMux tambem nao serviria: ele responde text/plain,
	// e o cliente (App/src/api.ts) recusa resposta que nao seja JSON com uma
	// mensagem que fala de content-type, nao de endereco errado. Aqui a
	// resposta tem a forma que o resto da API tem, entao o erro que chega na
	// tela e "nao encontrado" -- que e o que de fato aconteceu.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		erroJSON(w, http.StatusNotFound, "rota nao encontrada")
	})
	return mux
}

// ---------------------------------------------------------------- auth

type ctxChave struct{}

func deviceDoCtx(ctx context.Context) Device {
	d, _ := ctx.Value(ctxChave{}).(Device)
	return d
}

func (a *API) autenticar(prox func(http.ResponseWriter, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cabecalho := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(cabecalho, "Bearer ")
		if !ok || token == "" {
			erroJSON(w, http.StatusUnauthorized, "authorization Bearer ausente")
			return
		}

		dev, err := a.store.DeviceByTokenHash(r.Context(), HashToken(a.segredo, token))
		if errors.Is(err, ErrNaoEncontrado) {
			erroJSON(w, http.StatusUnauthorized, "token invalido")
			return
		}
		if err != nil {
			a.erroInterno(w, "lookup de device", err)
			return
		}
		// 403 e nao 401 de proposito: o token e valido, o device e que foi
		// desativado. O firmware pode parar de tentar em vez de reautenticar.
		if !dev.Ativo {
			erroJSON(w, http.StatusForbidden, "device inativo")
			return
		}

		prox(w, r.WithContext(context.WithValue(r.Context(), ctxChave{}, dev)))
	})
}

// ------------------------------------------------------------- ingestao

type leituraInput struct {
	Seq           *int64     `json:"seq"`
	MeasuredAt    *time.Time `json:"measured_at"`
	RawMV         *int32     `json:"raw_mv"`
	VddMV         *int32     `json:"vdd_mv"`
	KPa           *float32   `json:"kpa"`
	CalibrationID *string    `json:"calibration_id"`
}

type loteInput struct {
	DeviceID string         `json:"device_id"`
	Readings []leituraInput `json:"readings"`
}

type rejeicao struct {
	Index  int    `json:"index"`
	Seq    *int64 `json:"seq"`
	Reason string `json:"reason"`
}

type loteResposta struct {
	Accepted   int        `json:"accepted"`
	Duplicates int        `json:"duplicates"`
	Rejected   int        `json:"rejected"`
	Rejections []rejeicao `json:"rejections"`
	ServerTime time.Time  `json:"server_time"`
}

func (a *API) ingerir(w http.ResponseWriter, r *http.Request) {
	dev := deviceDoCtx(r.Context())
	agora := time.Now().UTC()

	var lote loteInput
	r.Body = http.MaxBytesReader(w, r.Body, maxCorpoBytes)
	if err := json.NewDecoder(r.Body).Decode(&lote); err != nil {
		var grande *http.MaxBytesError
		if errors.As(err, &grande) {
			erroJSON(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("corpo excede %d bytes; envie no maximo %d leituras por lote",
					maxCorpoBytes, MaxLeiturasPorLote))
			return
		}
		erroJSON(w, http.StatusBadRequest, "JSON invalido: "+err.Error())
		return
	}

	if lote.DeviceID == "" {
		erroJSON(w, http.StatusBadRequest, "device_id ausente")
		return
	}
	// Sem isto, o portador de um token escreveria na serie de outro no.
	if lote.DeviceID != dev.ID {
		erroJSON(w, http.StatusForbidden, "device_id nao corresponde ao token")
		return
	}
	if len(lote.Readings) > MaxLeiturasPorLote {
		erroJSON(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("lote com %d leituras excede o limite de %d por requisicao; fatie o buffer",
				len(lote.Readings), MaxLeiturasPorLote))
		return
	}

	validas := make([]Reading, 0, len(lote.Readings))
	indices := make([]int, 0, len(lote.Readings))
	rejeicoes := []rejeicao{}
	for i, in := range lote.Readings {
		leitura, motivo := validar(in, agora)
		if motivo != "" {
			rejeicoes = append(rejeicoes, rejeicao{Index: i, Seq: in.Seq, Reason: motivo})
			continue
		}
		validas = append(validas, leitura)
		indices = append(indices, i)
	}

	// Resolve as FKs antes de inserir: calibration_id desconhecido dentro de
	// um INSERT multi-linha abortaria o lote inteiro.
	validas, rejeicoes, ok := a.filtrarCalibracoes(w, r, dev.ID, validas, indices, rejeicoes)
	if !ok {
		return
	}

	aceitos, err := a.store.InserirLeituras(r.Context(), dev.ID, validas)
	if err != nil {
		a.erroInterno(w, "insert de leituras", err)
		return
	}

	duplicadas := make([]int64, 0, len(validas)-len(aceitos))
	for _, v := range validas {
		if _, ok := aceitos[v.Seq]; !ok {
			duplicadas = append(duplicadas, v.Seq)
		}
	}
	a.alertarResetDeSeq(r.Context(), dev.ID, validas, duplicadas)

	escreverJSON(w, http.StatusOK, loteResposta{
		Accepted:   len(aceitos),
		Duplicates: len(duplicadas),
		Rejected:   len(rejeicoes),
		Rejections: rejeicoes,
		ServerTime: agora,
	})
}

// validar aplica as faixas fisicas leitura a leitura. Devolve motivo nao
// vazio quando a leitura deve ser rejeitada sem derrubar o resto do lote.
func validar(in leituraInput, agora time.Time) (Reading, string) {
	switch {
	case in.Seq == nil:
		return Reading{}, "seq ausente"
	case in.MeasuredAt == nil:
		return Reading{}, "measured_at ausente"
	case in.RawMV == nil:
		return Reading{}, "raw_mv ausente"
	case in.KPa == nil:
		return Reading{}, "kpa ausente"
	// Exigido pela aplicacao ainda que a coluna seja nullable: sem a
	// calibracao referenciada a leitura nao pode ser reprocessada a partir
	// de raw_mv, que e justamente o motivo de guardar raw_mv.
	case in.CalibrationID == nil || *in.CalibrationID == "":
		return Reading{}, "calibration_id ausente"
	case *in.Seq < 0:
		return Reading{}, fmt.Sprintf("seq negativo: %d", *in.Seq)
	case *in.RawMV < rawMVMin || *in.RawMV > rawMVMax:
		return Reading{}, fmt.Sprintf("raw_mv fora da faixa [%d,%d]: %d", rawMVMin, rawMVMax, *in.RawMV)
	case *in.KPa < kPaMin || *in.KPa > kPaMax:
		return Reading{}, fmt.Sprintf("kpa fora da faixa [%d,%d]: %g", kPaMin, kPaMax, *in.KPa)
	}

	medido := in.MeasuredAt.UTC()
	if medido.Before(medidoMin) || medido.After(agora.Add(medidoFuturoMax)) {
		return Reading{}, fmt.Sprintf("measured_at fora da janela plausivel (%s ate agora+%s): %s",
			medidoMin.Format("2006-01-02"), medidoFuturoMax, medido.Format(time.RFC3339))
	}

	return Reading{
		Seq:           *in.Seq,
		MeasuredAt:    medido,
		RawMV:         *in.RawMV,
		VddMV:         in.VddMV,
		KPa:           *in.KPa,
		CalibrationID: in.CalibrationID,
	}, ""
}

func (a *API) filtrarCalibracoes(w http.ResponseWriter, r *http.Request, deviceID string,
	validas []Reading, indices []int, rejeicoes []rejeicao) ([]Reading, []rejeicao, bool) {

	ids := make([]string, 0, len(validas))
	vistos := map[string]struct{}{}
	for _, v := range validas {
		if _, ok := vistos[*v.CalibrationID]; !ok {
			vistos[*v.CalibrationID] = struct{}{}
			ids = append(ids, *v.CalibrationID)
		}
	}

	conhecidas, err := a.store.CalibracoesConhecidas(r.Context(), deviceID, ids)
	if err != nil {
		a.erroInterno(w, "lookup de calibracoes", err)
		return nil, nil, false
	}

	mantidas := make([]Reading, 0, len(validas))
	for i, v := range validas {
		if _, ok := conhecidas[*v.CalibrationID]; ok {
			mantidas = append(mantidas, v)
			continue
		}
		seq := v.Seq // copia: ponteiro para o slice original seria invalidado
		rejeicoes = append(rejeicoes, rejeicao{
			Index:  indices[i],
			Seq:    &seq,
			Reason: "calibration_id desconhecido para este device: " + *v.CalibrationID,
		})
	}
	return mantidas, rejeicoes, true
}

// alertarResetDeSeq distingue reenvio legitimo de contador reiniciado.
//
// Reenviar um lote inteiro apos perder a resposta e o caminho normal, e
// produz 100% de duplicatas -- entao contar duplicatas nao serve de alarme.
// O sintoma real de seq reiniciado (reflash, perda da NVS) e um seq que ja
// existe gravado com OUTRO measured_at: ai a leitura nova esta sendo
// engolida silenciosamente.
//
// ponytail: so loga. A correcao de verdade e boot_id na chave primaria,
// que muda o schema; fazer isso quando/se o alarme disparar em campo.
func (a *API) alertarResetDeSeq(ctx context.Context, deviceID string, validas []Reading, duplicadas []int64) {
	if len(duplicadas) == 0 {
		return
	}
	gravados, err := a.store.MedidosPorSeq(ctx, deviceID, duplicadas)
	if err != nil {
		a.log.WarnContext(ctx, "falha ao checar reset de seq", "device_id", deviceID, "erro", err)
		return
	}
	const tolerancia = time.Minute
	for _, v := range validas {
		gravado, ok := gravados[v.Seq]
		if !ok {
			continue
		}
		if d := gravado.Sub(v.MeasuredAt).Abs(); d > tolerancia {
			a.log.WarnContext(ctx, "possivel reset do contador seq: leitura nova descartada como duplicata",
				"device_id", deviceID, "seq", v.Seq,
				"measured_at_gravado", gravado, "measured_at_recebido", v.MeasuredAt,
				"divergencia", d)
			return // um exemplo por lote basta para diagnosticar
		}
	}
}

// ------------------------------------------------------------- consulta

func (a *API) listar(w http.ResponseWriter, r *http.Request) {
	dev, ok := a.autorizarDevice(w, r)
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

	limite := limiteConsultaPadrao
	if bruto := r.URL.Query().Get("limit"); bruto != "" {
		limite, err = strconv.Atoi(bruto)
		if err != nil || limite <= 0 {
			erroJSON(w, http.StatusBadRequest, "limit deve ser inteiro positivo")
			return
		}
		limite = min(limite, limiteConsultaMax)
	}

	leituras, err := a.store.ListarLeituras(r.Context(), dev, de, ate, limite)
	if err != nil {
		a.erroInterno(w, "consulta de leituras", err)
		return
	}
	escreverJSON(w, http.StatusOK, map[string]any{
		"device_id": dev,
		"from":      de,
		"to":        ate,
		"count":     len(leituras),
		"readings":  leituras,
	})
}

func (a *API) ultima(w http.ResponseWriter, r *http.Request) {
	dev, ok := a.autorizarDevice(w, r)
	if !ok {
		return
	}
	leitura, err := a.store.UltimaLeitura(r.Context(), dev)
	if errors.Is(err, ErrNaoEncontrado) {
		erroJSON(w, http.StatusNotFound, "device sem leituras")
		return
	}
	if err != nil {
		a.erroInterno(w, "consulta da ultima leitura", err)
		return
	}
	escreverJSON(w, http.StatusOK, leitura)
}

// autorizarDevice: o token e do proprio no, entao um device so le a
// propria serie. Um painel com visao de varios talhoes vai precisar de um
// token de leitura separado -- fora do escopo do prototipo.
func (a *API) autorizarDevice(w http.ResponseWriter, r *http.Request) (string, bool) {
	pedido := r.PathValue("id")
	if pedido != deviceDoCtx(r.Context()).ID {
		erroJSON(w, http.StatusForbidden, "token nao autoriza leitura deste device")
		return "", false
	}
	return pedido, true
}

func horaParam(r *http.Request, nome string, padrao time.Time) (time.Time, error) {
	bruto := r.URL.Query().Get(nome)
	if bruto == "" {
		return padrao, nil
	}
	t, err := time.Parse(time.RFC3339, bruto)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s deve ser RFC3339", nome)
	}
	return t.UTC(), nil
}

// ---------------------------------------------------------------- util

func escreverJSON(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}

func erroJSON(w http.ResponseWriter, status int, msg string) {
	escreverJSON(w, status, map[string]string{"error": msg})
}

func (a *API) erroInterno(w http.ResponseWriter, ctx string, err error) {
	a.log.Error("erro interno", "contexto", ctx, "erro", err)
	erroJSON(w, http.StatusInternalServerError, "erro interno")
}
