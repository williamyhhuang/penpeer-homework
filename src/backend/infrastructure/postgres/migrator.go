package postgres

import (
	"embed"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
)

// migrationFS 嵌入 infrastructure 層的 SQL migration 檔案
// 路徑相對於本檔案所在目錄（infrastructure/postgres/）
//
//go:embed migrations/*.sql
var migrationFS embed.FS

// RunMigrations 讀取內嵌的 SQL 檔案並依序執行
// 將 migration 邏輯放在 infrastructure 層，符合 DDD 六角架構：
// DB schema 屬於 Secondary/Driven Adapter，不應由 cmd（Primary Adapter）持有
func RunMigrations(db *sqlx.DB) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("讀取 migration 目錄失敗: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		sqlBytes, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("讀取 %s 失敗: %w", entry.Name(), err)
		}
		if _, err := db.Exec(string(sqlBytes)); err != nil {
			return fmt.Errorf("執行 %s 失敗: %w", entry.Name(), err)
		}
		log.Printf("Migration 執行完成: %s", entry.Name())
	}
	return nil
}
