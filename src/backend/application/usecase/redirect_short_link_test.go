package usecase_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/penpeer/shortlink/application/usecase"
	"github.com/penpeer/shortlink/domain/shortlink"
)

func TestRedirect_NormalUser_Returns302(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}
	cache     := newMockCache()

	// 預先放入一筆短網址
	link := &shortlink.ShortLink{
		Code:        "abc1234",
		OriginalURL: "https://www.google.com",
		CreatedAt:   time.Now(),
	}
	_ = linkRepo.Save(context.Background(), link)

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, nil)

	out, err := uc.Execute(context.Background(), usecase.RedirectInput{
		Code:      "abc1234",
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
	})

	if err != nil {
		t.Fatalf("預期成功但得到錯誤: %v", err)
	}
	if out.IsBot {
		t.Error("一般瀏覽器不應被判定為 Bot")
	}
	if out.OriginalURL != "https://www.google.com" {
		t.Errorf("原始 URL 不符：got %q", out.OriginalURL)
	}
}

func TestRedirect_FacebookBot_ReturnsOGHTML(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}
	cache     := newMockCache()

	link := &shortlink.ShortLink{
		Code:          "abc1234",
		OriginalURL:   "https://www.example.com",
		OGTitle:       "Test Title",
		OGDescription: "Test Description",
		OGImage:       "https://example.com/image.jpg",
		CreatedAt:     time.Now(),
	}
	_ = linkRepo.Save(context.Background(), link)

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, nil)

	out, err := uc.Execute(context.Background(), usecase.RedirectInput{
		Code:      "abc1234",
		UserAgent: "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uagent.php)",
	})

	if err != nil {
		t.Fatalf("預期成功但得到錯誤: %v", err)
	}
	if !out.IsBot {
		t.Error("Facebook Bot 應被判定為 Bot")
	}
	if out.OGHTML == "" {
		t.Error("Bot 應回傳 OG HTML")
	}
}

func TestRedirect_NotFound(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}
	cache     := newMockCache()

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, nil)

	_, err := uc.Execute(context.Background(), usecase.RedirectInput{
		Code:      "notexist",
		UserAgent: "Mozilla/5.0",
	})

	if err == nil {
		t.Error("短碼不存在時應回傳錯誤")
	}
}

// TestRedirect_NullCache_SkipsDB 驗證快取穿透防護：
// null cache 命中時不應再查 DB，且應直接回傳錯誤
func TestRedirect_NullCache_SkipsDB(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}
	cache     := newMockCache()

	// 預先把 "ghost" 標記為 null cache（模擬之前已確認 DB 不存在）
	_ = cache.SetNullCache(context.Background(), "ghost")

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, nil)

	_, err := uc.Execute(context.Background(), usecase.RedirectInput{
		Code:      "ghost",
		UserAgent: "Mozilla/5.0",
	})

	if err == nil {
		t.Fatal("null cache 命中時應回傳錯誤")
	}
	if linkRepo.findCalls.Load() != 0 {
		t.Errorf("null cache 命中時不應查 DB，但 FindByCode 被呼叫了 %d 次", linkRepo.findCalls.Load())
	}
}

// TestRedirect_Singleflight_DeduplicatesDB 驗證快取擊穿防護：
// 10 個並發請求同時 cache miss，FindByCode 應只被呼叫 1 次
func TestRedirect_Singleflight_DeduplicatesDB(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}
	cache     := newMockCache()

	link := &shortlink.ShortLink{
		Code:        "hot",
		OriginalURL: "https://hot.example.com",
		CreatedAt:   time.Now(),
	}
	_ = linkRepo.Save(context.Background(), link)
	// 不寫入 cache，讓所有請求都 cache miss

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, nil)

	const concurrent = 10
	var wg sync.WaitGroup
	wg.Add(concurrent)
	for i := 0; i < concurrent; i++ {
		go func() {
			defer wg.Done()
			_, _ = uc.Execute(context.Background(), usecase.RedirectInput{
				Code:      "hot",
				UserAgent: "Mozilla/5.0",
			})
		}()
	}
	wg.Wait()

	calls := linkRepo.findCalls.Load()
	if calls != 1 {
		t.Errorf("singleflight 應讓 FindByCode 只被呼叫 1 次，實際呼叫了 %d 次", calls)
	}
}

func TestRedirect_CacheHit(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}
	cache     := newMockCache()

	// 只存快取，不存 DB → 測試快取優先路徑
	link := &shortlink.ShortLink{
		Code:        "cached1",
		OriginalURL: "https://cached.example.com",
		CreatedAt:   time.Now(),
	}
	_ = cache.SetShortLink(context.Background(), link)

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, nil)

	out, err := uc.Execute(context.Background(), usecase.RedirectInput{
		Code:      "cached1",
		UserAgent: "Mozilla/5.0",
	})

	if err != nil {
		t.Fatalf("快取命中時應成功: %v", err)
	}
	if out.OriginalURL != "https://cached.example.com" {
		t.Errorf("URL 不符：got %q", out.OriginalURL)
	}
}

