package service_test
import ("context"; "testing"; "gowork/internal/testenv")
func TestAnnotationBug11(t *testing.T) { env:=testenv.New(t); ctx:=context.Background(); lv:=env.SetupLevelWithRule("B11-LV"); wh,s:=env.SetupWarehouse("B11-WH"); _=env.RegisterStoredArtifact("B11-A",lv.ID,wh.ID); if _,err:=env.Env.IngestSample(ctx,s.ID,40,20,env.Now()-30*3600); err!=nil {t.Fatal(err)}; rows,err:=env.Query.WarehouseRiskRanking(ctx); if err!=nil {t.Fatal(err)}; for _,r:=range rows {if r.UnitID==wh.ID && r.RecentBreaches!=0 {t.Fatalf("risk=%+v",r)}} }

func TestAnnotationControlAnnotationBug11(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
