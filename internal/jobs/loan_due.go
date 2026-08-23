package jobs

import (
	"context"
	"fmt"

	"gowork/internal/audit"
	"gowork/internal/clock"
	"gowork/internal/domain"
	"gowork/internal/repository"
	"gowork/internal/tx"
)

// LoanDueHandler 借展到期作业：超过应还日期仍未关闭的借展单标记逾期并记录审计。
func LoanDueHandler(txm *tx.Manager, clk clock.Clock, aud *audit.Recorder) Handler {
	return func(ctx context.Context) error {
		now := clk.Now().Unix()
		err := txm.Within(ctx, func(r *repository.Repositories) error {
			loans, err := r.Loans.ListByStatus(ctx, domain.LoanInTransit, domain.LoanExhibiting, domain.LoanReturned)
			if err != nil {
				return err
			}
			for _, l := range loans {
				if l.EndAt >= now || l.Overdue {
					continue
				}
				l.Overdue = true
				l.UpdatedAt = now
				if err := r.Loans.Update(ctx, &l); err != nil {
					return err
				}
				if err := aud.Record(ctx, r, "job", "loan.overdue", "loan", l.ID,
					fmt.Sprintf("借展单 %s 已逾期", l.Code)); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("借展到期作业失败: %w", err)
		}
		return nil
	}
}
