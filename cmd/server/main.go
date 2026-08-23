// Command server 古籍文物库房环境异常处置与借展交接服务入口。
// 读取 PORT 与 DB_PATH，单进程 HTTP JSON 服务，SQLite 文件持久化。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gowork/internal/audit"
	"gowork/internal/clock"
	"gowork/internal/config"
	"gowork/internal/httpx"
	"gowork/internal/jobs"
	"gowork/internal/service"
	"gowork/internal/sqlite"
	"gowork/internal/sqliterepo"
	"gowork/internal/tx"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "启动失败:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := newLogger(cfg.LogLevel)
	clk := clock.Real{}

	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	log.Info("数据库已连接", "path", cfg.DBPath)

	repo := sqliterepo.New(db)
	txm := tx.NewManager(db, sqliterepo.New)
	aud := audit.NewRecorder(clk)
	deps := &service.Deps{Repo: repo, Tx: txm, Clock: clk, Audit: aud, Cfg: cfg}

	srv := &httpx.Server{
		Artifacts: service.NewArtifactService(deps),
		Env:       service.NewEnvService(deps),
		Anomalies: service.NewAnomalyService(deps),
		Loans:     service.NewLoanService(deps),
		Checks:    service.NewCheckService(deps),
		Handovers: service.NewHandoverService(deps),
		Returns:   service.NewReturnService(deps),
		Queries:   service.NewQueryService(deps),
		Repo:      repo,
		DB:        db,
		Clock:     clk,
		Log:       log,
	}

	scheduler := jobs.NewScheduler(repo, clk, cfg, log, map[string]jobs.Handler{
		"patrol":           jobs.PatrolHandler(srv.Anomalies),
		"loan_due":         jobs.LoanDueHandler(txm, clk, aud),
		"handover_timeout": jobs.HandoverTimeoutHandler(txm, clk, aud, int64(cfg.HandoverTimeout.Seconds())),
	})
	jobCtx, stopJobs := context.WithCancel(context.Background())
	defer stopJobs()
	go scheduler.Run(jobCtx)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpx.NewHandler(srv),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("HTTP 服务启动", "port", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("HTTP 服务异常: %w", err)
	case sig := <-sigCh:
		log.Info("收到退出信号，开始优雅关闭", "signal", sig.String())
	}
	stopJobs()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("优雅关闭失败: %w", err)
	}
	log.Info("服务已关闭")
	return nil
}

// newLogger 构造结构化 JSON 日志。
func newLogger(level string) *slog.Logger {
	var lv slog.Level
	switch level {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lv}))
}
