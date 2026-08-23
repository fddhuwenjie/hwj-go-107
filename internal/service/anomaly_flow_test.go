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

// TestWarehouseRiskRankingMultipleAnomalies 风险榜派生字段一致性。
// 一个库房同时有两条未关闭 major 异常（加权分 3+3=6）和两次近期越界采样。
// 分项 severity_score 必须保持加权分（不得退化成异常条数），risk_score 必须等于
// severity_score + recent_breaches，且与 SQL 排序依据同公式，不得用减法。
func TestWarehouseRiskRankingMultipleAnomalies(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-M1")
	wh, sensor := env.SetupWarehouse("WH-M1")
	env.RegisterStoredArtifact("WJ-M1", lv.ID, wh.ID)
	rule, err := env.Repo.Rules.ActiveByLevel(ctx, lv.ID)
	if err != nil {
		t.Fatalf("取启用规则失败: %v", err)
	}

	now := env.Now()
	// 两次近期越界采样 → recent_breaches = 2，同时拿到合法的 sample_id。
	var sampleID int64
	for i := 0; i < 2; i++ {
		smp, err := env.Env.IngestSample(ctx, sensor.ID, 30, 70, now+int64(i)*60)
		if err != nil {
			t.Fatal(err)
		}
		sampleID = smp.ID
	}

	// 直接插入两条未关闭 major 异常，构造 OpenAnomalies > 1（巡检本身每单元只开一单）。
	for k := 0; k < 2; k++ {
		ev := &domain.AnomalyEvent{
			StorageUnitID: wh.ID, RuleVersionID: rule.ID, SampleID: sampleID,
			Severity: domain.SeverityMajor, Status: domain.AnomalyOpen, BreachCount: 3,
			Title: "major 异常", Version: 1, OpenedAt: now,
		}
		if err := env.Repo.Anomalies.Create(ctx, ev); err != nil {
			t.Fatalf("插入异常失败: %v", err)
		}
	}

	ranking, err := env.Query.WarehouseRiskRanking(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranking) != 1 {
		t.Fatalf("风险排序行数异常: %+v", ranking)
	}
	row := ranking[0]
	// 分项：两条 major → 加权分 3+3=6，不得退化成异常条数 2。
	if row.OpenAnomalies != 2 {
		t.Fatalf("open_anomalies 期望 2，实际 %d", row.OpenAnomalies)
	}
	if row.SeverityScore != 6 {
		t.Fatalf("severity_score 应保持加权分 6，被改成异常条数 %d", row.SeverityScore)
	}
	if row.RecentBreaches != 2 {
		t.Fatalf("recent_breaches 期望 2，实际 %d", row.RecentBreaches)
	}
	// 总分：加法而非减法，与 SQL 排序依据同公式。
	if row.RiskScore != row.SeverityScore+row.RecentBreaches {
		t.Fatalf("risk_score 应为 severity+recent=%d，实际 %d",
			row.SeverityScore+row.RecentBreaches, row.RiskScore)
	}
	if row.RiskScore != 8 {
		t.Fatalf("risk_score 期望 8（6+2），实际 %d", row.RiskScore)
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
