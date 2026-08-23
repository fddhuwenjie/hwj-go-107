package service_test

import (
	"context"
	"testing"
	"time"

	"gowork/internal/domain"
	"gowork/internal/testenv"
)

// TestAnomalyFullFlow 异常全链：巡检生成→隔离→处置→复核通过→关闭→藏品恢复。
func TestAnomalyFullFlow(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-N1")
	wh, sensor := env.SetupWarehouse("WH-N1")
	art := env.RegisterStoredArtifact("WJ-N1", lv.ID, wh.ID)

	// 连续 3 次越界 → 严重
	now := env.Now()
	for i := 0; i < 3; i++ {
		if _, err := env.Env.IngestSample(ctx, sensor.ID, 28, 70, now+int64(i)*60); err != nil {
			t.Fatal(err)
		}
	}
	env.Clock.Advance(time.Hour)
	if _, err := env.Anomaly.Patrol(ctx); err != nil {
		t.Fatal(err)
	}
	open, err := env.Repo.Anomalies.OpenByUnit(ctx, wh.ID)
	if err != nil || len(open) != 1 {
		t.Fatalf("期望 1 个未关闭异常，实际 %d", len(open))
	}
	ev := open[0]
	if ev.Severity != domain.SeverityMajor || ev.BreachCount != 3 {
		t.Fatalf("分级异常: %+v", ev)
	}

	// 继续越界 → 升级为危急，不重复开单
	for i := 3; i < 6; i++ {
		if _, err := env.Env.IngestSample(ctx, sensor.ID, 28, 70, now+int64(i)*60); err != nil {
			t.Fatal(err)
		}
	}
	env.Clock.Advance(time.Hour)
	if _, err := env.Anomaly.Patrol(ctx); err != nil {
		t.Fatal(err)
	}
	open, _ = env.Repo.Anomalies.OpenByUnit(ctx, wh.ID)
	if len(open) != 1 {
		t.Fatalf("不应重复开单，实际 %d", len(open))
	}
	ev = open[0]
	if ev.Severity != domain.SeverityCritical || ev.BreachCount != 6 {
		t.Fatalf("升级异常: %+v", ev)
	}

	// 隔离
	action, err := env.Anomaly.Isolate(ctx, ev.ID, ev.Version, "张三", "isolate", "转应急柜")
	if err != nil {
		t.Fatalf("隔离失败: %v", err)
	}
	art, _ = env.Artifact.Get(ctx, art.ID)
	if art.Status != domain.ArtifactIsolated {
		t.Fatalf("隔离后藏品应为 isolated，实际 %s", art.Status)
	}

	// 处置
	d2, err := env.Anomaly.AddDisposal(ctx, ev.ID, "李四", "dehumidify", "除湿")
	if err != nil {
		t.Fatalf("处置失败: %v", err)
	}
	_ = action

	// 复核不通过 → 退回处置
	ev2, err := env.Anomaly.Review(ctx, d2.ID, "王五", false, "仍超标")
	if err != nil {
		t.Fatal(err)
	}
	if ev2.Status != domain.AnomalyDisposing {
		t.Fatalf("复核不通过应退回 disposing，实际 %s", ev2.Status)
	}

	// 重新处置并复核通过 → 关闭，藏品恢复
	d3, err := env.Anomaly.AddDisposal(ctx, ev.ID, "李四", "transfer", "转移完成")
	if err != nil {
		t.Fatal(err)
	}
	ev3, err := env.Anomaly.Review(ctx, d3.ID, "王五", true, "复检合格")
	if err != nil {
		t.Fatal(err)
	}
	if ev3.Status != domain.AnomalyClosed || ev3.ClosedAt == nil {
		t.Fatalf("复核通过应关闭，实际 %s", ev3.Status)
	}
	art, _ = env.Artifact.Get(ctx, art.ID)
	if art.Status != domain.ArtifactStored {
		t.Fatalf("复核通过后藏品应恢复 stored，实际 %s", art.Status)
	}

	// 处置记录链完整
	actions, err := env.Anomaly.ListDisposals(ctx, ev.ID)
	if err != nil || len(actions) != 3 {
		t.Fatalf("处置记录应为 3 条，实际 %d", len(actions))
	}
}

// TestUpcomingWithAnomalies 临近借展仍有环境异常的藏品查询。
func TestUpcomingWithAnomalies(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-U1")
	wh, sensor := env.SetupWarehouse("WH-U1")
	art := env.RegisterStoredArtifact("WJ-U1", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)

	loan := env.LoanOf("LN-U1", art.ID)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatal(err)
	}
	// 产生未关闭异常
	now := env.Now()
	for i := 0; i < 2; i++ {
		if _, err := env.Env.IngestSample(ctx, sensor.ID, 30, 70, now+int64(i)*60); err != nil {
			t.Fatal(err)
		}
	}
	env.Clock.Advance(time.Hour)
	if _, err := env.Anomaly.Patrol(ctx); err != nil {
		t.Fatal(err)
	}

	rows, err := env.Query.UpcomingWithAnomalies(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ArtifactID != art.ID || rows[0].OpenAnomalies == 0 {
		t.Fatalf("临近借展异常查询异常: %+v", rows)
	}

	// days 窗口之外不出现
	rows2, err := env.Query.UpcomingWithAnomalies(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows2) != 0 {
		t.Fatalf("3 天窗口内不应出现，实际 %+v", rows2)
	}
}

// TestWarehouseRiskRanking 库房风险排序。
func TestWarehouseRiskRanking(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-K1")
	wh1, s1 := env.SetupWarehouse("WH-K1")
	wh2, s2 := env.SetupWarehouse("WH-K2")
	env.RegisterStoredArtifact("WJ-K1", lv.ID, wh1.ID)
	env.RegisterStoredArtifact("WJ-K2", lv.ID, wh2.ID)

	// WH-K2 连续越界更多 → 风险更高
	now := env.Now()
	for i := 0; i < 2; i++ {
		env.Env.IngestSample(ctx, s1.ID, 30, 70, now+int64(i)*60)
	}
	for i := 0; i < 5; i++ {
		env.Env.IngestSample(ctx, s2.ID, 30, 70, now+int64(i)*60)
	}
	env.Clock.Advance(time.Hour)
	if _, err := env.Anomaly.Patrol(ctx); err != nil {
		t.Fatal(err)
	}

	ranking, err := env.Query.WarehouseRiskRanking(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranking) < 2 {
		t.Fatalf("风险排序行数异常: %+v", ranking)
	}
	if ranking[0].UnitID != wh2.ID || ranking[0].RiskScore <= ranking[1].RiskScore {
		t.Fatalf("风险排序错误: %+v", ranking)
	}
}

// TestConsecutiveBreachSensors 传感器连续越界查询。
func TestConsecutiveBreachSensors(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-S1")
	_, s1 := env.SetupWarehouse("WH-S1")
	wh2, s2 := env.SetupWarehouse("WH-S2")
	env.RegisterStoredArtifact("WJ-S1", lv.ID, wh2.ID)

	now := env.Now()
	// s1 所在单元无藏品，不参与
	for i := 0; i < 4; i++ {
		env.Env.IngestSample(ctx, s1.ID, 30, 70, now+int64(i)*60)
	}
	// s2 连续 3 次越界
	for i := 0; i < 3; i++ {
		env.Env.IngestSample(ctx, s2.ID, 30, 70, now+int64(i)*60)
	}

	rows, err := env.Query.ConsecutiveBreachSensors(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Sensor.ID != s2.ID || rows[0].Consecutive != 3 {
		t.Fatalf("连续越界查询异常: %+v", rows)
	}
}
