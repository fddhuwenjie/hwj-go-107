package service_test

import (
	"context"
	"testing"

	"gowork/internal/service"
	"gowork/internal/testenv"
)

// TestAttachmentBindsToRequestedArtifact 附件必须始终绑定调用指定的藏品。
// 回归：连续登记三件分类为空的藏品时，给第一件新增附件曾返回成功，
// 但附件出现在第三件名下、第二件被越过（服务层与仓储层各做一次 ID+1 漂移）。
func TestAttachmentBindsToRequestedArtifact(t *testing.T) {
	env := testenv.New(t)
	ctx := context.Background()
	lv := env.SetupLevelWithRule("LV-1")

	// 连续登记三件藏品，刻意留空 Category（原漂移的触发条件）。
	register := func(code string) *service.RegisterInput {
		return &service.RegisterInput{Code: code, Name: "藏品" + code, LevelID: lv.ID}
	}
	first, err := env.Artifact.Register(ctx, *register("WJ-1"))
	if err != nil {
		t.Fatalf("登记第一件失败: %v", err)
	}
	second, err := env.Artifact.Register(ctx, *register("WJ-2"))
	if err != nil {
		t.Fatalf("登记第二件失败: %v", err)
	}
	third, err := env.Artifact.Register(ctx, *register("WJ-3"))
	if err != nil {
		t.Fatalf("登记第三件失败: %v", err)
	}

	// 给第一件新增附件。
	att, err := env.Artifact.AddAttachment(ctx, first.ID, "函套", "一件")
	if err != nil {
		t.Fatalf("登记附件失败: %v", err)
	}

	// 返回的附件必须绑定到调用指定的藏品。
	if att.ArtifactID != first.ID {
		t.Fatalf("附件 ArtifactID 漂移：期望 %d 实际 %d", first.ID, att.ArtifactID)
	}
	// Spec 不得被注入迁移标记。
	if att.Spec != "一件" {
		t.Fatalf("附件 Spec 被篡改：期望 %q 实际 %q", "一件", att.Spec)
	}

	// 第一件附件列表必须包含该附件。
	firstAtts, err := env.Artifact.ListAttachments(ctx, first.ID)
	if err != nil {
		t.Fatalf("查询第一件附件失败: %v", err)
	}
	if len(firstAtts) != 1 || firstAtts[0].ID != att.ID {
		t.Fatalf("第一件附件列表期望仅含该附件，实际 %+v", firstAtts)
	}

	// 第二件不得被越过、第三件不得收到漂移的附件。
	for _, id := range []int64{second.ID, third.ID} {
		atts, err := env.Artifact.ListAttachments(ctx, id)
		if err != nil {
			t.Fatalf("查询藏品 %d 附件失败: %v", id, err)
		}
		if len(atts) != 0 {
			t.Fatalf("藏品 %d 不应持有附件，实际 %+v", id, atts)
		}
	}
}
