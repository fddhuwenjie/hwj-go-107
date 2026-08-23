package service_test
import ("context"; "testing"; "gowork/internal/service"; "gowork/internal/testenv")
func TestAnnotationBug14(t *testing.T) { env:=testenv.New(t); ctx:=context.Background(); lv:=env.SetupLevelWithRule("B14-LV"); a1,_:=env.Artifact.Register(ctx,service.RegisterInput{Code:"B14-A1",Name:"甲",LevelID:lv.ID}); a2,_:=env.Artifact.Register(ctx,service.RegisterInput{Code:"B14-A2",Name:"乙",LevelID:lv.ID}); if _,err:=env.Artifact.AddAttachment(ctx,a1.ID,"甲盒","一件");err!=nil{t.Fatal(err)}; if _,err:=env.Artifact.AddAttachment(ctx,a2.ID,"乙盒","一件");err!=nil{t.Fatal(err)}; got,err:=env.Artifact.ListAttachments(ctx,a2.ID);if err!=nil{t.Fatal(err)};if len(got)!=1 || got[0].Name!="乙盒" || got[0].ArtifactID!=a2.ID {t.Fatalf("attachments=%+v",got)} }

func TestAnnotationControlAnnotationBug14(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
