package sqliterepo

import (
	"context"
	"database/sql"
	"strings"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

type levelRepo struct{ q repository.Querier }

func (r *levelRepo) Create(ctx context.Context, l *domain.PreservationLevel) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO preservation_levels (code, name, description, created_at) VALUES (?,?,?,?)`,
		l.Code, l.Name, l.Description, l.CreatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.Conflictf("保存等级编号 %q 已存在", l.Code)
		}
		return err
	}
	l.ID, err = lastID(res)
	return err
}

func (r *levelRepo) GetByID(ctx context.Context, id int64) (*domain.PreservationLevel, error) {
	var l domain.PreservationLevel
	err := r.q.QueryRowContext(ctx, `SELECT id, code, name, description, created_at FROM preservation_levels WHERE id=?`, id).
		Scan(&l.ID, &l.Code, &l.Name, &l.Description, &l.CreatedAt)
	if err != nil {
		return nil, notFound(err, "保存等级", id)
	}
	return &l, nil
}

func (r *levelRepo) List(ctx context.Context, p domain.Page) ([]domain.PreservationLevel, error) {
	p = p.Normalize()
	rows, err := r.q.QueryContext(ctx, `SELECT id, code, name, description, created_at FROM preservation_levels WHERE id > ? ORDER BY id LIMIT ?`,
		p.Cursor, p.Limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.PreservationLevel{}
	for rows.Next() {
		var l domain.PreservationLevel
		if err := rows.Scan(&l.ID, &l.Code, &l.Name, &l.Description, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

type ruleRepo struct{ q repository.Querier }

func scanRule(row interface{ Scan(...any) error }) (domain.ThresholdRuleVersion, error) {
	var r domain.ThresholdRuleVersion
	var activated sql.NullInt64
	err := row.Scan(&r.ID, &r.LevelID, &r.VersionNo, &r.TempMin, &r.TempMax,
		&r.HumidityMin, &r.HumidityMax, &r.ConsecutiveBreach, &r.Status, &r.CreatedAt, &activated)
	if activated.Valid {
		r.ActivatedAt = &activated.Int64
	}
	return r, err
}

const ruleCols = `id, level_id, version_no, temp_min, temp_max, humidity_min, humidity_max, consecutive_breach, status, created_at, activated_at`

func (r *ruleRepo) Create(ctx context.Context, rv *domain.ThresholdRuleVersion) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO threshold_rule_versions
		(level_id, version_no, temp_min, temp_max, humidity_min, humidity_max, consecutive_breach, status, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		rv.LevelID, rv.VersionNo, rv.TempMin, rv.TempMax, rv.HumidityMin, rv.HumidityMax,
		rv.ConsecutiveBreach, rv.Status, rv.CreatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.Conflictf("等级 %d 规则版本 %d 已存在", rv.LevelID, rv.VersionNo)
		}
		return err
	}
	rv.ID, err = lastID(res)
	return err
}

func (r *ruleRepo) GetByID(ctx context.Context, id int64) (*domain.ThresholdRuleVersion, error) {
	row := r.q.QueryRowContext(ctx, `SELECT `+ruleCols+` FROM threshold_rule_versions WHERE id=?`, id)
	rv, err := scanRule(row)
	if err != nil {
		return nil, notFound(err, "阈值规则版本", id)
	}
	return &rv, nil
}

func (r *ruleRepo) Activate(ctx context.Context, id int64, now int64) error {
	rv, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if rv.Status != domain.RuleDraft {
		return domain.Statef("仅草稿状态的规则可启用，当前为 %s", rv.Status)
	}
	if _, err := r.q.ExecContext(ctx, `UPDATE threshold_rule_versions SET status='retired' WHERE level_id=? AND status='active'`, rv.LevelID); err != nil {
		return err
	}
	if _, err := r.q.ExecContext(ctx, `UPDATE threshold_rule_versions SET status='active', activated_at=? WHERE id=?`, now, id); err != nil {
		return err
	}
	return nil
}

func (r *ruleRepo) ActiveByLevel(ctx context.Context, levelID int64) (*domain.ThresholdRuleVersion, error) {
	row := r.q.QueryRowContext(ctx, `SELECT `+ruleCols+` FROM threshold_rule_versions WHERE level_id=? AND status='active'`, levelID)
	rv, err := scanRule(row)
	if err != nil {
		return nil, notFound(err, "启用规则(等级)", levelID)
	}
	return &rv, nil
}

func (r *ruleRepo) ActiveByLevels(ctx context.Context, levelIDs []int64) ([]domain.ThresholdRuleVersion, error) {
	out := []domain.ThresholdRuleVersion{}
	if len(levelIDs) == 0 {
		return out, nil
	}
	q := `SELECT ` + ruleCols + ` FROM threshold_rule_versions WHERE status='active' AND level_id IN (` + placeholders(len(levelIDs)) + `)`
	rows, err := r.q.QueryContext(ctx, q, int64sToAny(levelIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		rv, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	return out, rows.Err()
}

func (r *ruleRepo) List(ctx context.Context, f repository.RuleFilter, p domain.Page) ([]domain.ThresholdRuleVersion, error) {
	p = p.Normalize()
	var sb strings.Builder
	sb.WriteString(`SELECT ` + ruleCols + ` FROM threshold_rule_versions WHERE id > ?`)
	args := []any{p.Cursor}
	if f.LevelID != nil {
		sb.WriteString(` AND level_id = ?`)
		args = append(args, *f.LevelID)
	}
	if f.Status != nil {
		sb.WriteString(` AND status = ?`)
		args = append(args, *f.Status)
	}
	sb.WriteString(` ORDER BY id LIMIT ?`)
	args = append(args, p.Limit+1)
	rows, err := r.q.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ThresholdRuleVersion{}
	for rows.Next() {
		rv, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	return out, rows.Err()
}
