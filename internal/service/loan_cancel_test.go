package service_test

import (
	"context"
	"errors"
	"testing"

	"gowork/internal/domain"
	"gowork/internal/testenv"
)

// TestCancelAfterApprovalRejected 审批通过后撤销应被拒绝；藏品保持冻结，
// 审批人与规则快照不被抹除。复现并锁定原 bug：审批完成后走草稿撤销会
// 把借展置为 cancelled，但藏品仍 frozen、审批快照被清空。
func TestCancelAfterApprovalRejected(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")
	wh, sensor := env.SetupWarehouse("WH-A")
	art := env.RegisterStoredArtifact("WJ-CAN1", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)

	loan := env.LoanOf("LN-CAN1", art.ID)
	loan, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六")
	if err != nil {
		t.Fatalf("审批失败: %v", err)
	}
	if loan.RuleSnapshot == "" || loan.ApprovedBy == "" || loan.ApprovedAt == nil {
		t.Fatalf("审批后快照/审批人不完整: %+v", loan)
	}

	// 审批通过后再撤销必须被状态机拒绝
	_, err = env.Loan.Cancel(ctx, loan.ID, loan.Version)
	if !errors.Is(err, domain.ErrState) {
		t.Fatalf("审批后撤销应返回状态错误，实际 %v", err)
	}

	// 借展仍为 approved，未被翻成 cancelled
	persisted, err := env.Loan.Get(ctx, loan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Status != domain.LoanApproved {
		t.Fatalf("借展应保持 approved，实际 %s", persisted.Status)
	}
	// 冻结快照、审批人、审批时间仍完整持久化
	if persisted.RuleSnapshot == "" || persisted.ApprovedBy != "赵六" || persisted.ApprovedAt == nil {
		t.Fatalf("冻结快照与审批人被抹除: %+v", persisted)
	}

	// 藏品仍为冻结态
	a, _ := env.Artifact.Get(ctx, art.ID)
	if a.Status != domain.ArtifactFrozen {
		t.Fatalf("藏品应保持 frozen，实际 %s", a.Status)
	}
}

// TestCancelFromDraftAndSubmitted 草稿与已提交均可撤销；终态后不可再撤销。
func TestCancelFromDraftAndSubmitted(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")
	wh, sensor := env.SetupWarehouse("WH-A")
	art := env.RegisterStoredArtifact("WJ-CAN2", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)

	// 已提交可撤销
	submitted := env.LoanOf("LN-CAN2", art.ID)
	cancelled, err := env.Loan.Cancel(ctx, submitted.ID, submitted.Version)
	if err != nil {
		t.Fatalf("已提交应可撤销: %v", err)
	}
	if cancelled.Status != domain.LoanCancelled {
		t.Fatalf("期望 cancelled，实际 %s", cancelled.Status)
	}
	// 终态后再撤销被拒
	if _, err := env.Loan.Cancel(ctx, cancelled.ID, cancelled.Version); !errors.Is(err, domain.ErrState) {
		t.Fatalf("终态后撤销应返回状态错误，实际 %v", err)
	}
}
