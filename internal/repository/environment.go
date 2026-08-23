package repository

import (
	"context"

	"gowork/internal/domain"
)

// LevelRepository 保存等级仓储。
type LevelRepository interface {
	Create(ctx context.Context, l *domain.PreservationLevel) error
	GetByID(ctx context.Context, id int64) (*domain.PreservationLevel, error)
	List(ctx context.Context, p domain.Page) ([]domain.PreservationLevel, error)
}

// RuleFilter 规则版本过滤。
type RuleFilter struct {
	LevelID *int64
	Status  *string
}

// RuleRepository 阈值规则版本仓储。
type RuleRepository interface {
	Create(ctx context.Context, r *domain.ThresholdRuleVersion) error
	GetByID(ctx context.Context, id int64) (*domain.ThresholdRuleVersion, error)
	// Activate 启用某版本：同等级其余启用版本退役，该版本置为 active。
	Activate(ctx context.Context, id int64, now int64) error
	ActiveByLevel(ctx context.Context, levelID int64) (*domain.ThresholdRuleVersion, error)
	// ActiveByLevels 批量取各等级启用规则。
	ActiveByLevels(ctx context.Context, levelIDs []int64) ([]domain.ThresholdRuleVersion, error)
	List(ctx context.Context, f RuleFilter, p domain.Page) ([]domain.ThresholdRuleVersion, error)
}

// SampleFilter 采样过滤。
type SampleFilter struct {
	StorageUnitID *int64
	SensorID      *int64
}

// SampleRepository 环境采样仓储。采样只增不改。
type SampleRepository interface {
	Create(ctx context.Context, s *domain.EnvSample) error
	// ListByUnitWindow 按采样时间升序取 [from,to] 内采样。
	ListByUnitWindow(ctx context.Context, unitID, from, to int64) ([]domain.EnvSample, error)
	// ListRecentBySensor 取某传感器最近 limit 条（返回升序）。
	ListRecentBySensor(ctx context.Context, sensorID int64, limit int) ([]domain.EnvSample, error)
	// ListUnprocessed 取未处理采样（升序）。
	ListUnprocessed(ctx context.Context, limit int) ([]domain.EnvSample, error)
	MarkProcessed(ctx context.Context, ids []int64) error
	List(ctx context.Context, f SampleFilter, p domain.Page) ([]domain.EnvSample, error)
}
