package service

import (
	"context"
	"encoding/json"
	"strings"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

// LoanService 借展申请服务：创建、提交、审批（冻结）、驳回、撤销。
type LoanService struct {
	d *Deps
}

// NewLoanService 构造借展服务。
func NewLoanService(d *Deps) *LoanService { return &LoanService{d: d} }

// CreateLoanInput 创建借展入参。
type CreateLoanInput struct {
	Code, Borrower, Venue, Purpose string
	StartAt, EndAt                 int64
	ArtifactIDs                    []int64
}

// Create 创建借展草稿。
func (s *LoanService) Create(ctx context.Context, in CreateLoanInput) (*domain.LoanApplication, error) {
	if strings.TrimSpace(in.Code) == "" || strings.TrimSpace(in.Borrower) == "" {
		return nil, domain.Invalidf("借展单号与借展方不能为空")
	}
	if len(in.ArtifactIDs) == 0 {
		return nil, domain.Invalidf("借展至少包含一件藏品")
	}
	if in.EndAt <= in.StartAt {
		return nil, domain.Invalidf("借展结束时间必须晚于开始时间")
	}
	now := s.d.now()
	loan := &domain.LoanApplication{
		Code: in.Code, Borrower: in.Borrower, Venue: in.Venue, Purpose: in.Purpose,
		StartAt: in.StartAt, EndAt: in.EndAt, Status: domain.LoanDraft,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		if err := r.Loans.Create(ctx, loan); err != nil {
			return err
		}
		for _, aid := range unique(in.ArtifactIDs) {
			a, err := r.Artifacts.GetByID(ctx, aid)
			if err != nil {
				return err
			}
			if a.Retired || a.OnActiveLoan() {
				return domain.Rulef("藏品 %d 当前状态 %s 不可加入借展", aid, a.Status)
			}
			item := &domain.LoanItem{LoanID: loan.ID, ArtifactID: aid, CreatedAt: now}
			if err := r.Loans.AddItem(ctx, item); err != nil {
				return err
			}
		}
		return s.d.Audit.Record(ctx, r, "system", "loan.create", "loan", loan.ID, loan.Code)
	})
	if err != nil {
		return nil, err
	}
	return loan, nil
}

// transition 简单状态推进（乐观锁）。
func (s *LoanService) transition(ctx context.Context, id, version int64, from []string, to, action string, mutate func(l *domain.LoanApplication)) (*domain.LoanApplication, error) {
	var out *domain.LoanApplication
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		l, err := r.Loans.GetByID(ctx, id)
		if err != nil {
			return err
		}
		ok := false
		for _, f := range from {
			if l.Status == f {
				ok = true
			}
		}
		if !ok {
			return domain.Statef("借展状态 %s 不允许执行 %s", l.Status, action)
		}
		if version != l.Version {
			return domain.Conflictf("借展版本冲突：期望 %d 实际 %d", version, l.Version)
		}
		l.Status = to
		l.UpdatedAt = s.d.now()
		if mutate != nil {
			mutate(l)
		}
		if err := r.Loans.Update(ctx, l); err != nil {
			return err
		}
		out = l
		return s.d.Audit.Record(ctx, r, "system", action, "loan", l.ID, "")
	})
	return out, err
}

// Submit 提交审批。
func (s *LoanService) Submit(ctx context.Context, id, version int64) (*domain.LoanApplication, error) {
	return s.transition(ctx, id, version, []string{domain.LoanDraft}, domain.LoanSubmitted, "loan.submit", nil)
}

// Cancel 撤销：仅草稿/已提交可撤销。审批通过后藏品已冻结、规则快照已落库，
// 不可再走草稿撤销；终止须走完整出库—归还—验收流程。
func (s *LoanService) Cancel(ctx context.Context, id, version int64) (*domain.LoanApplication, error) {
	return s.transition(ctx, id, version, []string{domain.LoanDraft, domain.LoanSubmitted}, domain.LoanCancelled, "loan.cancel", nil)
}

// Reject 审批驳回。
func (s *LoanService) Reject(ctx context.Context, id, version int64, reviewer, reason string) (*domain.LoanApplication, error) {
	if reason == "" {
		return nil, domain.Invalidf("驳回理由不能为空")
	}
	return s.transition(ctx, id, version, []string{domain.LoanSubmitted}, domain.LoanRejected, "loan.reject", func(l *domain.LoanApplication) {
		l.ApprovedBy = reviewer
		l.RejectReason = reason
	})
}

