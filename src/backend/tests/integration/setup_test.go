//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/penpeer/shortlink/application/usecase"
	bloomfilter "github.com/penpeer/shortlink/infrastructure/bloom"
	"github.com/penpeer/shortlink/infrastructure/postgres"
	rediscache "github.com/penpeer/shortlink/infrastructure/redis"
	"github.com/penpeer/shortlink/infrastructure/scraper"
	httpRouter "github.com/penpeer/shortlink/interfaces/http"
	"github.com/penpeer/shortlink/interfaces/http/handler"
	"github.com/penpeer/shortlink/interfaces/http/middleware"
	"gorm.io/gorm"
)

// testEnv 整合測試共享環境，TestMain 初始化後供所有測試使用
var testEnv struct {
	server      *httptest.Server
	db          *gorm.DB
	pgClickRepo *postgres.ClickRepo // 直接查 DB 驗證 click 事件用
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	// ── 啟動 PostgreSQL container ──────────────────────────────────────────
	pgReq := testcontainers.ContainerRequest{
		Image:        "postgres:18.3",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "shortlink",
		},
		// postgres:18.3 在 kernel 4.9 的 Docker Desktop 需放寬 seccomp 限制，否則無法啟動
		// 與 docker-compose.yml 的 security_opt: seccomp:unconfined 設定對應
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.SecurityOpt = []string{"seccomp:unconfined"}
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections").
				WithPollInterval(500*time.Millisecond).
				WithStartupTimeout(60*time.Second),
		),
	}
	pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: pgReq,
		Started:          true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "無法啟動 PostgreSQL container: %v\n", err)
		os.Exit(1)
	}
	defer pgContainer.Terminate(ctx) //nolint:errcheck

	pgHost, _ := pgContainer.Host(ctx)
	pgPort, _ := pgContainer.MappedPort(ctx, "5432/tcp")

	// ── 啟動 Redis container ───────────────────────────────────────────────
	redisReq := testcontainers.ContainerRequest{
		Image:        "redis:7.2.7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: redisReq,
		Started:          true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "無法啟動 Redis container: %v\n", err)
		os.Exit(1)
	}
	defer redisContainer.Terminate(ctx) //nolint:errcheck

	redisHost, _ := redisContainer.Host(ctx)
	redisPort, _ := redisContainer.MappedPort(ctx, "6379/tcp")

	// ── 建立 DB 連線並執行 migration ────────────────────────────────────────
	db, err := postgres.NewDB(pgHost, pgPort.Port(), "postgres", "postgres", "shortlink")
	if err != nil {
		fmt.Fprintf(os.Stderr, "無法連線 PostgreSQL: %v\n", err)
		os.Exit(1)
	}
	if err := postgres.RunMigrations(db); err != nil {
		fmt.Fprintf(os.Stderr, "Migration 失敗: %v\n", err)
		os.Exit(1)
	}
	testEnv.db = db

	// ── 建立 Redis 快取 ─────────────────────────────────────────────────────
	// 整合測試不需要 null cache 水位管控，設較小值即可
	cache := rediscache.NewCache(redisHost, redisPort.Port(), "", 0, rediscache.NullCacheConfig{
		MaxKeys:    100,
		EvictCount: 10,
	}, rediscache.DedupConfig{
		WindowDuration: 0, // 整合測試不限制去重時間窗口
		MaxKeys:        0, // 停用水位管控
		EvictCount:     0,
	})
	if err := cache.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "無法連線 Redis: %v\n", err)
		os.Exit(1)
	}

	// ── 依賴注入 ────────────────────────────────────────────────────────────
	linkRepo     := postgres.NewShortLinkRepo(db)
	referralRepo := postgres.NewReferralRepo(db)
	clickRepo    := postgres.NewClickRepo(db)
	ogScraper    := scraper.NewOGScraper()

	testEnv.pgClickRepo = clickRepo

	// Bloom filter：使用 Redis 分散式版本，與正式環境一致
	bloom := bloomfilter.NewRedis(cache.Client(), 10000, 0.01)

	// 整合測試 rate limit 設極高，避免連續請求被封鎖
	rlCfg := middleware.RateLimitConfig{RPS: 10000, Burst: 10000}

	createUC    := usecase.NewCreateShortLinkUseCase(linkRepo, referralRepo, cache, ogScraper, bloom)
	redirectUC  := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, bloom)
	previewUC   := usecase.NewGetPreviewUseCase(linkRepo)
	analyticsUC := usecase.NewGetAnalyticsUseCase(linkRepo, clickRepo)
	rankingUC   := usecase.NewGetRankingUseCase(clickRepo)

	linkHandler     := handler.NewLinkHandler(createUC, previewUC, analyticsUC, rankingUC)
	redirectHandler := handler.NewRedirectHandler(redirectUC)

	router := httpRouter.NewRouter(linkHandler, redirectHandler, rlCfg)

	// ── 啟動 httptest server ─────────────────────────────────────────────────
	testEnv.server = httptest.NewServer(router)
	defer testEnv.server.Close()

	os.Exit(m.Run())
}
