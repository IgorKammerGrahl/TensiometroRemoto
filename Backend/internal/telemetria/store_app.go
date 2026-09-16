package telemetria

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// AUTORIZACAO DO PLANO DE USUARIO
//
// Toda consulta deste arquivo recebe usuarioID como PRIMEIRO parametro, e
// nenhuma tem variante que o dispense. Isso nao e estilo: e o mecanismo.
//
// A autorizacao mora na clausula JOIN/WHERE da propria consulta, nunca num
// `if` que a precede. Um `if` esquecido num handler novo entrega a serie de
// outro produtor em silencio; um parametro esquecido nao compila. A escolha
// e entre um erro que o compilador pega e um vazamento que ninguem pega.
//
// O caminho e sempre o mesmo:
//
//	device -> talhoes -> usuario_talhoes -> usuario
//
// Consequencia deliberada: um device com talhao_id NULL nao casa nenhum
// JOIN e portanto e invisivel para TODOS os usuarios. E fail-closed -- o no
// de bancada recem-provisionado nao vaza para produtor nenhum enquanto
// alguem nao decidir em que talhao ele esta. Adota-se por fora, com
// `admin associar`.
//
// Zero linhas devolve ErrNaoEncontrado, que o handler traduz para 404 --
// nunca 403. 403 confirmaria que o recurso existe, e varrer ids passaria a
// render informacao. 404 nao distingue "nao existe" de "nao e seu".

// ErrJaExiste distingue colisao de chave (409) de falha de autorizacao
// (404). So e alcancavel depois que a autorizacao ja passou.
var ErrJaExiste = errors.New("ja existe")

const (
	// JanelaSessao e o prazo da sessao, reiniciado a cada requisicao
	// autenticada (renovacao deslizante, D10). Prazo fixo desloga o usuario
	// no meio do uso, sem aviso -- e o lugar onde isso acontece e numa
	// demonstracao.
	JanelaSessao = 30 * 24 * time.Hour

	// CustoBcrypt: 12, conforme o comentario de usuarios.senha_hash. Mora
	// aqui para que cmd/admin e cmd/seed nao divirjam do que a migration
	// documenta.
	CustoBcrypt = 12
)

type Usuario struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Nome  string `json:"nome"`
}

type Talhao struct {
	ID      string  `json:"id"`
	Nome    string  `json:"nome"`
	Cultura *string `json:"cultura"`
	// Limiares brutos viajam junto com a zona ja calculada: o grafico
	// precisa deles para desenhar as linhas de referencia (RF08), e ainda
	// assim o cliente nao re-deriva a regra de sinal. Nulos = faixa nao
	// configurada.
	KPaAlerta   *float32 `json:"kpa_alerta"`
	KPaEstresse *float32 `json:"kpa_estresse"`
}

type UltimaLeitura struct {
	MeasuredAt time.Time `json:"measured_at"`
	KPa        float32   `json:"kpa"`
	Zona       *string   `json:"zona"`
	// MedidoHaS existe para que a interface possa mostrar "sem dados ha 6 h"
	// em vez de um indicador verde alimentado por numero morto (4.5).
	MedidoHaS int64 `json:"medido_ha_s"`
}

type DeviceApp struct {
	ID        string         `json:"id"`
	Descricao string         `json:"descricao"`
	Ativo     bool           `json:"ativo"`
	Talhao    Talhao         `json:"talhao"`
	Ultima    *UltimaLeitura `json:"ultima"` // nil = no ainda sem leitura
}

type PontoSerie struct {
	T      time.Time `json:"t"`
	KPaMed float32   `json:"kpa_med"`
	KPaMin float32   `json:"kpa_min"`
	KPaMax float32   `json:"kpa_max"`
	N      int       `json:"n"`
	Zona   *string   `json:"zona"`
}

// ------------------------------------------------------------------ zona

const (
	ZonaConforto = "conforto"
	ZonaAlerta   = "alerta"
	ZonaEstresse = "estresse"
)

