package sqlite_test

import (
	"path/filepath"
	"testing"

	"gowork/internal/sqlite"
)

// TestFilePersistence 数据写入真实文件，关闭后重开仍存在（重启恢复的基础）。
func TestFilePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")
	db, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO preservation_levels (code, name, created_at) VALUES ('LV-1', '一级', 1)`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db2, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	var name string
	if err := db2.QueryRow(`SELECT name FROM preservation_levels WHERE code='LV-1'`).Scan(&name); err != nil {
		t.Fatalf("重开后数据丢失: %v", err)
	}
	if name != "一级" {
		t.Fatalf("数据不符: %s", name)
	}
}

// TestRejectMemory 禁止使用 :memory:。
func TestRejectMemory(t *testing.T) {
	if _, err := sqlite.Open(":memory:"); err == nil {
		t.Fatal("应拒绝 :memory:")
	}
}
