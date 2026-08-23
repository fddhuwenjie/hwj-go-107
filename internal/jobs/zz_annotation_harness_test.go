package jobs_test

import (
    "context"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/jobs"
    "gowork/internal/testenv"
)

func TestAnnotationBug17(t *testing.T) {
    env := testenv.New(t); ctx := context.Background(); now := env.Now()
    loan := &domain.LoanApplication{Code:"B17-L", Borrower:"馆甲", StartAt:now-200000, EndAt:now+1000, Status:domain.LoanInTransit, Version:1, CreatedAt:now-200000, UpdatedAt:now-200000}
    if err := env.Repo.Loans.Create(ctx, loan); err != nil { t.Fatal(err) }
    rows := []*domain.PackageHandover{
        {LoanID:loan.ID, Seq:1, FromPerson:"甲", ToPerson:"乙", HandedAt:now-100000, IdempotencyKey:"B17-1", CreatedAt:now-100000},
        {LoanID:loan.ID, Seq:2, FromPerson:"乙", ToPerson:"丙", HandedAt:now-100, IdempotencyKey:"B17-2", CreatedAt:now-100},
    }
    for _, h := range rows { if err := env.Repo.Handovers.Create(ctx, h); err != nil { t.Fatal(err) } }
    if err := jobs.HandoverTimeoutHandler(env.Txm, env.Clock, env.Audit, 3600)(ctx); err != nil { t.Fatal(err) }
    got, err := env.Repo.Loans.GetByID(ctx, loan.ID); if err != nil { t.Fatal(err) }
    if got.Attention { t.Fatalf("recently handed loan marked attention: %+v", got) }
}

func TestAnnotationControlAnnotationBug17(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
