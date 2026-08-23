package service_test

import (
    "context"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/testenv"
)

func TestAnnotationBug18(t *testing.T) {
    env := testenv.New(t); ctx := context.Background(); now := env.Now()
    level := env.SetupLevelWithRule("B18"); unit, _ := env.SetupWarehouse("B18-WH")
    art := env.RegisterStoredArtifact("B18-A", level.ID, unit.ID)
    loan := &domain.LoanApplication{Code:"B18-L", Borrower:"馆甲", StartAt:now, EndAt:now+10, Status:domain.LoanReturned, Version:1, CreatedAt:now, UpdatedAt:now}
    if err := env.Repo.Loans.Create(ctx, loan); err != nil { t.Fatal(err) }
    pack := domain.MarshalPackaging([]domain.PackagingEntry{{AttachmentID:101, Name:"锦盒"}, {AttachmentID:102, Name:"封条"}})
    if err := env.Repo.Loans.AddItem(ctx, &domain.LoanItem{LoanID:loan.ID, ArtifactID:art.ID, FrozenLevelID:level.ID, FrozenUnitID:unit.ID, PackagingSnapshot:pack, CreatedAt:now}); err != nil { t.Fatal(err) }
    check := &domain.InventoryCheck{LoanID:loan.ID, Direction:domain.CheckIn, IdempotencyKey:"B18-IN", Operator:"甲", Complete:true, CheckedAt:now, CreatedAt:now}
    if err := env.Repo.Checks.Create(ctx, check); err != nil { t.Fatal(err) }
    for _, id := range []int64{0,101,102} { if err := env.Repo.Checks.CreateItem(ctx, &domain.InventoryCheckItem{CheckID:check.ID, ArtifactID:art.ID, AttachmentID:id, Present:true}); err != nil { t.Fatal(err) } }
    diff, err := env.Query.PackagingDiff(ctx, loan.ID, domain.CheckIn); if err != nil { t.Fatal(err) }
    if len(diff) != 3 { t.Fatalf("diff=%+v", diff) }
    for _, row := range diff { if row.Diff != "ok" || !row.Present { t.Fatalf("false packaging diff: %+v", diff) } }
}

func TestAnnotationControlAnnotationBug18(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
