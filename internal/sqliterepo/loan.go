package sqliterepo

import (
	"context"
	"database/sql"
	"strings"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

type loanRepo struct{ q repository.Querier }

const loanCols = `id, code, borrower, venue, purpose, start_at, end_at, status, rule_snapshot, approved_by, approved_at, reject_reason, overdue, attention, version, created_at, updated_at`

func scanLoan(row interface{ Scan(...any) error }) (domain.LoanApplication, error) {
	var l domain.LoanApplication
	var approvedAt sql.NullInt64
	var overdue, attention int
	err := row.Scan(&l.ID, &l.Code, &l.Borrower, &l.Venue, &l.Purpose, &l.StartAt, &l.EndAt,
		&l.Status, &l.RuleSnapshot, &l.ApprovedBy, &approvedAt, &l.RejectReason,
		&overdue, &attention, &l.Version, &l.CreatedAt, &l.UpdatedAt)
	if approvedAt.Valid {
		l.ApprovedAt = &approvedAt.Int64
	}
	l.Overdue = overdue != 0
	l.Attention = attention != 0
	return l, err
}

func (r *loanRepo) Create(ctx context.Context, l *domain.LoanApplication) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO loan_applications
		(code, borrower, venue, purpose, start_at, end_at, status, version, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		l.Code, l.Borrower, l.Venue, l.Purpose, l.StartAt, l.EndAt, l.Status, l.Version, l.CreatedAt, l.UpdatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.Conflictf("借展单号 %q 已存在", l.Code)
		}
		return err
	}
	l.ID, err = lastID(res)
	return err
}

func (r *loanRepo) GetByID(ctx context.Context, id int64) (*domain.LoanApplication, error) {
	row := r.q.QueryRowContext(ctx, `SELECT `+loanCols+` FROM loan_applications WHERE id=?`, id)
	l, err := scanLoan(row)
	if err != nil {
		return nil, notFound(err, "借展申请", id)
	}
	return &l, nil
}

func (r *loanRepo) Update(ctx context.Context, l *domain.LoanApplication) error {
	res, err := r.q.ExecContext(ctx, `UPDATE loan_applications SET status=?, rule_snapshot=?, approved_by=?, approved_at=?,
		reject_reason=?, overdue=?, attention=?, version=version+1, updated_at=?
		WHERE id=? AND version=?`,
		l.Status, l.RuleSnapshot, l.ApprovedBy, l.ApprovedAt, l.RejectReason,
		boolToInt(l.Overdue), boolToInt(l.Attention), l.UpdatedAt, l.ID, l.Version)
	if err != nil {
		return err
	}
	if err := optimistic(res, "借展申请", l.ID); err != nil {
		return err
	}
	l.Version++
	return nil
}

func (r *loanRepo) List(ctx context.Context, f repository.LoanFilter, p domain.Page) ([]domain.LoanApplication, error) {
	p = p.Normalize()
	var sb strings.Builder
	sb.WriteString(`SELECT ` + loanCols + ` FROM loan_applications WHERE id > ?`)
	args := []any{p.Cursor}
	if f.Status != nil {
		sb.WriteString(` AND status = ?`)
		args = append(args, *f.Status)
	}
	sb.WriteString(` ORDER BY id LIMIT ?`)
	args = append(args, p.Limit+1)
	return r.queryLoans(ctx, sb.String(), args...)
}

func (r *loanRepo) ListByStatus(ctx context.Context, statuses ...string) ([]domain.LoanApplication, error) {
	if len(statuses) == 0 {
		return []domain.LoanApplication{}, nil
	}
	args := make([]any, len(statuses))
	for i, s := range statuses {
		args[i] = s
	}
	statusClause := `status IN (` + placeholders(len(statuses)) + `)`
	q := `SELECT ` + loanCols + ` FROM loan_applications WHERE ` + statusClause + ` ORDER BY id`
	return r.queryLoans(ctx, q, args...)
}

