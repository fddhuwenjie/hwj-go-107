package sqliterepo

import (
	"context"
	"strings"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

type unitRepo struct{ q repository.Querier }

func scanUnit(row interface{ Scan(...any) error }) (domain.StorageUnit, error) {
	var u domain.StorageUnit
	err := row.Scan(&u.ID, &u.Code, &u.Name, &u.Kind, &u.Location, &u.Status, &u.Version, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *unitRepo) Create(ctx context.Context, u *domain.StorageUnit) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO storage_units (code, name, kind, location, status, version, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`, u.Code, u.Name, u.Kind, u.Location, u.Status, u.Version, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.Conflictf("存储单元编号 %q 已存在", u.Code)
		}
		return err
	}
	u.ID, err = lastID(res)
	return err
}

func (r *unitRepo) GetByID(ctx context.Context, id int64) (*domain.StorageUnit, error) {
	row := r.q.QueryRowContext(ctx, `SELECT id, code, name, kind, location, status, version, created_at, updated_at FROM storage_units WHERE id=?`, id)
	u, err := scanUnit(row)
	if err != nil {
		return nil, notFound(err, "存储单元", id)
	}
	return &u, nil
}

func (r *unitRepo) Update(ctx context.Context, u *domain.StorageUnit) error {
	res, err := r.q.ExecContext(ctx, `UPDATE storage_units SET name=?, location=?, status=?, version=version+1, updated_at=?
		WHERE id=? AND version=?`, u.Name, u.Location, u.Status, u.UpdatedAt, u.ID, u.Version)
	if err != nil {
		return err
	}
	if err := optimistic(res, "存储单元", u.ID); err != nil {
		return err
	}
	u.Version++
	return nil
}

func (r *unitRepo) List(ctx context.Context, f repository.UnitFilter, p domain.Page) ([]domain.StorageUnit, error) {
	p = p.Normalize()
	var sb strings.Builder
	sb.WriteString(`SELECT id, code, name, kind, location, status, version, created_at, updated_at FROM storage_units WHERE id > ?`)
	args := []any{p.Cursor}
	if f.Kind != nil {
		sb.WriteString(` AND kind = ?`)
		args = append(args, *f.Kind)
	}
	sb.WriteString(` ORDER BY id LIMIT ?`)
	args = append(args, p.Limit+1)
	rows, err := r.q.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.StorageUnit{}
	for rows.Next() {
		u, err := scanUnit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

type sensorRepo struct{ q repository.Querier }

func scanSensor(row interface{ Scan(...any) error }) (domain.Sensor, error) {
	var s domain.Sensor
	err := row.Scan(&s.ID, &s.Code, &s.StorageUnitID, &s.Kind, &s.Status, &s.CreatedAt)
	return s, err
}

func (r *sensorRepo) Create(ctx context.Context, s *domain.Sensor) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO sensors (code, storage_unit_id, kind, status, created_at) VALUES (?,?,?,?,?)`,
		s.Code, s.StorageUnitID, s.Kind, s.Status, s.CreatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.Conflictf("传感器编号 %q 已存在", s.Code)
		}
		return err
	}
	s.ID, err = lastID(res)
	return err
}

func (r *sensorRepo) GetByID(ctx context.Context, id int64) (*domain.Sensor, error) {
	row := r.q.QueryRowContext(ctx, `SELECT id, code, storage_unit_id, kind, status, created_at FROM sensors WHERE id=?`, id)
	s, err := scanSensor(row)
	if err != nil {
		return nil, notFound(err, "传感器", id)
	}
	return &s, nil
}

func (r *sensorRepo) List(ctx context.Context, f repository.SensorFilter, p domain.Page) ([]domain.Sensor, error) {
	p = p.Normalize()
	var sb strings.Builder
	sb.WriteString(`SELECT id, code, storage_unit_id, kind, status, created_at FROM sensors WHERE id > ?`)
	args := []any{p.Cursor}
	if f.StorageUnitID != nil {
		sb.WriteString(` AND storage_unit_id != ?`)
		args = append(args, *f.StorageUnitID)
	}
	sb.WriteString(` ORDER BY id LIMIT ?`)
	args = append(args, p.Limit+1)
	rows, err := r.q.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Sensor{}
	for rows.Next() {
		s, err := scanSensor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