// Approve 审批通过：冻结藏品状态、保存等级、包装清单与审批规则快照。
// 前置约束：藏品均在库、所在库房无未关闭异常、库房前置窗口采样连续合格。
func (s *LoanService) Approve(ctx context.Context, id, version int64, reviewer string) (*domain.LoanApplication, error) {
	if reviewer == "" {
		return nil, domain.Invalidf("审批人不能为空")
	}
	var out *domain.LoanApplication
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		l, err := r.Loans.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if l.Status != domain.LoanSubmitted {
			return domain.Statef("借展状态 %s 不允许审批", l.Status)
		}
		if version != l.Version {
			return domain.Conflictf("借展版本冲突：期望 %d 实际 %d", version, l.Version)
		}
		items, err := r.Loans.ItemsByLoan(ctx, id)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return domain.Rulef("借展单无藏品，不能审批")
		}
		artifactIDs := make([]int64, 0, len(items))
		for _, it := range items {
			artifactIDs = append(artifactIDs, it.ArtifactID)
		}
		attachments, err := r.Attachments.ListByArtifacts(ctx, artifactIDs)
		if err != nil {
			return err
		}
		now := s.d.now()
		levelIDs := []int64{}
		frozen := domain.FrozenRules{PreLoanWindowSeconds: int64(s.d.Cfg.PreLoanWindow.Seconds())}
		unitChecked := map[int64]bool{}
		for i := range items {
			it := &items[i]
			a, err := r.Artifacts.GetByID(ctx, it.ArtifactID)
			if err != nil {
				return err
			}
			if a.Status != domain.ArtifactStored || a.StorageUnitID == nil {
				return domain.Rulef("藏品 %d 状态 %s 不满足出库条件（须在库且已分配库位）", a.ID, a.Status)
			}
			unitID := *a.StorageUnitID
			levelIDs = append(levelIDs, a.LevelID)
			if !unitChecked[unitID] {
				if err := s.d.checkEnvEligibility(ctx, r, unitID, []int64{a.LevelID}, "库房环境"); err != nil {
					return err
				}
				unitChecked[unitID] = true
			}
			// 冻结：状态、保存等级、库位、包装清单
			entries := []domain.PackagingEntry{}
			for _, at := range attachments[a.ID] {
				entries = append(entries, domain.PackagingEntry{AttachmentID: at.ID, Name: at.Name, Spec: at.Spec})
			}
			it.FrozenStatus = a.Status
			it.FrozenLevelID = a.LevelID
			it.FrozenUnitID = unitID
			it.PackagingSnapshot = domain.MarshalPackaging(entries)
			if err := r.Loans.FreezeItem(ctx, it); err != nil {
				return err
			}
			a.Status = domain.ArtifactFrozen
			a.UpdatedAt = now
			if err := r.Artifacts.Update(ctx, a); err != nil {
				return err
			}
			if err := s.d.Audit.SnapshotArtifact(ctx, r, a, "借展审批冻结"); err != nil {
				return err
			}
		}
		activeRules, err := r.Rules.ActiveByLevels(ctx, unique(levelIDs))
		if err != nil {
			return err
		}
		frozen.Rules = activeRules
		snap, err := json.Marshal(frozen)
		if err != nil {
			return err
		}
		l.RuleSnapshot = string(snap)
		l.Status = domain.LoanApproved
		l.ApprovedBy = reviewer
		l.ApprovedAt = &now
		l.UpdatedAt = now
		if err := r.Loans.Update(ctx, l); err != nil {
			return err
		}
		out = l
		return s.d.Audit.Record(ctx, r, reviewer, "loan.approve", "loan", l.ID, "冻结快照并审批通过")
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Get 借展详情。
func (s *LoanService) Get(ctx context.Context, id int64) (*domain.LoanApplication, error) {
	return s.d.Repo.Loans.GetByID(ctx, id)
}

// List 借展分页。
func (s *LoanService) List(ctx context.Context, f repository.LoanFilter, p domain.Page) (domain.Paged[domain.LoanApplication], error) {
	p = p.Normalize()
	items, err := s.d.Repo.Loans.List(ctx, f, p)
	if err != nil {
		return domain.Paged[domain.LoanApplication]{}, err
	}
	return domain.BuildPaged(items, p.Limit, func(l domain.LoanApplication) int64 { return l.ID }), nil
}

// Items 借展藏品行。
func (s *LoanService) Items(ctx context.Context, loanID int64) ([]domain.LoanItem, error) {
	return s.d.Repo.Loans.ItemsByLoan(ctx, loanID)
}
