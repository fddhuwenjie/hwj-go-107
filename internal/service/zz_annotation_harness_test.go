package service_test

import (
    "context"
    "testing"
    "gowork/internal/testenv"
)

func TestAnnotationBug15(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B15")
    unit, _ := env.SetupWarehouse("B15-WH")
    first := env.RegisterStoredArtifact("B15-A", level.ID, unit.ID)
    second := env.RegisterStoredArtifact("B15-B", level.ID, unit.ID)
    saved, err := env.Artifact.Get(ctx, first.ID)
    if err != nil { t.Fatal(err) }
    if _, err := env.Artifact.Get(ctx, second.ID); err != nil { t.Fatal(err) }
    if saved.ID != first.ID || saved.Code != "B15-A" { t.Fatalf("saved response mutated: %+v", saved) }
}

func TestAnnotationControlAnnotationBug15(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
