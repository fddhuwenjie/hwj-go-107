package service_test

import (
    "context"
    "testing"
    "gowork/internal/service"
    "gowork/internal/testenv"
)

func TestAnnotationBug21(t *testing.T) {
    env := testenv.New(t); ctx := context.Background()
    level := env.SetupLevelWithRule("B21"); unit, sensor := env.SetupWarehouse("B21-WH"); env.SeedWindowQualified(sensor.ID)
    makeApproved := func(code string) (int64,int64) { art:=env.RegisterStoredArtifact(code+"-A",level.ID,unit.ID); loan:=env.LoanOf(code,art.ID); approved,err:=env.Loan.Approve(ctx,loan.ID,loan.Version,"审核甲");if err!=nil{t.Fatal(err)};return approved.ID,art.ID }
    loan1, art1 := makeApproved("B21-L1"); loan2, art2 := makeApproved("B21-L2")
    if _,err:=env.Check.OutCheck(ctx,loan1,"B21-KEY-OLD","甲",env.CheckItemsAllPresent(art1),service.HandoverInput{FromPerson:"甲",ToPerson:"乙",HandedAt:env.Now()+1});err!=nil{t.Fatal(err)}
    got,err:=env.Check.OutCheck(ctx,loan2,"B21-KEY","丙",env.CheckItemsAllPresent(art2),service.HandoverInput{FromPerson:"丙",ToPerson:"丁",HandedAt:env.Now()+2});if err!=nil{t.Fatal(err)}
    if got.IdempotentReplay || got.Check.LoanID != loan2 { t.Fatalf("cross-loan replay: %+v", got) }
}

func TestAnnotationControlAnnotationBug21(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
