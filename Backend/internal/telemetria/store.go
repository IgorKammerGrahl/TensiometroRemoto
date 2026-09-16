package telemetria

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNaoEncontrado e devolvido quando device ou leitura nao existem.
var ErrNaoEncontrado = errors.New("nao encontrado")

type Device struct {
	ID        string `json:"id"`
	Descricao string `json:"descricao"`
	Ativo     bool   `json:"ativo"`
}

type Reading struct {
	Seq           int64     `json:"seq"`
	MeasuredAt    time.Time `json:"measured_at"`
	ReceivedAt    time.Time `json:"received_at"`
	RawMV         int32     `json:"raw_mv"`
	VddMV         *int32    `json:"vdd_mv"`
	KPa           float32   `json:"kpa"`
	CalibrationID *string   `json:"calibration_id"`
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// NovoPool abre o pool ja fixado em UTC. TIMESTAMPTZ armazena UTC de
// qualquer forma, mas fixar a sessao garante que o que sai do banco tambem
// chega em UTC, sem depender do fuso do host.
func NovoPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	return pgxpool.NewWithConfig(ctx, cfg)
}

func (s *Store) DeviceByTokenHash(ctx context.Context, hash string) (Device, error) {
	var d Device
	err := s.pool.QueryRow(ctx,
		`SELECT id, descricao, ativo FROM devices WHERE token_hash = $1`, hash,
	).Scan(&d.ID, &d.Descricao, &d.Ativo)
	if errors.Is(err, pgx.ErrNoRows) {
		return Device{}, ErrNaoEncontrado
	}
	return d, err
}

func (s *Store) DeviceExiste(ctx context.Context, id string) (bool, error) {
	var existe bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM devices WHERE id = $1)`, id).Scan(&existe)
	return existe, err
}

// CalibracoesConhecidas devolve, dentre os ids pedidos, os que existem e
// pertencem ao device. Resolver isso antes do INSERT evita que uma unica
// FK invalida derrube o lote inteiro.
func (s *Store) CalibracoesConhecidas(ctx context.Context, deviceID string, ids []string) (map[string]struct{}, error) {
	conhecidas := make(map[string]struct{}, len(ids))
	if len(ids) == 0 {
		return conhecidas, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id FROM calibrations WHERE device_id = $1 AND id = ANY($2)`, deviceID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		conhecidas[id] = struct{}{}
	}
	return conhecidas, rows.Err()
}

// InserirLeituras grava o lote em um unico INSERT e devolve os seq que de
// fato entraram. Os que faltarem colidiram com a chave (device_id, seq) e
// sao duplicatas. Seq repetido dentro do proprio lote tambem cai aqui:
// DO NOTHING resolve conflito especulativo no mesmo comando (ao contrario
// de DO UPDATE, que erraria).
func (s *Store) InserirLeituras(ctx context.Context, deviceID string, rs []Reading) (map[int64]struct{}, error) {
	aceitos := make(map[int64]struct{}, len(rs))
	if len(rs) == 0 {
		return aceitos, nil
	}

	seqs := make([]int64, len(rs))
	medidos := make([]time.Time, len(rs))
	raws := make([]int32, len(rs))
	vdds := make([]*int32, len(rs))
	kpas := make([]float32, len(rs))
	cals := make([]*string, len(rs))
	for i, r := range rs {
		seqs[i], medidos[i], raws[i] = r.Seq, r.MeasuredAt, r.RawMV
		vdds[i], kpas[i], cals[i] = r.VddMV, r.KPa, r.CalibrationID
	}

	rows, err := s.pool.Query(ctx, `
		INSERT INTO readings (device_id, seq, measured_at, raw_mv, vdd_mv, kpa, calibration_id)
		SELECT $1, u.seq, u.measured_at, u.raw_mv, u.vdd_mv, u.kpa, u.calibration_id
		  FROM unnest($2::bigint[], $3::timestamptz[], $4::int[], $5::int[], $6::real[], $7::text[])
		    AS u(seq, measured_at, raw_mv, vdd_mv, kpa, calibration_id)
		ON CONFLICT (device_id, seq) DO NOTHING
		RETURNING seq`,
		deviceID, seqs, medidos, raws, vdds, kpas, cals)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var seq int64
		if err := rows.Scan(&seq); err != nil {
			return nil, err
		}
		aceitos[seq] = struct{}{}
	}
	return aceitos, rows.Err()
}

// MedidosPorSeq devolve o measured_at ja gravado para os seq pedidos.
// Usado so para diferenciar reenvio legitimo de reset de contador.
func (s *Store) MedidosPorSeq(ctx context.Context, deviceID string, seqs []int64) (map[int64]time.Time, error) {
	out := make(map[int64]time.Time, len(seqs))
	if len(seqs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT seq, measured_at FROM readings WHERE device_id = $1 AND seq = ANY($2)`, deviceID, seqs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var seq int64
		var t time.Time
		if err := rows.Scan(&seq, &t); err != nil {
			return nil, err
		}
		out[seq] = t
	}
	return out, rows.Err()
}

func (s *Store) ListarLeituras(ctx context.Context, deviceID string, de, ate time.Time, limite int) ([]Reading, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT seq, measured_at, received_at, raw_mv, vdd_mv, kpa, calibration_id
		  FROM readings
		 WHERE device_id = $1 AND measured_at >= $2 AND measured_at <= $3
		 ORDER BY measured_at DESC, seq DESC
		 LIMIT $4`, deviceID, de, ate, limite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return coletar(rows)
}

func (s *Store) UltimaLeitura(ctx context.Context, deviceID string) (Reading, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT seq, measured_at, received_at, raw_mv, vdd_mv, kpa, calibration_id
		  FROM readings
		 WHERE device_id = $1
		 ORDER BY measured_at DESC, seq DESC
		 LIMIT 1`, deviceID)
	if err != nil {
		return Reading{}, err
	}
	defer rows.Close()
	lidas, err := coletar(rows)
	if err != nil {
		return Reading{}, err
	}
	if len(lidas) == 0 {
		return Reading{}, ErrNaoEncontrado
	}
	return lidas[0], nil
}

func coletar(rows pgx.Rows) ([]Reading, error) {
	leituras := []Reading{}
	for rows.Next() {
		var r Reading
		if err := rows.Scan(&r.Seq, &r.MeasuredAt, &r.ReceivedAt,
			&r.RawMV, &r.VddMV, &r.KPa, &r.CalibrationID); err != nil {
			return nil, err
		}
		r.MeasuredAt = r.MeasuredAt.UTC()
		r.ReceivedAt = r.ReceivedAt.UTC()
		leituras = append(leituras, r)
	}
	return leituras, rows.Err()
}
