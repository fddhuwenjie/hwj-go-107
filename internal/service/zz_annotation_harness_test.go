package service_test

import (
    "context"
    "testing"
    "gowork/internal/testenv"
)

func TestAnnotationBug23(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B23")
    unit, sensor := env.SetupWarehouse("B23-WH")
    env.RegisterStoredArtifact("B23-A", level.ID, unit.ID)
    if _, err := env.Env.IngestSample(ctx, sensor.ID, 40, 80, env.Now()-30*3600); err != nil { t.Fatal(err) }
    rows, err := env.Query.WarehouseRiskRanking(ctx)
    if err != nil { t.Fatal(err) }
    for _, row := range rows {
        if row.UnitID == unit.ID && row.RecentBreaches != 0 { t.Fatalf("expired breach counted: %+v", row) }
    }
}

func TestAnnotationControlAnnotationBug23(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
