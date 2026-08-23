package service_test
import ("context"; "errors"; "testing"; "gowork/internal/domain"; "gowork/internal/service"; "gowork/internal/testenv")
func TestAnnotationBug10(t *testing.T) {
 env:=testenv.New(t); ctx:=context.Background(); lv:=env.SetupLevelWithRule("B10-LV"); wh,s:=env.SetupWarehouse("B10-WH"); a1:=env.RegisterStoredArtifact("B10-A1",lv.ID,wh.ID); a2:=env.RegisterStoredArtifact("B10-A2",lv.ID,wh.ID); env.SeedWindowQualified(s.ID); l:=env.LoanOf("B10-L",a1.ID,a2.ID)
 if _,err:=env.Loan.Approve(ctx,l.ID,l.Version,"审核甲"); err!=nil {t.Fatal(err)}
 if _,err:=env.DB.Exec(`UPDATE loan_applications SET status='returned' WHERE id=?`,l.ID); err!=nil {t.Fatal(err)}
 if _,err:=env.DB.Exec(`UPDATE artifacts SET status='returned_pending' WHERE id=?`,a1.ID); err!=nil {t.Fatal(err)}
 check:=&domain.InventoryCheck{LoanID:l.ID,Direction:domain.CheckIn,IdempotencyKey:"B10-IN",Operator:"清点",Complete:true,CheckedAt:env.Now(),CreatedAt:env.Now()}; if err:=env.Repo.Checks.Create(ctx,check); err!=nil {t.Fatal(err)}
 _,err:=env.Return.Accept(ctx,l.ID,domain.AcceptPass,"复核乙",""); if err==nil {t.Fatal("accept unexpectedly succeeded")}
 if _,e:=env.Repo.Acceptances.AcceptanceByLoan(ctx,l.ID); !errors.Is(e,domain.ErrNotFound) {t.Fatalf("orphan acceptance err=%v",e)}
 _=service.HandoverInput{}
}

func TestAnnotationControlAnnotationBug10(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