// TestDedup_SameIPSameCode_CountedOnce 同一 IP 對同一短碼連點兩次，DB 只應寫入 1 筆
func TestDedup_SameIPSameCode_CountedOnce(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}
	cache     := newMockCache()

	link := &shortlink.ShortLink{
		Code:        "dedup1",
		OriginalURL: "https://example.com",
		CreatedAt:   time.Now(),
	}
	_ = linkRepo.Save(context.Background(), link)

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, nil)

	input := usecase.RedirectInput{
		Code:        "dedup1",
		UserAgent:   "Mozilla/5.0",
		ClientIP:    "192.168.1.1", // 私有 IP，GeoIP 查詢會直接跳過，不發外部請求
		CFIPCountry: "TW",          // 直接提供地區，避免 GeoIP fallback 延遲
	}

	// 第一次點擊
	_, _ = uc.Execute(context.Background(), input)
	// 第二次點擊（相同 IP，相同 code）
	_, _ = uc.Execute(context.Background(), input)

	// asyncSaveClick 是 goroutine，需等待它完成
	time.Sleep(50 * time.Millisecond)

	if len(clickRepo.events) != 1 {
		t.Errorf("同 IP 同 code 連點兩次，DB 應只有 1 筆，實際有 %d 筆", len(clickRepo.events))
	}
}

// TestDedup_SameIPDifferentCode_CountedSeparately 同一 IP 對兩個不同短碼各點一次，應各計一筆
func TestDedup_SameIPDifferentCode_CountedSeparately(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}
	cache     := newMockCache()

	for _, code := range []string{"codeA", "codeB"} {
		_ = linkRepo.Save(context.Background(), &shortlink.ShortLink{
			Code:        code,
			OriginalURL: "https://example.com/" + code,
			CreatedAt:   time.Now(),
		})
	}

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, nil)

	_, _ = uc.Execute(context.Background(), usecase.RedirectInput{
		Code: "codeA", UserAgent: "Mozilla/5.0", ClientIP: "192.168.1.1", CFIPCountry: "TW",
	})
	_, _ = uc.Execute(context.Background(), usecase.RedirectInput{
		Code: "codeB", UserAgent: "Mozilla/5.0", ClientIP: "192.168.1.1", CFIPCountry: "TW",
	})

	time.Sleep(50 * time.Millisecond)

	if len(clickRepo.events) != 2 {
		t.Errorf("同 IP 點兩個不同 code，應有 2 筆，實際有 %d 筆", len(clickRepo.events))
	}
}

// TestDedup_DifferentIPSameCode_BothCounted 兩個不同 IP 對同一短碼各點一次，應各計一筆
func TestDedup_DifferentIPSameCode_BothCounted(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}
	cache     := newMockCache()

	_ = linkRepo.Save(context.Background(), &shortlink.ShortLink{
		Code:        "shared",
		OriginalURL: "https://example.com/shared",
		CreatedAt:   time.Now(),
	})

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, nil)

	_, _ = uc.Execute(context.Background(), usecase.RedirectInput{
		Code: "shared", UserAgent: "Mozilla/5.0", ClientIP: "192.168.0.1", CFIPCountry: "TW",
	})
	_, _ = uc.Execute(context.Background(), usecase.RedirectInput{
		Code: "shared", UserAgent: "Mozilla/5.0", ClientIP: "192.168.0.2", CFIPCountry: "TW",
	})

	time.Sleep(50 * time.Millisecond)

	if len(clickRepo.events) != 2 {
		t.Errorf("不同 IP 點同一 code，應有 2 筆，實際有 %d 筆", len(clickRepo.events))
	}
}

// TestDedup_EmptyIP_UsesUAFingerprint ClientIP 為空時（Bot 常見），同 UA 兩次點擊只計一次
func TestDedup_EmptyIP_UsesUAFingerprint(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}
	cache     := newMockCache()

	_ = linkRepo.Save(context.Background(), &shortlink.ShortLink{
		Code:        "botcode",
		OriginalURL: "https://example.com/bot",
		CreatedAt:   time.Now(),
	})

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, nil)

	botUA := "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uagent.php)"
	input := usecase.RedirectInput{
		Code:      "botcode",
		UserAgent: botUA,
		ClientIP:  "", // Bot 沒有 ClientIP
	}

	_, _ = uc.Execute(context.Background(), input)
	_, _ = uc.Execute(context.Background(), input) // 第二次，相同 UA，相同 code

	time.Sleep(50 * time.Millisecond)

	if len(clickRepo.events) != 1 {
		t.Errorf("相同 Bot UA 點同一 code 兩次，DB 應只有 1 筆，實際有 %d 筆", len(clickRepo.events))
	}
}

// TestDedup_RedisError_AllowsThrough IsNewClick 回傳錯誤時（Redis 故障），應採寬鬆策略放行寫入
func TestDedup_RedisError_AllowsThrough(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}
	cache     := newMockCache()
	// 注入 Redis 錯誤
	cache.dedupError = fmt.Errorf("redis: connection refused")

	_ = linkRepo.Save(context.Background(), &shortlink.ShortLink{
		Code:        "errcode",
		OriginalURL: "https://example.com/err",
		CreatedAt:   time.Now(),
	})

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache, nil)

	_, _ = uc.Execute(context.Background(), usecase.RedirectInput{
		Code: "errcode", UserAgent: "Mozilla/5.0", ClientIP: "192.168.1.1", CFIPCountry: "TW",
	})
	_, _ = uc.Execute(context.Background(), usecase.RedirectInput{
		Code: "errcode", UserAgent: "Mozilla/5.0", ClientIP: "192.168.1.1", CFIPCountry: "TW",
	})

	time.Sleep(50 * time.Millisecond)

	// Redis 故障時寬鬆放行，兩次點擊都應寫入（寧多計不漏計）
	if len(clickRepo.events) != 2 {
		t.Errorf("Redis 故障時應放行所有點擊，預期 2 筆，實際有 %d 筆", len(clickRepo.events))
	}
}
