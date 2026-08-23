package service

import (
	"context"
	"strings"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

// EnvService 环境域服务：存储单元、传感器、保存等级、阈值规则、环境采样。
type EnvService struct {
	d *Deps
}

// NewEnvService 构造环境域服务。
func NewEnvService(d *Deps) *EnvService { return &EnvService{d: d} }

// CreateUnit 创建库房/展柜。
func (s *EnvService) CreateUnit(ctx context.Context, code, name, kind, location string) (*domain.StorageUnit, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" {
		return nil, domain.Invalidf("单元编号与名称不能为空")
	}
	if kind != domain.UnitWarehouse && kind != domain.UnitShowcase {
		return nil, domain.Invalidf("单元类型必须为 warehouse 或 showcase")
	}
	now := s.d.now()
	u := &domain.StorageUnit{Code: code, Name: name, Kind: kind, Location: location,
		Status: domain.UnitActive, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := s.d.Repo.Units.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// ListUnits 单元分页。
func (s *EnvService) ListUnits(ctx context.Context, f repository.UnitFilter, p domain.Page) (domain.Paged[domain.StorageUnit], error) {
	p = p.Normalize()
	items, err := s.d.Repo.Units.List(ctx, f, p)
	if err != nil {
		return domain.Paged[domain.StorageUnit]{}, err
	}
	return domain.BuildPaged(items, p.Limit, func(u domain.StorageUnit) int64 { return u.ID }), nil
}

// CreateSensor 注册传感器。
func (s *EnvService) CreateSensor(ctx context.Context, code string, unitID int64, kind string) (*domain.Sensor, error) {
	if strings.TrimSpace(code) == "" {
		return nil, domain.Invalidf("传感器编号不能为空")
	}
	u, err := s.d.Repo.Units.GetByID(ctx, unitID)
	if err != nil {
		return nil, err
	}
	if u.Status != domain.UnitActive {
		return nil, domain.Rulef("单元 %d 已停用，不能注册传感器", unitID)
	}
	if kind == "" {
		kind = "th"
	}
	sensor := &domain.Sensor{Code: code, StorageUnitID: unitID, Kind: kind, Status: "active", CreatedAt: s.d.now()}
	if err := s.d.Repo.Sensors.Create(ctx, sensor); err != nil {
		return nil, err
	}
	return sensor, nil
}

// ListSensors 传感器分页。
func (s *EnvService) ListSensors(ctx context.Context, f repository.SensorFilter, p domain.Page) (domain.Paged[domain.Sensor], error) {
	p = p.Normalize()
	items, err := s.d.Repo.Sensors.List(ctx, f, p)
	if err != nil {
		return domain.Paged[domain.Sensor]{}, err
	}
	return domain.BuildPaged(items, p.Limit, func(x domain.Sensor) int64 { return x.ID }), nil
}

// CreateLevel 创建保存等级。
func (s *EnvService) CreateLevel(ctx context.Context, code, name, desc string) (*domain.PreservationLevel, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" {
		return nil, domain.Invalidf("等级编号与名称不能为空")
	}
	l := &domain.PreservationLevel{Code: code, Name: name, Description: desc, CreatedAt: s.d.now()}
	if err := s.d.Repo.Levels.Create(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

// ListLevels 等级分页。
func (s *EnvService) ListLevels(ctx context.Context, p domain.Page) (domain.Paged[domain.PreservationLevel], error) {
	p = p.Normalize()
	items, err := s.d.Repo.Levels.List(ctx, p)
	if err != nil {
		return domain.Paged[domain.PreservationLevel]{}, err
	}
	return domain.BuildPaged(items, p.Limit, func(l domain.PreservationLevel) int64 { return l.ID }), nil
}

// CreateRule 创建阈值规则版本（draft，版本号自动递增）。
func (s *EnvService) CreateRule(ctx context.Context, levelID int64, tempMin, tempMax, humMin, humMax float64, consecutive int) (*domain.ThresholdRuleVersion, error) {
	if tempMin >= tempMax || humMin >= humMax {
		return nil, domain.Invalidf("阈值区间非法")
	}
	if consecutive <= 0 {
		consecutive = 1
	}
	if _, err := s.d.Repo.Levels.GetByID(ctx, levelID); err != nil {
		return nil, err
	}
	var rule *domain.ThresholdRuleVersion
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		existing, err := r.Rules.List(ctx, repository.RuleFilter{LevelID: &levelID}, domain.Page{Limit: 200})
		if err != nil {
			return err
		}
		maxNo := 0
		for _, e := range existing {
			if e.VersionNo > maxNo {
				maxNo = e.VersionNo
			}
		}
		rule = &domain.ThresholdRuleVersion{
			LevelID: levelID, VersionNo: maxNo + 1,
			TempMin: tempMin, TempMax: tempMax, HumidityMin: humMin, HumidityMax: humMax,
			ConsecutiveBreach: consecutive, Status: domain.RuleDraft, CreatedAt: s.d.now(),
		}
		return r.Rules.Create(ctx, rule)
	})
	if err != nil {
		return nil, err
	}
	return rule, nil
}

// ActivateRule 启用规则版本（同事务退役同等级旧版本）。
func (s *EnvService) ActivateRule(ctx context.Context, id int64) (*domain.ThresholdRuleVersion, error) {
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		if err := r.Rules.Activate(ctx, id, s.d.now()); err != nil {
			return err
		}
		return s.d.Audit.Record(ctx, r, "system", "rule.activate", "threshold_rule", id, "")
	})
	if err != nil {
		return nil, err
	}
	return s.d.Repo.Rules.GetByID(ctx, id)
}

