package postgres

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewDB 建立並驗證 PostgreSQL 連線池，回傳 *gorm.DB
func NewDB(host, port, user, password, dbname string) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		// 生產環境只記錄 Warn 以上，避免 SQL 查詢日誌淹沒 log
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("連線 PostgreSQL 失敗: %w", err)
	}

	// GORM 底層仍是 database/sql，透過 db.DB() 設定連線池參數
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("取得底層 sql.DB 失敗: %w", err)
	}
	// 設定連線池，避免過多閒置連線
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	return db, nil
}
