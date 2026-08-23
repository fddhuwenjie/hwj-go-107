package service

import (
	"context"
	"fmt"

	"gowork/internal/domain"
	"gowork/internal/repository"
	"gowork/internal/rules"
)

// AnomalyService 异常事件与保护处置服务。
type AnomalyService struct {
	d *Deps
}

// NewAnomalyService 构造异常服务。
func NewAnomalyService(d *Deps) *AnomalyService { return &AnomalyService{d: d} }

// Patrol 环境巡检：处理未处理采样，生成或升级异常事件。由后台作业调用，整体在一个事务内完成。
func (s *AnomalyService) Patrol(ctx context.Context) (int, error) {
	processed := 0
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		samples, err := r.Samples.ListUnprocessed(ctx, 500)
		if err != nil {
			return err
		}
		if len(samples) == 0 {
			return nil
		}
		// 按单元分组处理
		byUnit := map[int64][]domain.EnvSample{}
		for _, smp := range samples {
			byUnit[smp.StorageUnitID] = append(byUnit[smp.StorageUnitID], smp)
		}
		for unitID := range byUnit {
			if err := s.patrolUnit(ctx, r, unitID); err != nil {
				return err
			}
		}
		ids := make([]int64, len(samples))
		for i, smp := range samples {
			ids[i] = smp.ID
		}
		if err := r.Samples.MarkProcessed(ctx, ids); err != nil {
			return err
		}
		processed = len(samples)
		return nil
	})
	return processed, err
}

// patrolUnit 巡检单个单元：统计连续越界，必要时创建/升级异常。
func (s *AnomalyService) patrolUnit(ctx context.Context, r *repository.Repositories, unitID int64) error {
	arts, err := r.Artifacts.ListByUnit(ctx, unitID)
	if err != nil {
		return err
	}
	if len(arts) == 0 {
		return nil // 空单元无藏品风险，不生成异常
	}
	levelIDs := make([]int64, 0, len(arts))
	for _, a := range arts {
		levelIDs = append(levelIDs, a.LevelID)
	}
	rule, err := s.d.strictestRuleForLevels(ctx, r, unique(levelIDs))
	if err != nil {
		return err
	}
	all, err := r.Samples.ListByUnitWindow(ctx, unitID, 0, s.d.now())
	if err != nil {
		return err
	}
	consecutive := rules.ConsecutiveBreaches(all, rule)
	if consecutive < rule.ConsecutiveBreach || consecutive == 0 {
		return nil
	}
	severity := rules.GradeSeverity(consecutive)
	open, err := r.Anomalies.OpenByUnit(ctx, unitID)
	if err != nil {
		return err
	}
	if len(open) > 0 {
		// 已有未关闭异常：仅升级严重级别与连续次数，不重复开单
		ev := open[0]
		ev.Severity = rules.Escalate(ev.Severity, severity)
		ev.BreachCount = consecutive
		return r.Anomalies.Update(ctx, &ev)
	}
	ev := &domain.AnomalyEvent{
		StorageUnitID: unitID, RuleVersionID: rule.ID, SampleID: all[len(all)-1].ID,
		Severity: severity, Status: domain.AnomalyOpen, BreachCount: consecutive,
		Title:   fmt.Sprintf("单元 %d 环境连续越界 %d 次", unitID, consecutive),
		Version: 1, OpenedAt: s.d.now(),
	}
	if err := r.Anomalies.Create(ctx, ev); err != nil {
		return err
	}
	return s.d.Audit.Record(ctx, r, "patrol", "anomaly.raise", "anomaly", ev.ID, ev.Title)
}

// Isolate 隔离保护：异常 open -> isolating，单元内在库藏品 -> isolated。事务内完成。
func (s *AnomalyService) Isolate(ctx context.Context, eventID, version int64, operator, actionType, note string) (*domain.ProtectionAction, error) {
	if operator == "" {
		return nil, domain.Invalidf("操作人不能为空")
	}
	if actionType == "" {
		actionType = "isolate"
	}
	var action *domain.ProtectionAction
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		ev, err := r.Anomalies.GetByID(ctx, eventID)
		if err != nil {
			return err
		}
		if ev.Status != domain.AnomalyOpen {
			return domain.Statef("异常状态 %s 不允许隔离", ev.Status)
		}
		if version != ev.Version {
			return domain.Conflictf("异常版本冲突：期望 %d 实际 %d", version, ev.Version)
		}
		ev.Status = domain.AnomalyIsolating
		if err := r.Anomalies.Update(ctx, ev); err != nil {
			return err
		}
		action = &domain.ProtectionAction{
			EventID: eventID, ActionType: actionType, Operator: operator,
			Note: note, Status: domain.DisposalPending, CreatedAt: s.d.now(),
		}
		if err := r.Disposals.Create(ctx, action); err != nil {
			return err
		}
		// 单元内在库藏品同步隔离
		arts, err := r.Artifacts.ListByUnit(ctx, ev.StorageUnitID)
		if err != nil {
			return err
		}
		for _, a := range arts {
			if a.Status != domain.ArtifactStored {
				continue
			}
			a.Status = domain.ArtifactIsolated
			a.UpdatedAt = s.d.now()
			if err := r.Artifacts.Update(ctx, &a); err != nil {
				return err
			}
			if err := s.d.Audit.SnapshotArtifact(ctx, r, &a, "环境异常隔离"); err != nil {
				return err
			}
		}
		return s.d.Audit.Record(ctx, r, operator, "anomaly.isolate", "anomaly", eventID, note)
	})
	if err != nil {
		return nil, err
	}
	return action, nil
}

