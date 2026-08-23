package jobs_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"gowork/internal/domain"
	"gowork/internal/jobs"
	"gowork/internal/service"
	"gowork/internal/testenv"
)

// newScheduler 构造测试调度器。
func newScheduler(env *testenv.Env, handlers map[string]jobs.Handler) *jobs.Scheduler {
	return jobs.NewScheduler(env.Repo, env.Clock, env.Cfg, testLogger(), handlers)
}

// TestJobRetryBackoff 作业失败后指数退避重试，最终成功。
func TestJobRetryBackoff(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()

	attempts := 0
	handlers := map[string]jobs.Handler{
		domain.JobKindPatrol: func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return errors.New("模拟失败")
			}
			return nil
		},
		domain.JobKindLoanDue:         func(ctx context.Context) error { return nil },
		domain.JobKindHandoverTimeout: func(ctx context.Context) error { return nil },
	}
	s := newScheduler(env, handlers)

	// 第 1 轮：失败，attempts=1，退避 2s
	if _, err := s.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	jobList, _ := env.Repo.Jobs.List(ctx, domain.Page{Limit: 10})
	var patrol *domain.Job
	for i := range jobList {
		if jobList[i].Kind == domain.JobKindPatrol {
			patrol = &jobList[i]
		}
	}
	if patrol == nil || patrol.Status != domain.JobPending || patrol.Attempts != 1 {
		t.Fatalf("首次失败后应 pending/attempts=1: %+v", patrol)
	}

	// 未到退避时间，不重试
	if _, err := s.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatalf("退避时间内不应重试，实际执行 %d 次", attempts)
	}

	// 推进 2s 到退避点：第 2 次仍失败，attempts=2，退避 4s
	env.Clock.Advance(2 * time.Second)
	if _, err := s.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("应第 2 次执行，实际 %d", attempts)
	}

	// 推进 4s：第 3 次成功
	env.Clock.Advance(4 * time.Second)
	if _, err := s.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("应第 3 次执行，实际 %d", attempts)
	}
	jobList, _ = env.Repo.Jobs.List(ctx, domain.Page{Limit: 10})
	for _, j := range jobList {
		if j.Kind == domain.JobKindPatrol && j.Status != domain.JobDone {
			t.Fatalf("成功后应为 done: %+v", j)
		}
	}
}

// TestJobFailedTerminal 达到最大尝试次数后进入 failed 终态。
func TestJobFailedTerminal(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()

	handlers := map[string]jobs.Handler{
		domain.JobKindPatrol:          func(ctx context.Context) error { return errors.New("永远失败") },
		domain.JobKindLoanDue:         func(ctx context.Context) error { return nil },
		domain.JobKindHandoverTimeout: func(ctx context.Context) error { return nil },
	}
	s := newScheduler(env, handlers)

	// MaxAttempts=3：执行 3 次后 failed
	for i := 0; i < 3; i++ {
		if _, err := s.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		env.Clock.Advance(10 * time.Second)
	}
	jobList, _ := env.Repo.Jobs.List(ctx, domain.Page{Limit: 10})
	for _, j := range jobList {
		if j.Kind == domain.JobKindPatrol {
			if j.Status != domain.JobFailed || j.Attempts != 3 {
				t.Fatalf("应为 failed 终态: %+v", j)
			}
		}
	}
}

// TestRestartRecovery 重启恢复：running 作业恢复为 pending 并重新执行。
func TestRestartRecovery(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()

	// 模拟崩溃：手工插入 running 作业
	now := env.Now()
	stuck := &domain.Job{
		Kind: domain.JobKindPatrol, Status: domain.JobRunning, MaxAttempts: 3,
		RunAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := env.Repo.Jobs.Enqueue(ctx, stuck); err != nil {
		t.Fatal(err)
	}

	// 关闭并重新打开同一数据库文件，模拟重启
	env.DB.Close()
	env2 := testenv.Reopen(t, env.DBPath, env.Clock)
	executed := false
	s := jobs.NewScheduler(env2.Repo, env2.Clock, env2.Cfg, testLogger(), map[string]jobs.Handler{
		domain.JobKindPatrol: func(ctx context.Context) error { executed = true; return nil },
	})
	n, err := s.Recover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应恢复 1 个 running 作业，实际 %d", n)
	}
	if _, err := s.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if !executed {
		t.Fatalf("恢复的作业应被重新执行")
	}
}

// TestLoanDueJob 借展到期作业标记逾期。
func TestLoanDueJob(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-D1")
	wh, sensor := env.SetupWarehouse("WH-D1")
	art := env.RegisterStoredArtifact("WJ-D1", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)

	// 借展期 [now+1h, now+2h]，推进时钟到 3h 后
	now := env.Now()
	loan, err := env.Loan.Create(ctx, service.CreateLoanInput{
		Code: "LN-D1", Borrower: "省博物馆", StartAt: now + 3600, EndAt: now + 7200,
		ArtifactIDs: []int64{art.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Loan.Submit(ctx, loan.ID, loan.Version); err != nil {
		t.Fatal(err)
	}
	loan, _ = env.Loan.Get(ctx, loan.ID)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatal(err)
	}
	// 出库 + 展陈确认，进入展陈中
	if _, err := env.Check.OutCheck(ctx, loan.ID, "out-d1", "甲", env.CheckItemsAllPresent(art.ID),
		service.HandoverInput{FromPerson: "A", ToPerson: "B", HandedAt: env.Now() + 100}); err != nil {
		t.Fatal(err)
	}
	_, scSensor := env.SetupShowcase("SC-D1")
	env.SeedWindowQualified(scSensor.ID)
	if _, err := env.Handover.ConfirmExhibition(ctx, loan.ID, scSensor.StorageUnitID, "C", ""); err != nil {
		t.Fatal(err)
	}

	env.Clock.Advance(3 * time.Hour)
	handler := jobs.LoanDueHandler(env.Txm, env.Clock, env.Audit)
	if err := handler(ctx); err != nil {
		t.Fatal(err)
	}
	l, _ := env.Loan.Get(ctx, loan.ID)
	if !l.Overdue {
		t.Fatalf("借展应被标记逾期")
	}
	// 逾期查询
	overdue, err := env.Query.OverdueLoans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(overdue) != 1 || overdue[0].ID != loan.ID {
		t.Fatalf("逾期查询异常: %+v", overdue)
	}
}

// TestHandoverTimeoutJob 交接超时作业标记关注。
func TestHandoverTimeoutJob(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-T1")
	wh, sensor := env.SetupWarehouse("WH-T1")
	art := env.RegisterStoredArtifact("WJ-T1", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)
	loan := env.LoanOf("LN-T1", art.ID)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Check.OutCheck(ctx, loan.ID, "out-t1", "甲", env.CheckItemsAllPresent(art.ID),
		service.HandoverInput{FromPerson: "A", ToPerson: "B", HandedAt: env.Now() + 100}); err != nil {
		t.Fatal(err)
	}

	// 推进超过交接超时阈值（48h）
	env.Clock.Advance(49 * time.Hour)
	handler := jobs.HandoverTimeoutHandler(env.Txm, env.Clock, env.Audit, int64(env.Cfg.HandoverTimeout.Seconds()))
	if err := handler(ctx); err != nil {
		t.Fatal(err)
	}
	l, _ := env.Loan.Get(ctx, loan.ID)
	if !l.Attention {
		t.Fatalf("交接超时应标记关注")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
