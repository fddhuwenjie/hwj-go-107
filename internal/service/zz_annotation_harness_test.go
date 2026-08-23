package service_test

import (
    "context"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/testenv"
)

func TestAnnotationBug05(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    now := env.Now()
    rows := []*domain.LoanApplication{
        {Code:"B05-TRANSIT", Borrower:"馆甲", StartAt:now-100, EndAt:now, Status:domain.LoanInTransit, Version:1, CreatedAt:now-200, UpdatedAt:now-200},
        {Code:"B05-APPROVED", Borrower:"馆乙", StartAt:now-200, EndAt:now-10, Status:domain.LoanApproved, Version:1, CreatedAt:now-300, UpdatedAt:now-300},
    }
    for _, row := range rows { if err := env.Repo.Loans.Create(ctx, row); err != nil { t.Fatal(err) } }
    got, err := env.Query.OverdueLoans(ctx)
    if err != nil { t.Fatal(err) }
    if len(got) != 0 { t.Fatalf("unexpected overdue rows: %+v", got) }
}

func TestAnnotationControlAnnotationBug05(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
