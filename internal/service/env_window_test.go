package service_test

import (
	"context"
	"testing"
	"time"

	"gowork/internal/testenv"
)

// TestEnvWindowNoSamples 前置窗口无采样时审批拒绝。
func TestEnvWindowNoSamples(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")
	wh, _ := env.SetupWarehouse("WH-A")
	art := env.RegisterStoredArtifact("WJ-W1", lv.ID, wh.ID)

	loan := env.LoanOf("LN-W1", art.ID)
	_, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六")
	testenv.MustErr(t, err, "无有效环境采样")
}

// TestEnvWindowBreach 窗口内存在越界采样时审批拒绝。
func TestEnvWindowBreach(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")
	wh, sensor := env.SetupWarehouse("WH-A")
	art := env.RegisterStoredArtifact("WJ-W2", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)

	// 窗口中部注入一次越界
	mid := env.Now() - int64(env.Cfg.PreLoanWindow.Seconds())/2
	if _, err := env.Env.IngestSample(ctx, sensor.ID, 30, 50, mid); err != nil {
		t.Fatal(err)
	}
	loan := env.LoanOf("LN-W2", art.ID)
	_, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六")
	testenv.MustErr(t, err, "越界采样")
}

// TestEnvWindowGap 采样间隔超过允许最大间隔时审批拒绝。
func TestEnvWindowGap(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")
	wh, sensor := env.SetupWarehouse("WH-A")
	art := env.RegisterStoredArtifact("WJ-W3", lv.ID, wh.ID)

	// 每 2 小时一条，超过 EnvMaxGap(1h)
	now := env.Now()
	from := now - int64(env.Cfg.PreLoanWindow.Seconds())
	env.SeedQualifiedSamples(sensor.ID, from, now, 7200)

	loan := env.LoanOf("LN-W3", art.ID)
	_, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六")
	testenv.MustErr(t, err, "采样间隔")
}

// TestEnvWindowTailCoverage 窗口末段缺少采样时审批拒绝。
func TestEnvWindowTailCoverage(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")
	wh, sensor := env.SetupWarehouse("WH-A")
	art := env.RegisterStoredArtifact("WJ-W4", lv.ID, wh.ID)

	// 末段 3 小时无采样（早于 now-3h 的最后采样）
	now := env.Now()
	from := now - int64(env.Cfg.PreLoanWindow.Seconds())
	env.SeedQualifiedSamples(sensor.ID, from, now-3*3600, 1800)

	loan := env.LoanOf("LN-W4", art.ID)
	_, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六")
	testenv.MustErr(t, err, "窗口末段")
}

// TestEnvWindowQualifiedOK 窗口连续合格时审批通过，时钟推进后窗口滑动需重新满足。
func TestEnvWindowQualifiedOK(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")
	wh, sensor := env.SetupWarehouse("WH-A")
	art := env.RegisterStoredArtifact("WJ-W5", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)

	loan := env.LoanOf("LN-W5", art.ID)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatalf("窗口合格应审批通过: %v", err)
	}

	// 时钟推进 2 小时且无新采样，新借展审批应因覆盖不足失败
	art2 := env.RegisterStoredArtifact("WJ-W6", lv.ID, wh.ID)
	env.Clock.Advance(2 * time.Hour)
	loan2 := env.LoanOf("LN-W6", art2.ID)
	_, err := env.Loan.Approve(ctx, loan2.ID, loan2.Version, "赵六")
	testenv.MustErr(t, err, "窗口")
}
