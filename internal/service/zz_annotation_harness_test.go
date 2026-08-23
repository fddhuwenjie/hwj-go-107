package service_test

import (
    "context"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/testenv"
)

func TestAnnotationBug30(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B30")
    rule, err := env.Repo.Rules.ActiveByLevel(ctx, level.ID)
    if err != nil { t.Fatal(err) }
    unit, sensor := env.SetupWarehouse("B30-WH")
    env.RegisterStoredArtifact("B30-A", level.ID, unit.ID)
    first, err := env.Env.IngestSample(ctx, sensor.ID, 40, 80, env.Now()-1)
    if err != nil { t.Fatal(err) }
    for i := 0; i < 2; i++ {
        ev := &domain.AnomalyEvent{StorageUnitID:unit.ID, RuleVersionID:rule.ID, SampleID:first.ID, Severity:domain.SeverityMajor, Status:domain.AnomalyOpen, BreachCount:3, Title:"major", Version:1, OpenedAt:env.Now()}
        if err := env.Repo.Anomalies.Create(ctx, ev); err != nil { t.Fatal(err) }
    }
    if _, err := env.Env.IngestSample(ctx, sensor.ID, 40, 80, env.Now()-2); err != nil { t.Fatal(err) }
    rows, err := env.Query.WarehouseRiskRanking(ctx)
    if err != nil { t.Fatal(err) }
    for _, row := range rows { if row.UnitID == unit.ID { if row.SeverityScore != 6 || row.RecentBreaches != 2 || row.RiskScore != 8 { t.Fatalf("inconsistent risk row: %+v", row) }; return } }
    t.Fatal("target warehouse missing")
}

func TestAnnotationControlAnnotationBug30(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
