package postgres

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// NewDB 建立並驗證 PostgreSQL 連線池
func NewDB(host, port, user, password, dbname string) (*sqlx.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("連線 PostgreSQL 失敗: %w", err)
	}
	// 設定連線池，避免過多閒置連線
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	return db, nil
}
