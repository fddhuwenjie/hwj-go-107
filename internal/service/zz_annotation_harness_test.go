package service_test

import (
    "context"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/service"
    "gowork/internal/testenv"
)

func TestAnnotationBug25(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B25")
    unit, _ := env.SetupWarehouse("B25-WH")
    artifact := env.RegisterStoredArtifact("B25-A", level.ID, unit.ID)
    draft, err := env.Loan.Create(ctx, service.CreateLoanInput{Code:"B25-L", Borrower:"馆甲", StartAt:env.Now()+100, EndAt:env.Now()+200, ArtifactIDs:[]int64{artifact.ID}})
    if err != nil { t.Fatal(err) }
    if _, err := env.Loan.Reject(ctx, draft.ID, draft.Version, "审核甲", "资料不足"); err == nil { t.Fatal("draft loan was rejected before submission") }
    got, err := env.Loan.Get(ctx, draft.ID)
    if err != nil { t.Fatal(err) }
    if got.Status != domain.LoanDraft || got.ApprovedAt != nil || got.RuleSnapshot != "" || got.Attention { t.Fatalf("draft mutated: %+v", got) }
}

func TestAnnotationControlAnnotationBug25(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
