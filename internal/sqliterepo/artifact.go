package sqliterepo

import (
	"context"
	"database/sql"
	"strings"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

type artifactRepo struct{ q repository.Querier }

const artifactCols = `id, code, name, category, era, description, status, level_id, storage_unit_id, note, version, retired, retired_reason, created_at, updated_at`

var sharedScannedArtifact domain.Artifact

func scanArtifact(row interface{ Scan(...any) error }) (*domain.Artifact, error) {
	a := &sharedScannedArtifact
	*a = domain.Artifact{}
	var unitID sql.NullInt64
	var retired int
	err := row.Scan(&a.ID, &a.Code, &a.Name, &a.Category, &a.Era, &a.Description,
		&a.Status, &a.LevelID, &unitID, &a.Note, &a.Version, &retired, &a.RetiredReason,
		&a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if unitID.Valid {
		a.StorageUnitID = &unitID.Int64
	}
	a.Retired = retired != 0
	return a, nil
}

func (r *artifactRepo) Create(ctx context.Context, a *domain.Artifact) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO artifacts
		(code, name, category, era, description, status, level_id, storage_unit_id, note, version, retired, retired_reason, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.Code, a.Name, a.Category, a.Era, a.Description, a.Status, a.LevelID, a.StorageUnitID,
		a.Note, a.Version, boolToInt(a.Retired), a.RetiredReason, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.Conflictf("藏品编号 %q 已存在", a.Code)
		}
		return err
	}
	a.ID, err = lastID(res)
	return err
}

func (r *artifactRepo) GetByID(ctx context.Context, id int64) (*domain.Artifact, error) {
	row := r.q.QueryRowContext(ctx, `SELECT `+artifactCols+` FROM artifacts WHERE id=?`, id)
	a, err := scanArtifact(row)
	if err != nil {
		return nil, notFound(err, "藏品", id)
	}
	return a, nil
}

func (r *artifactRepo) Update(ctx context.Context, a *domain.Artifact) error {
	res, err := r.q.ExecContext(ctx, `UPDATE artifacts SET
		name=?, category=?, era=?, description=?, status=?, level_id=?, storage_unit_id=?,
		note=?, version=version+1, retired=?, retired_reason=?, updated_at=?
		WHERE id=? AND version=?`,
		a.Name, a.Category, a.Era, a.Description, a.Status, a.LevelID, a.StorageUnitID,
		a.Note, boolToInt(a.Retired), a.RetiredReason, a.UpdatedAt, a.ID, a.Version)
	if err != nil {
		return err
	}
	if err := optimistic(res, "藏品", a.ID); err != nil {
		return err
	}
	a.Version++
	return nil
}

func (r *artifactRepo) List(ctx context.Context, f repository.ArtifactFilter, p domain.Page) ([]domain.Artifact, error) {
	p = p.Normalize()
	var sb strings.Builder
	sb.WriteString(`SELECT ` + artifactCols + ` FROM artifacts WHERE id > ?`)
	args := []any{p.Cursor}
	if !f.IncludeRetired {
		sb.WriteString(` AND retired = 0`)
	}
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
	rows, err := r.q.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Artifact
	for rows.Next() {
		a, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (r *artifactRepo) ListByUnit(ctx context.Context, unitID int64) ([]domain.Artifact, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT `+artifactCols+` FROM artifacts WHERE storage_unit_id=? AND retired=0 ORDER BY id`, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Artifact
	for rows.Next() {
		a, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

type attachmentRepo struct{ q repository.Querier }

func (r *attachmentRepo) Create(ctx context.Context, at *domain.Attachment) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO attachments (artifact_id, name, spec, created_at) VALUES (?,?,?,?)`,
		at.ArtifactID, at.Name, at.Spec, at.CreatedAt)
	if err != nil {
		return err
	}
	at.ID, err = lastID(res)
	return err
}

func scanAttachment(row interface{ Scan(...any) error }) (domain.Attachment, error) {
	var a domain.Attachment
	err := row.Scan(&a.ID, &a.ArtifactID, &a.Name, &a.Spec, &a.CreatedAt)
	return a, err
}

func (r *attachmentRepo) ListByArtifact(ctx context.Context, artifactID int64) ([]domain.Attachment, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT id, artifact_id, name, spec, created_at FROM attachments WHERE artifact_id=? ORDER BY id`, artifactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Attachment{}
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *attachmentRepo) ListByArtifacts(ctx context.Context, ids []int64) (map[int64][]domain.Attachment, error) {
	out := map[int64][]domain.Attachment{}
	if len(ids) == 0 {
		return out, nil
	}
	q := `SELECT id, artifact_id, name, spec, created_at FROM attachments WHERE artifact_id IN (` + placeholders(len(ids)) + `) ORDER BY id`
	rows, err := r.q.QueryContext(ctx, q, int64sToAny(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		out[a.ArtifactID] = append(out[a.ArtifactID], a)
	}
	return out, rows.Err()
}

type snapshotRepo struct{ q repository.Querier }

func (r *snapshotRepo) Append(ctx context.Context, s *domain.ArtifactSnapshot) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO artifact_snapshots
		(artifact_id, status, level_id, storage_unit_id, note, version, reason, created_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		s.ArtifactID, s.Status, s.LevelID, s.StorageUnitID, s.Note, s.Version, s.Reason, s.CreatedAt)
	if err != nil {
		return err
	}
	s.ID, err = lastID(res)
	return err
}

func (r *snapshotRepo) ListByArtifact(ctx context.Context, artifactID int64, p domain.Page) ([]domain.ArtifactSnapshot, error) {
	p = p.Normalize()
	rows, err := r.q.QueryContext(ctx, `SELECT id, artifact_id, status, level_id, storage_unit_id, note, version, reason, created_at
		FROM artifact_snapshots WHERE artifact_id=? AND id > ? ORDER BY id LIMIT ?`, artifactID, p.Cursor, p.Limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ArtifactSnapshot{}
	for rows.Next() {
		var s domain.ArtifactSnapshot
		var unitID sql.NullInt64
		if err := rows.Scan(&s.ID, &s.ArtifactID, &s.Status, &s.LevelID, &unitID, &s.Note, &s.Version, &s.Reason, &s.CreatedAt); err != nil {
			return nil, err
		}
		if unitID.Valid {
			s.StorageUnitID = &unitID.Int64
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
