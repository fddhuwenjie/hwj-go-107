package sqliterepo

import (
	"context"
	"database/sql"
	"strings"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

type anomalyRepo struct{ q repository.Querier }

const anomalyCols = `id, storage_unit_id, rule_version_id, sample_id, severity, status, breach_count, title, version, opened_at, closed_at`

func scanAnomaly(row interface{ Scan(...any) error }) (domain.AnomalyEvent, error) {
	var e domain.AnomalyEvent
	var closed sql.NullInt64
	err := row.Scan(&e.ID, &e.StorageUnitID, &e.RuleVersionID, &e.SampleID, &e.Severity,
		&e.Status, &e.BreachCount, &e.Title, &e.Version, &e.OpenedAt, &closed)
	if closed.Valid {
		e.ClosedAt = &closed.Int64
	}
	return e, err
}

func (r *anomalyRepo) Create(ctx context.Context, e *domain.AnomalyEvent) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO anomaly_events
		(storage_unit_id, rule_version_id, sample_id, severity, status, breach_count, title, version, opened_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		e.StorageUnitID, e.RuleVersionID, e.SampleID, e.Severity, e.Status, e.BreachCount, e.Title, e.Version, e.OpenedAt)
	if err != nil {
		return err
	}
	e.ID, err = lastID(res)
	return err
}

func (r *anomalyRepo) GetByID(ctx context.Context, id int64) (*domain.AnomalyEvent, error) {
	row := r.q.QueryRowContext(ctx, `SELECT `+anomalyCols+` FROM anomaly_events WHERE id=?`, id)
	e, err := scanAnomaly(row)
	if err != nil {
		return nil, notFound(err, "异常事件", id)
	}
	return &e, nil
}

func (r *anomalyRepo) Update(ctx context.Context, e *domain.AnomalyEvent) error {
	res, err := r.q.ExecContext(ctx, `UPDATE anomaly_events SET severity=?, status=?, breach_count=?, title=?, version=version+1, closed_at=?
		WHERE id=? AND version=?`,
		e.Severity, e.Status, e.BreachCount, e.Title, e.ClosedAt, e.ID, e.Version)
	if err != nil {
		return err
	}
	if err := optimistic(res, "异常事件", e.ID); err != nil {
		return err
	}
	e.Version++
	return nil
}

func (r *anomalyRepo) query(ctx context.Context, q string, args ...any) ([]domain.AnomalyEvent, error) {
	rows, err := r.q.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AnomalyEvent{}
	for rows.Next() {
		e, err := scanAnomaly(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *anomalyRepo) OpenByUnit(ctx context.Context, unitID int64) ([]domain.AnomalyEvent, error) {
	return r.query(ctx, `SELECT `+anomalyCols+` FROM anomaly_events WHERE storage_unit_id=? AND status != 'closed' ORDER BY id`, unitID)
}

func (r *anomalyRepo) ListOpen(ctx context.Context) ([]domain.AnomalyEvent, error) {
	return r.query(ctx, `SELECT `+anomalyCols+` FROM anomaly_events WHERE status != 'closed' ORDER BY id`)
}

func (r *anomalyRepo) List(ctx context.Context, f repository.AnomalyFilter, p domain.Page) ([]domain.AnomalyEvent, error) {
	p = p.Normalize()
	var sb strings.Builder
	sb.WriteString(`SELECT ` + anomalyCols + ` FROM anomaly_events WHERE id > ?`)
	args := []any{p.Cursor}
	if f.Status != nil {
		sb.WriteString(` AND status = ?`)
		args = append(args, *f.Status)
	}
	if f.StorageUnitID != nil {
		sb.WriteString(` AND storage_unit_id = ?`)
		args = append(args, *f.StorageUnitID)
	}
	sb.WriteString(` ORDER BY id LIMIT ?`)
	args = append(args, p.Limit+1)
	return r.query(ctx, sb.String(), args...)
}

type disposalRepo struct{ q repository.Querier }

func scanDisposal(row interface{ Scan(...any) error }) (domain.ProtectionAction, error) {
	var a domain.ProtectionAction
	var reviewedAt sql.NullInt64
	err := row.Scan(&a.ID, &a.EventID, &a.ActionType, &a.Operator, &a.Note, &a.Status,
		&a.ReviewedBy, &reviewedAt, &a.CreatedAt)
	if reviewedAt.Valid {
		a.ReviewedAt = &reviewedAt.Int64
	}
	return a, err
}

func (r *disposalRepo) Create(ctx context.Context, a *domain.ProtectionAction) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO protection_actions
		(event_id, action_type, operator, note, status, created_at) VALUES (?,?,?,?,?,?)`,
		a.EventID, a.ActionType, a.Operator, a.Note, a.Status, a.CreatedAt)
	if err != nil {
		return err
	}
	a.ID, err = lastID(res)
	return err
}

func (r *disposalRepo) GetByID(ctx context.Context, id int64) (*domain.ProtectionAction, error) {
	row := r.q.QueryRowContext(ctx, `SELECT id, event_id, action_type, operator, note, status, reviewed_by, reviewed_at, created_at
		FROM protection_actions WHERE id=?`, id)
	a, err := scanDisposal(row)
	if err != nil {
		return nil, notFound(err, "保护处置", id)
	}
	return &a, nil
}

func (r *disposalRepo) Update(ctx context.Context, a *domain.ProtectionAction) error {
	_, err := r.q.ExecContext(ctx, `UPDATE protection_actions SET status=?, reviewed_by=?, reviewed_at=? WHERE id=?`,
		a.Status, a.ReviewedBy, a.ReviewedAt, a.ID)
	return err
}

func (r *disposalRepo) ListByEvent(ctx context.Context, eventID int64) ([]domain.ProtectionAction, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT id, event_id, action_type, operator, note, status, reviewed_by, reviewed_at, created_at
		FROM protection_actions WHERE event_id=? ORDER BY id`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ProtectionAction{}
	for rows.Next() {
		a, err := scanDisposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
