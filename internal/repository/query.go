package repository

import (
	"context"

	"gowork/internal/domain"
)

// UpcomingAnomalyRow 临近借展仍有环境异常的查询行。
type UpcomingAnomalyRow struct {
	Loan          domain.LoanApplication `json:"loan"`
	ArtifactID    int64                  `json:"artifact_id"`
	ArtifactCode  string                 `json:"artifact_code"`
	ArtifactName  string                 `json:"artifact_name"`
	UnitID        int64                  `json:"storage_unit_id"`
	UnitName      string                 `json:"storage_unit_name"`
	OpenAnomalies int64                  `json:"open_anomalies"`
}

// WarehouseRiskRow 库房风险排序行。
type WarehouseRiskRow struct {
	UnitID         int64  `json:"unit_id"`
	UnitCode       string `json:"unit_code"`
	UnitName       string `json:"unit_name"`
	OpenAnomalies  int64  `json:"open_anomalies"`
	SeverityScore  int64  `json:"severity_score"`
	RecentBreaches int64  `json:"recent_breaches"`
	RiskScore      int64  `json:"risk_score"`
}

// QueryRepository 专题查询仓储。
type QueryRepository interface {
	// UpcomingLoansWithAnomalies 查询 [now, until] 内开始、藏品所在单元仍有未关闭异常的借展。
	UpcomingLoansWithAnomalies(ctx context.Context, now, until int64) ([]UpcomingAnomalyRow, error)
	// WarehouseRiskRanking 库房风险排序，since 为统计越界采样的起始时间。
	WarehouseRiskRanking(ctx context.Context, since int64) ([]WarehouseRiskRow, error)
}
