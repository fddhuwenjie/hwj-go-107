// Package audit 提供审计记录辅助，审计条目随业务事务一同提交或回滚。
package audit

import (
	"context"
	"fmt"

	"gowork/internal/clock"
	"gowork/internal/domain"
	"gowork/internal/repository"
)

// Recorder 审计记录器。
type Recorder struct {
	clk clock.Clock
}

// NewRecorder 构造审计记录器。
func NewRecorder(clk clock.Clock) *Recorder {
	return &Recorder{clk: clk}
}

// Record 在当前仓储（可能处于事务中）追加一条审计。
func (rc *Recorder) Record(ctx context.Context, r *repository.Repositories, actor, action, entityType string, entityID int64, detail string) error {
	entry := &domain.AuditLog{
		Actor:      actor,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Detail:     detail,
		CreatedAt:  rc.clk.Now().Unix(),
	}
	if err := r.Audit.Append(ctx, entry); err != nil {
		return fmt.Errorf("写入审计日志失败: %w", err)
	}
	return nil
}

// SnapshotArtifact 追加藏品状态历史快照。
func (rc *Recorder) SnapshotArtifact(ctx context.Context, r *repository.Repositories, a *domain.Artifact, reason string) error {
	snap := &domain.ArtifactSnapshot{
		ArtifactID:    a.ID,
		Status:        a.Status,
		LevelID:       a.LevelID,
		StorageUnitID: a.StorageUnitID,
		Note:          a.Note,
		Version:       a.Version,
		Reason:        reason,
		CreatedAt:     rc.clk.Now().Unix(),
	}
	if err := r.Snapshots.Append(ctx, snap); err != nil {
		return fmt.Errorf("写入藏品快照失败: %w", err)
	}
	return nil
}
