package service_test

import (
	"context"
	"testing"

	"gowork/internal/service"
	"gowork/internal/testenv"
)

// setupInTransitLoan 构造一个已进入在途状态的借展。
func setupInTransitLoan(t *testing.T, env *testenv.Env, code string) (loanID, artifactID int64) {
	t.Helper()
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-" + code)
	wh, sensor := env.SetupWarehouse("WH-" + code)
	art := env.RegisterStoredArtifact("WJ-"+code, lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)
	loan := env.LoanOf("LN-"+code, art.ID)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Check.OutCheck(ctx, loan.ID, "out-"+code, "甲", env.CheckItemsAllPresent(art.ID),
		service.HandoverInput{FromPerson: "库管A", ToPerson: "押运B", HandedAt: env.Now() + 100, Location: "发货区"}); err != nil {
		t.Fatal(err)
	}
	return loan.ID, art.ID
}

// TestHandoverOrder 交接时间必须严格递增。
func TestHandoverOrder(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	loanID, _ := setupInTransitLoan(t, env, "H1")

	// 时间不大于上一段，拒绝
	_, _, err := env.Handover.AddHandover(ctx, loanID, "ho-x",
		service.HandoverInput{FromPerson: "押运B", ToPerson: "接收C", HandedAt: env.Now() + 100})
	testenv.MustErr(t, err, "严格递增")

	// 正常递增通过
	_, _, err = env.Handover.AddHandover(ctx, loanID, "ho-y",
		service.HandoverInput{FromPerson: "押运B", ToPerson: "接收C", HandedAt: env.Now() + 200})
	if err != nil {
		t.Fatalf("递增交接应通过: %v", err)
	}
}

// TestHandoverIdentityNoRepeat 交接人身份不能重复替代。
func TestHandoverIdentityNoRepeat(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	loanID, _ := setupInTransitLoan(t, env, "H2")

	// 接收方与上一段接收方重复（押运B 自己接回），拒绝
	_, _, err := env.Handover.AddHandover(ctx, loanID, "ho-r",
		service.HandoverInput{FromPerson: "押运B", ToPerson: "押运B", HandedAt: env.Now() + 200})
	testenv.MustErr(t, err, "重复替代")
}

// TestHandoverChainContinuity 交接链必须连续：交出方=上段接收方。
func TestHandoverChainContinuity(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	loanID, _ := setupInTransitLoan(t, env, "H3")

	_, _, err := env.Handover.AddHandover(ctx, loanID, "ho-c",
		service.HandoverInput{FromPerson: "陌生人X", ToPerson: "接收C", HandedAt: env.Now() + 200})
	testenv.MustErr(t, err, "交接链断裂")
}

// TestHandoverIdempotency 重复交接请求幂等键去重。
func TestHandoverIdempotency(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	loanID, _ := setupInTransitLoan(t, env, "H4")

	in := service.HandoverInput{FromPerson: "押运B", ToPerson: "接收C", HandedAt: env.Now() + 200, Location: "省博"}
	h1, replay1, err := env.Handover.AddHandover(ctx, loanID, "ho-idem", in)
	if err != nil || replay1 {
		t.Fatalf("首次交接应成功且非回放: %v %v", err, replay1)
	}
	h2, replay2, err := env.Handover.AddHandover(ctx, loanID, "ho-idem", in)
	if err != nil {
		t.Fatalf("重复交接应幂等返回: %v", err)
	}
	if !replay2 || h2.ID != h1.ID {
		t.Fatalf("幂等回放异常: %+v %+v", h1, h2)
	}
	list, err := env.Repo.Handovers.ListByLoan(ctx, loanID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 { // 首段 + 追加的一段
		t.Fatalf("幂等键不应产生重复交接，实际 %d 段", len(list))
	}
}

// TestOutCheckIdempotency 重复出库清点幂等返回且整体只生效一次。
func TestOutCheckIdempotency(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-I1")
	wh, sensor := env.SetupWarehouse("WH-I1")
	art := env.RegisterStoredArtifact("WJ-I1", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)
	loan := env.LoanOf("LN-I1", art.ID)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatal(err)
	}

	in := service.HandoverInput{FromPerson: "A", ToPerson: "B", HandedAt: env.Now() + 100}
	r1, err := env.Check.OutCheck(ctx, loan.ID, "out-idem", "甲", env.CheckItemsAllPresent(art.ID), in)
	if err != nil || r1.IdempotentReplay {
		t.Fatalf("首次清点应成功: %v", err)
	}
	r2, err := env.Check.OutCheck(ctx, loan.ID, "out-idem", "甲", env.CheckItemsAllPresent(art.ID), in)
	if err != nil {
		t.Fatalf("重复清点应幂等返回: %v", err)
	}
	if !r2.IdempotentReplay || r2.Check.ID != r1.Check.ID {
		t.Fatalf("幂等回放异常")
	}
	loan2, _ := env.Loan.Get(ctx, loan.ID)
	if loan2.Status != "in_transit" {
		t.Fatalf("借展状态异常 %s", loan2.Status)
	}
}
