package repository

import (
	"context"

	"gowork/internal/domain"
)

// AuditFilter 审计过滤。
type AuditFilter struct {
	EntityType *string
	EntityID   *int64
}

// AuditRepository 审计日志仓储。
type AuditRepository interface {
	Append(ctx context.Context, l *domain.AuditLog) error
	List(ctx context.Context, f AuditFilter, p domain.Page) ([]domain.AuditLog, error)
}

// JobRepository 后台作业仓储。
type JobRepository interface {
	Enqueue(ctx context.Context, j *domain.Job) error
	// Due 取到期可执行作业（pending 且 run_at<=now）。
	Due(ctx context.Context, now int64, limit int) ([]domain.Job, error)
	// Claim 领取作业：pending -> running。
	Claim(ctx context.Context, id, now int64) error
	// Complete 标记完成。
	Complete(ctx context.Context, id, now int64) error
	// Retry 失败后重排队：attempts+1，退避到 runAt；达到上限则置 failed。
	Retry(ctx context.Context, id int64, attempts int, runAt int64, lastErr string, failed bool, now int64) error
	// RecoverRunning 启动恢复：running -> pending，返回恢复条数。
	RecoverRunning(ctx context.Context, now int64) (int64, error)
	// HasActiveByKind 是否存在该类型 pending/running 作业（周期作业去重）。
	HasActiveByKind(ctx context.Context, kind string) (bool, error)
	List(ctx context.Context, p domain.Page) ([]domain.Job, error)
}