// ListRules 规则分页。
func (s *EnvService) ListRules(ctx context.Context, f repository.RuleFilter, p domain.Page) (domain.Paged[domain.ThresholdRuleVersion], error) {
	p = p.Normalize()
	items, err := s.d.Repo.Rules.List(ctx, f, p)
	if err != nil {
		return domain.Paged[domain.ThresholdRuleVersion]{}, err
	}
	return domain.BuildPaged(items, p.Limit, func(x domain.ThresholdRuleVersion) int64 { return x.ID }), nil
}

// IngestSample 接收环境采样。采样时间早于相关借展验收完成时间的数据标记为迟到，
// 仅入库存档，不参与异常判定，不改变已完成的借展验收结论。
func (s *EnvService) IngestSample(ctx context.Context, sensorID int64, temperature, humidity float64, sampledAt int64) (*domain.EnvSample, error) {
	sensor, err := s.d.Repo.Sensors.GetByID(ctx, sensorID)
	if err != nil {
		return nil, err
	}
	if sensor.Status != "active" {
		return nil, domain.Rulef("传感器 %d 已停用", sensorID)
	}
	now := s.d.now()
	if sampledAt <= 0 {
		sampledAt = now
	}
	if sampledAt > now+300 {
		return nil, domain.Invalidf("采样时间不能显著晚于当前时间")
	}
	sample := &domain.EnvSample{
		SensorID: sensorID, StorageUnitID: sensor.StorageUnitID,
		Temperature: temperature, Humidity: humidity,
		SampledAt: sampledAt, ReceivedAt: now,
	}
	if t, ok, err := s.d.Repo.Loans.LatestAcceptanceTimeByUnit(ctx, sensor.StorageUnitID); err != nil {
		return nil, err
	} else if ok {
		acceptedAt := t
		if sampledAt < acceptedAt {
			sample.Late = true
		}
	}
	if err := s.d.Repo.Samples.Create(ctx, sample); err != nil {
		return nil, err
	}
	return sample, nil
}

// ListSamples 采样分页。
func (s *EnvService) ListSamples(ctx context.Context, f repository.SampleFilter, p domain.Page) (domain.Paged[domain.EnvSample], error) {
	p = p.Normalize()
	items, err := s.d.Repo.Samples.List(ctx, f, p)
	if err != nil {
		return domain.Paged[domain.EnvSample]{}, err
	}
	return domain.BuildPaged(items, p.Limit, func(x domain.EnvSample) int64 { return x.ID }), nil
}
