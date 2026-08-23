// Package testenv 提供基于真实临时 SQLite 文件与假时钟的测试环境脚手架。
package testenv

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"gowork/internal/audit"
	"gowork/internal/clock"
	"gowork/internal/config"
	"gowork/internal/domain"
	"gowork/internal/repository"
	"gowork/internal/service"
	"gowork/internal/sqlite"
	"gowork/internal/sqliterepo"
	"gowork/internal/tx"
)

// BaseTime 测试基准时间（固定，保证窗口计算可预期）。
var BaseTime = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

// Env 测试环境。
type Env struct {
	T        *testing.T
	DB       *sql.DB
	DBPath   string
	Repo     *repository.Repositories
	Txm      *tx.Manager
	Clock    *clock.Fake
	Cfg      config.Config
	Audit    *audit.Recorder
	Deps     *service.Deps
	Artifact *service.ArtifactService
	Env      *service.EnvService
	Anomaly  *service.AnomalyService
	Loan     *service.LoanService
	Check    *service.CheckService
	Handover *service.HandoverService
	Return   *service.ReturnService
	Query    *service.QueryService
}

// New 创建测试环境：真实临时 SQLite 文件 + 假时钟。
func New(t *testing.T) *Env {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := config.Config{
		DBPath:          dbPath,
		PreLoanWindow:   72 * time.Hour,
		EnvMaxGap:       time.Hour,
		HandoverTimeout: 48 * time.Hour,
		JobInterval:     time.Second,
		JobMaxAttempts:  3,
		ShutdownTimeout: 5 * time.Second,
	}
	clk := clock.NewFake(BaseTime)
	repo := sqliterepo.New(db)
	txm := tx.NewManager(db, sqliterepo.New)
	aud := audit.NewRecorder(clk)
	deps := &service.Deps{Repo: repo, Tx: txm, Clock: clk, Audit: aud, Cfg: cfg}
	return &Env{
		T: t, DB: db, DBPath: dbPath, Repo: repo, Txm: txm, Clock: clk, Cfg: cfg, Audit: aud, Deps: deps,
		Artifact: service.NewArtifactService(deps),
		Env:      service.NewEnvService(deps),
		Anomaly:  service.NewAnomalyService(deps),
		Loan:     service.NewLoanService(deps),
		Check:    service.NewCheckService(deps),
		Handover: service.NewHandoverService(deps),
		Return:   service.NewReturnService(deps),
		Query:    service.NewQueryService(deps),
	}
}

// Now 当前假时钟 Unix 秒。
func (e *Env) Now() int64 { return e.Clock.Now().Unix() }

// Reopen 在已有数据库文件上重建环境（模拟进程重启），复用给定时钟。
func Reopen(t *testing.T, dbPath string, clk *clock.Fake) *Env {
	t.Helper()
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("重新打开数据库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	cfg := config.Config{
		DBPath:          dbPath,
		PreLoanWindow:   72 * time.Hour,
		EnvMaxGap:       time.Hour,
		HandoverTimeout: 48 * time.Hour,
		JobInterval:     time.Second,
		JobMaxAttempts:  3,
		ShutdownTimeout: 5 * time.Second,
	}
	repo := sqliterepo.New(db)
	txm := tx.NewManager(db, sqliterepo.New)
	aud := audit.NewRecorder(clk)
	deps := &service.Deps{Repo: repo, Tx: txm, Clock: clk, Audit: aud, Cfg: cfg}
	return &Env{
		T: t, DB: db, DBPath: dbPath, Repo: repo, Txm: txm, Clock: clk, Cfg: cfg, Audit: aud, Deps: deps,
		Artifact: service.NewArtifactService(deps),
		Env:      service.NewEnvService(deps),
		Anomaly:  service.NewAnomalyService(deps),
		Loan:     service.NewLoanService(deps),
		Check:    service.NewCheckService(deps),
		Handover: service.NewHandoverService(deps),
		Return:   service.NewReturnService(deps),
		Query:    service.NewQueryService(deps),
	}
}

// SetupLevelWithRule 创建保存等级并启用一条阈值规则（14-24℃，45-60%RH）。
func (e *Env) SetupLevelWithRule(code string) *domain.PreservationLevel {
	e.T.Helper()
	lv, err := e.Env.CreateLevel(context.Background(), code, "等级"+code, "")
	if err != nil {
		e.T.Fatalf("创建等级失败: %v", err)
	}
	rv, err := e.Env.CreateRule(context.Background(), lv.ID, 14, 24, 45, 60, 2)
	if err != nil {
		e.T.Fatalf("创建规则失败: %v", err)
	}
	if _, err := e.Env.ActivateRule(context.Background(), rv.ID); err != nil {
		e.T.Fatalf("启用规则失败: %v", err)
	}
	return lv
}

