package repository

import (
	"context"

	"gowork/internal/domain"
)

// AnomalyFilter 异常过滤。
type AnomalyFilter struct {
	Status        *string
	StorageUnitID *int64
}

// AnomalyRepository 异常事件仓储。
type AnomalyRepository interface {
	Create(ctx context.Context, e *domain.AnomalyEvent) error
	GetByID(ctx context.Context, id int64) (*domain.AnomalyEvent, error)
	// Update 乐观锁更新。
	Update(ctx context.Context, e *domain.AnomalyEvent) error
	// OpenByUnit 查询单元未关闭异常。
	OpenByUnit(ctx context.Context, unitID int64) ([]domain.AnomalyEvent, error)
	// ListOpen 查询全部未关闭异常。
	ListOpen(ctx context.Context) ([]domain.AnomalyEvent, error)
	List(ctx context.Context, f AnomalyFilter, p domain.Page) ([]domain.AnomalyEvent, error)
}

// DisposalRepository 保护处置仓储。
type DisposalRepository interface {
	Create(ctx context.Context, a *domain.ProtectionAction) error
	GetByID(ctx context.Context, id int64) (*domain.ProtectionAction, error)
	Update(ctx context.Context, a *domain.ProtectionAction) error
	ListByEvent(ctx context.Context, eventID int64) ([]domain.ProtectionAction, error)
}
