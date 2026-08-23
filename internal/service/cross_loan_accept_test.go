package service_test

import (
	"context"
	"testing"

	"gowork/internal/domain"
	"gowork/internal/service"
	"gowork/internal/testenv"
)

// TestAcceptBindsOwnCheck 验收必须绑定本借展清点单，禁止跨借展证据串联。
//
// 复现场景：两张借展单依次完成归还清点后，先验收第一张。
// 缺陷行为：归还验收查到的清点单是全局最近一条（第二张借展的清点），
// 形成跨借展证据串联。修复后，验收记录的 CheckID 必须等于第一张借展的归还清点单。
func TestAcceptBindsOwnCheck(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()

	lv := env.SetupLevelWithRule("LV-X1")
	wh, whSensor := env.SetupWarehouse("WH-X1")
	sc, scSensor := env.SetupShowcase("SC-X1")
	_ = sc

	env.SeedWindowQualified(whSensor.ID)
	env.SeedWindowQualified(scSensor.ID)

	// 两件在库藏品分属两张借展单
	art1 := env.RegisterStoredArtifact("WJ-X1", lv.ID, wh.ID)
	art2 := env.RegisterStoredArtifact("WJ-X2", lv.ID, wh.ID)

	loan1 := env.LoanOf("LN-X1", art1.ID)
	loan2 := env.LoanOf("LN-X2", art2.ID)

	for _, loan := range []*domain.LoanApplication{loan1, loan2} {
		var err error
		if loan.ID == loan1.ID {
			loan1, err = env.Loan.Approve(ctx, loan1.ID, loan1.Version, "赵六")
		} else {
			loan2, err = env.Loan.Approve(ctx, loan2.ID, loan2.Version, "赵六")
		}
		if err != nil {
			t.Fatalf("审批失败: %v", err)
		}
	}

	// 第一张借展：出库清点 + 首段交接 → 在途 → 展陈 → 上展
	if _, err := env.Check.OutCheck(ctx, loan1.ID, "out-x1", "甲",
		env.CheckItemsAllPresent(art1.ID),
		service.HandoverInput{FromPerson: "库管A", ToPerson: "押运B", HandedAt: env.Now() + 100, Location: "发货区"}); err != nil {
		t.Fatalf("loan1 出库清点失败: %v", err)
	}
	if _, err := env.Handover.ConfirmExhibition(ctx, loan1.ID, scSensor.StorageUnitID, "C", ""); err != nil {
		t.Fatalf("loan1 展陈确认失败: %v", err)
	}

	// 第二张借展：出库清点 + 首段交接 → 在途 → 展陈 → 上展
	if _, err := env.Check.OutCheck(ctx, loan2.ID, "out-x2", "甲",
		env.CheckItemsAllPresent(art2.ID),
		service.HandoverInput{FromPerson: "库管A", ToPerson: "押运B", HandedAt: env.Now() + 200, Location: "发货区"}); err != nil {
		t.Fatalf("loan2 出库清点失败: %v", err)
	}
	if _, err := env.Handover.ConfirmExhibition(ctx, loan2.ID, scSensor.StorageUnitID, "C", ""); err != nil {
		t.Fatalf("loan2 展陈确认失败: %v", err)
	}

	// 两张借展依次完成归还清点（loan1 先于 loan2）
	in1, err := env.Check.InCheck(ctx, loan1.ID, "in-x1", "乙", env.CheckItemsAllPresent(art1.ID))
	if err != nil {
		t.Fatalf("loan1 归还清点失败: %v", err)
	}
	in2, err := env.Check.InCheck(ctx, loan2.ID, "in-x2", "乙", env.CheckItemsAllPresent(art2.ID))
	if err != nil {
		t.Fatalf("loan2 归还清点失败: %v", err)
	}
	// in2 比 in1 更晚创建，全局 id 更大
	if in2.Check.ID <= in1.Check.ID {
		t.Fatalf("测试前提不满足：loan2 清点单 id(%d) 应大于 loan1(%d)", in2.Check.ID, in1.Check.ID)
	}

	// 先验收第一张借展
	acc, err := env.Return.Accept(ctx, loan1.ID, domain.AcceptPass, "王五", "完好")
	if err != nil {
		t.Fatalf("loan1 归还验收失败: %v", err)
	}

	// 关键断言：验收记录的 CheckID 必须绑定本借展（loan1）的归还清点单，
	// 而不是全局最近的 loan2 清点单。
	if acc.CheckID != in1.Check.ID {
		t.Fatalf("验收记录跨借展串联：acc.CheckID=%d，期望本借展清点 %d（错误引用了 loan2 的 %d）",
			acc.CheckID, in1.Check.ID, in2.Check.ID)
	}

	// 仓储层按借展+方向查询应只返回本借展清点，且与验收记录一致。
	got, err := env.Repo.Checks.ByLoanAndDirection(ctx, loan1.ID, domain.CheckIn)
	if err != nil {
		t.Fatalf("查询 loan1 归还清点失败: %v", err)
	}
	if got.ID != in1.Check.ID || got.LoanID != loan1.ID {
		t.Fatalf("ByLoanAndDirection 范围错误：got{ID:%d LoanID:%d}，期望{ID:%d LoanID:%d}",
			got.ID, got.LoanID, in1.Check.ID, loan1.ID)
	}

	// loan1 关闭，loan2 仍处于待验收（未被误关）
	l1, _ := env.Loan.Get(ctx, loan1.ID)
	if l1.Status != domain.LoanClosed {
		t.Fatalf("loan1 期望 closed，实际 %s", l1.Status)
	}
	l2, _ := env.Loan.Get(ctx, loan2.ID)
	if l2.Status != domain.LoanReturned {
		t.Fatalf("loan2 不应被误关，期望 returned，实际 %s", l2.Status)
	}
}
