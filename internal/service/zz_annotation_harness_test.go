package service_test

import (
    "context"
    "testing"
    "gowork/internal/service"
    "gowork/internal/testenv"
)

func TestAnnotationBug09(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B09A-LV")
    unit, sensor := env.SetupWarehouse("B09A-WH")
    artifact := env.RegisterStoredArtifact("B09A-A", level.ID, unit.ID)
    env.SeedWindowQualified(sensor.ID)
    loan := env.LoanOf("B09A-L", artifact.ID)
    if _, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "审核甲"); err != nil { t.Fatal(err) }
    if _, err := env.Check.OutCheck(ctx, loan.ID, "B09A-OUT", "清点甲", env.CheckItemsAllPresent(artifact.ID), service.HandoverInput{FromPerson:"库管甲", ToPerson:"押运乙", HandedAt:env.Now()+10}); err != nil { t.Fatal(err) }
    first, replay, err := env.Handover.AddHandover(ctx, loan.ID, "B09-key", service.HandoverInput{FromPerson:"押运乙", ToPerson:"接收丙", HandedAt:env.Now()+20})
    if err != nil || replay { t.Fatalf("first=%+v replay=%v err=%v", first, replay, err) }
    second, replay, err := env.Handover.AddHandover(ctx, loan.ID, "B09-key-next", service.HandoverInput{FromPerson:"接收丙", ToPerson:"布展丁", HandedAt:env.Now()+30})
    if err != nil { t.Fatal(err) }
    if replay || second.ID == first.ID || second.Seq != first.Seq+1 { t.Fatalf("cross-request replay: first=%+v second=%+v replay=%v", first, second, replay) }
}

func TestAnnotationControlAnnotationBug09(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
