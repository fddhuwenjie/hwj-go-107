package service_test

import (
    "context"
    "sync"
    "testing"
    "gowork/internal/domain"
    "gowork/internal/repository"
    "gowork/internal/testenv"
)

func TestAnnotationBug28(t *testing.T) {
    env := testenv.New(t)
    ctx := context.Background()
    level := env.SetupLevelWithRule("B28")
    start := make(chan struct{})
    var wg sync.WaitGroup
    type result struct { id int64; err error }
    got := make(chan result, 2)
    for i := 0; i < 2; i++ { wg.Add(1); go func(i int) { defer wg.Done(); <-start; r, e := env.Env.CreateRule(ctx, level.ID, 10+float64(i), 25+float64(i), 40, 65, 3); if r != nil { got <- result{r.ID,e} } else { got <- result{0,e} } }(i) }
    close(start); wg.Wait(); close(got)
    successes := 0
    ids := map[int64]bool{}
    for r := range got { if r.err == nil { successes++; ids[r.id] = true } }
    page, err := env.Env.ListRules(ctx, repository.RuleFilter{LevelID:&level.ID}, domain.Page{Limit:20})
    if err != nil { t.Fatal(err) }
    if successes == 2 && (len(ids) != 2 || len(page.Items) != 3) { t.Fatalf("two successful creates were not both preserved: successes=%d ids=%v rows=%+v", successes, ids, page.Items) }
}

func TestAnnotationControlAnnotationBug28(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
