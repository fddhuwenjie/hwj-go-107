package service_test
import ("context"; "testing"; "gowork/internal/domain"; "gowork/internal/testenv")
func TestAnnotationBug07(t *testing.T) {
 env:=testenv.New(t); ctx:=context.Background(); lv:=env.SetupLevelWithRule("B07-LV"); wh,s:=env.SetupWarehouse("B07-WH"); a:=env.RegisterStoredArtifact("B07-A",lv.ID,wh.ID); env.SeedWindowQualified(s.ID); l:=env.LoanOf("B07-L",a.ID)
 approved,err:=env.Loan.Approve(ctx,l.ID,l.Version,"审核甲"); if err!=nil { t.Fatal(err) }
 if _,err=env.Loan.Reject(ctx,approved.ID,approved.Version,"复核乙","补充意见"); err==nil { t.Fatal("approved loan was rejected") }
 got,_:=env.Artifact.Get(ctx,a.ID); if got.Status!=domain.ArtifactFrozen { t.Fatalf("artifact=%+v",got) }
}

func TestAnnotationControlAnnotationBug07(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
