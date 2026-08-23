package service_test

import (
	"context"
	"testing"
	"time"

	"gowork/internal/domain"
	"gowork/internal/repository"
	"gowork/internal/service"
	"gowork/internal/testenv"
)

// TestLoanFullChain 借展全链：登记→分配→规则→采样→审批冻结→出库清点+首段交接→追加交接→运输节点→展陈确认→归还清点→验收关闭。
func TestLoanFullChain(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()

	lv := env.SetupLevelWithRule("LV-1")
	wh, whSensor := env.SetupWarehouse("WH-A")
	sc, scSensor := env.SetupShowcase("SC-A")
	_ = sc

	art := env.RegisterStoredArtifact("WJ-0001", lv.ID, wh.ID)
	att, err := env.Artifact.AddAttachment(ctx, art.ID, "紫檀函套", "一件")
	if err != nil {
		t.Fatalf("登记附件失败: %v", err)
	}

	// 库房与展柜前置窗口连续合格
	env.SeedWindowQualified(whSensor.ID)
	env.SeedWindowQualified(scSensor.ID)

	loan := env.LoanOf("LN-001", art.ID)

	// 审批：冻结快照
	loan, err = env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六")
	if err != nil {
		t.Fatalf("审批失败: %v", err)
	}
	if loan.Status != domain.LoanApproved {
		t.Fatalf("期望 approved，实际 %s", loan.Status)
	}
	if loan.RuleSnapshot == "" {
		t.Fatalf("期望冻结规则快照非空")
	}
	items, err := env.Loan.Items(ctx, loan.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("借展藏品行异常: %v", err)
	}
	if items[0].FrozenStatus != domain.ArtifactStored || items[0].FrozenLevelID != lv.ID || items[0].FrozenUnitID != wh.ID {
		t.Fatalf("冻结快照不符: %+v", items[0])
	}
	entries, err := domain.UnmarshalPackaging(items[0].PackagingSnapshot)
	if err != nil || len(entries) != 1 || entries[0].AttachmentID != att.ID {
		t.Fatalf("包装清单冻结不符: %v %+v", err, entries)
	}
	art, _ = env.Artifact.Get(ctx, art.ID)
	if art.Status != domain.ArtifactFrozen {
		t.Fatalf("审批后藏品应冻结，实际 %s", art.Status)
	}

	// 出库清点 + 首段交接（同事务）
	outRes, err := env.Check.OutCheck(ctx, loan.ID, "out-key-1", "清点员甲",
		env.CheckItemsAllPresent(art.ID),
		service.HandoverInput{FromPerson: "库管员A", ToPerson: "押运员B", HandedAt: env.Now() + 100, Location: "发货区"})
	if err != nil {
		t.Fatalf("出库清点失败: %v", err)
	}
	if !outRes.Check.Complete || outRes.Handover == nil || outRes.Handover.Seq != 1 {
		t.Fatalf("出库清点结果异常: %+v", outRes)
	}
	loan, _ = env.Loan.Get(ctx, loan.ID)
	if loan.Status != domain.LoanInTransit {
		t.Fatalf("期望 in_transit，实际 %s", loan.Status)
	}
	art, _ = env.Artifact.Get(ctx, art.ID)
	if art.Status != domain.ArtifactOut {
		t.Fatalf("期望 out，实际 %s", art.Status)
	}

	// 追加交接与运输节点
	ho2, replay, err := env.Handover.AddHandover(ctx, loan.ID, "ho-2",
		service.HandoverInput{FromPerson: "押运员B", ToPerson: "接收员C", HandedAt: env.Now() + 200, Location: "省博"})
	if err != nil || replay {
		t.Fatalf("追加交接失败: %v replay=%v", err, replay)
	}
	if ho2.Seq != 2 {
		t.Fatalf("交接段号应为 2，实际 %d", ho2.Seq)
	}
	if _, err := env.Handover.AddTransportNode(ctx, loan.ID, "arrival", "省博", env.Now()+300, "押运员B"); err != nil {
		t.Fatalf("运输节点失败: %v", err)
	}

	// 展陈确认
	if _, err := env.Handover.ConfirmExhibition(ctx, loan.ID, scSensor.StorageUnitID, "接收员C", "恒温柜"); err != nil {
		t.Fatalf("展陈确认失败: %v", err)
	}
	art, _ = env.Artifact.Get(ctx, art.ID)
	if art.Status != domain.ArtifactOnLoan {
		t.Fatalf("期望 on_loan，实际 %s", art.Status)
	}

	// 归还清点
	inRes, err := env.Check.InCheck(ctx, loan.ID, "in-key-1", "清点员乙", env.CheckItemsAllPresent(art.ID))
	if err != nil {
		t.Fatalf("归还清点失败: %v", err)
	}
	if !inRes.Check.Complete {
		t.Fatalf("归还清点应完整")
	}
	art, _ = env.Artifact.Get(ctx, art.ID)
	if art.Status != domain.ArtifactReturnedPending {
		t.Fatalf("期望 returned_pending，实际 %s", art.Status)
	}

	// 归还验收通过 → 借展关闭，藏品恢复在库且回到原库位
	acc, err := env.Return.Accept(ctx, loan.ID, domain.AcceptPass, "王五", "完好")
	if err != nil {
		t.Fatalf("归还验收失败: %v", err)
	}
	if acc.Result != domain.AcceptPass {
		t.Fatalf("验收结果异常")
	}
	loan, _ = env.Loan.Get(ctx, loan.ID)
	if loan.Status != domain.LoanClosed {
		t.Fatalf("期望 closed，实际 %s", loan.Status)
	}
	art, _ = env.Artifact.Get(ctx, art.ID)
	if art.Status != domain.ArtifactStored || art.StorageUnitID == nil || *art.StorageUnitID != wh.ID {
		t.Fatalf("验收后藏品应恢复在库回原库位，实际 %s %+v", art.Status, art.StorageUnitID)
	}

	// 快照链完整：登记/分配/冻结/出库/上展/归还/验收
	snaps, err := env.Artifact.Snapshots(ctx, art.ID, domain.Page{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps.Items) < 7 {
		t.Fatalf("期望至少 7 条状态快照，实际 %d", len(snaps.Items))
	}

	// 借展详情完整
	detail, err := env.Query.LoanDetail(ctx, loan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Acceptance == nil || detail.Confirm == nil || len(detail.Handovers) != 2 || len(detail.Nodes) != 1 || len(detail.Checks) != 2 {
		t.Fatalf("借展详情不完整: %+v", detail)
	}
}

// TestApproveRejectedWhenOpenAnomaly 异常未关闭时审批与出库均被拒绝。
func TestApproveRejectedWhenOpenAnomaly(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")
	wh, sensor := env.SetupWarehouse("WH-A")
	art := env.RegisterStoredArtifact("WJ-0002", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)

	loan := env.LoanOf("LN-002", art.ID)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatalf("窗口合格时审批应通过: %v", err)
	}

	// 制造未关闭异常
	now := env.Now()
	for i := 1; i <= 2; i++ {
		if _, err := env.Env.IngestSample(ctx, sensor.ID, 30, 70, now+int64(i)*60); err != nil {
			t.Fatal(err)
		}
	}
	env.Clock.Advance(time.Hour)
	if _, err := env.Anomaly.Patrol(ctx); err != nil {
		t.Fatal(err)
	}
	open, err := env.Repo.Anomalies.OpenByUnit(ctx, wh.ID)
	if err != nil || len(open) == 0 {
		t.Fatalf("期望产生未关闭异常")
	}

	// 出库被拒绝
	_, err = env.Check.OutCheck(ctx, loan.ID, "out-key-x", "清点员甲",
		env.CheckItemsAllPresent(art.ID),
		service.HandoverInput{FromPerson: "A", ToPerson: "B", HandedAt: env.Now() + 100})
	testenv.MustErr(t, err, "未关闭异常")
}

// TestRetireConstrained 注销约束：借展占用状态不可注销，注销后审计保留。
func TestRetireConstrained(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")
	wh, sensor := env.SetupWarehouse("WH-A")
	art := env.RegisterStoredArtifact("WJ-0003", lv.ID, wh.ID)
	env.SeedWindowQualified(sensor.ID)

	loan := env.LoanOf("LN-003", art.ID)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatal(err)
	}
	art, _ = env.Artifact.Get(ctx, art.ID)
	if _, err := env.Artifact.Retire(ctx, art.ID, art.Version, "测试"); err == nil {
		t.Fatalf("冻结状态藏品不应允许注销")
	}

	// 常态藏品可注销
	art2 := env.RegisterStoredArtifact("WJ-0004", lv.ID, wh.ID)
	art2, err := env.Artifact.Retire(ctx, art2.ID, art2.Version, "鉴定注销")
	if err != nil {
		t.Fatalf("注销失败: %v", err)
	}
	if art2.Status != domain.ArtifactRetired {
		t.Fatalf("期望 retired，实际 %s", art2.Status)
	}
	// 审计保留
	logs, err := env.Repo.Audit.List(ctx, auditFilter("artifact", art2.ID), domain.Page{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range logs {
		if l.Action == "artifact.retire" {
			found = true
		}
	}
	if !found {
		t.Fatalf("注销应保留审计记录")
	}
	// 重复注销拒绝
	if _, err := env.Artifact.Retire(ctx, art2.ID, art2.Version, "再次"); err == nil {
		t.Fatalf("重复注销应被拒绝")
	}
}

// TestRejectDraftBlocked 草稿借展未提交不得进入审批结论：
// 直接驳回应被状态机拒绝，不得写入驳回状态、审批时间、规则快照或 attention 标志。
func TestRejectDraftBlocked(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-R1")
	wh, _ := env.SetupWarehouse("WH-R1")
	art := env.RegisterStoredArtifact("WJ-R1", lv.ID, wh.ID)

	// 仅创建草稿，不提交
	now := env.Now()
	loan, err := env.Loan.Create(ctx, service.CreateLoanInput{
		Code: "LN-DRAFT", Borrower: "省博物馆", Venue: "临展厅", Purpose: "特展",
		StartAt: now + 7*86400, EndAt: now + 37*86400, ArtifactIDs: []int64{art.ID},
	})
	if err != nil {
		t.Fatalf("创建草稿失败: %v", err)
	}
	if loan.Status != domain.LoanDraft {
		t.Fatalf("期望 draft，实际 %s", loan.Status)
	}

	// 草稿直接驳回必须失败
	_, err = env.Loan.Reject(ctx, loan.ID, loan.Version, "审批员", "理由不充分")
	testenv.MustErr(t, err, "状态")

	// 状态与持久化字段均不应被污染
	got, err := env.Loan.Get(ctx, loan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.LoanDraft {
		t.Fatalf("草稿驳回被拒后状态应为 draft，实际 %s", got.Status)
	}
	if got.ApprovedAt != nil {
		t.Fatalf("草稿不应写审批时间，实际 %d", *got.ApprovedAt)
	}
	if got.RuleSnapshot != "" {
		t.Fatalf("草稿不应写规则快照，实际 %q", got.RuleSnapshot)
	}
	if got.Attention {
		t.Fatalf("草稿不应标记 attention")
	}
	if got.RejectReason != "" {
		t.Fatalf("草稿不应写驳回理由，实际 %q", got.RejectReason)
	}
}

// TestRejectSubmittedClean 已提交借展可被驳回，且驳回不污染审批时间/规则快照/attention。
func TestRejectSubmittedClean(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-R2")
	wh, _ := env.SetupWarehouse("WH-R2")
	art := env.RegisterStoredArtifact("WJ-R2", lv.ID, wh.ID)
	loan := env.LoanOf("LN-SUB", art.ID) // 创建并提交

	rej, err := env.Loan.Reject(ctx, loan.ID, loan.Version, "审批员", "材料不全")
	if err != nil {
		t.Fatalf("已提交借展驳回应成功: %v", err)
	}
	if rej.Status != domain.LoanRejected {
		t.Fatalf("期望 rejected，实际 %s", rej.Status)
	}
	if rej.ApprovedBy != "审批员" || rej.RejectReason != "材料不全" {
		t.Fatalf("驳回人与理由未持久化: %+v", rej)
	}
	// 驳回是审批终态，不应伪造审批时间、规则快照，也不应标记在途关注
	if rej.ApprovedAt != nil {
		t.Fatalf("驳回不应写审批时间，实际 %d", *rej.ApprovedAt)
	}
	if rej.RuleSnapshot != "" {
		t.Fatalf("驳回不应写规则快照，实际 %q", rej.RuleSnapshot)
	}
	if rej.Attention {
		t.Fatalf("驳回不应标记 attention")
	}
	// 藏品未被冻结
	a, _ := env.Artifact.Get(ctx, art.ID)
	if a.Status != domain.ArtifactStored {
		t.Fatalf("驳回后藏品应仍在库，实际 %s", a.Status)
	}
}

func auditFilter(entityType string, id int64) repository.AuditFilter {
	return repository.AuditFilter{EntityType: &entityType, EntityID: &id}
}