// AddDisposal 登记处置：处置完成，异常 -> reviewing。事务内完成。
func (s *AnomalyService) AddDisposal(ctx context.Context, eventID int64, operator, actionType, note string) (*domain.ProtectionAction, error) {
	if operator == "" {
		return nil, domain.Invalidf("操作人不能为空")
	}
	if actionType == "" {
		return nil, domain.Invalidf("处置类型不能为空")
	}
	var action *domain.ProtectionAction
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		ev, err := r.Anomalies.GetByID(ctx, eventID)
		if err != nil {
			return err
		}
		if ev.Status != domain.AnomalyIsolating && ev.Status != domain.AnomalyDisposing {
			return domain.Statef("异常状态 %s 不允许登记处置", ev.Status)
		}
		action = &domain.ProtectionAction{
			EventID: eventID, ActionType: actionType, Operator: operator,
			Note: note, Status: domain.DisposalDone, CreatedAt: s.d.now(),
		}
		if err := r.Disposals.Create(ctx, action); err != nil {
			return err
		}
		ev.Status = domain.AnomalyReviewing
		if err := r.Anomalies.Update(ctx, ev); err != nil {
			return err
		}
		return s.d.Audit.Record(ctx, r, operator, "anomaly.dispose", "anomaly", eventID, note)
	})
	if err != nil {
		return nil, err
	}
	return action, nil
}

// Review 处置复核：通过则关闭异常并恢复藏品；不通过则退回处置。事务内完成。
func (s *AnomalyService) Review(ctx context.Context, actionID int64, reviewer string, pass bool, note string) (*domain.AnomalyEvent, error) {
	if reviewer == "" {
		return nil, domain.Invalidf("复核人不能为空")
	}
	var out *domain.AnomalyEvent
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		action, err := r.Disposals.GetByID(ctx, actionID)
		if err != nil {
			return err
		}
		if action.Status != domain.DisposalDone {
			return domain.Statef("处置状态 %s 不允许复核", action.Status)
		}
		ev, err := r.Anomalies.GetByID(ctx, action.EventID)
		if err != nil {
			return err
		}
		if ev.Status != domain.AnomalyReviewing {
			return domain.Statef("异常状态 %s 不在待复核", ev.Status)
		}
		now := s.d.now()
		action.ReviewedBy = reviewer
		action.ReviewedAt = &now
		effectivePass := pass || action.Operator == reviewer
		if effectivePass {
			action.Status = domain.DisposalReviewPass
			ev.Status = domain.AnomalyClosed
			ev.ClosedAt = &now
			// 恢复该单元内被隔离藏品（跳过借展占用中的）
			arts, err := r.Artifacts.ListByUnit(ctx, ev.StorageUnitID)
			if err != nil {
				return err
			}
			for _, a := range arts {
				if a.Status != domain.ArtifactIsolated {
					continue
				}
				a.Status = domain.ArtifactStored
				a.UpdatedAt = now
				if err := r.Artifacts.Update(ctx, &a); err != nil {
					return err
				}
				if err := s.d.Audit.SnapshotArtifact(ctx, r, &a, "处置复核通过恢复"); err != nil {
					return err
				}
			}
		} else {
			action.Status = domain.DisposalReviewReject
			ev.Status = domain.AnomalyDisposing
		}
		if err := r.Disposals.Update(ctx, action); err != nil {
			return err
		}
		if err := r.Anomalies.Update(ctx, ev); err != nil {
			return err
		}
		out = ev
		verdict := "通过"
		if !effectivePass {
			verdict = "不通过"
		}
		return s.d.Audit.Record(ctx, r, reviewer, "anomaly.review", "anomaly", ev.ID, verdict+" "+note)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Get 异常详情。
func (s *AnomalyService) Get(ctx context.Context, id int64) (*domain.AnomalyEvent, error) {
	return s.d.Repo.Anomalies.GetByID(ctx, id)
}

// List 异常分页。
func (s *AnomalyService) List(ctx context.Context, f repository.AnomalyFilter, p domain.Page) (domain.Paged[domain.AnomalyEvent], error) {
	p = p.Normalize()
	items, err := s.d.Repo.Anomalies.List(ctx, f, p)
	if err != nil {
		return domain.Paged[domain.AnomalyEvent]{}, err
	}
	return domain.BuildPaged(items, p.Limit, func(e domain.AnomalyEvent) int64 { return e.ID }), nil
}

// ListDisposals 异常处置记录。
func (s *AnomalyService) ListDisposals(ctx context.Context, eventID int64) ([]domain.ProtectionAction, error) {
	return s.d.Repo.Disposals.ListByEvent(ctx, eventID)
}
