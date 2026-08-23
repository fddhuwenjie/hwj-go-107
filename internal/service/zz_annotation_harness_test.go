package service_test
import ("context"; "testing"; "gowork/internal/service"; "gowork/internal/testenv")
func TestAnnotationBug08(t *testing.T) {
 env:=testenv.New(t); ctx:=context.Background(); lv:=env.SetupLevelWithRule("B08-LV")
 var arts []int64; for _,code:=range []string{"B08-A","B08-B","B08-C"} { a,err:=env.Artifact.Register(ctx,service.RegisterInput{Code:code,Name:code,LevelID:lv.ID}); if err!=nil {t.Fatal(err)}; arts=append(arts,a.ID) }
 if _,err:=env.Artifact.AddAttachment(ctx,arts[0],"原装木盒","一件"); err!=nil { t.Fatal(err) }
 got,err:=env.Artifact.ListAttachments(ctx,arts[0]); if err!=nil {t.Fatal(err)}; if len(got)!=1 || got[0].ArtifactID!=arts[0] { t.Fatalf("target attachments=%+v",got) }
}

func TestAnnotationControlAnnotationBug08(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
