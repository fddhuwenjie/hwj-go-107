package repository

import (
	"context"

	"gowork/internal/domain"
)

// ArtifactFilter 藏品分页过滤。
type ArtifactFilter struct {
	Status         *string
	StorageUnitID  *int64
	IncludeRetired bool
}

// ArtifactRepository 藏品仓储。
type ArtifactRepository interface {
	Create(ctx context.Context, a *domain.Artifact) error
	GetByID(ctx context.Context, id int64) (*domain.Artifact, error)
	// Update 乐观锁更新：WHERE id=? AND version=?，成功后 version+1。
	Update(ctx context.Context, a *domain.Artifact) error
	List(ctx context.Context, f ArtifactFilter, p domain.Page) ([]domain.Artifact, error)
	// ListByUnit 查询某单元内全部未注销藏品。
	ListByUnit(ctx context.Context, unitID int64) ([]domain.Artifact, error)
}

// AttachmentRepository 附件仓储。
type AttachmentRepository interface {
	Create(ctx context.Context, at *domain.Attachment) error
	ListByArtifact(ctx context.Context, artifactID int64) ([]domain.Attachment, error)
	// ListByArtifacts 批量查询，返回 artifact_id -> 附件列表。
	ListByArtifacts(ctx context.Context, artifactIDs []int64) (map[int64][]domain.Attachment, error)
}

// SnapshotRepository 藏品状态历史快照仓储。
type SnapshotRepository interface {
	Append(ctx context.Context, s *domain.ArtifactSnapshot) error
	ListByArtifact(ctx context.Context, artifactID int64, p domain.Page) ([]domain.ArtifactSnapshot, error)
}
