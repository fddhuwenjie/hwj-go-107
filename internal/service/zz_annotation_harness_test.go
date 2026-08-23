package service_test
import ("context"; "testing"; "gowork/internal/domain"; "gowork/internal/testenv")
func TestAnnotationBug12(t *testing.T) { env:=testenv.New(t); ctx:=context.Background(); lv:=env.SetupLevelWithRule("B12-LV"); wh,s:=env.SetupWarehouse("B12-WH"); a1:=env.RegisterStoredArtifact("B12-A1",lv.ID,wh.ID); a2:=env.RegisterStoredArtifact("B12-A2",lv.ID,wh.ID); env.SeedWindowQualified(s.ID); l:=env.LoanOf("B12-L",a1.ID,a2.ID); if _,err:=env.Loan.Approve(ctx,l.ID,l.Version,"审核甲"); err!=nil {t.Fatal(err)}; c:=&domain.InventoryCheck{LoanID:l.ID,Direction:domain.CheckOut,IdempotencyKey:"B12",Operator:"甲",CheckedAt:env.Now(),CreatedAt:env.Now()}; if err:=env.Repo.Checks.Create(ctx,c); err!=nil {t.Fatal(err)}; for _,it:=range []domain.InventoryCheckItem{{CheckID:c.ID,ArtifactID:a1.ID,Present:true},{CheckID:c.ID,ArtifactID:a2.ID,Present:false}} {x:=it;if err:=env.Repo.Checks.CreateItem(ctx,&x);err!=nil{t.Fatal(err)}}; diff,err:=env.Query.PackagingDiff(ctx,l.ID,domain.CheckOut); if err!=nil{t.Fatal(err)}; if len(diff)<2 || !diff[0].Present || diff[1].Present {t.Fatalf("diff=%+v",diff)} }

func TestAnnotationControlAnnotationBug12(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
