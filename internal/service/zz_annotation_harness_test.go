package service_test

import (
    "context"
    "testing"
    "gowork/internal/testenv"
)

func TestAnnotationBug03(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B03")
    unit, _ := env.SetupWarehouse("B03-WH")
    artifact := env.RegisterStoredArtifact("B03-A", level.ID, unit.ID)
    second, _ := env.SetupWarehouse("B03-WH2")
    stale := artifact.Version
    first, err := env.Artifact.AssignLocation(ctx, artifact.ID, second.ID, stale)
    if err != nil { t.Fatal(err) }
    if _, err := env.Artifact.AssignLocation(ctx, artifact.ID, unit.ID, stale); err == nil {
        t.Fatal("stale location update succeeded")
    }
    got, err := env.Artifact.Get(ctx, artifact.ID)
    if err != nil { t.Fatal(err) }
    if got.StorageUnitID == nil || *got.StorageUnitID != *first.StorageUnitID { t.Fatalf("artifact location overwritten: %+v", got) }
}

func TestAnnotationControlAnnotationBug03(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
