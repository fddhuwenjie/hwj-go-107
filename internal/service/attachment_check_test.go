package service_test

import (
	"context"
	"testing"
	"time"

	"gowork/internal/domain"
	"gowork/internal/service"
	"gowork/internal/testenv"
)

// TestAttachmentCoverage 出库清点必须覆盖全部藏品与附件，缺一拒绝。
func TestAttachmentCoverage(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")
	wh, sensor := env.SetupWarehouse("WH-A")
	art := env.RegisterStoredArtifact("WJ-A1", lv.ID, wh.ID)
	att, err := env.Artifact.AddAttachment(ctx, art.ID, "函套", "一件")
	if err != nil {
		t.Fatal(err)
	}
	env.SeedWindowQualified(sensor.ID)
	loan := env.LoanOf("LN-A1", art.ID)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatal(err)
	}

	// 缺少附件行
	bad := []service.CheckItemInput{{ArtifactID: art.ID, Present: true}}
	_, err = env.Check.OutCheck(ctx, loan.ID, "k1", "甲", bad,
		service.HandoverInput{FromPerson: "A", ToPerson: "B", HandedAt: env.Now() + 100})
	testenv.MustErr(t, err, "未覆盖藏品")

	// 附件不在场，出库拒绝
	bad2 := []service.CheckItemInput{{ArtifactID: art.ID, Present: true,
		Attachments: []service.AttachmentPresent{{AttachmentID: att.ID, Present: false}}}}
	_, err = env.Check.OutCheck(ctx, loan.ID, "k2", "甲", bad2,
		service.HandoverInput{FromPerson: "A", ToPerson: "B", HandedAt: env.Now() + 100})
	testenv.MustErr(t, err, "禁止出库")

	// 完整清点通过
	_, err = env.Check.OutCheck(ctx, loan.ID, "k3", "甲",
		env.CheckItemsAllPresent(art.ID),
		service.HandoverInput{FromPerson: "A", ToPerson: "B", HandedAt: env.Now() + 100})
	if err != nil {
		t.Fatalf("完整清点应通过: %v", err)
	}
}

// TestPackagingDiff 归还清点缺失附件时包装清单差异可查询。
func TestPackagingDiff(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")
	wh, whSensor := env.SetupWarehouse("WH-A")
	_, scSensor := env.SetupShowcase("SC-A")
	art := env.RegisterStoredArtifact("WJ-A2", lv.ID, wh.ID)
	att, _ := env.Artifact.AddAttachment(ctx, art.ID, "书签", "一枚")
	env.SeedWindowQualified(whSensor.ID)
	env.SeedWindowQualified(scSensor.ID)

	loan := env.LoanOf("LN-A2", art.ID)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Check.OutCheck(ctx, loan.ID, "out-k", "甲", env.CheckItemsAllPresent(art.ID),
		service.HandoverInput{FromPerson: "A", ToPerson: "B", HandedAt: env.Now() + 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Handover.ConfirmExhibition(ctx, loan.ID, scSensor.StorageUnitID, "C", ""); err != nil {
		t.Fatal(err)
	}

	// 归还清点：附件缺失
	items := []service.CheckItemInput{{ArtifactID: art.ID, Present: true,
		Attachments: []service.AttachmentPresent{{AttachmentID: att.ID, Present: false}}}}
	res, err := env.Check.InCheck(ctx, loan.ID, "in-k", "乙", items)
	if err != nil {
		t.Fatal(err)
	}
	if res.Check.Complete {
		t.Fatalf("附件缺失时清点名不应完整")
	}

	diff, err := env.Query.PackagingDiff(ctx, loan.ID, domain.CheckIn)
	if err != nil {
		t.Fatal(err)
	}
	var missing int
	for _, d := range diff {
		if d.Diff == "missing_attachment" && d.AttachmentID == att.ID {
			missing++
		}
	}
	if missing != 1 {
		t.Fatalf("期望 1 条附件缺失差异，实际 %d: %+v", missing, diff)
	}

	// 验收不通过 → 藏品隔离
	if _, err := env.Return.Accept(ctx, loan.ID, domain.AcceptRejected, "王五", "附件遗失"); err != nil {
		t.Fatal(err)
	}
	art, _ = env.Artifact.Get(ctx, art.ID)
	if art.Status != domain.ArtifactIsolated {
		t.Fatalf("验收不通过应隔离，实际 %s", art.Status)
	}
}

// TestLateSampleNotOverrideAcceptance 迟到环境数据不覆盖已完成的借展验收。
func TestLateSampleNotOverrideAcceptance(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")
	wh, whSensor := env.SetupWarehouse("WH-A")
	_, scSensor := env.SetupShowcase("SC-A")
	art := env.RegisterStoredArtifact("WJ-L1", lv.ID, wh.ID)
	env.SeedWindowQualified(whSensor.ID)
	env.SeedWindowQualified(scSensor.ID)

	loan := env.LoanOf("LN-L1", art.ID)
	if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "赵六"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Check.OutCheck(ctx, loan.ID, "out-k", "甲", env.CheckItemsAllPresent(art.ID),
		service.HandoverInput{FromPerson: "A", ToPerson: "B", HandedAt: env.Now() + 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Handover.ConfirmExhibition(ctx, loan.ID, scSensor.StorageUnitID, "C", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Check.InCheck(ctx, loan.ID, "in-k", "乙", env.CheckItemsAllPresent(art.ID)); err != nil {
		t.Fatal(err)
	}
	before := env.Now() - 86400 // 早于验收的越界采样（迟到数据）
	if _, err := env.Return.Accept(ctx, loan.ID, domain.AcceptPass, "王五", ""); err != nil {
		t.Fatal(err)
	}

	late, err := env.Env.IngestSample(ctx, whSensor.ID, 35, 80, before)
	if err != nil {
		t.Fatal(err)
	}
	if !late.Late {
		t.Fatalf("验收完成时间之前的采样应标记为迟到数据")
	}

	// 巡检不得利用迟到数据生成异常，藏品状态保持验收结果
	env.Clock.Advance(time.Hour)
	if _, err := env.Anomaly.Patrol(ctx); err != nil {
		t.Fatal(err)
	}
	open, err := env.Repo.Anomalies.OpenByUnit(ctx, wh.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 0 {
		t.Fatalf("迟到数据不应触发异常: %+v", open)
	}
	art, _ = env.Artifact.Get(ctx, art.ID)
	if art.Status != domain.ArtifactStored {
		t.Fatalf("验收结论不应被迟到数据覆盖，实际 %s", art.Status)
	}
	loan, _ = env.Loan.Get(ctx, loan.ID)
	if loan.Status != domain.LoanClosed {
		t.Fatalf("借展应保持关闭，实际 %s", loan.Status)
	}
}
