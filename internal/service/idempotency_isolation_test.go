package service_test

import (
	"context"
	"testing"

	"gowork/internal/domain"
	"gowork/internal/service"
	"gowork/internal/testenv"
)

// TestOutCheckIdempotencyKeyPrefixIsolation 两个并行借展用互为前缀的幂等键不应串台。
// 旧实现用 `idempotency_key LIKE ? || '%'` 做前缀匹配查询回放：当一个借展的幂等键是
// 另一个借展幂等键的前缀时，后发请求被误判为另一借展清点单的回放，返回错误借展的清点
// 明细与交接记录，且自身借展状态不推进。修复后幂等键精确匹配，不同借展隔离回放。
func TestOutCheckIdempotencyKeyPrefixIsolation(t *testing.T) {
	ctx := context.Background()

	// 两种前缀方向都要覆盖：先长后短、先短后长。
	cases := []struct {
		name string
		keyA string // 借展 A 使用的幂等键
		keyB string // 借展 B 使用的幂等键
	}{
		{"long_then_short_prefix", "out-key-extra", "out-key"}, // B 的键是 A 的键的前缀
		{"short_then_long_prefix", "out-key", "out-key-extra"}, // A 的键是 B 的键的前缀
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := testenv.New(t)
			lv := env.SetupLevelWithRule("LV-PFX-" + tc.name)
			wh, sensor := env.SetupWarehouse("WH-PFX-" + tc.name)
			artA := env.RegisterStoredArtifact("WJ-PFX-A-"+tc.name, lv.ID, wh.ID)
			artB := env.RegisterStoredArtifact("WJ-PFX-B-"+tc.name, lv.ID, wh.ID)
			env.SeedWindowQualified(sensor.ID)
			loanA := env.LoanOf("LN-PFX-A-"+tc.name, artA.ID)
			loanB := env.LoanOf("LN-PFX-B-"+tc.name, artB.ID)
			if _, err := env.Loan.Approve(ctx, loanA.ID, loanA.Version, "赵六"); err != nil {
				t.Fatal(err)
			}
			if _, err := env.Loan.Approve(ctx, loanB.ID, loanB.Version, "赵六"); err != nil {
				t.Fatal(err)
			}

			in := service.HandoverInput{FromPerson: "A", ToPerson: "B", HandedAt: env.Now() + 100}
			rA, err := env.Check.OutCheck(ctx, loanA.ID, tc.keyA, "甲", env.CheckItemsAllPresent(artA.ID), in)
			if err != nil || rA.IdempotentReplay {
				t.Fatalf("借展A首次清点应成功: %v replay=%v", err, rA.IdempotentReplay)
			}
			rB, err := env.Check.OutCheck(ctx, loanB.ID, tc.keyB, "乙", env.CheckItemsAllPresent(artB.ID), in)
			if err != nil || rB.IdempotentReplay {
				t.Fatalf("借展B首次清点应成功（不应被当成A的回放）: %v replay=%v", err, rB.IdempotentReplay)
			}

			// B 的清点必须归属借展B，不可复用借展A的清点单
			if rB.Check.LoanID != loanB.ID {
				t.Fatalf("借展B清点归属错误: 期望 loan=%d 实际 loan=%d（串到借展A）", loanB.ID, rB.Check.LoanID)
			}
			if rB.Check.ID == rA.Check.ID {
				t.Fatalf("借展B不应复用借展A的清点单")
			}
			// B 的借展状态必须推进到在途
			if loanB2, _ := env.Loan.Get(ctx, loanB.ID); loanB2.Status != domain.LoanInTransit {
				t.Fatalf("借展B状态应推进到 in_transit，实际 %s", loanB2.Status)
			}
			// 首段交接也应归属借展B
			if rB.Handover == nil || rB.Handover.LoanID != loanB.ID {
				t.Fatalf("借展B首段交接归属错误: %+v", rB.Handover)
			}
		})
	}
}

