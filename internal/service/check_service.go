package service

import (
	"context"
	"errors"

	"gowork/internal/domain"
	"gowork/internal/repository"
	"gowork/internal/rules"
)

// CheckService 出入库清点服务。
type CheckService struct {
	d *Deps
}

// NewCheckService 构造清点服务。
func NewCheckService(d *Deps) *CheckService { return &CheckService{d: d} }

// CheckItemInput 清点明细入参。
type CheckItemInput struct {
	ArtifactID  int64               `json:"artifact_id"`
	Present     bool                `json:"present"`
	Note        string              `json:"note,omitempty"`
	Attachments []AttachmentPresent `json:"attachments"`
}

// AttachmentPresent 附件清点入参。
type AttachmentPresent struct {
	AttachmentID int64 `json:"attachment_id"`
	Present      bool  `json:"present"`
}

// HandoverInput 交接入参。
type HandoverInput struct {
	FromPerson string `json:"from_person"`
	ToPerson   string `json:"to_person"`
	HandedAt   int64  `json:"handed_at"`
	Location   string `json:"location"`
}

// CheckResult 清点结果。
type CheckResult struct {
	Check            *domain.InventoryCheck      `json:"check"`
	Items            []domain.InventoryCheckItem `json:"items"`
	Handover         *domain.PackageHandover     `json:"handover,omitempty"`
	IdempotentReplay bool                        `json:"idempotent_replay"`
}

// validateCoverage 校验清点请求覆盖借展全部藏品与附件。
// 返回 expected 集合信息用于写明细。
func (s *CheckService) validateCoverage(ctx context.Context, r *repository.Repositories, loanID int64, in []CheckItemInput) ([]domain.LoanItem, map[int64][]domain.PackagingEntry, error) {
	items, err := r.Loans.ItemsByLoan(ctx, loanID)
	if err != nil {
		return nil, nil, err
	}
	if len(items) == 0 {
		return nil, nil, domain.Rulef("借展单无藏品")
	}
	expectedAtt := map[int64][]domain.PackagingEntry{}
	for _, it := range items {
		entries, err := domain.UnmarshalPackaging(it.PackagingSnapshot)
		if err != nil {
			return nil, nil, err
		}
		expectedAtt[it.ArtifactID] = entries
	}
	gotArtifacts := map[int64]CheckItemInput{}
	for _, it := range in {
		gotArtifacts[it.ArtifactID] = it
	}
	for _, li := range items {
		got, ok := gotArtifacts[li.ArtifactID]
		if !ok {
			return nil, nil, domain.Rulef("清点未覆盖藏品 %d", li.ArtifactID)
		}
		gotAtt := map[int64]bool{}
		for _, ap := range got.Attachments {
			gotAtt[ap.AttachmentID] = true
		}
		for _, entry := range expectedAtt[li.ArtifactID] {
			if !gotAtt[entry.AttachmentID] {
				return nil, nil, domain.Rulef("清点未覆盖藏品 %d 的附件 %d", li.ArtifactID, entry.AttachmentID)
			}
		}
	}
	return items, expectedAtt, nil
}

