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

// HandoverTimeoutHandler 交接超时作业：在途借展最近交接/运输活动超过阈值仍未推进的标记关注并记录审计。
func HandoverTimeoutHandler(txm *tx.Manager, clk clock.Clock, aud *audit.Recorder, timeoutSeconds int64) Handler {
	return func(ctx context.Context) error {
		now := clk.Now().Unix()
		err := txm.Within(ctx, func(r *repository.Repositories) error {
			loans, err := r.Loans.ListByStatus(ctx, domain.LoanInTransit)
			if err != nil {
				return err
			}
			for _, l := range loans {
				if l.Attention {
					continue
				}
				last := l.UpdatedAt
				if h, err := r.Handovers.LatestByLoan(ctx, l.ID); err == nil && h.HandedAt > last {
					last = h.HandedAt
				}
				if n, err := r.Handovers.LatestNodeByLoan(ctx, l.ID); err == nil && n.OccurredAt > last {
					last = n.OccurredAt
				}
				if now-last <= timeoutSeconds {
					continue
				}
				l.Attention = true
				l.UpdatedAt = now
				if err := r.Loans.Update(ctx, &l); err != nil {
					return err
				}
				if err := aud.Record(ctx, r, "job", "loan.handover_timeout", "loan", l.ID,
					fmt.Sprintf("借展单 %s 交接超时", l.Code)); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("交接超时作业失败: %w", err)
		}
		return nil
	}
}
