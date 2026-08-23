// Package sqlite 负责打开嵌入式 SQLite 连接并执行 schema 迁移。
package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动，无 CGO
)

// Open 打开 SQLite 数据库文件并执行迁移。
// path 必须为真实文件路径，禁止 :memory:。
func Open(path string) (*sql.DB, error) {
	if path == "" || path == ":memory:" {
		return nil, fmt.Errorf("DB_PATH 必须为真实文件路径，禁止 :memory:")
	}
	// 开启外键与 WAL，busy_timeout 避免单写者冲突直接报错。
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	// SQLite 单写者，限制连接数避免 database is locked。
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	if err := Migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
