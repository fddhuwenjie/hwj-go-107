package service_test

import (
    "context"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/testenv"
)

func TestAnnotationBug19(t *testing.T) {
    env := testenv.New(t); ctx := context.Background(); now := env.Now()
    level := env.SetupLevelWithRule("B19"); unit, sensor := env.SetupWarehouse("B19-WH")
    art := env.RegisterStoredArtifact("B19-A", level.ID, unit.ID)
    art.Status = domain.ArtifactIsolated; art.UpdatedAt = now; if err := env.Repo.Artifacts.Update(ctx, art); err != nil { t.Fatal(err) }
    rule, err := env.Repo.Rules.ActiveByLevel(ctx, level.ID); if err != nil { t.Fatal(err) }
    sample, err := env.Env.IngestSample(ctx, sensor.ID, 30, 70, now); if err != nil { t.Fatal(err) }
    ev := &domain.AnomalyEvent{StorageUnitID:unit.ID, RuleVersionID:rule.ID, SampleID:sample.ID, Severity:domain.SeverityMajor, Status:domain.AnomalyReviewing, BreachCount:3, Title:"B19", Version:1, OpenedAt:now}
    if err := env.Repo.Anomalies.Create(ctx, ev); err != nil { t.Fatal(err) }
    action := &domain.ProtectionAction{EventID:ev.ID, ActionType:"adjust", Operator:"复核甲", Note:"", Status:domain.DisposalDone, CreatedAt:now}
    if err := env.Repo.Disposals.Create(ctx, action); err != nil { t.Fatal(err) }
    got, err := env.Anomaly.Review(ctx, action.ID, "复核甲", false, "需要继续整改"); if err != nil { t.Fatal(err) }
    if got.Status != domain.AnomalyDisposing { t.Fatalf("event=%+v", got) }
    saved, err := env.Repo.Artifacts.GetByID(ctx, art.ID); if err != nil { t.Fatal(err) }
    if saved.Status != domain.ArtifactIsolated { t.Fatalf("artifact restored: %+v", saved) }
}

func TestAnnotationControlAnnotationBug19(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
