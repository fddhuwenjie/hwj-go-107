package service_test

import (
    "context"
    "testing"
    "gowork/internal/service"
    "gowork/internal/testenv"
)

func TestAnnotationBug06(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B06")
    unit, sensor := env.SetupWarehouse("B06-WH")
    artifact := env.RegisterStoredArtifact("B06-A", level.ID, unit.ID)
    env.SeedWindowQualified(sensor.ID)
    loan := env.LoanOf("B06-L", artifact.ID)
    approved, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "审核甲")
    if err != nil { t.Fatal(err) }
    _, err = env.Check.OutCheck(ctx, approved.ID, "B06-OUT", "清点甲", env.CheckItemsAllPresent(artifact.ID), service.HandoverInput{FromPerson:"库管甲", ToPerson:"押运乙", HandedAt:env.Now()+10})
    if err != nil { t.Fatal(err) }
    detail, err := env.Query.LoanDetail(ctx, approved.ID)
    if err != nil { t.Fatal(err) }
    if len(detail.Checks) != 1 { t.Fatalf("checks=%+v", detail.Checks) }
}

func TestAnnotationControlAnnotationBug06(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
