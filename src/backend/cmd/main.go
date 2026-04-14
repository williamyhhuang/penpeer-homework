package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/penpeer/shortlink/application/usecase"
	bloomfilter "github.com/penpeer/shortlink/infrastructure/bloom"
	"github.com/penpeer/shortlink/infrastructure/postgres"
	rediscache "github.com/penpeer/shortlink/infrastructure/redis"
	"github.com/penpeer/shortlink/infrastructure/scraper"
	httpRouter "github.com/penpeer/shortlink/interfaces/http"
	"github.com/penpeer/shortlink/interfaces/http/handler"
	"github.com/penpeer/shortlink/interfaces/http/middleware"
)

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

	// null cache 水位管控設定
	nullCacheMaxKeys   := getEnvInt("NULL_CACHE_MAX_KEYS",   10_000)
	nullCacheEvictCnt  := getEnvInt("NULL_CACHE_EVICT_COUNT", 1_000)

	// rate limit 設定（per-IP，redirect 路徑）
	rateLimitRPS   := getEnvInt("RATE_LIMIT_RPS",   30)
	rateLimitBurst := getEnvInt("RATE_LIMIT_BURST",  60)

	// bloom filter 設定
	bloomCapacity := getEnvInt("BLOOM_CAPACITY", 1_000_000)

	// ── PostgreSQL ────────────────────────────────────────────────────────
	db, err := postgres.NewDB(dbHost, dbPort, dbUser, dbPassword, dbName)
	if err != nil {
		log.Fatalf("無法連線 PostgreSQL: %v", err)
	}
	defer db.Close()

	// 執行 DB migration（由 infrastructure/postgres 層負責，符合 DDD 架構）
	if err := postgres.RunMigrations(db); err != nil {
		log.Fatalf("DB migration 失敗: %v", err)
	}

	// ── Redis ─────────────────────────────────────────────────────────────
	cache := rediscache.NewCache(redisHost, redisPort, "", 0, rediscache.NullCacheConfig{
		MaxKeys:    int64(nullCacheMaxKeys),
		EvictCount: int64(nullCacheEvictCnt),
	})
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
	bloom := bloomfilter.New(uint(bloomCapacity), 0.01)
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
	rlCfg  := middleware.RateLimitConfig{RPS: rateLimitRPS, Burst: rateLimitBurst}
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

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
