package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/penpeer/shortlink/application/usecase"
	"github.com/penpeer/shortlink/config"
	bloomfilter "github.com/penpeer/shortlink/infrastructure/bloom"
	"github.com/penpeer/shortlink/infrastructure/postgres"
	rediscache "github.com/penpeer/shortlink/infrastructure/redis"
	"github.com/penpeer/shortlink/infrastructure/scraper"
	httpRouter "github.com/penpeer/shortlink/interfaces/http"
	"github.com/penpeer/shortlink/interfaces/http/handler"
	"github.com/penpeer/shortlink/interfaces/http/middleware"
)

func main() {
	// ── 載入非機敏設定（config/app.yaml，內嵌於 binary）────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("載入應用程式設定失敗: %v", err)
	}

	// ── 讀取機敏環境變數（.env / Docker 環境注入）────────────────────────
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
	defer func() {
		// *gorm.DB 底層是 *sql.DB，需透過 db.DB() 取得後才能 Close
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	// 執行 DB migration（由 infrastructure/postgres 層負責，符合 DDD 架構）
	if err := postgres.RunMigrations(db); err != nil {
		log.Fatalf("DB migration 失敗: %v", err)
	}

	// ── Redis ─────────────────────────────────────────────────────────────
	cache := rediscache.NewCache(redisHost, redisPort, "", 0,
		rediscache.NullCacheConfig{
			MaxKeys:    cfg.NullCache.MaxKeys,
			EvictCount: cfg.NullCache.EvictCount,
		},
		rediscache.DedupConfig{
			WindowDuration: time.Duration(cfg.ClickDedup.WindowHours) * time.Hour,
			MaxKeys:        cfg.ClickDedup.MaxKeys,
			EvictCount:     cfg.ClickDedup.EvictCount,
		},
	)
	if err := cache.Ping(context.Background()); err != nil {
		log.Fatalf("無法連線 Redis: %v", err)
	}

	// ── 依賴注入（Hexagonal Architecture 組裝）────────────────────────────
	linkRepo     := postgres.NewShortLinkRepo(db)
	referralRepo := postgres.NewReferralRepo(db)
	clickRepo    := postgres.NewClickRepo(db)
	ogScraper    := scraper.NewOGScraper()

	// ── Bloom Filter 初始化（啟動時從 DB 載入所有短碼）──────────────────────
	// 誤判率 1%：最多 1% 的不存在 code 被誤判為存在（仍需查 Redis/DB 確認）
	bloom := bloomfilter.New(uint(cfg.Bloom.Capacity), 0.01)
	if codes, err := linkRepo.FindAllCodes(context.Background()); err != nil {
		log.Printf("警告：bloom filter 初始化失敗（%v），退化為無 bloom filter 模式", err)
	} else {
		for _, code := range codes {
			bloom.Add(code)
		}
		log.Printf("Bloom filter 初始化完成，載入 %d 筆短碼", len(codes))
	}

	createUC    := usecase.NewCreateShortLinkUseCase(linkRepo, referralRepo, cache, ogScraper, bloom)
	redirectUC  := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, bloom)
	previewUC   := usecase.NewGetPreviewUseCase(linkRepo)
	analyticsUC := usecase.NewGetAnalyticsUseCase(linkRepo, clickRepo)
	rankingUC   := usecase.NewGetRankingUseCase(clickRepo)

	linkHandler     := handler.NewLinkHandler(createUC, previewUC, analyticsUC, rankingUC)
	redirectHandler := handler.NewRedirectHandler(redirectUC)

	// ── 啟動 HTTP 伺服器 ──────────────────────────────────────────────────
	rlCfg  := middleware.RateLimitConfig{RPS: cfg.RateLimit.RPS, Burst: cfg.RateLimit.Burst}
	router := httpRouter.NewRouter(linkHandler, redirectHandler, rlCfg)
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

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
