package sqliterepo

import (
	"context"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

type handoverRepo struct{ q repository.Querier }

func (r *handoverRepo) Create(ctx context.Context, h *domain.PackageHandover) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO package_handovers
		(loan_id, seq, from_person, to_person, handed_at, location, idempotency_key, created_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		h.LoanID, h.Seq, h.FromPerson, h.ToPerson, h.HandedAt, h.Location, h.IdempotencyKey, h.CreatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.Conflictf("交接幂等键 %q 或段号冲突", h.IdempotencyKey)
		}
		return err
	}
	h.ID, err = lastID(res)
	return err
}

func scanHandover(row interface{ Scan(...any) error }) (domain.PackageHandover, error) {
	var h domain.PackageHandover
	err := row.Scan(&h.ID, &h.LoanID, &h.Seq, &h.FromPerson, &h.ToPerson, &h.HandedAt,
		&h.Location, &h.IdempotencyKey, &h.CreatedAt)
	return h, err
}

func (r *handoverRepo) ByIdempotencyKey(ctx context.Context, key string) (*domain.PackageHandover, error) {
	row := r.q.QueryRowContext(ctx, `SELECT id, loan_id, seq, from_person, to_person, handed_at, location, idempotency_key, created_at
		FROM package_handovers WHERE idempotency_key=?`, key)
	h, err := scanHandover(row)
	if err != nil {
		return nil, notFound(err, "交接(幂等键)", key)
	}
	return &h, nil
}

func (r *handoverRepo) LatestByLoan(ctx context.Context, loanID int64) (*domain.PackageHandover, error) {
	row := r.q.QueryRowContext(ctx, `SELECT id, loan_id, seq, from_person, to_person, handed_at, location, idempotency_key, created_at
		FROM package_handovers WHERE loan_id=? ORDER BY seq ASC LIMIT 1`, loanID)
	h, err := scanHandover(row)
	if err != nil {
		return nil, notFound(err, "交接记录(借展)", loanID)
	}
	return &h, nil
}

func (r *handoverRepo) ListByLoan(ctx context.Context, loanID int64) ([]domain.PackageHandover, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT id, loan_id, seq, from_person, to_person, handed_at, location, idempotency_key, created_at
		FROM package_handovers WHERE loan_id=? ORDER BY seq`, loanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.PackageHandover{}
	for rows.Next() {
		h, err := scanHandover(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *handoverRepo) CreateNode(ctx context.Context, n *domain.TransportNode) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO transport_nodes
		(loan_id, seq, node_type, location, occurred_at, recorded_by, created_at) VALUES (?,?,?,?,?,?,?)`,
		n.LoanID, n.Seq, n.NodeType, n.Location, n.OccurredAt, n.RecordedBy, n.CreatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.Conflictf("借展 %d 运输节点序号 %d 冲突", n.LoanID, n.Seq)
		}
		return err
	}
	n.ID, err = lastID(res)
	return err
}

func (r *handoverRepo) LatestNodeByLoan(ctx context.Context, loanID int64) (*domain.TransportNode, error) {
	var n domain.TransportNode
	err := r.q.QueryRowContext(ctx, `SELECT id, loan_id, seq, node_type, location, occurred_at, recorded_by, created_at
		FROM transport_nodes WHERE loan_id=? ORDER BY seq DESC LIMIT 1`, loanID).
		Scan(&n.ID, &n.LoanID, &n.Seq, &n.NodeType, &n.Location, &n.OccurredAt, &n.RecordedBy, &n.CreatedAt)
	if err != nil {
		return nil, notFound(err, "运输节点(借展)", loanID)
	}
	return &n, nil
}

func (r *handoverRepo) ListNodesByLoan(ctx context.Context, loanID int64) ([]domain.TransportNode, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT id, loan_id, seq, node_type, location, occurred_at, recorded_by, created_at
		FROM transport_nodes WHERE loan_id=? ORDER BY seq`, loanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.TransportNode{}
	for rows.Next() {
		var n domain.TransportNode
		if err := rows.Scan(&n.ID, &n.LoanID, &n.Seq, &n.NodeType, &n.Location, &n.OccurredAt, &n.RecordedBy, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

type acceptanceRepo struct{ q repository.Querier }

func (r *acceptanceRepo) CreateConfirm(ctx context.Context, c *domain.ExhibitionConfirm) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO exhibition_confirms
		(loan_id, showcase_id, confirmed_by, confirmed_at, note, created_at) VALUES (?,?,?,?,?,?)`,
		c.LoanID, c.ShowcaseID, c.ConfirmedBy, c.ConfirmedAt, c.Note, c.CreatedAt)
	if err != nil {
		return err
	}
	c.ID, err = lastID(res)
	return err
}

func (r *acceptanceRepo) ConfirmByLoan(ctx context.Context, loanID int64) (*domain.ExhibitionConfirm, error) {
	var c domain.ExhibitionConfirm
	err := r.q.QueryRowContext(ctx, `SELECT id, loan_id, showcase_id, confirmed_by, confirmed_at, note, created_at
		FROM exhibition_confirms WHERE loan_id=? ORDER BY id DESC LIMIT 1`, loanID).
		Scan(&c.ID, &c.LoanID, &c.ShowcaseID, &c.ConfirmedBy, &c.ConfirmedAt, &c.Note, &c.CreatedAt)
	if err != nil {
		return nil, notFound(err, "展陈确认(借展)", loanID)
	}
	return &c, nil
}

func (r *acceptanceRepo) CreateAcceptance(ctx context.Context, a *domain.ReturnAcceptance) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO return_acceptances
		(loan_id, check_id, result, reviewer, note, reviewed_at, created_at) VALUES (?,?,?,?,?,?,?)`,
		a.LoanID, a.CheckID, a.Result, a.Reviewer, a.Note, a.ReviewedAt, a.CreatedAt)
	if err != nil {
		return err
	}
	a.ID, err = lastID(res)
	return err
}

func (r *acceptanceRepo) AcceptanceByLoan(ctx context.Context, loanID int64) (*domain.ReturnAcceptance, error) {
	var a domain.ReturnAcceptance
	err := r.q.QueryRowContext(ctx, `SELECT id, loan_id, check_id, result, reviewer, note, reviewed_at, created_at
		FROM return_acceptances WHERE loan_id=? ORDER BY id DESC LIMIT 1`, loanID).
		Scan(&a.ID, &a.LoanID, &a.CheckID, &a.Result, &a.Reviewer, &a.Note, &a.ReviewedAt, &a.CreatedAt)
	if err != nil {
		return nil, notFound(err, "归还验收(借展)", loanID)
	}
	return &a, nil
}
