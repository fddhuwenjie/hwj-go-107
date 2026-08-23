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

// setupReturnedLoan 构造一个已归还待验收的借展（含多件藏品），借展 returned、藏品 returned_pending。
func setupReturnedLoan(t *testing.T, env *testenv.Env, code string, n int) (loanID int64, artIDs []int64) {
	t.Helper()
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-" + code)
	wh, sensor := env.SetupWarehouse("WH-" + code)
	sc, scSensor := env.SetupShowcase("SC-" + code)
	for i := 0; i < n; i++ {
		artIDs = append(artIDs, env.RegisterStoredArtifact(fmt.Sprintf("WJ-%s%d", code, i), lv.ID, wh.ID).ID)
	}
	env.SeedWindowQualified(sensor.ID)
	env.SeedWindowQualified(scSensor.ID)
	loan := env.LoanOf("LN-"+code, artIDs...)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Check.OutCheck(ctx, loan.ID, "out-"+code, "甲", env.CheckItemsAllPresent(artIDs...),
		service.HandoverInput{FromPerson: "库管A", ToPerson: "押运B", HandedAt: env.Now() + 100, Location: "发货区"}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Handover.ConfirmExhibition(ctx, loan.ID, sc.ID, "C", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Check.InCheck(ctx, loan.ID, "in-"+code, "乙", env.CheckItemsAllPresent(artIDs...)); err != nil {
		t.Fatal(err)
	}
	return loan.ID, artIDs
}

// TestAcceptancePassWithNotesAtomicity pass_with_notes 验收须同成同败：
// 任一件藏品被其他流程改离 returned_pending，验收失败且不残留验收证据、藏品不恢复、借展不关闭。
func TestAcceptancePassWithNotesAtomicity(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	loanID, artIDs := setupReturnedLoan(t, env, "AW", 2)

	// 其他流程把其中一件藏品移出待验收（例如隔离），制造跨仓储事务边界冲突
	env.DB.Exec(`UPDATE artifacts SET status='isolated', version=version+1 WHERE id=?`, artIDs[0])

	_, err := env.Return.Accept(ctx, loanID, domain.AcceptPassWithNotes, "王五", "略有磨损")
	testenv.MustErr(t, err, "不在待验收")

	// 验收证据不应残留：详情查询看不到验收记录
	if _, err := env.Repo.Acceptances.AcceptanceByLoan(ctx, loanID); err == nil {
		t.Fatalf("验收失败后不应残留验收记录")
	}
	detail, derr := env.Query.LoanDetail(ctx, loanID)
	if derr != nil {
		t.Fatal(derr)
	}
	if detail.Acceptance != nil {
		t.Fatalf("借展详情不应包含验收记录，实际 %+v", detail.Acceptance)
	}
	// 借展应保持 returned（未关闭）
	if detail.Loan.Status != domain.LoanReturned {
		t.Fatalf("验收失败后借展应保持 returned，实际 %s", detail.Loan.Status)
	}
	// 藏品不应被恢复：被改乱的保持被改乱状态，另一件仍待验收
	a0, _ := env.Artifact.Get(ctx, artIDs[0])
	if a0.Status != domain.ArtifactIsolated {
		t.Fatalf("被改乱的藏品不应被恢复，实际 %s", a0.Status)
	}
	a1, _ := env.Artifact.Get(ctx, artIDs[1])
	if a1.Status != domain.ArtifactReturnedPending {
		t.Fatalf("另一件藏品应保持 returned_pending，实际 %s", a1.Status)
	}
	// 验收审计不应残留
	var auditCnt int
	env.DB.QueryRow(`SELECT COUNT(1) FROM audit_logs WHERE entity_type='loan' AND entity_id=? AND action='loan.return_acceptance'`, loanID).Scan(&auditCnt)
	if auditCnt != 0 {
		t.Fatalf("验收失败后不应残留验收审计")
	}

	// 复原藏品状态后，pass_with_notes 应能整体成功：验收、藏品恢复、借展关闭同成
	env.DB.Exec(`UPDATE artifacts SET status='returned_pending', version=version+1 WHERE id=?`, artIDs[0])
	acc, err := env.Return.Accept(ctx, loanID, domain.AcceptPassWithNotes, "王五", "完好")
	if err != nil {
		t.Fatalf("复原后 pass_with_notes 验收应成功: %v", err)
	}
	if acc.Result != domain.AcceptPassWithNotes {
		t.Fatalf("验收结果异常 %s", acc.Result)
	}
	loan, _ := env.Loan.Get(ctx, loanID)
	if loan.Status != domain.LoanClosed {
		t.Fatalf("验收成功后借展应 closed，实际 %s", loan.Status)
	}
	for _, id := range artIDs {
		a, _ := env.Artifact.Get(ctx, id)
		if a.Status != domain.ArtifactStored || a.StorageUnitID == nil {
			t.Fatalf("验收成功后藏品应恢复在库，实际 %s %+v", a.Status, a.StorageUnitID)
		}
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
