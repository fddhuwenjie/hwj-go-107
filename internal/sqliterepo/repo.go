// Package sqliterepo 提供 repository 接口的 SQLite 实现。
package sqliterepo

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"gowork/internal/domain"
	"gowork/internal/repository"
)

// New 基于给定 Querier 构造仓储聚合。
func New(q repository.Querier) *repository.Repositories {
	return &repository.Repositories{
		Artifacts:   &artifactRepo{q: q},
		Attachments: &attachmentRepo{q: q},
		Snapshots:   &snapshotRepo{q: q},
		Units:       &unitRepo{q: q},
		Sensors:     &sensorRepo{q: q},
		Levels:      &levelRepo{q: q},
		Rules:       &ruleRepo{q: q},
		Samples:     &sampleRepo{q: q},
		Anomalies:   &anomalyRepo{q: q},
		Disposals:   &disposalRepo{q: q},
		Loans:       &loanRepo{q: q},
		Checks:      &checkRepo{q: q},
		Handovers:   &handoverRepo{q: q},
		Acceptances: &acceptanceRepo{q: q},
		Audit:       &auditRepo{q: q},
		Jobs:        &jobRepo{q: q},
		Queries:     &queryRepo{q: q},
	}
}

// lastID 读取自增主键。
func lastID(res sql.Result) (int64, error) {
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("读取自增主键失败: %w", err)
	}
	return id, nil
}

// notFound 将 sql.ErrNoRows 映射为领域 NotFound。
func notFound(err error, what string, id any) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NotFoundf("%s %v 不存在", what, id)
	}
	return err
}

// optimistic 校验乐观锁更新是否命中。
func optimistic(res sql.Result, what string, id int64) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.Conflictf("%s %d 版本冲突或不存在", what, id)
	}
	return nil
}

// placeholders 生成 ?,?,? 占位串。
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// int64sToAny 转换参数切片。
func int64sToAny(ids []int64) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}
