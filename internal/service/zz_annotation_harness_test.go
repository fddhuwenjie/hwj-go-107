package service_test

import (
    "context"
    "testing"
    "gowork/internal/service"
    "gowork/internal/testenv"
)

func TestAnnotationBug13(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B13-LV")
    unit, sensor := env.SetupWarehouse("B13-WH")
    artifact := env.RegisterStoredArtifact("B13-A", level.ID, unit.ID)
    env.SeedWindowQualified(sensor.ID)
    loan := env.LoanOf("B13-L", artifact.ID)
    if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "审核甲"); err != nil { t.Fatal(err) }
    if _, err := env.Check.OutCheck(ctx, loan.ID, "B13-OUT", "清点甲", env.CheckItemsAllPresent(artifact.ID), service.HandoverInput{FromPerson:"库管甲", ToPerson:"押运乙", HandedAt:env.Now()+10}); err != nil { t.Fatal(err) }
    node, err := env.Handover.AddTransportNode(ctx, loan.ID, "arrival", "省博卸货区", env.Now()+20, "押运乙")
    if err != nil { t.Fatal(err) }
    got, err := env.Loan.Get(ctx, loan.ID); if err != nil { t.Fatal(err) }
    if got.Status != "in_transit" || node.Location != "省博卸货区" || node.RecordedBy != "押运乙" { t.Fatalf("loan=%+v node=%+v",got,node) }
}

func TestAnnotationControlAnnotationBug13(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