func (r *loanRepo) queryLoans(ctx context.Context, q string, args ...any) ([]domain.LoanApplication, error) {
	rows, err := r.q.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.LoanApplication{}
	for rows.Next() {
		l, err := scanLoan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *loanRepo) AddItem(ctx context.Context, it *domain.LoanItem) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO loan_items
		(loan_id, artifact_id, frozen_status, frozen_level_id, frozen_unit_id, packaging_snapshot, created_at)
		VALUES (?,?,?,?,?,?,?)`,
		it.LoanID, it.ArtifactID, it.FrozenStatus, it.FrozenLevelID, it.FrozenUnitID, it.PackagingSnapshot, it.CreatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.Conflictf("藏品 %d 已在借展单 %d 中", it.ArtifactID, it.LoanID)
		}
		return err
	}
	it.ID, err = lastID(res)
	return err
}

func (r *loanRepo) ItemsByLoan(ctx context.Context, loanID int64) ([]domain.LoanItem, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT id, loan_id, artifact_id, frozen_status, frozen_level_id, frozen_unit_id, packaging_snapshot, created_at
		FROM loan_items WHERE loan_id=? ORDER BY id`, loanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.LoanItem{}
	for rows.Next() {
		var it domain.LoanItem
		if err := rows.Scan(&it.ID, &it.LoanID, &it.ArtifactID, &it.FrozenStatus, &it.FrozenLevelID,
			&it.FrozenUnitID, &it.PackagingSnapshot, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *loanRepo) FreezeItem(ctx context.Context, it *domain.LoanItem) error {
	_, err := r.q.ExecContext(ctx, `UPDATE loan_items SET frozen_status=?, frozen_level_id=?, frozen_unit_id=?, packaging_snapshot=? WHERE id=?`,
		it.FrozenStatus, it.FrozenLevelID, it.FrozenUnitID, it.PackagingSnapshot, it.ID)
	return err
}

func (r *loanRepo) LatestAcceptanceTimeByUnit(ctx context.Context, unitID int64) (int64, bool, error) {
	var t sql.NullInt64
	err := r.q.QueryRowContext(ctx, `SELECT MAX(ra.reviewed_at) FROM return_acceptances ra
		JOIN loan_items li ON li.loan_id = ra.loan_id
		WHERE li.frozen_unit_id = ?`, unitID).Scan(&t)
	if err != nil {
		return 0, false, err
	}
	return t.Int64, t.Valid, nil
}

type checkRepo struct{ q repository.Querier }

func (r *checkRepo) Create(ctx context.Context, c *domain.InventoryCheck) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO inventory_checks
		(loan_id, direction, idempotency_key, operator, complete, checked_at, created_at)
		VALUES (?,?,?,?,?,?,?)`,
		c.LoanID, c.Direction, c.IdempotencyKey, c.Operator, boolToInt(c.Complete), c.CheckedAt, c.CreatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.Conflictf("幂等键 %q 已使用", c.IdempotencyKey)
		}
		return err
	}
	c.ID, err = lastID(res)
	return err
}

func (r *checkRepo) CreateItem(ctx context.Context, it *domain.InventoryCheckItem) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO inventory_check_items (check_id, artifact_id, attachment_id, present, note)
		VALUES (?,?,?,?,?)`, it.CheckID, it.ArtifactID, it.AttachmentID, boolToInt(it.Present), it.Note)
	if err != nil {
		return err
	}
	it.ID, err = lastID(res)
	return err
}

func (r *checkRepo) ItemsByCheck(ctx context.Context, checkID int64) ([]domain.InventoryCheckItem, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT id, check_id, artifact_id, attachment_id, present, note
		FROM inventory_check_items WHERE check_id=? ORDER BY id`, checkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.InventoryCheckItem{}
	for rows.Next() {
		var it domain.InventoryCheckItem
		var present int
		if err := rows.Scan(&it.ID, &it.CheckID, &it.ArtifactID, &it.AttachmentID, &present, &it.Note); err != nil {
			return nil, err
		}
		it.Present = present != 0
		out = append(out, it)
	}
	return out, rows.Err()
}

func scanCheck(row interface{ Scan(...any) error }) (domain.InventoryCheck, error) {
	var c domain.InventoryCheck
	var complete int
	err := row.Scan(&c.ID, &c.LoanID, &c.Direction, &c.IdempotencyKey, &c.Operator, &complete, &c.CheckedAt, &c.CreatedAt)
	c.Complete = complete != 0
	return c, err
}

func (r *checkRepo) ByLoanAndDirection(ctx context.Context, loanID int64, direction string) (*domain.InventoryCheck, error) {
	row := r.q.QueryRowContext(ctx, `SELECT id, loan_id, direction, idempotency_key, operator, complete, checked_at, created_at
		FROM inventory_checks WHERE loan_id=? AND direction=? ORDER BY id DESC LIMIT 1`, loanID, direction)
	c, err := scanCheck(row)
	if err != nil {
		return nil, notFound(err, "清点单", loanID)
	}
	return &c, nil
}

func (r *checkRepo) ByIdempotencyKey(ctx context.Context, key string) (*domain.InventoryCheck, error) {
	row := r.q.QueryRowContext(ctx, `SELECT id, loan_id, direction, idempotency_key, operator, complete, checked_at, created_at
		FROM inventory_checks WHERE idempotency_key=?`, key)
	c, err := scanCheck(row)
	if err != nil {
		return nil, notFound(err, "清点单(幂等键)", key)
	}
	return &c, nil
}
