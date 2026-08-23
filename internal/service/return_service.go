package service

import (
	"context"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

// ReturnService 归还验收服务：验收与借展关闭在同一事务内完成。
type ReturnService struct {
	d *Deps
}

// NewReturnService 构造归还验收服务。
func NewReturnService(d *Deps) *ReturnService { return &ReturnService{d: d} }

// Accept 归还验收：结果唯一决定藏品归还后的状态；借展单同事务关闭。
//
//	pass -> stored；pass_with_notes -> stored（记录意见）；rejected -> isolated。
func (s *ReturnService) Accept(ctx context.Context, loanID int64, result, reviewer, note string) (*domain.ReturnAcceptance, error) {
	switch result {
	case domain.AcceptPass, domain.AcceptPassWithNotes, domain.AcceptRejected:
	default:
		return nil, domain.Invalidf("验收结果非法：%s", result)
	}
	if reviewer == "" {
		return nil, domain.Invalidf("复核人不能为空")
	}
	var out *domain.ReturnAcceptance
	err := s.d.Tx.Within(ctx, func(r *repository.Repositories) error {
		l, err := r.Loans.GetByID(ctx, loanID)
		if err != nil {
			return err
		}
		if l.Status != domain.LoanReturned {
			return domain.Statef("借展状态 %s 不允许归还验收", l.Status)
		}
		check, err := r.Checks.ByLoanAndDirection(ctx, loanID, domain.CheckIn)
		if err != nil {
			return err
		}
		items, err := r.Loans.ItemsByLoan(ctx, loanID)
		if err != nil {
			return err
		}
		now := s.d.now()
		acc := &domain.ReturnAcceptance{
			LoanID: loanID, CheckID: check.ID, Result: result, Reviewer: reviewer,
			Note: note, ReviewedAt: now, CreatedAt: now,
		}
		if err := r.Acceptances.CreateAcceptance(ctx, acc); err != nil {
			return err
		}
		for _, it := range items {
			a, err := r.Artifacts.GetByID(ctx, it.ArtifactID)
			if err != nil {
				return err
			}
			if a.Status != domain.ArtifactReturnedPending {
				return domain.Statef("藏品 %d 状态 %s 不在待验收", a.ID, a.Status)
			}
			switch result {
			case domain.AcceptRejected:
				a.Status = domain.ArtifactIsolated
			default:
				a.Status = domain.ArtifactStored
				a.StorageUnitID = &it.FrozenUnitID
			}
			if note != "" {
				a.Note = note
			}
			a.UpdatedAt = now
			if err := r.Artifacts.Update(ctx, a); err != nil {
				return err
			}
			if err := s.d.Audit.SnapshotArtifact(ctx, r, a, "归还验收："+result); err != nil {
				return err
			}
		}
		l.Status = domain.LoanClosed
		l.Overdue = false
		l.Attention = false
		l.UpdatedAt = now
		if err := r.Loans.Update(ctx, l); err != nil {
			return err
		}
		out = acc
		return s.d.Audit.Record(ctx, r, reviewer, "loan.return_acceptance", "loan", loanID, result)
	})
	return out, err
}
