package service_test

import (
    "context"
    "testing"
    "gowork/internal/service"
    "gowork/internal/testenv"
)

func TestAnnotationBug02(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B02")
    showcase, _ := env.SetupShowcase("B02-SC")
    artifact, err := env.Artifact.Register(ctx, service.RegisterInput{Code:"B02-A", Name:"藏品B02", LevelID:level.ID})
    if err != nil { t.Fatal(err) }
    if _, err := env.Artifact.AssignLocation(ctx, artifact.ID, showcase.ID, artifact.Version); err == nil {
        t.Fatal("showcase accepted as initial warehouse location")
    }
}

func TestAnnotationControlAnnotationBug02(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
