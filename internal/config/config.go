// Package config 负责从环境变量加载服务配置。
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config 服务配置。
type Config struct {
	// Port HTTP 监听端口，来自 PORT。
	Port int
	// DBPath SQLite 文件路径，来自 DB_PATH，禁止为 :memory:。
	DBPath string
	// LogLevel 日志级别，来自 LOG_LEVEL，默认 info。
	LogLevel string
	// PreLoanWindow 借展前置环境合格窗口，默认 72 小时。
	PreLoanWindow time.Duration
	// EnvMaxGap 前置窗口内允许的最大采样间隔，默认 60 分钟。
	EnvMaxGap time.Duration
	// HandoverTimeout 交接超时阈值，默认 48 小时。
	HandoverTimeout time.Duration
	// JobInterval 后台作业调度周期，默认 5 秒。
	JobInterval time.Duration
	// JobMaxAttempts 作业最大尝试次数，默认 5。
	JobMaxAttempts int
	// ShutdownTimeout 优雅关闭超时，默认 10 秒。
	ShutdownTimeout time.Duration
}

// Load 从环境变量加载配置。
func Load() (Config, error) {
	cfg := Config{
		Port:            8080,
		LogLevel:        "info",
		PreLoanWindow:   72 * time.Hour,
		EnvMaxGap:       time.Hour,
		HandoverTimeout: 48 * time.Hour,
		JobInterval:     5 * time.Second,
		JobMaxAttempts:  5,
		ShutdownTimeout: 10 * time.Second,
	}
	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p <= 0 || p > 65535 {
			return Config{}, fmt.Errorf("非法端口 PORT=%q", v)
		}
		cfg.Port = p
	}
	cfg.DBPath = os.Getenv("DB_PATH")
	if cfg.DBPath == "" {
		return Config{}, fmt.Errorf("缺少必需环境变量 DB_PATH")
	}
	if cfg.DBPath == ":memory:" {
		return Config{}, fmt.Errorf("DB_PATH 不允许为 :memory:，必须持久化到文件")
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	var err error
	if cfg.PreLoanWindow, err = durationEnv("PRE_LOAN_WINDOW_HOURS", cfg.PreLoanWindow, time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.EnvMaxGap, err = durationEnv("ENV_MAX_GAP_MINUTES", cfg.EnvMaxGap, time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.HandoverTimeout, err = durationEnv("HANDOVER_TIMEOUT_HOURS", cfg.HandoverTimeout, time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.JobInterval, err = durationEnv("JOB_INTERVAL_SECONDS", cfg.JobInterval, time.Second); err != nil {
		return Config{}, err
	}
	if v := os.Getenv("JOB_MAX_ATTEMPTS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return Config{}, fmt.Errorf("非法 JOB_MAX_ATTEMPTS=%q", v)
		}
		cfg.JobMaxAttempts = n
	}
	return cfg, nil
}

// durationEnv 读取以 unit 为单位的时长环境变量。
func durationEnv(key string, def time.Duration, unit time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("非法时长配置 %s=%q", key, v)
	}
	return time.Duration(n * float64(unit)), nil
}
