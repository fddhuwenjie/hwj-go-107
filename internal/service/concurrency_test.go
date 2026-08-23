package service_test

import (
	"context"
	"fmt"
	"testing"

	"gowork/internal/domain"
	"gowork/internal/repository"
	"gowork/internal/service"
	"gowork/internal/testenv"
)

// TestOptimisticLockArtifact 藏品更新乐观锁：旧版本拒绝。
func TestOptimisticLockArtifact(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-O1")
	wh, _ := env.SetupWarehouse("WH-O1")
	art := env.RegisterStoredArtifact("WJ-O1", lv.ID, wh.ID)

	stale := art.Version
	// 第一次修改成功
	if _, err := env.Artifact.Update(ctx, art.ID, service.UpdateInput{
		Name: "新名称", Category: art.Category, Era: art.Era, Version: stale,
	}); err != nil {
		t.Fatalf("首次修改应成功: %v", err)
	}
	// 旧版本再改，冲突
	_, err := env.Artifact.Update(ctx, art.ID, service.UpdateInput{
		Name: "再次", Category: art.Category, Era: art.Era, Version: stale,
	})
	testenv.MustErr(t, err, "版本冲突")
}

// TestOptimisticLockLoanApprove 借展审批乐观锁。
func TestOptimisticLockLoanApprove(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-O2")
	wh, sensor := env.SetupWarehouse("WH-O2")
	art := env.RegisterStoredArtifact("WJ-O2", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)
	loan := env.LoanOf("LN-O2", art.ID)

	_, err := env.Loan.Approve(ctx, loan.ID, loan.Version+99, "赵六")
	testenv.MustErr(t, err, "版本冲突")
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatalf("正确版本应通过: %v", err)
	}
}

// TestOutCheckRollback 出库清点中途失败整体回滚：借展与藏品状态不变、无清点残留。
func TestOutCheckRollback(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-R1")
	wh, sensor := env.SetupWarehouse("WH-R1")
	art1 := env.RegisterStoredArtifact("WJ-R1", lv.ID, wh.ID)
	art2 := env.RegisterStoredArtifact("WJ-R2", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)
	loan := env.LoanOf("LN-R1", art1.ID, art2.ID)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatal(err)
	}

	// 第二件藏品缺失 → 校验失败，事务回滚
	items := env.CheckItemsAllPresent(art1.ID, art2.ID)
	items[1].Present = false
	_, err := env.Check.OutCheck(ctx, loan.ID, "out-rb", "甲", items,
		service.HandoverInput{FromPerson: "A", ToPerson: "B", HandedAt: env.Now() + 100})
	testenv.MustErr(t, err, "禁止出库")

	loan2, _ := env.Loan.Get(ctx, loan.ID)
	if loan2.Status != "approved" {
		t.Fatalf("回滚后借展应保持 approved，实际 %s", loan2.Status)
	}
	for _, id := range []int64{art1.ID, art2.ID} {
		a, _ := env.Artifact.Get(ctx, id)
		if a.Status != "frozen" {
			t.Fatalf("回滚后藏品应保持 frozen，实际 %s", a.Status)
		}
	}
	if _, err := env.Repo.Checks.ByLoanAndDirection(ctx, loan.ID, "out"); err == nil {
		t.Fatalf("回滚后不应存在清点单")
	}
	if list, _ := env.Repo.Handovers.ListByLoan(ctx, loan.ID); len(list) != 0 {
		t.Fatalf("回滚后不应存在交接记录")
	}
	var auditCnt int
	env.DB.QueryRow(`SELECT COUNT(1) FROM audit_logs WHERE entity_type='loan' AND entity_id=? AND action='loan.out_check'`, loan.ID).Scan(&auditCnt)
	if auditCnt != 0 {
		t.Fatalf("回滚后不应有出库审计")
	}
}

// TestAcceptanceRollback 归还验收中途失败整体回滚。
func TestAcceptanceRollback(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	loanID, artID := setupInTransitLoan(t, env, "R2")
	_, scSensor := env.SetupShowcase("SC-R2")
	env.SeedWindowQualified(scSensor.ID)
	if _, err := env.Handover.ConfirmExhibition(ctx, loanID, scSensor.StorageUnitID, "C", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Check.InCheck(ctx, loanID, "in-r2", "乙", env.CheckItemsAllPresent(artID)); err != nil {
		t.Fatal(err)
	}
	// 手工把藏品状态改乱，触发验收中的状态校验失败
	env.DB.Exec(`UPDATE artifacts SET status='stored', version=version+1 WHERE id=?`, artID)

	_, err := env.Return.Accept(ctx, loanID, "pass", "王五", "")
	testenv.MustErr(t, err, "不在待验收")

	loan, _ := env.Loan.Get(ctx, loanID)
	if loan.Status != "returned" {
		t.Fatalf("回滚后借展应保持 returned，实际 %s", loan.Status)
	}
	if _, err := env.Repo.Acceptances.AcceptanceByLoan(ctx, loanID); err == nil {
		t.Fatalf("回滚后不应存在验收记录")
	}
}

// TestStablePagination 键集分页稳定：多页无重复无遗漏，顺序按主键。
func TestStablePagination(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-P1")
	wh, _ := env.SetupWarehouse("WH-P1")
	for i := 0; i < 25; i++ {
		env.RegisterStoredArtifact(fmt.Sprintf("WJ-P%02d", i), lv.ID, wh.ID)
	}

	seen := map[int64]bool{}
	cursor := int64(0)
	pages := 0
	for {
		out, err := env.Artifact.List(ctx, repository.ArtifactFilter{}, domain.Page{Cursor: cursor, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		pages++
		prev := cursor
		for _, a := range out.Items {
			if seen[a.ID] {
				t.Fatalf("分页出现重复 id=%d", a.ID)
			}
			if a.ID <= prev {
				t.Fatalf("分页顺序不稳定 id=%d prev=%d", a.ID, prev)
			}
			seen[a.ID] = true
			prev = a.ID
		}
		if out.NextCursor == 0 {
			break
		}
		cursor = out.NextCursor
		if pages > 10 {
			t.Fatalf("分页未收敛")
		}
	}
	if len(seen) != 25 {
		t.Fatalf("分页覆盖不全：%d/25", len(seen))
	}
	if pages != 3 { // 10+10+5
		t.Fatalf("页数异常 %d", pages)
	}
}
