// Package tx 提供真实 SQLite 事务管理：Within 内所有仓储操作要么整体提交，要么整体回滚。
package tx

import (
	"context"
	"database/sql"
	"fmt"

	"gowork/internal/repository"
)

// Manager 事务管理器。
type Manager struct {
	db       *sql.DB
	newRepos func(q repository.Querier) *repository.Repositories
}

// NewManager 构造事务管理器。
func NewManager(db *sql.DB, newRepos func(q repository.Querier) *repository.Repositories) *Manager {
	return &Manager{db: db, newRepos: newRepos}
}

// Within 在单个 SQLite 事务内执行 fn；fn 返回错误则回滚，否则提交。
func (m *Manager) Within(ctx context.Context, fn func(r *repository.Repositories) error) (err error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(m.newRepos(tx)); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}
