package simulador

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Cliente fala com o plano de USUARIO da API (/api/v1/app/*): login,
// cadastro de no e de calibracao, lista de nos. E o mesmo caminho da tela de
// RF10 -- o simulador nao escreve no banco por fora, ao contrario do
// cmd/seed, para que o cadastro de N nos exercite a API que um produtor usa.
type Cliente struct {
	base   string
	http   *http.Client
	cookie string
}

func NovoCliente(base string) *Cliente {
	return &Cliente{
		base: strings.TrimRight(base, "/"),
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// Entrar faz login e guarda o cookie de sessao.
//
// O cookie e lido da resposta e reenviado a mao, sem cookiejar: o servidor
// o emite com Secure, e o jar do Go nao devolve cookie Secure por http:// --
// nem para localhost, que e onde a carga roda. Com jar, todo pedido depois
// do login sairia sem sessao e voltaria 401.
func (c *Cliente) Entrar(ctx context.Context, email, senha string) error {
	corpo, _ := json.Marshal(map[string]string{"email": email, "senha": senha})
	resp, err := c.pedir(ctx, http.MethodPost, "/api/v1/app/login", corpo)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return erroDaResposta("login", resp)
	}
	for _, ck := range resp.Cookies() {
		if ck.Name == "sessao" && ck.Value != "" {
			c.cookie = ck.Value
			return nil
		}
	}
	return fmt.Errorf("login: resposta 204 sem cookie de sessao")
}

// Sair encerra a sessao. Sem isso cada execucao deixaria uma linha em
// `sessoes` viva pela janela inteira de 30 dias.
func (c *Cliente) Sair(ctx context.Context) error {
	if c.cookie == "" {
		return nil
	}
	resp, err := c.pedir(ctx, http.MethodPost, "/api/v1/app/logout", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return erroDaResposta("logout", resp)
	}
	c.cookie = ""
	return nil
}

// NoDaAPI e o recorte de DeviceApp que o simulador le.
type NoDaAPI struct {
	ID       string `json:"id"`
	Simulado bool   `json:"simulado"`
	Ativo    bool   `json:"ativo"`
}

// CriarNo cadastra um no com simulado=true e devolve o token.
//
// Confere a marca NA RESPOSTA, e nao so no pedido: um servidor anterior a
// migration 0004 aceita o corpo, ignora o campo e cadastra um no real. Se
// isso acontecer, o erro sai daqui, antes de qualquer leitura ser enviada --
// o no fica cadastrado sem serie, e e esse o estrago maximo.
func (c *Cliente) CriarNo(ctx context.Context, id, descricao, talhaoID string) (string, error) {
	corpo, _ := json.Marshal(map[string]any{
		"id": id, "descricao": descricao, "talhao_id": talhaoID, "simulado": true,
	})
	resp, err := c.pedir(ctx, http.MethodPost, "/api/v1/app/devices", corpo)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", erroDaResposta("cadastro de "+id, resp)
	}
	var out struct {
		Device NoDaAPI `json:"device"`
		Token  string  `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("cadastro de %s: resposta ilegivel: %w", id, err)
	}
	if !out.Device.Simulado {
		return "", fmt.Errorf("cadastro de %s: o servidor gravou o no SEM a marca de simulado "+
			"(migration 0004 aplicada?). Nada foi enviado para ele", id)
	}
	if out.Token == "" {
		return "", fmt.Errorf("cadastro de %s: resposta sem token", id)
	}
	return out.Token, nil
}

// CriarCalibracao registra a calibracao sintetica do no e devolve o id que
// o servidor sorteou. Sem ela toda leitura volta rejeitada.
func (c *Cliente) CriarCalibracao(ctx context.Context, deviceID string) (string, error) {
	corpo, _ := json.Marshal(map[string]any{
		"v_zero_kpa":    VZeroV,
		"k_v_por_kpa":   KVPorKPa,
		"fator_divisor": FatorDivisor,
		"vdd_ensaio_mv": VddMV,
		"nota":          "calibracao sintetica do cmd/simulador; nao houve ensaio",
	})
	resp, err := c.pedir(ctx, http.MethodPost, "/api/v1/app/devices/"+url.PathEscape(deviceID)+"/calibracoes", corpo)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", erroDaResposta("calibracao de "+deviceID, resp)
	}
	var out struct {
		Calibracao struct {
			ID string `json:"id"`
		} `json:"calibracao"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Calibracao.ID == "" {
		return "", fmt.Errorf("calibracao de %s: resposta sem id", deviceID)
	}
	return out.Calibracao.ID, nil
}

// Nos devolve os nos que o usuario enxerga, por id.
func (c *Cliente) Nos(ctx context.Context) (map[string]NoDaAPI, error) {
	resp, err := c.pedir(ctx, http.MethodGet, "/api/v1/app/devices", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, erroDaResposta("lista de nos", resp)
	}
	var out struct {
		Devices []NoDaAPI `json:"devices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("lista de nos: resposta ilegivel: %w", err)
	}
	porID := make(map[string]NoDaAPI, len(out.Devices))
	for _, d := range out.Devices {
		porID[d.ID] = d
	}
	return porID, nil
}

func (c *Cliente) pedir(ctx context.Context, metodo, caminho string, corpo []byte) (*http.Response, error) {
	var leitor io.Reader
	if corpo != nil {
		leitor = bytes.NewReader(corpo)
	}
	r, err := http.NewRequestWithContext(ctx, metodo, c.base+caminho, leitor)
	if err != nil {
		return nil, err
	}
	if corpo != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if c.cookie != "" {
		r.AddCookie(&http.Cookie{Name: "sessao", Value: c.cookie})
	}
	return c.http.Do(r)
}

func erroDaResposta(oque string, resp *http.Response) error {
	var corpo struct {
		Error string `json:"error"`
	}
	bruto, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if json.Unmarshal(bruto, &corpo) == nil && corpo.Error != "" {
		return fmt.Errorf("%s: %d %s", oque, resp.StatusCode, corpo.Error)
	}
	return fmt.Errorf("%s: %d %s", oque, resp.StatusCode, strings.TrimSpace(string(bruto)))
}

// ------------------------------------------------------- plano do no

// leituraJSON e lote seguem o contrato de POST /api/v1/readings, campo a
// campo como Circuito/sketch/payload.h monta.
type leituraJSON struct {
	Seq           int64   `json:"seq"`
	MeasuredAt    string  `json:"measured_at"`
	RawMV         int32   `json:"raw_mv"`
	VddMV         int32   `json:"vdd_mv"`
	KPa           float32 `json:"kpa"`
	CalibrationID string  `json:"calibration_id"`
}

type lote struct {
	DeviceID string        `json:"device_id"`
	Readings []leituraJSON `json:"readings"`
}

// Envio e o resultado de um POST, do ponto de vista do no.
type Envio struct {
	Status int // 0 quando a requisicao nem chegou a ter resposta
	// Erro e a categoria (ver classificarErro); ErroBruto, a mensagem
	// original, para o relatorio guardar alguns exemplos.
	Erro, ErroBruto string
	Latencia        time.Duration
	// Contagens que o servidor devolveu no corpo do 200.
	Aceitas, Duplicadas, Rejeitadas int
}

// Enviar faz o POST de UMA leitura, como o firmware da etapa 1: uma leitura
// por requisicao, Bearer do proprio no, corpo JSON.
//
// A latencia e medida do inicio do Do ao fim da leitura do corpo, e entao
// inclui abrir a conexao quando o transporte nao reaproveita uma -- que e o
// que o no paga a cada envio.
func Enviar(ctx context.Context, hc *http.Client, base string, no No, seq int64, t time.Time, m Medida) Envio {
	corpo, _ := json.Marshal(lote{
		DeviceID: no.ID,
		Readings: []leituraJSON{{
			Seq: seq,
			// Segundos inteiros em UTC, o formato de formatarInstanteUTC.
			MeasuredAt:    t.UTC().Truncate(time.Second).Format(time.RFC3339),
			RawMV:         m.RawMV,
			VddMV:         VddMV,
			KPa:           m.KPa,
			CalibrationID: no.CalibracaoID,
		}},
	})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/readings", bytes.NewReader(corpo))
	if err != nil {
		return Envio{Erro: err.Error()}
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+no.Token)

	inicio := time.Now()
	resp, err := hc.Do(r)
	if err != nil {
		return Envio{Erro: classificarErro(err), ErroBruto: err.Error(), Latencia: time.Since(inicio)}
	}
	bruto, errLeitura := io.ReadAll(resp.Body)
	resp.Body.Close()
	e := Envio{Status: resp.StatusCode, Latencia: time.Since(inicio)}
	if errLeitura != nil {
		e.Erro, e.ErroBruto = classificarErro(errLeitura), errLeitura.Error()
		return e
	}
	if resp.StatusCode == http.StatusOK {
		var out struct {
			Accepted   int `json:"accepted"`
			Duplicates int `json:"duplicates"`
			Rejected   int `json:"rejected"`
		}
		if err := json.Unmarshal(bruto, &out); err != nil {
			e.Erro = "corpo do 200 ilegivel"
			return e
		}
		e.Aceitas, e.Duplicadas, e.Rejeitadas = out.Accepted, out.Duplicates, out.Rejected
	}
	return e
}

// classificarErro reduz o erro de transporte a uma categoria estavel para
// contagem. A mensagem crua traz porta efemera e endereco, e cada falha
// viraria uma categoria propria no relatorio.
func classificarErro(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "Client.Timeout") || strings.Contains(s, "deadline exceeded"):
		return "timeout"
	case strings.Contains(s, "connection refused"):
		return "conexao recusada"
	case strings.Contains(s, "connection reset"):
		return "conexao reiniciada"
	case strings.Contains(s, "cannot assign requested address"):
		return "sem porta local livre"
	case strings.Contains(s, "too many open files"):
		return "sem descritor de arquivo"
	case strings.Contains(s, "EOF"):
		return "EOF"
	case strings.Contains(s, "context canceled"):
		return "cancelado"
	}
	return "outro"
}