// Zona classifica uma leitura contra os limiares do talhao.
//
// ATENCAO AO SINAL. Tensao de agua no solo e negativa, e mais negativa
// significa mais seco: -60 kPa e mais critico que -20 kPa. E o inverso da
// intuicao "valor maior e pior", e e onde a comparacao e escrita errada na
// primeira tentativa. A comparacao correta e `>=`, que le como "esta acima
// do limiar, ou seja, menos seco que ele":
//
//	kpa >= kpa_alerta    -> conforto
//	kpa >= kpa_estresse  -> alerta
//	senao                -> estresse
//
// Dois limiares e tres zonas derivadas, e nao tres faixas com min/max:
// buraco e sobreposicao tornam-se impossiveis por construcao, nao por
// checagem.
//
// As fronteiras caem na zona MENOS severa: com -30/-60, exatamente -30 e
// conforto e exatamente -60 e alerta. Trocar `>=` por `>` desloca as duas
// fronteiras; trocar por `<=` inverte o significado inteiro e o sistema
// segue respondendo 200. Por isso a regra existe uma unica vez, aqui --
// nao tambem num CASE em SQL, nao tambem em TypeScript no cliente.
// Duplicar a regra duplica a chance de inverter o operador.
//
// Faixa nao configurada devolve nil, e a interface mostra "nao configurado".
// nil NAO e conforto: indicador verde sem limiar definido e pior que
// indicador nenhum, porque o produtor decide irrigacao com base nele.
func Zona(kpa float32, alerta, estresse *float32) *string {
	if alerta == nil || estresse == nil {
		return nil
	}
	z := ZonaEstresse
	switch {
	case kpa >= *alerta:
		z = ZonaConforto
	case kpa >= *estresse:
		z = ZonaAlerta
	}
	return &z
}

// --------------------------------------------------------------- sessao

// CriarSessao grava a sessao ja com o prazo cheio. O token em claro nunca
// chega aqui -- so o HMAC, pelo mesmo motivo do token de dispositivo.
func (s *Store) CriarSessao(ctx context.Context, tokenHash, usuarioID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessoes (token_hash, usuario_id, expira_em)
		VALUES ($1, $2, now() + make_interval(secs => $3))`,
		tokenHash, usuarioID, JanelaSessao.Seconds())
	return err
}

// RenovarSessao resolve os passos 2 e 3 do middleware numa unica instrucao:
// valida o token, renova o prazo e devolve o estado da conta.
//
// Validar e renovar sao o MESMO UPDATE de proposito. Uma ida ao banco,
// atomica, e zero linhas cobre os dois casos de falha -- token inexistente e
// token expirado -- de forma indistinguivel para o cliente. Nao da para
// descobrir que um token ja existiu.
//
// O JOIN com usuarios traz `ativo` no mesmo round-trip, mas quem decide o
// que fazer com ele e o middleware: a renovacao acontece antes da checagem
// de conta ativa, exatamente na ordem da 3.6. Renovar a sessao de uma conta
// desativada e inofensivo -- ela leva 403 em qualquer caminho.
//
// ponytail: grava uma tupla por requisicao autenticada (MVCC escreve mesmo
// quando o valor nao muda). Teto conhecido e aceitavel no prototipo. Se
// virar gargalo, renovar so na metade final da janela com SELECT seguido de
// UPDATE condicional -- duas idas ao banco no caso raro, uma no comum.
func (s *Store) RenovarSessao(ctx context.Context, tokenHash string) (usuarioID string, ativo bool, err error) {
	err = s.pool.QueryRow(ctx, `
		UPDATE sessoes AS s
		   SET expira_em = now() + make_interval(secs => $2)
		  FROM usuarios u
		 WHERE s.token_hash = $1
		   AND s.expira_em  > now()
		   AND u.id         = s.usuario_id
		RETURNING s.usuario_id, u.ativo`,
		tokenHash, JanelaSessao.Seconds()).Scan(&usuarioID, &ativo)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, ErrNaoEncontrado
	}
	return usuarioID, ativo, err
}

// DescartarSessao serve ao logout e a limpeza da linha expirada que o
// middleware acabou de recusar.
func (s *Store) DescartarSessao(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessoes WHERE token_hash = $1`, tokenHash)
	return err
}

