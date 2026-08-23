package service_test

import (
    "context"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/repository"
    "gowork/internal/testenv"
)

func TestAnnotationBug29(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B29")
    unit, sensor := env.SetupWarehouse("B29-WH")
    artifact := env.RegisterStoredArtifact("B29-A", level.ID, unit.ID)
    loan := &domain.LoanApplication{Code:"B29-L", Borrower:"馆甲", StartAt:env.Now()-10, EndAt:env.Now(), Status:domain.LoanClosed, Version:1, CreatedAt:env.Now()-20, UpdatedAt:env.Now()}
    if err := env.Repo.Loans.Create(ctx, loan); err != nil { t.Fatal(err) }
    item := &domain.LoanItem{LoanID:loan.ID, ArtifactID:artifact.ID, FrozenStatus:artifact.Status, FrozenLevelID:level.ID, FrozenUnitID:unit.ID, PackagingSnapshot:"[]"}
    if err := env.Repo.Loans.AddItem(ctx, item); err != nil { t.Fatal(err) }
    check := &domain.InventoryCheck{LoanID:loan.ID, Direction:domain.CheckIn, IdempotencyKey:"B29-IN", Operator:"清点", Complete:true, CheckedAt:env.Now(), CreatedAt:env.Now()}
    if err := env.Repo.Checks.Create(ctx, check); err != nil { t.Fatal(err) }
    if _, err := env.DB.ExecContext(ctx, `INSERT INTO return_acceptances (loan_id,check_id,result,reviewer,note,reviewed_at,created_at) VALUES (?,?,?,?,?,?,?)`, loan.ID, check.ID, "pass", "复核", "", env.Now(), env.Now()); err != nil { t.Fatal(err) }
    sample, err := env.Env.IngestSample(ctx, sensor.ID, 20, 50, env.Now())
    if err != nil { t.Fatal(err) }
    if !sample.Late { t.Fatal("acceptance-boundary sample was not marked late") }
    page, err := env.Env.ListSamples(ctx, repository.SampleFilter{SensorID:&sensor.ID}, domain.Page{Limit:20})
    if err != nil { t.Fatal(err) }
    if len(page.Items) != 1 || !page.Items[0].Late { t.Fatalf("late flag lost after read: %+v", page.Items) }
}

func TestAnnotationControlAnnotationBug29(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
