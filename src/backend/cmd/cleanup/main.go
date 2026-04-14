package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/penpeer/shortlink/application/usecase"
	"github.com/penpeer/shortlink/infrastructure/postgres"
)

func main() {
	// ── 讀取環境變數 ────────────────────────────────────────────────────────
	dbHost     := getEnv("DB_HOST",     "localhost")
	dbPort     := getEnv("DB_PORT",     "5432")
	dbUser     := getEnv("DB_USER",     "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbName     := getEnv("DB_NAME",     "shortlink")

	// 點擊事件保留天數，超過此天數的事件將被封存
	retentionDays := getEnvInt("CLICK_RETENTION_DAYS", 90)
	// 封存任務執行間隔（小時）
	intervalHours := getEnvInt("ARCHIVE_CLEANUP_INTERVAL_HOURS", 24)

	log.Printf("Cleanup 啟動：保留天數=%d 天，執行間隔=%d 小時", retentionDays, intervalHours)

	// ── 連線 PostgreSQL ────────────────────────────────────────────────────
	db, err := postgres.NewDB(dbHost, dbPort, dbUser, dbPassword, dbName)
	if err != nil {
		log.Fatalf("無法連線 PostgreSQL: %v", err)
	}
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	// ── 執行 Migration（確保 archive 表存在）────────────────────────────────
	// cleanup container 也執行 migration，確保 002_archive.sql 的封存表已建立
	// RunMigrations 採 IF NOT EXISTS，與 server 同時執行也安全
	if err := postgres.RunMigrations(db); err != nil {
		log.Fatalf("DB migration 失敗: %v", err)
	}

	// ── 組裝 Use Cases ───────────────────────────────────────────────────
	archiveRepo   := postgres.NewArchiveRepo(db)
	archiveLinks  := usecase.NewArchiveExpiredLinksUseCase(archiveRepo)
	archiveClicks := usecase.NewArchiveOldClicksUseCase(archiveRepo, retentionDays)

	// ── 執行封存的主函式 ──────────────────────────────────────────────────
	runArchive := func() {
		ctx := context.Background()

		// Step 1：先封存過期 short_links（帶走其 referral_codes + click_events）
		linkCount, err := archiveLinks.Execute(ctx)
		if err != nil {
			log.Printf("[ERROR] 封存過期短網址失敗: %v", err)
		} else {
			log.Printf("[OK] 封存過期短網址 %d 筆", linkCount)
		}

		// Step 2：再封存超過保留天數的舊點擊事件（屬於 active short_links 的舊資料）
		clickCount, err := archiveClicks.Execute(ctx)
		if err != nil {
			log.Printf("[ERROR] 封存舊點擊事件失敗: %v", err)
		} else {
			log.Printf("[OK] 封存舊點擊事件 %d 筆", clickCount)
		}
	}

	// 啟動時立即執行一次（不等第一個 interval）
	log.Println("啟動時執行首次封存...")
	runArchive()

	// ── 定時執行 ─────────────────────────────────────────────────────────
	ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			log.Println("定時封存開始...")
			runArchive()
		case <-quit:
			log.Println("收到關機信號，cleanup 正常退出")
			return
		}
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("警告：環境變數 %s=%q 無法解析為整數，使用預設值 %d", key, v, defaultVal)
		return defaultVal
	}
	return n
}
