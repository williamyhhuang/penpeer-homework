package usecase_test

import (
	"context"
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