// LimparSessoesExpiradas roda de forma oportunista no login. Sem cron: o
// caminho de autenticacao nunca mais toca numa linha expirada, entao elas
// nao atrapalham ninguem -- so acumulam espaco indefinidamente. Uma varrida
// no login, que ja e raro e ja paga 250 ms de bcrypt, resolve.
func (s *Store) LimparSessoesExpiradas(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessoes WHERE expira_em < now()`)
	return err
}

// --------------------------------------------------------------- usuario

// UsuarioPorEmail devolve tambem o hash e o estado da conta porque quem
// chama e o login, que precisa dos tres. lower(email) casa o indice unico:
// o produtor nao deve descobrir que "Joao@" e "joao@" sao contas diferentes.
func (s *Store) UsuarioPorEmail(ctx context.Context, email string) (u Usuario, senhaHash string, ativo bool, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT id, email, nome, senha_hash, ativo
		  FROM usuarios WHERE lower(email) = lower($1)`, email,
	).Scan(&u.ID, &u.Email, &u.Nome, &senhaHash, &ativo)
	if errors.Is(err, pgx.ErrNoRows) {
		return Usuario{}, "", false, ErrNaoEncontrado
	}
	return u, senhaHash, ativo, err
}

func (s *Store) Usuario(ctx context.Context, usuarioID string) (Usuario, error) {
	var u Usuario
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, nome FROM usuarios WHERE id = $1`, usuarioID,
	).Scan(&u.ID, &u.Email, &u.Nome)
	if errors.Is(err, pgx.ErrNoRows) {
		return Usuario{}, ErrNaoEncontrado
	}
	return u, err
}

// --------------------------------------------------------------- talhoes

func (s *Store) TalhoesDoUsuario(ctx context.Context, usuarioID string) ([]Talhao, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.nome, t.cultura, t.kpa_alerta, t.kpa_estresse
		  FROM talhoes t
		  JOIN usuario_talhoes ut ON ut.talhao_id = t.id AND ut.usuario_id = $1
		 ORDER BY t.nome`, usuarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	talhoes := []Talhao{}
	for rows.Next() {
		var t Talhao
		if err := rows.Scan(&t.ID, &t.Nome, &t.Cultura, &t.KPaAlerta, &t.KPaEstresse); err != nil {
			return nil, err
		}
		talhoes = append(talhoes, t)
	}
	return talhoes, rows.Err()
}

type TalhaoPatch struct {
	Cultura     *string
	KPaAlerta   *float32
	KPaEstresse *float32
}

// AtualizarTalhao aplica UC04. Campo ausente no PATCH e campo preservado --
// dai o COALESCE. O EXISTS e a autorizacao: talhao nao concedido produz
// zero linhas, e zero linhas e 404.
//
// O CHECK do banco continua sendo a ultima palavra sobre a coerencia dos
// limiares. A aplicacao valida antes so para poder dizer o motivo ao
// usuario; o banco valida para que nenhum outro caminho de escrita (carga
// manual, seed, CSV) contorne a regra fisica.
func (s *Store) AtualizarTalhao(ctx context.Context, usuarioID, talhaoID string, p TalhaoPatch) (Talhao, error) {
	var t Talhao
	err := s.pool.QueryRow(ctx, `
		UPDATE talhoes AS t
		   SET cultura      = COALESCE($3, t.cultura),
		       kpa_alerta   = COALESCE($4, t.kpa_alerta),
		       kpa_estresse = COALESCE($5, t.kpa_estresse)
		 WHERE t.id = $2
		   AND EXISTS (SELECT 1 FROM usuario_talhoes
		                WHERE usuario_id = $1 AND talhao_id = t.id)
		RETURNING t.id, t.nome, t.cultura, t.kpa_alerta, t.kpa_estresse`,
		usuarioID, talhaoID, p.Cultura, p.KPaAlerta, p.KPaEstresse,
	).Scan(&t.ID, &t.Nome, &t.Cultura, &t.KPaAlerta, &t.KPaEstresse)
	if errors.Is(err, pgx.ErrNoRows) {
		return Talhao{}, ErrNaoEncontrado
	}
	return t, err
}

// --------------------------------------------------------------- devices

// selecaoDeviceApp e a unica projecao de device do plano de usuario.
//
// Os dois JOIN sao a autorizacao inteira. O JOIN com talhoes ja e interno --
// device orfao (talhao_id NULL) morre ali, antes mesmo da concessao ser
// consultada. O JOIN com usuario_talhoes exige a concessao do usuario $1.
//
// $2 apenas RESTRINGE o conjunto ja autorizado a um device; nulo devolve a
// lista inteira. Ele vem do path e nunca alarga o alcance -- essa e a
// propriedade que permite usar a mesma consulta para lista e detalhe sem
// abrir uma porta.
//
// LEFT JOIN LATERAL para a ultima leitura: o no recem-cadastrado ainda nao
// tem serie, e precisa aparecer na lista mesmo assim (com "sem dados"), nao
// sumir dela.
const selecaoDeviceApp = `
	SELECT d.id, d.descricao, d.ativo,
	       t.id, t.nome, t.cultura, t.kpa_alerta, t.kpa_estresse,
	       ult.measured_at, ult.kpa
	  FROM devices d
	  JOIN talhoes t          ON t.id = d.talhao_id
	  JOIN usuario_talhoes ut ON ut.talhao_id = d.talhao_id AND ut.usuario_id = $1
	  LEFT JOIN LATERAL (
	         SELECT r.measured_at, r.kpa
	           FROM readings r
	          WHERE r.device_id = d.id
	          ORDER BY r.measured_at DESC, r.seq DESC
	          LIMIT 1
	       ) ult ON true
	 WHERE $2::text IS NULL OR d.id = $2`

