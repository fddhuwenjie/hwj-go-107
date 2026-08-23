package service_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

// TestRuleVersionNoSilentReplace 仓储层：同等级同版本号重复创建必须返回冲突，
// 绝不静默替换已存在的版本行（修复 INSERT OR REPLACE 的覆盖缺陷）。
func TestRuleVersionNoSilentReplace(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv, err := env.Env.CreateLevel(ctx, "LV-VR", "等级", "")
	if err != nil {
		t.Fatal(err)
	}
	// 先写入一条 version_no=2 的规则
	first := &domain.ThresholdRuleVersion{
		LevelID: lv.ID, VersionNo: 2, TempMin: 14, TempMax: 24,
		HumidityMin: 45, HumidityMax: 60, ConsecutiveBreach: 2,
		Status: domain.RuleDraft, CreatedAt: env.Now(),
	}
	if err := env.Repo.Rules.Create(ctx, first); err != nil {
		t.Fatalf("首次创建规则失败: %v", err)
	}
	originalID := first.ID

	// 重复写入同版本号：应冲突而非替换
	dup := &domain.ThresholdRuleVersion{
		LevelID: lv.ID, VersionNo: 2, TempMin: 15, TempMax: 25,
		HumidityMin: 40, HumidityMax: 65, ConsecutiveBreach: 3,
		Status: domain.RuleDraft, CreatedAt: env.Now(),
	}
	err = env.Repo.Rules.Create(ctx, dup)
	testenv.MustErr(t, err, "已存在")
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("重复版本号应返回冲突错误，实际: %v", err)
	}

	// 列表中仍只有原始那一条 version_no=2，且 ID 不变
	list, err := env.Repo.Rules.List(ctx, repository.RuleFilter{LevelID: &lv.ID}, domain.Page{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	count2 := 0
	for _, r := range list {
		if r.VersionNo == 2 {
			count2++
			if r.ID != originalID {
				t.Fatalf("version_no=2 行被替换：原 id=%d 现 id=%d", originalID, r.ID)
			}
		}
	}
	if count2 != 1 {
		t.Fatalf("version_no=2 应仅 1 条，实际 %d 条", count2)
	}
}

// TestCreateRuleConcurrentAtomic 同一保存等级并发创建下一版阈值规则：
// 两次调用必须各自成功且获得不同的版本号与 ID，列表最终保留两条第二、第三版，
// 较早创建的规则不被静默替换。
func TestCreateRuleConcurrentAtomic(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv, err := env.Env.CreateLevel(ctx, "LV-CC", "等级", "")
	if err != nil {
		t.Fatal(err)
	}
	// 初始已有第一版
	first, err := env.Env.CreateRule(ctx, lv.ID, 14, 24, 45, 60, 2)
	if err != nil {
		t.Fatalf("创建初始规则失败: %v", err)
	}
	if first.VersionNo != 1 {
		t.Fatalf("初始规则版本号应为 1，实际 %d", first.VersionNo)
	}

	const concurrency = 6
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []*domain.ThresholdRuleVersion
		errs    []error
	)
	wg.Add(concurrency)
	start := make(chan struct{})
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			<-start // 同时起跑，最大化并发竞争
			rv, err := env.Env.CreateRule(ctx, lv.ID, 15, 25, 40, 65, 3)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			results = append(results, rv)
		}()
	}
	close(start)
	wg.Wait()

	if len(errs) != 0 {
		t.Fatalf("并发创建不应有失败，错误: %v", errs)
	}
	if len(results) != concurrency {
		t.Fatalf("应有 %d 条并发创建成功，实际 %d", concurrency, len(results))
	}

	// 每次成功创建的版本号必须唯一，且无静默替换
	versionSet := map[int]bool{}
	idSet := map[int64]bool{}
	for _, rv := range results {
		if rv.VersionNo < 2 {
			t.Fatalf("并发创建的版本号应 >=2，实际 %d", rv.VersionNo)
		}
		if versionSet[rv.VersionNo] {
			t.Fatalf("出现重复版本号 %d（静默替换或并发泄漏）", rv.VersionNo)
		}
		versionSet[rv.VersionNo] = true
		if idSet[rv.ID] {
			t.Fatalf("出现重复规则 ID %d", rv.ID)
		}
		idSet[rv.ID] = true
	}
	// 版本号应为 2..(2+concurrency-1) 连续
	if len(versionSet) != concurrency {
		t.Fatalf("版本号集合大小 %d 与并发数 %d 不符", len(versionSet), concurrency)
	}
	for v := 2; v < 2+concurrency; v++ {
		if !versionSet[v] {
			t.Fatalf("缺少期望版本号 %d（版本号不连续）", v)
		}
	}

	// 列表最终应保留全部版本：第一版 + 2..(2+concurrency-1)，无静默替换
	list, err := env.Repo.Rules.List(ctx, repository.RuleFilter{LevelID: &lv.ID}, domain.Page{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1+concurrency {
		t.Fatalf("规则列表应保留 %d 条（初始1+并发%d），实际 %d 条（存在静默替换）",
			1+concurrency, concurrency, len(list))
	}
}
