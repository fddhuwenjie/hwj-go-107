package sqliterepo

import (
	"context"
	"strings"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

// isUnique 判定 SQLite 唯一约束冲突。
func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "constraint failed: UNIQUE")
}

type auditRepo struct{ q repository.Querier }

func (r *auditRepo) Append(ctx context.Context, l *domain.AuditLog) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO audit_logs (actor, action, entity_type, entity_id, detail, created_at)
		VALUES (?,?,?,?,?,?)`, l.Actor, l.Action, l.EntityType, l.EntityID, l.Detail, l.CreatedAt)
	if err != nil {
		return err
	}
	l.ID, err = lastID(res)
	return err
}

func (r *auditRepo) List(ctx context.Context, f repository.AuditFilter, p domain.Page) ([]domain.AuditLog, error) {
	p = p.Normalize()
	var sb strings.Builder
	sb.WriteString(`SELECT id, actor, action, entity_type, entity_id, detail, created_at FROM audit_logs WHERE id > ?`)
	args := []any{p.Cursor}
	if f.EntityType != nil {
		sb.WriteString(` AND entity_type = ?`)
		args = append(args, *f.EntityType)
	}
	if f.EntityID != nil {
		sb.WriteString(` AND entity_id = ?`)
		args = append(args, *f.EntityID)
	}
	sb.WriteString(` ORDER BY id LIMIT ?`)
	args = append(args, p.Limit+1)
	rows, err := r.q.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AuditLog{}
	for rows.Next() {
		var l domain.AuditLog
		if err := rows.Scan(&l.ID, &l.Actor, &l.Action, &l.EntityType, &l.EntityID, &l.Detail, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

type jobRepo struct{ q repository.Querier }

func (r *jobRepo) Enqueue(ctx context.Context, j *domain.Job) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO jobs (kind, payload, status, attempts, max_attempts, run_at, last_error, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		j.Kind, j.Payload, j.Status, j.Attempts, j.MaxAttempts, j.RunAt, j.LastError, j.CreatedAt, j.UpdatedAt)
	if err != nil {
		return err
	}
	j.ID, err = lastID(res)
	return err
}

func scanJob(row interface{ Scan(...any) error }) (domain.Job, error) {
	var j domain.Job
	err := row.Scan(&j.ID, &j.Kind, &j.Payload, &j.Status, &j.Attempts, &j.MaxAttempts,
		&j.RunAt, &j.LastError, &j.CreatedAt, &j.UpdatedAt)
	return j, err
}

const jobCols = `id, kind, payload, status, attempts, max_attempts, run_at, last_error, created_at, updated_at`

func (r *jobRepo) Due(ctx context.Context, now int64, limit int) ([]domain.Job, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT `+jobCols+` FROM jobs
		WHERE status='pending' AND run_at<=? ORDER BY id LIMIT ?`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *jobRepo) Claim(ctx context.Context, id, now int64) error {
	res, err := r.q.ExecContext(ctx, `UPDATE jobs SET status='running', updated_at=? WHERE id=? AND status='pending'`, now, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.Conflictf("作业 %d 已被领取", id)
	}
	return nil
}

func (r *jobRepo) Complete(ctx context.Context, id, now int64) error {
	_, err := r.q.ExecContext(ctx, `UPDATE jobs SET status='done', updated_at=? WHERE id=?`, now, id)
	return err
}

func (r *jobRepo) Retry(ctx context.Context, id int64, attempts int, runAt int64, lastErr string, failed bool, now int64) error {
	status := domain.JobPending
	if failed {
		status = domain.JobFailed
	}
	_, err := r.q.ExecContext(ctx, `UPDATE jobs SET status=?, attempts=?, run_at=?, last_error=?, updated_at=? WHERE id=?`,
		status, attempts, runAt, lastErr, now, id)
	return err
}

func (r *jobRepo) RecoverRunning(ctx context.Context, now int64) (int64, error) {
	res, err := r.q.ExecContext(ctx, `UPDATE jobs SET status='pending', updated_at=? WHERE status='running'`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *jobRepo) HasActiveByKind(ctx context.Context, kind string) (bool, error) {
	var n int
	err := r.q.QueryRowContext(ctx, `SELECT COUNT(1) FROM jobs WHERE kind=? AND status IN ('pending','running')`, kind).Scan(&n)
	return n > 0, err
}

func (r *jobRepo) List(ctx context.Context, p domain.Page) ([]domain.Job, error) {
	p = p.Normalize()
	rows, err := r.q.QueryContext(ctx, `SELECT `+jobCols+` FROM jobs WHERE id > ? ORDER BY id LIMIT ?`, p.Cursor, p.Limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
