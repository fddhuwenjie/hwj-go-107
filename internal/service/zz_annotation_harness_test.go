package service_test

import (
    "context"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/testenv"
)

func TestAnnotationBug20(t *testing.T) {
    env := testenv.New(t); ctx := context.Background(); now := env.Now()
    level := env.SetupLevelWithRule("B20"); unit, _ := env.SetupWarehouse("B20-WH")
    makeReturned := func(code string) (*domain.LoanApplication, *domain.InventoryCheck) {
        art := env.RegisterStoredArtifact(code+"-A", level.ID, unit.ID); art.Status=domain.ArtifactReturnedPending; art.UpdatedAt=now; if err:=env.Repo.Artifacts.Update(ctx,art); err!=nil { t.Fatal(err) }
        loan:=&domain.LoanApplication{Code:code,Borrower:"馆",StartAt:now-10,EndAt:now+10,Status:domain.LoanReturned,Version:1,CreatedAt:now,UpdatedAt:now}; if err:=env.Repo.Loans.Create(ctx,loan);err!=nil{t.Fatal(err)}
        if err:=env.Repo.Loans.AddItem(ctx,&domain.LoanItem{LoanID:loan.ID,ArtifactID:art.ID,FrozenLevelID:level.ID,FrozenUnitID:unit.ID,PackagingSnapshot:"[]",CreatedAt:now});err!=nil{t.Fatal(err)}
        c:=&domain.InventoryCheck{LoanID:loan.ID,Direction:domain.CheckIn,IdempotencyKey:code+"-IN",Operator:"甲",Complete:true,CheckedAt:now,CreatedAt:now};if err:=env.Repo.Checks.Create(ctx,c);err!=nil{t.Fatal(err)};return loan,c
    }
    first, firstCheck := makeReturned("B20-L1"); _, _ = makeReturned("B20-L2")
    acc, err := env.Return.Accept(ctx, first.ID, domain.AcceptPass, "复核甲", ""); if err != nil { t.Fatal(err) }
    if acc.CheckID != firstCheck.ID { t.Fatalf("acceptance bound to foreign check: %+v", acc) }
}

func TestAnnotationControlAnnotationBug20(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
