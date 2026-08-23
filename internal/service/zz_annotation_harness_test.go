package service_test

import (
    "context"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/testenv"
)

func TestAnnotationBug24(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B24")
    rule, err := env.Repo.Rules.ActiveByLevel(ctx, level.ID)
    if err != nil { t.Fatal(err) }
    u1, s1 := env.SetupWarehouse("B24-CRIT")
    u2, s2 := env.SetupWarehouse("B24-MIN")
    smp1, err := env.Env.IngestSample(ctx, s1.ID, 20, 50, env.Now())
    if err != nil { t.Fatal(err) }
    smp2, err := env.Env.IngestSample(ctx, s2.ID, 20, 50, env.Now())
    if err != nil { t.Fatal(err) }
    for _, e := range []*domain.AnomalyEvent{
        {StorageUnitID:u1.ID, RuleVersionID:rule.ID, SampleID:smp1.ID, Severity:domain.SeverityCritical, Status:domain.AnomalyOpen, BreachCount:5, Title:"critical", Version:1, OpenedAt:env.Now()},
        {StorageUnitID:u2.ID, RuleVersionID:rule.ID, SampleID:smp2.ID, Severity:domain.SeverityMinor, Status:domain.AnomalyOpen, BreachCount:1, Title:"minor", Version:1, OpenedAt:env.Now()},
    } { if err := env.Repo.Anomalies.Create(ctx, e); err != nil { t.Fatal(err) } }
    rows, err := env.Query.WarehouseRiskRanking(ctx)
    if err != nil { t.Fatal(err) }
    if len(rows) < 2 || rows[0].UnitID != u1.ID || rows[0].SeverityScore != 5 || rows[0].RiskScore != 5 {
        t.Fatalf("risk ranking=%+v", rows)
    }
}

func TestAnnotationControlAnnotationBug24(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