// OutCheck 出库清点 + 首段交接，同一事务：清点单、清点明细、首段交接、藏品状态与借展状态。
func (s *CheckService) OutCheck(ctx context.Context, loanID int64, idemKey, operator string, in []CheckItemInput, ho HandoverInput) (*CheckResult, error) {
	if idemKey == "" || operator == "" {
		return nil, domain.Invalidf("幂等键与清点人不能为空")
	}
	// 幂等回放
	if existing, err := s.d.Repo.Checks.ByIdempotencyKey(ctx, idemKey); err == nil {
		items, err := s.d.Repo.Checks.ItemsByCheck(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
		ho, _ := s.d.Repo.Handovers.LatestByLoan(ctx, existing.LoanID)
		return &CheckResult{Check: existing, Items: items, Handover: ho, IdempotentReplay: true}, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	var result *CheckResult
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		l, err := r.Loans.GetByID(ctx, loanID)
		if err != nil {
			return err
		}
		if l.Status != domain.LoanApproved {
			return domain.Statef("借展状态 %s 不允许出库清点", l.Status)
		}
		items, expectedAtt, err := s.validateCoverage(ctx, r, loanID, in)
		if err != nil {
			return err
		}
		inMap := map[int64]CheckItemInput{}
		for _, it := range in {
			inMap[it.ArtifactID] = it
		}
		// 出库要求全部在场
		for _, li := range items {
			if !inMap[li.ArtifactID].Present {
				return domain.Rulef("藏品 %d 清点缺失，禁止出库", li.ArtifactID)
			}
			for _, ap := range inMap[li.ArtifactID].Attachments {
				if !ap.Present {
					return domain.Rulef("附件 %d 清点缺失，禁止出库", ap.AttachmentID)
				}
			}
		}
		// 无未关闭异常
		checkedUnits := map[int64]bool{}
		for _, li := range items {
			if checkedUnits[li.FrozenUnitID] {
				continue
			}
			checkedUnits[li.FrozenUnitID] = true
			open, err := r.Anomalies.OpenByUnit(ctx, li.FrozenUnitID)
			if err != nil {
				return err
			}
			if len(open) > 0 {
				return domain.Rulef("库房 %d 存在未关闭异常，禁止出库", li.FrozenUnitID)
			}
		}
		now := s.d.now()
		check := &domain.InventoryCheck{
			LoanID: loanID, Direction: domain.CheckOut, IdempotencyKey: idemKey,
			Operator: operator, Complete: true, CheckedAt: now, CreatedAt: now,
		}
		if err := r.Checks.Create(ctx, check); err != nil {
			return err
		}
		result = &CheckResult{Check: check}
		for _, li := range items {
			ci := &domain.InventoryCheckItem{CheckID: check.ID, ArtifactID: li.ArtifactID, Present: inMap[li.ArtifactID].Present, Note: inMap[li.ArtifactID].Note}
			if err := r.Checks.CreateItem(ctx, ci); err != nil {
				return err
			}
			result.Items = append(result.Items, *ci)
			for _, ap := range inMap[li.ArtifactID].Attachments {
				ai := &domain.InventoryCheckItem{CheckID: check.ID, ArtifactID: li.ArtifactID, AttachmentID: ap.AttachmentID, Present: ap.Present}
				if err := r.Checks.CreateItem(ctx, ai); err != nil {
					return err
				}
				result.Items = append(result.Items, *ai)
			}
			_ = expectedAtt
		}
		// 首段交接
		if err := rules.HandoverOrderValid(nil, ho.FromPerson, ho.ToPerson, ho.HandedAt); err != nil {
			return err
		}
		handover := &domain.PackageHandover{
			LoanID: loanID, Seq: 1, FromPerson: ho.FromPerson, ToPerson: ho.ToPerson,
			HandedAt: ho.HandedAt, Location: ho.Location, IdempotencyKey: idemKey + ":ho1", CreatedAt: now,
		}
		if err := r.Handovers.Create(ctx, handover); err != nil {
			return err
		}
		result.Handover = handover
		// 藏品状态 -> out
		for _, li := range items {
			a, err := r.Artifacts.GetByID(ctx, li.ArtifactID)
			if err != nil {
				return err
			}
			a.Status = domain.ArtifactOut
			a.UpdatedAt = now
			if err := r.Artifacts.Update(ctx, a); err != nil {
				return err
			}
			if err := s.d.Audit.SnapshotArtifact(ctx, r, a, "出库清点完成"); err != nil {
				return err
			}
		}
		l.Status = domain.LoanInTransit
		l.UpdatedAt = now
		if err := r.Loans.Update(ctx, l); err != nil {
			return err
		}
		return s.d.Audit.Record(ctx, r, operator, "loan.out_check", "loan", loanID, "出库清点与首段交接完成")
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// InCheck 归还清点：loan exhibiting -> returned，藏品 -> returned_pending。
func (s *CheckService) InCheck(ctx context.Context, loanID int64, idemKey, operator string, in []CheckItemInput) (*CheckResult, error) {
	if idemKey == "" || operator == "" {
		return nil, domain.Invalidf("幂等键与清点人不能为空")
	}
	if existing, err := s.d.Repo.Checks.ByIdempotencyKey(ctx, idemKey); err == nil {
		items, err := s.d.Repo.Checks.ItemsByCheck(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
		return &CheckResult{Check: existing, Items: items, IdempotentReplay: true}, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	var result *CheckResult
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		l, err := r.Loans.GetByID(ctx, loanID)
		if err != nil {
			return err
		}
		if l.Status != domain.LoanExhibiting {
			return domain.Statef("借展状态 %s 不允许归还清点", l.Status)
		}
		items, _, err := s.validateCoverage(ctx, r, loanID, in)
		if err != nil {
			return err
		}
		inMap := map[int64]CheckItemInput{}
		for _, it := range in {
			inMap[it.ArtifactID] = it
		}
		complete := true
		for _, li := range items {
			if !inMap[li.ArtifactID].Present {
				complete = false
			}
			for _, ap := range inMap[li.ArtifactID].Attachments {
				if !ap.Present {
					complete = false
				}
			}
		}
		now := s.d.now()
		check := &domain.InventoryCheck{
			LoanID: loanID, Direction: domain.CheckIn, IdempotencyKey: idemKey,
			Operator: operator, Complete: complete, CheckedAt: now, CreatedAt: now,
		}
		if err := r.Checks.Create(ctx, check); err != nil {
			return err
		}
		result = &CheckResult{Check: check}
		for _, li := range items {
			ci := &domain.InventoryCheckItem{CheckID: check.ID, ArtifactID: li.ArtifactID, Present: inMap[li.ArtifactID].Present, Note: inMap[li.ArtifactID].Note}
			if err := r.Checks.CreateItem(ctx, ci); err != nil {
				return err
			}
			result.Items = append(result.Items, *ci)
			for _, ap := range inMap[li.ArtifactID].Attachments {
				ai := &domain.InventoryCheckItem{CheckID: check.ID, ArtifactID: li.ArtifactID, AttachmentID: ap.AttachmentID, Present: ap.Present}
				if err := r.Checks.CreateItem(ctx, ai); err != nil {
					return err
				}
				result.Items = append(result.Items, *ai)
			}
		}
		for _, li := range items {
			a, err := r.Artifacts.GetByID(ctx, li.ArtifactID)
			if err != nil {
				return err
			}
			a.Status = domain.ArtifactReturnedPending
			a.UpdatedAt = now
			if err := r.Artifacts.Update(ctx, a); err != nil {
				return err
			}
			if err := s.d.Audit.SnapshotArtifact(ctx, r, a, "归还清点完成待验收"); err != nil {
				return err
			}
		}
		l.Status = domain.LoanReturned
		l.UpdatedAt = now
		if err := r.Loans.Update(ctx, l); err != nil {
			return err
		}
		return s.d.Audit.Record(ctx, r, operator, "loan.in_check", "loan", loanID, "归还清点完成")
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
