package service_test

import (
    "context"
    "testing"
    "gowork/internal/service"
    "gowork/internal/testenv"
)

func TestAnnotationBug22(t *testing.T) {
    env := testenv.New(t); ctx := context.Background()
    level, err := env.Env.CreateLevel(ctx,"B22","无规则等级",""); if err != nil { t.Fatal(err) }
    unit, sensor := env.SetupWarehouse("B22-WH")
    art,err:=env.Artifact.Register(ctx,service.RegisterInput{Code:"B22-A",Name:"藏品B22",LevelID:level.ID});if err!=nil{t.Fatal(err)}
    if _,err=env.Artifact.AssignLocation(ctx,art.ID,unit.ID,art.Version);err!=nil{t.Fatal(err)}
    if _,err=env.Env.IngestSample(ctx,sensor.ID,40,80,env.Now());err!=nil{t.Fatal(err)}
    if _,err=env.Anomaly.Patrol(ctx);err==nil{t.Fatal("patrol without active rule unexpectedly succeeded")}
    pending,err:=env.Repo.Samples.ListUnprocessed(ctx,10);if err!=nil{t.Fatal(err)}
    if len(pending)!=1 { t.Fatalf("failed patrol consumed samples: %+v",pending) }
}

func TestAnnotationControlAnnotationBug22(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
