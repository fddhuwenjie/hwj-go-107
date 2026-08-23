package service_test

import (
    "context"
    "sync"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/service"
    "gowork/internal/testenv"
)

func TestAnnotationBug27(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B27")
    unit, sensor := env.SetupWarehouse("B27-WH")
    artifact := env.RegisterStoredArtifact("B27-A", level.ID, unit.ID)
    env.SeedWindowQualified(sensor.ID)
    loan := env.LoanOf("B27-L", artifact.ID)
    approved, err := env.Loan.Approve(ctx, loan.ID, loan.Version, "审核甲")
    if err != nil { t.Fatal(err) }
    out, err := env.Check.OutCheck(ctx, approved.ID, "B27-OUT", "清点甲", env.CheckItemsAllPresent(artifact.ID), service.HandoverInput{FromPerson:"库管", ToPerson:"押运", HandedAt:env.Now()+1})
    if err != nil { t.Fatal(err) }
    if _, err := env.Handover.AddTransportNode(ctx, approved.ID, "departure", "馆库", env.Now()+2, "调度0"); err != nil { t.Fatal(err) }
    at := env.Now()+3
    start := make(chan struct{})
    var wg sync.WaitGroup
    errs := make(chan error, 2)
    for i := 0; i < 2; i++ { wg.Add(1); go func(i int) { defer wg.Done(); <-start; _, e := env.Handover.AddTransportNode(ctx, out.Check.LoanID, "transit", "中转", at, string(rune('A'+i))); errs <- e }(i) }
    close(start); wg.Wait(); close(errs)
    success := 0
    for e := range errs { if e == nil { success++ } }
    nodes, err := env.Repo.Handovers.ListNodesByLoan(ctx, approved.ID)
    if err != nil { t.Fatal(err) }
    if success == 2 && len(nodes) != 3 { t.Fatalf("both writes reported success but nodes=%+v", nodes) }
    _ = domain.LoanInTransit
}

func TestAnnotationControlAnnotationBug27(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
