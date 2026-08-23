package service_test

import (
	"context"
	"testing"

	"gowork/internal/domain"
	"gowork/internal/testenv"
)

// TestWarehouseRiskRankingWindowAndLate 风险排序统计窗口只覆盖最近 24 小时并排除迟到数据。
//
// 修复前两个缺陷：
//   - 统计窗口为 48 小时，把 30 小时前已结束的越界也计入“近一天”风险；
//   - 迟到补传采样因 received_at 较新被算入，导致已恢复库房持续高风险。
//
// 统计窗口必须只依据采样发生时间（sampled_at）覆盖最近 24 小时，并排除迟到数据。
func TestWarehouseRiskRankingWindowAndLate(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-W1") // 14-24℃ / 45-60%RH
	wh, sensor := env.SetupWarehouse("WH-W1")
	env.RegisterStoredArtifact("WJ-W1", lv.ID, wh.ID) // 单元有在库藏品+启用规则，越界才计入

	now := env.Now()

	// 1) 30 小时前的越界：超出最近 24 小时窗口，不应计入（旧 48h 窗口会误计入）。
	if _, err := env.Env.IngestSample(ctx, sensor.ID, 30, 70, now-30*3600); err != nil {
		t.Fatal(err)
	}

	// 2) 迟到补传采样：sampled_at 落在最近 24 小时内、received_at 较新，但 late=1，
	//    不应因接收时间较新而被计入（旧 received_at>=since 子句会误计入）。
	if err := env.Repo.Samples.Create(ctx, &domain.EnvSample{
		SensorID: sensor.ID, StorageUnitID: wh.ID,
		Temperature: 30, Humidity: 70,
		SampledAt: now - 3600, ReceivedAt: now, Late: true,
	}); err != nil {
		t.Fatal(err)
	}

	// 3) 对照：最近 30 分钟内的正常越界采样应计入 1 条，证明窗口仍在工作而非全盘排除。
	if _, err := env.Env.IngestSample(ctx, sensor.ID, 30, 70, now-1800); err != nil {
		t.Fatal(err)
	}

	ranking, err := env.Query.WarehouseRiskRanking(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range ranking {
		if r.UnitID != wh.ID {
			continue
		}
		if r.RecentBreaches != 1 {
			t.Fatalf("仅最近 1 条正常越界应计入（排除 30h 前与迟到补传），实际 recent_breaches=%d 行=%+v",
				r.RecentBreaches, r)
		}
		if r.RiskScore != 1 {
			t.Fatalf("无未关闭异常时风险分应等于越界数 1，实际 %d", r.RiskScore)
		}
		return
	}
	t.Fatalf("风险排序缺少库房 %d: %+v", wh.ID, ranking)
}
