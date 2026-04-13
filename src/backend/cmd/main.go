package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/penpeer/shortlink/application/usecase"
	"github.com/penpeer/shortlink/infrastructure/postgres"
	rediscache "github.com/penpeer/shortlink/infrastructure/redis"
	"github.com/penpeer/shortlink/infrastructure/scraper"
	httpRouter "github.com/penpeer/shortlink/interfaces/http"
	"github.com/penpeer/shortlink/interfaces/http/handler"
)

// embed 路徑相對於本檔案所在目錄（cmd/）
//
//go:embed migrations/*.sql
var migrationFS embed.FS

func main() {
	// ── 讀取環境變數 ──────────────────────────────────────────────────────
	dbHost     := getEnv("DB_HOST",     "localhost")
	dbPort     := getEnv("DB_PORT",     "5432")
	dbUser     := getEnv("DB_USER",     "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbName     := getEnv("DB_NAME",     "shortlink")
	redisHost  := getEnv("REDIS_HOST",  "localhost")
	redisPort  := getEnv("REDIS_PORT",  "6379")
	serverPort := getEnv("SERVER_PORT", "8080")

	// ── PostgreSQL ────────────────────────────────────────────────────────
	db, err := postgres.NewDB(dbHost, dbPort, dbUser, dbPassword, dbName)
	if err != nil {
		log.Fatalf("無法連線 PostgreSQL: %v", err)
	}
	defer db.Close()

	// 執行 DB migration（讀取內嵌 SQL 自動建表）
	if err := runMigrations(db, migrationFS); err != nil {
		log.Fatalf("DB migration 失敗: %v", err)
	}

	// ── Redis ─────────────────────────────────────────────────────────────
	cache := rediscache.NewCache(redisHost, redisPort, "", 0)
	if err := cache.Ping(context.Background()); err != nil {
		log.Fatalf("無法連線 Redis: %v", err)
	}

	// ── 依賴注入（Hexagonal Architecture 組裝）────────────────────────────
	linkRepo     := postgres.NewShortLinkRepo(db)
	referralRepo := postgres.NewReferralRepo(db)
	clickRepo    := postgres.NewClickRepo(db)
	ogScraper    := scraper.NewOGScraper()

	createUC    := usecase.NewCreateShortLinkUseCase(linkRepo, referralRepo, cache, ogScraper)
	redirectUC  := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache)
	previewUC   := usecase.NewGetPreviewUseCase(linkRepo)
	analyticsUC := usecase.NewGetAnalyticsUseCase(linkRepo, clickRepo)
	rankingUC   := usecase.NewGetRankingUseCase(clickRepo)

	linkHandler     := handler.NewLinkHandler(createUC, previewUC, analyticsUC, rankingUC)
	redirectHandler := handler.NewRedirectHandler(redirectUC)

	// ── 啟動 HTTP 伺服器 ──────────────────────────────────────────────────
	router := httpRouter.NewRouter(linkHandler, redirectHandler)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", serverPort),
		Handler: router,
	}

	// 優雅關機：接收 SIGINT/SIGTERM 後等待進行中的請求完成
	go func() {
		log.Printf("伺服器啟動於 :%s", serverPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("伺服器錯誤: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在優雅關機...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("強制關機: %v", err)
	}
	log.Println("伺服器已關閉")
}

// runMigrations 讀取內嵌的 SQL 檔案並依序執行
func runMigrations(db *sqlx.DB, fs embed.FS) error {
	entries, err := fs.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("讀取 migration 目錄失敗: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		sqlBytes, err := fs.ReadFile("migrations/" + entry.Name())
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

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
