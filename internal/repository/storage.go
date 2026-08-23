package repository

import (
	"context"

	"gowork/internal/domain"
)

// UnitFilter 存储单元过滤。
type UnitFilter struct {
	Kind *string
}

// StorageUnitRepository 存储单元仓储。
type StorageUnitRepository interface {
	Create(ctx context.Context, u *domain.StorageUnit) error
	GetByID(ctx context.Context, id int64) (*domain.StorageUnit, error)
	Update(ctx context.Context, u *domain.StorageUnit) error
	List(ctx context.Context, f UnitFilter, p domain.Page) ([]domain.StorageUnit, error)
}

// SensorFilter 传感器过滤。
type SensorFilter struct {
	StorageUnitID *int64
}

// SensorRepository 传感器仓储。
type SensorRepository interface {
	Create(ctx context.Context, s *domain.Sensor) error
	GetByID(ctx context.Context, id int64) (*domain.Sensor, error)
	List(ctx context.Context, f SensorFilter, p domain.Page) ([]domain.Sensor, error)
}
