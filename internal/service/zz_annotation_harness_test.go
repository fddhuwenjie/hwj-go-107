package service_test

import (
    "context"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/repository"
    "gowork/internal/testenv"
)

func TestAnnotationBug26(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    _, first := env.SetupWarehouse("B26-FIRST")
    target, targetSensor := env.SetupShowcase("B26-TARGET")
    id := target.ID
    page, err := env.Env.ListSensors(ctx, repository.SensorFilter{StorageUnitID:&id}, domain.Page{Limit:20})
    if err != nil { t.Fatal(err) }
    if len(page.Items) != 1 || page.Items[0].ID != targetSensor.ID || page.Items[0].ID == first.ID {
        t.Fatalf("cross-unit sensors returned as target: %+v", page.Items)
    }
}

func TestAnnotationControlAnnotationBug26(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
