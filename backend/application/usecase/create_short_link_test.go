package usecase_test

import (
	"context"
	"testing"

	"github.com/penpeer/shortlink/application/usecase"
	"github.com/penpeer/shortlink/infrastructure/scraper"
)

func TestCreateShortLink_Success(t *testing.T) {
	linkRepo     := newMockShortLinkRepo()
	referralRepo := newMockReferralRepo()
	cache        := newMockCache()
	sc           := scraper.NewOGScraper()

	uc := usecase.NewCreateShortLinkUseCase(linkRepo, referralRepo, cache, sc)

	out, err := uc.Execute(context.Background(), usecase.CreateShortLinkInput{
		URL: "https://www.google.com",
	})

	if err != nil {
		t.Fatalf("預期成功但得到錯誤: %v", err)
	}
	if out.ShortLink == nil {
		t.Fatal("預期回傳 ShortLink，但得到 nil")
	}
	if out.ShortLink.Code == "" {
		t.Error("短碼不應為空")
	}
	if out.ShortLink.OriginalURL != "https://www.google.com" {
		t.Errorf("原始 URL 不符：got %q", out.ShortLink.OriginalURL)
	}
	if out.ReferralCode != nil {
		t.Error("未要求推薦碼，不應回傳 ReferralCode")
	}

	// 驗證短網址已存入 DB 與快取
	if _, exists := linkRepo.store[out.ShortLink.Code]; !exists {
		t.Error("短網址未存入 DB")
	}
	if _, exists := cache.store[out.ShortLink.Code]; !exists {
		t.Error("短網址未存入快取")
	}
}

func TestCreateShortLink_WithReferral(t *testing.T) {
	linkRepo     := newMockShortLinkRepo()
	referralRepo := newMockReferralRepo()
	cache        := newMockCache()
	sc           := scraper.NewOGScraper()

	uc := usecase.NewCreateShortLinkUseCase(linkRepo, referralRepo, cache, sc)

	out, err := uc.Execute(context.Background(), usecase.CreateShortLinkInput{
		URL:             "https://www.example.com",
		ReferralOwnerID: "user123",
	})

	if err != nil {
		t.Fatalf("預期成功但得到錯誤: %v", err)
	}
	if out.ReferralCode == nil {
		t.Fatal("預期回傳 ReferralCode，但得到 nil")
	}
	if out.ReferralCode.OwnerID != "user123" {
		t.Errorf("推薦碼擁有者不符：got %q", out.ReferralCode.OwnerID)
	}
}

func TestCreateShortLink_InvalidURL(t *testing.T) {
	linkRepo     := newMockShortLinkRepo()
	referralRepo := newMockReferralRepo()
	cache        := newMockCache()
	sc           := scraper.NewOGScraper()

	uc := usecase.NewCreateShortLinkUseCase(linkRepo, referralRepo, cache, sc)

	tests := []struct {
		name string
		url  string
	}{
		{"空字串", ""},
		{"無 scheme", "www.google.com"},
		{"ftp scheme", "ftp://files.example.com"},
		{"無 host", "https://"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), usecase.CreateShortLinkInput{URL: tt.url})
			if err == nil {
				t.Errorf("預期錯誤，但 %q 通過驗證", tt.url)
			}
		})
	}
}