// TestOutCheckIdempotencyCrossLoanReplayRejected 跨借展复用同一幂等键应被拒绝，而非回放另一借展结果。
func TestOutCheckIdempotencyCrossLoanReplayRejected(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-CRS")
	wh, sensor := env.SetupWarehouse("WH-CRS")
	artA := env.RegisterStoredArtifact("WJ-CRS-A", lv.ID, wh.ID)
	artB := env.RegisterStoredArtifact("WJ-CRS-B", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)
	loanA := env.LoanOf("LN-CRS-A", artA.ID)
	loanB := env.LoanOf("LN-CRS-B", artB.ID)
	if _, err := env.Loan.Approve(ctx, loanA.ID, loanA.Version, "赵六"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Loan.Approve(ctx, loanB.ID, loanB.Version, "赵六"); err != nil {
		t.Fatal(err)
	}

	in := service.HandoverInput{FromPerson: "A", ToPerson: "B", HandedAt: env.Now() + 100}
	if _, err := env.Check.OutCheck(ctx, loanA.ID, "shared-key", "甲", env.CheckItemsAllPresent(artA.ID), in); err != nil {
		t.Fatal(err)
	}
	// 同一幂等键用在另一借展上：必须冲突，而不是把借展A的结果回放给借展B
	_, err := env.Check.OutCheck(ctx, loanB.ID, "shared-key", "乙", env.CheckItemsAllPresent(artB.ID), in)
	testenv.MustErr(t, err, "已用于借展")
	// 借展B状态不应推进
	if loanB2, _ := env.Loan.Get(ctx, loanB.ID); loanB2.Status != domain.LoanApproved {
		t.Fatalf("借展B状态不应推进，实际 %s", loanB2.Status)
	}
}

// TestInCheckIdempotencyCrossLoanReplayRejected 归还清点跨借展复用幂等键同样拒绝。
func TestInCheckIdempotencyCrossLoanReplayRejected(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-CRI")
	whA, sensorA := env.SetupWarehouse("WH-CRI-A")
	whB, sensorB := env.SetupWarehouse("WH-CRI-B")
	scA, scSensorA := env.SetupShowcase("SC-CRI-A")
	scB, scSensorB := env.SetupShowcase("SC-CRI-B")
	artA := env.RegisterStoredArtifact("WJ-CRI-A", lv.ID, whA.ID)
	artB := env.RegisterStoredArtifact("WJ-CRI-B", lv.ID, whB.ID)
	env.SeedWindowQualified(sensorA.ID)
	env.SeedWindowQualified(sensorB.ID)
	env.SeedWindowQualified(scSensorA.ID)
	env.SeedWindowQualified(scSensorB.ID)
	loanA := env.LoanOf("LN-CRI-A", artA.ID)
	loanB := env.LoanOf("LN-CRI-B", artB.ID)
	if _, err := env.Loan.Approve(ctx, loanA.ID, loanA.Version, "赵六"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Loan.Approve(ctx, loanB.ID, loanB.Version, "赵六"); err != nil {
		t.Fatal(err)
	}
	// 两条借展各自推进到 exhibiting
	handToExhibiting(t, env, ctx, loanA.ID, artA.ID, scA.ID, "hkA")
	handToExhibiting(t, env, ctx, loanB.ID, artB.ID, scB.ID, "hkB")

	if _, err := env.Check.InCheck(ctx, loanA.ID, "in-shared", "乙", env.CheckItemsAllPresent(artA.ID)); err != nil {
		t.Fatal(err)
	}
	// 跨借展复用归还清点幂等键：拒绝
	_, err := env.Check.InCheck(ctx, loanB.ID, "in-shared", "乙", env.CheckItemsAllPresent(artB.ID))
	testenv.MustErr(t, err, "已用于借展")
	if loanB2, _ := env.Loan.Get(ctx, loanB.ID); loanB2.Status != domain.LoanExhibiting {
		t.Fatalf("借展B状态不应推进，实际 %s", loanB2.Status)
	}
}

// TestHandoverIdempotencyCrossLoanReplayRejected 追加交接跨借展复用幂等键拒绝。
func TestHandoverIdempotencyCrossLoanReplayRejected(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	loanA, _ := setupInTransitLoan(t, env, "HCR-A")
	loanB, _ := setupInTransitLoan(t, env, "HCR-B")

	in := service.HandoverInput{FromPerson: "押运B", ToPerson: "接收C", HandedAt: env.Now() + 200, Location: "省博"}
	if _, _, err := env.Handover.AddHandover(ctx, loanA, "ho-shared", in); err != nil {
		t.Fatal(err)
	}
	// 同一幂等键用在借展B上：冲突，而非回放借展A的交接
	_, _, err := env.Handover.AddHandover(ctx, loanB, "ho-shared", in)
	testenv.MustErr(t, err, "已用于借展")
	// 借展B不应多出一条交接记录
	if list, _ := env.Repo.Handovers.ListByLoan(ctx, loanB); len(list) != 1 {
		t.Fatalf("借展B交接记录异常 %d", len(list))
	}
}

// handToExhibiting 将借展推进到 exhibiting 状态（出库清点+首段交接→展陈确认）。
func handToExhibiting(t *testing.T, env *testenv.Env, ctx context.Context, loanID, artID, showcaseID int64, key string) {
	t.Helper()
	in := service.HandoverInput{FromPerson: "库管A", ToPerson: "押运B", HandedAt: env.Now() + 100, Location: "发货区"}
	if _, err := env.Check.OutCheck(ctx, loanID, "out-"+key, "甲", env.CheckItemsAllPresent(artID), in); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Handover.ConfirmExhibition(ctx, loanID, showcaseID, "接收C", "恒温柜"); err != nil {
		t.Fatal(err)
	}
}
