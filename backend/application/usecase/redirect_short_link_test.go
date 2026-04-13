package usecase_test

import (
	"context"
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

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache)

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

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache)

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

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache)

	_, err := uc.Execute(context.Background(), usecase.RedirectInput{
		Code:      "notexist",
		UserAgent: "Mozilla/5.0",
	})

	if err == nil {
		t.Error("短碼不存在時應回傳錯誤")
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

	uc := usecase.NewRedirectShortLinkUseCase(linkRepo, clickRepo, cache)

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
