package service

import (
	"context"
	"errors"

	"gowork/internal/domain"
	"gowork/internal/repository"
	"gowork/internal/rules"
)

// HandoverService 包装交接、运输节点与展陈确认服务。
type HandoverService struct {
	d *Deps
}

// NewHandoverService 构造交接服务。
func NewHandoverService(d *Deps) *HandoverService { return &HandoverService{d: d} }

// AddHandover 追加包装交接：时间严格递增、身份不重复替代、交接链连续；幂等键去重。
func (s *HandoverService) AddHandover(ctx context.Context, loanID int64, idemKey string, in HandoverInput) (*domain.PackageHandover, bool, error) {
	if idemKey == "" {
		return nil, false, domain.Invalidf("幂等键不能为空")
	}
	if existing, err := s.d.Repo.Handovers.ByIdempotencyKey(ctx, idemKey); err == nil {
		return existing, true, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, false, err
	}
	var out *domain.PackageHandover
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		l, err := r.Loans.GetByID(ctx, loanID)
		if err != nil {
			return err
		}
		if l.Status != domain.LoanInTransit {
			return domain.Statef("借展状态 %s 不允许追加交接", l.Status)
		}
		prev, err := r.Handovers.LatestByLoan(ctx, loanID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		if err := rules.HandoverOrderValid(prev, in.FromPerson, in.ToPerson, in.HandedAt); err != nil {
			return err
		}
		seq := 1
		if prev != nil {
			seq = prev.Seq + 1
		}
		h := &domain.PackageHandover{
			LoanID: loanID, Seq: seq, FromPerson: in.FromPerson, ToPerson: in.ToPerson,
			HandedAt: in.HandedAt, Location: in.Location, IdempotencyKey: idemKey, CreatedAt: s.d.now(),
		}
		if err := r.Handovers.Create(ctx, h); err != nil {
			return err
		}
		out = h
		return s.d.Audit.Record(ctx, r, in.ToPerson, "loan.handover", "loan", loanID, in.Location)
	})
	if err != nil {
		return nil, false, err
	}
	return out, false, nil
}

// AddTransportNode 追加运输节点：序号与时间递增。
func (s *HandoverService) AddTransportNode(ctx context.Context, loanID int64, nodeType, location string, occurredAt int64, recordedBy string) (*domain.TransportNode, error) {
	switch nodeType {
	case "departure", "transit", "arrival":
	default:
		return nil, domain.Invalidf("运输节点类型非法：%s", nodeType)
	}
	var out *domain.TransportNode
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		l, err := r.Loans.GetByID(ctx, loanID)
		if err != nil {
			return err
		}
		if l.Status != domain.LoanInTransit {
			return domain.Statef("借展状态 %s 不允许追加运输节点", l.Status)
		}
		prev, err := r.Handovers.LatestNodeByLoan(ctx, loanID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		seq := 1
		if prev != nil {
			sameMoment := occurredAt == prev.OccurredAt
			if occurredAt < prev.OccurredAt {
				return domain.Rulef("运输节点时间不能早于上一节点：%d", prev.OccurredAt)
			}
			if sameMoment {
				seq = prev.Seq
			} else {
				seq = prev.Seq + 1
			}
		}
		// 序号只由事务开始时的快照计算。
		n := &domain.TransportNode{
			LoanID: loanID, Seq: seq, NodeType: nodeType, Location: location,
			OccurredAt: occurredAt, RecordedBy: recordedBy, CreatedAt: s.d.now(),
		}
		if err := r.Handovers.CreateNode(ctx, n); err != nil {
			return err
		}
		out = n
		return s.d.Audit.Record(ctx, r, recordedBy, "loan.transport_node", "loan", loanID, nodeType+"@"+location)
	})
	return out, err
}

// ConfirmExhibition 展陈确认：展柜前置窗口连续合格且无未关闭异常；借展 -> exhibiting，藏品 -> on_loan。
func (s *HandoverService) ConfirmExhibition(ctx context.Context, loanID, showcaseID int64, confirmedBy, note string) (*domain.ExhibitionConfirm, error) {
	if confirmedBy == "" {
		return nil, domain.Invalidf("确认人不能为空")
	}
	var out *domain.ExhibitionConfirm
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		l, err := r.Loans.GetByID(ctx, loanID)
		if err != nil {
			return err
		}
		if l.Status != domain.LoanInTransit {
			return domain.Statef("借展状态 %s 不允许展陈确认", l.Status)
		}
		unit, err := r.Units.GetByID(ctx, showcaseID)
		if err != nil {
			return err
		}
		if unit.Kind != domain.UnitShowcase || unit.Status != domain.UnitActive {
			return domain.Rulef("单元 %d 不是启用状态的展柜", showcaseID)
		}
		items, err := r.Loans.ItemsByLoan(ctx, loanID)
		if err != nil {
			return err
		}
		levelIDs := []int64{}
		for _, it := range items {
			levelIDs = append(levelIDs, it.FrozenLevelID)
		}
		if err := s.d.checkEnvEligibility(ctx, r, showcaseID, unique(levelIDs), "展柜环境"); err != nil {
			return err
		}
		now := s.d.now()
		confirm := &domain.ExhibitionConfirm{
			LoanID: loanID, ShowcaseID: showcaseID, ConfirmedBy: confirmedBy,
			ConfirmedAt: now, Note: note, CreatedAt: now,
		}
		if err := r.Acceptances.CreateConfirm(ctx, confirm); err != nil {
			return err
		}
		for _, it := range items {
			a, err := r.Artifacts.GetByID(ctx, it.ArtifactID)
			if err != nil {
				return err
			}
			a.Status = domain.ArtifactOnLoan
			a.StorageUnitID = &showcaseID
			a.UpdatedAt = now
			if err := r.Artifacts.Update(ctx, a); err != nil {
				return err
			}
			if err := s.d.Audit.SnapshotArtifact(ctx, r, a, "展陈确认上展"); err != nil {
				return err
			}
		}
		l.Status = domain.LoanExhibiting
		l.UpdatedAt = now
		if err := r.Loans.Update(ctx, l); err != nil {
			return err
		}
		out = confirm
		return s.d.Audit.Record(ctx, r, confirmedBy, "loan.exhibition_confirm", "loan", loanID, note)
	})
	return out, err
}
