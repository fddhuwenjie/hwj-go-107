package service_test

import (
    "context"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/repository"
    "gowork/internal/service"
    "gowork/internal/testenv"
)

func TestAnnotationBug04(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B04")
    unit, _ := env.SetupWarehouse("B04-WH")
    artifact := env.RegisterStoredArtifact("B04-A", level.ID, unit.ID)
    _, err := env.Loan.Create(ctx, service.CreateLoanInput{Code:"B04-L", Borrower:"馆甲", StartAt:env.Now()+100, EndAt:env.Now()+200, ArtifactIDs:[]int64{artifact.ID, 999999}})
    if err == nil { t.Fatal("invalid loan create unexpectedly succeeded") }
    page, err := env.Loan.List(ctx, repository.LoanFilter{}, domain.Page{Limit:20})
    if err != nil { t.Fatal(err) }
    if len(page.Items) != 0 { t.Fatalf("orphan loan persisted: %+v", page.Items) }
}

func TestAnnotationControlAnnotationBug04(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