// DevicesDoUsuario devolve a lista de UC02, pior primeiro.
//
// ORDER BY kpa ASC. Nao e engano de digitacao: kPa e negativo e o pior caso
// e o mais negativo, entao "pior primeiro" e crescente. Quem le "pior
// primeiro" escreve DESC e inverte a tela toda sem quebrar nada.
//
// NULLS LAST manda para o fim os nos que ainda nao mediram nada -- ausencia
// de leitura nao e a pior situacao da lista, e apenas ausencia.
func (s *Store) DevicesDoUsuario(ctx context.Context, usuarioID string) ([]DeviceApp, error) {
	rows, err := s.pool.Query(ctx,
		selecaoDeviceApp+` ORDER BY ult.kpa ASC NULLS LAST, d.id`, usuarioID, nil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return coletarDevicesApp(rows)
}

func (s *Store) DeviceDoUsuario(ctx context.Context, usuarioID, deviceID string) (DeviceApp, error) {
	rows, err := s.pool.Query(ctx, selecaoDeviceApp, usuarioID, deviceID)
	if err != nil {
		return DeviceApp{}, err
	}
	defer rows.Close()

	ds, err := coletarDevicesApp(rows)
	if err != nil {
		return DeviceApp{}, err
	}
	if len(ds) == 0 {
		return DeviceApp{}, ErrNaoEncontrado
	}
	return ds[0], nil
}

func coletarDevicesApp(rows pgx.Rows) ([]DeviceApp, error) {
	agora := time.Now()
	devices := []DeviceApp{}
	for rows.Next() {
		var d DeviceApp
		var medido *time.Time
		var kpa *float32
		if err := rows.Scan(&d.ID, &d.Descricao, &d.Ativo,
			&d.Talhao.ID, &d.Talhao.Nome, &d.Talhao.Cultura,
			&d.Talhao.KPaAlerta, &d.Talhao.KPaEstresse,
			&medido, &kpa); err != nil {
			return nil, err
		}
		if medido != nil && kpa != nil {
			// A zona e preenchida aqui, e nao no handler, para que um
			// handler novo nao tenha como esquecer de classificar.
			d.Ultima = &UltimaLeitura{
				MeasuredAt: medido.UTC(),
				KPa:        *kpa,
				Zona:       Zona(*kpa, d.Talhao.KPaAlerta, d.Talhao.KPaEstresse),
				MedidoHaS:  int64(agora.Sub(*medido).Seconds()),
			}
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// SerieDoUsuario agrega a serie em buckets de bucketS segundos (UC03).
//
// A agregacao existe porque `ORDER BY measured_at DESC LIMIT 100` sobre um
// intervalo de meses devolve as 100 leituras mais recentes, nao uma amostra
// do intervalo: a tela plotaria um dia rotulado como seis meses. Grafico
// mentiroso e pior que grafico incompleto.
//
// Duas propriedades que a forma do GROUP BY garante:
//
//   - Lacuna sobrevive a agregacao. Bucket sem leitura simplesmente nao gera
//     linha; o periodo offline continua sendo ausencia de dado, e nao uma
//     reta interpolada entre as bordas.
//   - A zona do bucket sai de min(kpa) -- o mais negativo, o pior caso. Uma
//     hora que tocou estresse nao pode aparecer como conforto porque a media
//     do intervalo ficou confortavel.
//
// O JOIN de autorizacao se repete aqui mesmo com o handler ja tendo passado
// por DeviceDoUsuario. A redundancia e o ponto: a consulta e autorizada por
// si, nao por disciplina de quem a chama.
func (s *Store) SerieDoUsuario(ctx context.Context, usuarioID, deviceID string, de, ate time.Time, bucketS int) ([]PontoSerie, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT to_timestamp(floor(extract(epoch FROM r.measured_at) / $3::float8) * $3::float8) AS t,
		       avg(r.kpa)::real AS kpa_med,
		       min(r.kpa)       AS kpa_min,
		       max(r.kpa)       AS kpa_max,
		       count(*)         AS n
		  FROM readings r
		  JOIN devices d          ON d.id = r.device_id
		  JOIN usuario_talhoes ut ON ut.talhao_id = d.talhao_id AND ut.usuario_id = $1
		 WHERE r.device_id = $2 AND r.measured_at BETWEEN $4 AND $5
		 GROUP BY 1
		 ORDER BY 1`, usuarioID, deviceID, bucketS, de, ate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pontos := []PontoSerie{}
	for rows.Next() {
		var p PontoSerie
		if err := rows.Scan(&p.T, &p.KPaMed, &p.KPaMin, &p.KPaMax, &p.N); err != nil {
			return nil, err
		}
		p.T = p.T.UTC()
		pontos = append(pontos, p)
	}
	return pontos, rows.Err()
}

// CriarDevice cadastra o no (RF10) e devolve ErrNaoEncontrado quando o
// talhao de destino nao e concedido.
//
// INSERT ... SELECT FROM usuario_talhoes, e nao INSERT precedido de um
// SELECT de checagem: a autorizacao e a propria fonte de linhas do INSERT.
// Talhao nao concedido produz zero linhas e nada e inserido -- nao ha
// janela entre checar e escrever.
//
// A ordem tambem importa: como a autorizacao decide se existe linha a
// inserir, ela acontece ANTES de qualquer conflito de chave. Cadastrar um id
// ja existente num talhao alheio devolve 404, nao 409; o 409 so aparece
// depois que a autorizacao passou.
func (s *Store) CriarDevice(ctx context.Context, usuarioID, id, descricao, talhaoID, tokenHash string) error {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO devices (id, descricao, talhao_id, token_hash)
		SELECT $2, $3, $4::uuid, $5
		  FROM usuario_talhoes
		 WHERE usuario_id = $1 AND talhao_id = $4::uuid`,
		usuarioID, id, descricao, talhaoID, tokenHash)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrJaExiste
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

type DevicePatch struct {
	Descricao *string
	TalhaoID  *string
	Ativo     *bool
}

// AtualizarDevice aplica RF10 (editar/desativar) exigindo DUAS concessoes
// quando o PATCH move o no de talhao.
//
//   - ORIGEM: o usuario precisa alcancar o no onde ele esta hoje. Sem isso,
//     qualquer um puxaria um no alheio para dentro do proprio alcance.
//   - DESTINO: exigida so quando $4 vem preenchido. Sem isso, o dono de um
//     no o empurraria para um talhao que nao e seu -- inclusive para fora do
//     alcance de quem o vigiava.
//
// Verificar so uma das duas deixa uma das duas direcoes aberta. Sao dois
// EXISTS porque sao dois riscos distintos, nao redundancia.
//
// A concessao de origem le d.talhao_id, o valor ANTIGO: o WHERE e avaliado
// antes do SET. Device orfao (talhao_id NULL) nao casa o EXISTS de origem,
// entao nao ha PATCH que o adote pela API -- so `admin associar`.
//
// COALESCE faz campo ausente significar campo preservado. Consequencia:
// nao existe caminho na API para zerar talhao_id e tornar um no invisivel
// para todo mundo. Orfanar e operacao de bancada, nao de aplicativo.
func (s *Store) AtualizarDevice(ctx context.Context, usuarioID, deviceID string, p DevicePatch) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE devices AS d
		   SET descricao = COALESCE($3, d.descricao),
		       talhao_id = COALESCE($4::uuid, d.talhao_id),
		       ativo     = COALESCE($5, d.ativo)
		 WHERE d.id = $2
		   AND EXISTS (SELECT 1 FROM usuario_talhoes
		                WHERE usuario_id = $1 AND talhao_id = d.talhao_id)
		   AND ($4::uuid IS NULL
		        OR EXISTS (SELECT 1 FROM usuario_talhoes
		                    WHERE usuario_id = $1 AND talhao_id = $4::uuid))`,
		usuarioID, deviceID, p.Descricao, p.TalhaoID, p.Ativo)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNaoEncontrado
	}
	return nil
}
