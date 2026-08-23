// Package jobs 后台作业调度：环境巡检、借展到期、交接超时与失败重试，可重启恢复。
package jobs

import (
	"context"
	"log/slog"
	"time"

	"gowork/internal/clock"
	"gowork/internal/config"
	"gowork/internal/domain"
	"gowork/internal/repository"
)

// Handler 作业处理器。
type Handler func(ctx context.Context) error

// Scheduler 作业调度器。作业持久化于 jobs 表：
// 进程重启时 running 作业恢复为 pending，失败作业按指数退避重试。
type Scheduler struct {
	repo     *repository.Repositories
	clk      clock.Clock
	cfg      config.Config
	log      *slog.Logger
	handlers map[string]Handler
	periodic []string
}

// NewScheduler 构造调度器并注册处理器。
func NewScheduler(repo *repository.Repositories, clk clock.Clock, cfg config.Config, log *slog.Logger, handlers map[string]Handler) *Scheduler {
	periodic := []string{domain.JobKindPatrol, domain.JobKindLoanDue, domain.JobKindHandoverTimeout}
	return &Scheduler{repo: repo, clk: clk, cfg: cfg, log: log, handlers: handlers, periodic: periodic}
}

// Recover 启动恢复：中断的 running 作业恢复为 pending。
func (s *Scheduler) Recover(ctx context.Context) (int64, error) {
	n, err := s.repo.Jobs.RecoverRunning(ctx, s.clk.Now().Unix())
	if err != nil {
		return 0, err
	}
	if n > 0 {
		s.log.Info("作业恢复", "recovered", n)
	}
	return n, nil
}

// EnqueuePeriodic 为每个周期类型入队到期作业（幂等去重：同类型已有活动作业则跳过）。
func (s *Scheduler) EnqueuePeriodic(ctx context.Context) error {
	now := s.clk.Now().Unix()
	for _, kind := range s.periodic {
		active, err := s.repo.Jobs.HasActiveByKind(ctx, kind)
		if err != nil {
			return err
		}
		if active {
			continue
		}
		job := &domain.Job{
			Kind: kind, Status: domain.JobPending, MaxAttempts: s.cfg.JobMaxAttempts,
			RunAt: now, CreatedAt: now, UpdatedAt: now,
		}
		if err := s.repo.Jobs.Enqueue(ctx, job); err != nil {
			return err
		}
	}
	return nil
}

// RunOnce 执行一轮：入队周期作业 + 处理到期作业。返回执行成功的作业数。
func (s *Scheduler) RunOnce(ctx context.Context) (int, error) {
	if err := s.EnqueuePeriodic(ctx); err != nil {
		return 0, err
	}
	now := s.clk.Now().Unix()
	due, err := s.repo.Jobs.Due(ctx, now, 20)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, job := range due {
		if err := s.execute(ctx, job); err != nil {
			s.log.Error("作业执行失败", "kind", job.Kind, "id", job.ID, "error", err)
			continue
		}
		done++
	}
	return done, nil
}

// execute 领取并执行单个作业，失败时按指数退避重试或置为失败终态。
func (s *Scheduler) execute(ctx context.Context, job domain.Job) error {
	now := s.clk.Now().Unix()
	if err := s.repo.Jobs.Claim(ctx, job.ID, now); err != nil {
		return err
	}
	h, ok := s.handlers[job.Kind]
	if !ok {
		return s.repo.Jobs.Retry(ctx, job.ID, job.Attempts+1, now, "未注册的作业类型: "+job.Kind, true, now)
	}
	err := h(ctx)
	finish := s.clk.Now().Unix()
	if err == nil {
		return s.repo.Jobs.Complete(ctx, job.ID, finish)
	}
	attempts := job.Attempts + 1
	failed := attempts >= job.MaxAttempts
	runAt := finish + Backoff(attempts)
	return s.repo.Jobs.Retry(ctx, job.ID, attempts, runAt, err.Error(), failed, finish)
}

// Run 周期性运行调度循环，直到 ctx 取消。
func (s *Scheduler) Run(ctx context.Context) {
	if _, err := s.Recover(ctx); err != nil {
		s.log.Error("作业恢复失败", "error", err)
	}
	ticker := time.NewTicker(s.cfg.JobInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.RunOnce(ctx); err != nil {
				s.log.Error("作业调度失败", "error", err)
			}
		}
	}
}
