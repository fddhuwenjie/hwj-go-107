package service_test

import (
    "context"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/testenv"
)

func TestAnnotationBug01(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B01")
    unit, sensor := env.SetupWarehouse("B01-WH")
    artifact := env.RegisterStoredArtifact("B01-A", level.ID, unit.ID)
    env.SeedWindowQualified(sensor.ID)
    loan := env.LoanOf("B01-L", artifact.ID)
    approved, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "审核甲")
    if err != nil { t.Fatal(err) }
    if _, err := env.Loan.Cancel(ctx, approved.ID, approved.Version); err == nil {
        t.Fatal("approved loan was cancelled")
    }
    got, err := env.Artifact.Get(ctx, artifact.ID)
    if err != nil { t.Fatal(err) }
    if got.Status != domain.ArtifactFrozen { t.Fatalf("artifact status=%s", got.Status) }
}

func TestAnnotationControlAnnotationBug01(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
