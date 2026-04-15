package usecase_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/penpeer/shortlink/application/usecase"
	"github.com/penpeer/shortlink/domain/click"
	"github.com/penpeer/shortlink/domain/shortlink"
	"github.com/penpeer/shortlink/infrastructure/scraper"
)

// TestClickWorkerPool_Shutdown_DrainsQueue 驗證 Shutdown() 會等待佇列中所有任務完成才返回
func TestClickWorkerPool_Shutdown_DrainsQueue(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}
	cache     := newMockCache()

	_ = linkRepo.Save(context.Background(), &shortlink.ShortLink{
		Code:        "drain1",
		OriginalURL: "https://example.com",
		CreatedAt:   time.Now(),
	})

	// 1 worker，佇列 20，確保多筆任務都能進入佇列而不被丟棄
	uc := usecase.NewRedirectShortLinkUseCase(
		linkRepo, clickRepo, cache, nil,
		usecase.ClickWorkerConfig{Workers: 1, QueueSize: 20},
	)

	// 送出多筆請求（不同 IP 確保不被去重過濾掉）
	const requests = 5
	for i := 0; i < requests; i++ {
		ip := "10.0.0." + string(rune('1'+i))
		_, _ = uc.Execute(context.Background(), usecase.RedirectInput{
			Code:        "drain1",
			UserAgent:   "Mozilla/5.0",
			ClientIP:    ip,
			CFIPCountry: "TW",
		})
	}

	// Shutdown 必須等到所有已入佇列的任務處理完才返回（不應 panic 或 hang）
	done := make(chan struct{})
	go func() {
		uc.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// 正常完成
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown() 在 3 秒內未完成，可能 goroutine 洩漏")
	}

	if clickRepo.Len() < 1 {
		t.Errorf("Shutdown 後 worker 應已完成，預期至少 1 筆點擊，實際 %d 筆", clickRepo.Len())
	}
}

// TestClickWorkerPool_QueueFull_DropsClick 驗證佇列滿時新任務被丟棄，不開新 goroutine
func TestClickWorkerPool_QueueFull_DropsClick(t *testing.T) {
	linkRepo := newMockShortLinkRepo()
	cache    := newMockCache()

	_ = linkRepo.Save(context.Background(), &shortlink.ShortLink{
		Code:        "full1",
		OriginalURL: "https://example.com",
		CreatedAt:   time.Now(),
	})

	// blockRepo：Save() 卡住直到 unblock，模擬高延遲 DB
	var blockMu sync.Mutex
	blockMu.Lock() // 初始鎖定，worker 第一個任務會卡住
	blockRepo := &blockingClickRepo{mu: &blockMu}

	// 1 worker（會被第一個任務卡住），佇列大小 1
	// 結果：最多 1（worker 卡住處理中）+ 1（佇列等待）= 2 個任務被接受，其他被丟棄
	uc := usecase.NewRedirectShortLinkUseCase(
		linkRepo, blockRepo, cache, nil,
		usecase.ClickWorkerConfig{Workers: 1, QueueSize: 1},
	)

	// 送出 5 個不同 IP 的請求（繞過去重）
	for i := 0; i < 5; i++ {
		ip := "192.168.1." + string(rune('1'+i))
		_, _ = uc.Execute(context.Background(), usecase.RedirectInput{
			Code:        "full1",
			UserAgent:   "Mozilla/5.0",
			ClientIP:    ip,
			CFIPCountry: "TW",
		})
	}

	// 解除 worker 阻塞
	blockMu.Unlock()

	done := make(chan struct{})
	go func() {
		uc.Shutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown() 超時")
	}

	// 最多儲存 2 筆（1 worker 卡住 + 1 佇列），其餘 3 筆應被丟棄
	saved := blockRepo.saveCount.Load()
	if saved > 2 {
		t.Errorf("有界 pool（1 worker + queue 1）最多應儲存 2 筆，實際儲存 %d 筆", saved)
	}
}

// TestOGSemaphore_Shutdown_NoHang 驗證 CreateUseCase.Shutdown() 在 OG goroutine 進行中能正常返回
func TestOGSemaphore_Shutdown_NoHang(t *testing.T) {
	linkRepo     := newMockShortLinkRepo()
	referralRepo := newMockReferralRepo()
	cache        := newMockCache()

	// 用真實 scraper（連線會快速失敗，10ms 內結束）
	sc := scraper.NewOGScraper()

	uc := usecase.NewCreateShortLinkUseCase(
		linkRepo, referralRepo, cache, sc, nil,
		usecase.OGWorkerConfig{Concurrency: 5},
	)

	for i := 0; i < 3; i++ {
		_, _ = uc.Execute(context.Background(), usecase.CreateShortLinkInput{
			URL: "https://localhost:19999", // 不存在的 port，scraper 快速失敗
		})
	}

	done := make(chan struct{})
	go func() {
		uc.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// 正常完成
	case <-time.After(15 * time.Second):
		t.Fatal("CreateUseCase.Shutdown() 超時，可能 OG goroutine 洩漏")
	}
}

// ── 輔助 mock ─────────────────────────────────────────────────────────────────

// blockingClickRepo Save() 會等待 mu 解除，用於測試 worker 被阻塞的情境
type blockingClickRepo struct {
	mu        *sync.Mutex
	saveCount atomic.Int64
}

func (r *blockingClickRepo) Save(_ context.Context, _ *click.ClickEvent) error {
	r.mu.Lock()
	r.saveCount.Add(1)
	r.mu.Unlock()
	return nil
}

func (r *blockingClickRepo) GetRanking(_ context.Context) ([]click.CodeRanking, error) {
	return nil, nil
}

func (r *blockingClickRepo) GetStatsByCode(_ context.Context, _ string) (*click.ClickStats, error) {
	return &click.ClickStats{
		ByPlatform:   make(map[click.Platform]int64),
		ByDeviceType: make(map[click.DeviceType]int64),
		ByRegion:     make(map[string]int64),
		ByReferral:   make(map[string]int64),
	}, nil
}
