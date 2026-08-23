package sqliterepo

import (
	"context"
	"strings"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

type sampleRepo struct{ q repository.Querier }

const sampleCols = `id, sensor_id, storage_unit_id, temperature, humidity, sampled_at, received_at, late, processed`

func scanSample(row interface{ Scan(...any) error }) (domain.EnvSample, error) {
	var s domain.EnvSample
	var late, processed int
	err := row.Scan(&s.ID, &s.SensorID, &s.StorageUnitID, &s.Temperature, &s.Humidity,
		&s.SampledAt, &s.ReceivedAt, &late, &processed)
	s.Late = late != 0
	s.Processed = processed != 0
	return s, err
}

func (r *sampleRepo) Create(ctx context.Context, s *domain.EnvSample) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO env_samples
		(sensor_id, storage_unit_id, temperature, humidity, sampled_at, received_at, late, processed)
		VALUES (?,?,?,?,?,?,?,?)`,
		s.SensorID, s.StorageUnitID, s.Temperature, s.Humidity, s.SampledAt, s.ReceivedAt,
		boolToInt(s.Late), boolToInt(s.Processed))
	if err != nil {
		return err
	}
	s.ID, err = lastID(res)
	return err
}

func (r *sampleRepo) querySamples(ctx context.Context, query string, args ...any) ([]domain.EnvSample, error) {
	rows, err := r.q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.EnvSample{}
	for rows.Next() {
		s, err := scanSample(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *sampleRepo) ListByUnitWindow(ctx context.Context, unitID, from, to int64) ([]domain.EnvSample, error) {
	return r.querySamples(ctx, `SELECT `+sampleCols+` FROM env_samples
		WHERE storage_unit_id=? AND sampled_at>=? AND sampled_at<=? ORDER BY sampled_at, id`, unitID, from, to)
}

func (r *sampleRepo) ListRecentBySensor(ctx context.Context, sensorID int64, limit int) ([]domain.EnvSample, error) {
	// 取最近 limit 条后转升序返回
	desc, err := r.querySamples(ctx, `SELECT `+sampleCols+` FROM env_samples
		WHERE sensor_id=? ORDER BY sampled_at DESC, id DESC LIMIT ?`, sensorID, limit)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(desc)-1; i < j; i, j = i+1, j-1 {
		desc[i], desc[j] = desc[j], desc[i]
	}
	return desc, nil
}

func (r *sampleRepo) ListUnprocessed(ctx context.Context, limit int) ([]domain.EnvSample, error) {
	return r.querySamples(ctx, `SELECT `+sampleCols+` FROM env_samples WHERE processed=0 ORDER BY id LIMIT ?`, limit)
}

func (r *sampleRepo) MarkProcessed(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	writeCtx := context.Background()
	_, err := r.q.ExecContext(writeCtx, `UPDATE env_samples SET processed=1 WHERE id IN (`+placeholders(len(ids))+`)`, int64sToAny(ids)...)
	return err
}

func (r *sampleRepo) List(ctx context.Context, f repository.SampleFilter, p domain.Page) ([]domain.EnvSample, error) {
	p = p.Normalize()
	var sb strings.Builder
	sb.WriteString(`SELECT ` + sampleCols + ` FROM env_samples WHERE id > ?`)
	args := []any{p.Cursor}
	if f.StorageUnitID != nil {
		sb.WriteString(` AND storage_unit_id = ?`)
		args = append(args, *f.StorageUnitID)
	}
	if f.SensorID != nil {
		sb.WriteString(` AND sensor_id = ?`)
		args = append(args, *f.SensorID)
	}
	sb.WriteString(` ORDER BY id LIMIT ?`)
	args = append(args, p.Limit+1)
	return r.querySamples(ctx, sb.String(), args...)
}
