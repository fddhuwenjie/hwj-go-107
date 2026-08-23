package sqlite

import (
	"database/sql"
	"fmt"
)

// Migrate 执行 schema 迁移。当前为幂等建表，后续可演进为版本化迁移。
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("schema 迁移失败: %w", err)
	}
	return nil
}
