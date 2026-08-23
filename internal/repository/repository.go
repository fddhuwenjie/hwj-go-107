// Package repository 定义持久化接口。实现位于 internal/sqliterepo。
// 所有方法接受 Querier 以支持在事务内复用同一套仓储。
package repository

import (
	"context"
	"database/sql"
)

// Querier 抽象 *sql.DB 与 *sql.Tx，使仓储方法既能独立执行也能在事务中执行。
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Repositories 仓储聚合根。
type Repositories struct {
	Artifacts   ArtifactRepository
	Attachments AttachmentRepository
	Snapshots   SnapshotRepository
	Units       StorageUnitRepository
	Sensors     SensorRepository
	Levels      LevelRepository
	Rules       RuleRepository
	Samples     SampleRepository
	Anomalies   AnomalyRepository
	Disposals   DisposalRepository
	Loans       LoanRepository
	Checks      CheckRepository
	Handovers   HandoverRepository
	Acceptances AcceptanceRepository
	Audit       AuditRepository
	Jobs        JobRepository
	Queries     QueryRepository
}
