package service_test

import (
    "context"
    "errors"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/testenv"
)

func TestAnnotationBug16(t *testing.T) {
    env := testenv.New(t); ctx := context.Background(); now := env.Now()
    level := env.SetupLevelWithRule("B16"); unit, _ := env.SetupWarehouse("B16-WH")
    art := env.RegisterStoredArtifact("B16-A", level.ID, unit.ID)
    loan := &domain.LoanApplication{Code:"B16-L", Borrower:"馆甲", StartAt:now-10, EndAt:now+10, Status:domain.LoanReturned, Version:1, CreatedAt:now, UpdatedAt:now}
    if err := env.Repo.Loans.Create(ctx, loan); err != nil { t.Fatal(err) }
    if err := env.Repo.Loans.AddItem(ctx, &domain.LoanItem{LoanID:loan.ID, ArtifactID:art.ID, FrozenStatus:domain.ArtifactStored, FrozenLevelID:level.ID, FrozenUnitID:unit.ID, PackagingSnapshot:"[]", CreatedAt:now}); err != nil { t.Fatal(err) }
    check := &domain.InventoryCheck{LoanID:loan.ID, Direction:domain.CheckIn, IdempotencyKey:"B16-IN", Operator:"清点甲", Complete:true, CheckedAt:now, CreatedAt:now}
    if err := env.Repo.Checks.Create(ctx, check); err != nil { t.Fatal(err) }
    if _, err := env.Return.Accept(ctx, loan.ID, domain.AcceptPassWithNotes, "复核甲", "包装有轻微磨损"); err == nil { t.Fatal("invalid artifact state accepted") }
    if got, err := env.Repo.Acceptances.AcceptanceByLoan(ctx, loan.ID); err == nil || got != nil || !errors.Is(err, domain.ErrNotFound) { t.Fatalf("orphan acceptance=%+v err=%v", got, err) }
}

func TestAnnotationControlAnnotationBug16(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