// SetupWarehouse 创建库房与传感器。
func (e *Env) SetupWarehouse(code string) (*domain.StorageUnit, *domain.Sensor) {
	e.T.Helper()
	return e.setupUnit(code, domain.UnitWarehouse)
}

// SetupShowcase 创建展柜与传感器。
func (e *Env) SetupShowcase(code string) (*domain.StorageUnit, *domain.Sensor) {
	e.T.Helper()
	return e.setupUnit(code, domain.UnitShowcase)
}

func (e *Env) setupUnit(code, kind string) (*domain.StorageUnit, *domain.Sensor) {
	e.T.Helper()
	u, err := e.Env.CreateUnit(context.Background(), code, "单元"+code, kind, "B1")
	if err != nil {
		e.T.Fatalf("创建单元失败: %v", err)
	}
	s, err := e.Env.CreateSensor(context.Background(), code+"-S1", u.ID, "th")
	if err != nil {
		e.T.Fatalf("创建传感器失败: %v", err)
	}
	return u, s
}

// SeedQualifiedSamples 在给定时间范围内按 step 间隔写入合格采样（20℃/50%RH），覆盖前置窗口。
func (e *Env) SeedQualifiedSamples(sensorID int64, from, to, step int64) {
	e.T.Helper()
	for ts := from; ts <= to; ts += step {
		if _, err := e.Env.IngestSample(context.Background(), sensorID, 20, 50, ts); err != nil {
			e.T.Fatalf("写入采样失败: %v", err)
		}
	}
}

// SeedWindowQualified 让单元在当前时间的前置窗口内连续合格。
func (e *Env) SeedWindowQualified(sensorID int64) {
	e.T.Helper()
	now := e.Now()
	from := now - int64(e.Cfg.PreLoanWindow.Seconds())
	e.SeedQualifiedSamples(sensorID, from, now, 1800)
}

// RegisterStoredArtifact 登记并分配库位的在库藏品。
func (e *Env) RegisterStoredArtifact(code string, levelID, unitID int64) *domain.Artifact {
	e.T.Helper()
	a, err := e.Artifact.Register(context.Background(), service.RegisterInput{
		Code: code, Name: "藏品" + code, Category: "古籍", Era: "宋", LevelID: levelID,
	})
	if err != nil {
		e.T.Fatalf("登记藏品失败: %v", err)
	}
	a, err = e.Artifact.AssignLocation(context.Background(), a.ID, unitID, a.Version)
	if err != nil {
		e.T.Fatalf("分配库位失败: %v", err)
	}
	return a
}

// LoanOf 便捷创建并提交借展。
func (e *Env) LoanOf(code string, artifactIDs ...int64) *domain.LoanApplication {
	e.T.Helper()
	now := e.Now()
	l, err := e.Loan.Create(context.Background(), service.CreateLoanInput{
		Code: code, Borrower: "省博物馆", Venue: "临展厅", Purpose: "特展",
		StartAt: now + 7*86400, EndAt: now + 37*86400, ArtifactIDs: artifactIDs,
	})
	if err != nil {
		e.T.Fatalf("创建借展失败: %v", err)
	}
	l, err = e.Loan.Submit(context.Background(), l.ID, l.Version)
	if err != nil {
		e.T.Fatalf("提交借展失败: %v", err)
	}
	return l
}

// CheckItemsAllPresent 构造全部在场的清点入参（含附件）。
func (e *Env) CheckItemsAllPresent(artifactIDs ...int64) []service.CheckItemInput {
	e.T.Helper()
	out := []service.CheckItemInput{}
	for _, aid := range artifactIDs {
		atts, err := e.Artifact.ListAttachments(context.Background(), aid)
		if err != nil {
			e.T.Fatalf("查询附件失败: %v", err)
		}
		item := service.CheckItemInput{ArtifactID: aid, Present: true}
		for _, at := range atts {
			item.Attachments = append(item.Attachments, service.AttachmentPresent{AttachmentID: at.ID, Present: true})
		}
		out = append(out, item)
	}
	return out
}

// MustErr 断言返回错误且包含子串。
func MustErr(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("期望错误包含 %q，实际无错误", substr)
	}
	if substr != "" && !contains(err.Error(), substr) {
		t.Fatalf("期望错误包含 %q，实际为 %q", substr, err.Error())
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub || len(s) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// String 调试输出。
func (e *Env) String() string {
	return fmt.Sprintf("testenv@%s", e.DBPath)
}
