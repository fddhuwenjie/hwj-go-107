package sqliterepo

import (
	"context"

	"gowork/internal/repository"
)

type queryRepo struct{ q repository.Querier }

// UpcomingLoansWithAnomalies 临近开始但藏品库房仍有未关闭异常的借展行。
func (r *queryRepo) UpcomingLoansWithAnomalies(ctx context.Context, now, until int64) ([]repository.UpcomingAnomalyRow, error) {
	rows, err := r.q.QueryContext(ctx, `
SELECT la.id, la.code, la.borrower, la.venue, la.start_at, la.end_at, la.status,
       a.id, a.code, a.name, u.id, u.name, COUNT(ae.id) AS open_cnt
FROM loan_applications la
JOIN loan_items li ON li.loan_id = la.id
JOIN artifacts a ON a.id = li.artifact_id
JOIN storage_units u ON u.id = li.frozen_unit_id
JOIN anomaly_events ae ON ae.storage_unit_id = li.frozen_unit_id AND ae.status != 'closed'
WHERE la.status = 'approved' AND la.start_at >= ? AND la.start_at <= ?
GROUP BY la.id, a.id, u.id
ORDER BY la.id, a.id`, now, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []repository.UpcomingAnomalyRow{}
	for rows.Next() {
		var row repository.UpcomingAnomalyRow
		if err := rows.Scan(&row.Loan.ID, &row.Loan.Code, &row.Loan.Borrower, &row.Loan.Venue,
			&row.Loan.StartAt, &row.Loan.EndAt, &row.Loan.Status,
			&row.ArtifactID, &row.ArtifactCode, &row.ArtifactName,
			&row.UnitID, &row.UnitName, &row.OpenAnomalies); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// WarehouseRiskRanking 库房风险排序：未关闭异常级别加权 + 近期越界采样数。
// 越界判定借助阈值规则，在 SQL 中以最严格启用规则区间近似；精确判定在服务层补充。
func (r *queryRepo) WarehouseRiskRanking(ctx context.Context, since int64) ([]repository.WarehouseRiskRow, error) {
	rows, err := r.q.QueryContext(ctx, `
WITH open_ae AS (
    SELECT storage_unit_id,
           COUNT(*) AS open_cnt,
           SUM(CASE severity WHEN 'critical' THEN 5 WHEN 'major' THEN 3 WHEN 'minor' THEN 1 ELSE 0 END) AS sev_score
    FROM anomaly_events
    WHERE status != 'closed'
    GROUP BY storage_unit_id
),
recent_breach AS (
    SELECT s.storage_unit_id, COUNT(*) AS breach_cnt
    FROM env_samples s
    JOIN artifacts a ON a.storage_unit_id = s.storage_unit_id AND a.retired = 0
    JOIN threshold_rule_versions rv ON rv.level_id = a.level_id AND rv.status = 'active'
    WHERE (s.sampled_at >= ? OR s.received_at >= ?) AND (s.late = 0 OR s.processed = 1)
      AND (s.temperature < rv.temp_min OR s.temperature > rv.temp_max
           OR s.humidity < rv.humidity_min OR s.humidity > rv.humidity_max)
    GROUP BY s.storage_unit_id
)
SELECT u.id, u.code, u.name,
       COALESCE(o.open_cnt, 0), COALESCE(o.sev_score, 0), COALESCE(b.breach_cnt, 0)
FROM storage_units u
LEFT JOIN open_ae o ON o.storage_unit_id = u.id
LEFT JOIN recent_breach b ON b.storage_unit_id = u.id
WHERE u.kind = 'warehouse'
ORDER BY (COALESCE(o.sev_score, 0) + COALESCE(b.breach_cnt, 0)) DESC, u.id`, since, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []repository.WarehouseRiskRow{}
	for rows.Next() {
		var row repository.WarehouseRiskRow
		if err := rows.Scan(&row.UnitID, &row.UnitCode, &row.UnitName,
			&row.OpenAnomalies, &row.SeverityScore, &row.RecentBreaches); err != nil {
			return nil, err
		}
		row.RiskScore = row.SeverityScore + row.RecentBreaches
		out = append(out, row)
	}
	return out, rows.Err()
}
