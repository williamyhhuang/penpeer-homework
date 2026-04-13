package usecase

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/penpeer/shortlink/application/codegen"
	"github.com/penpeer/shortlink/domain/referral"
	"github.com/penpeer/shortlink/domain/shortlink"
	"github.com/penpeer/shortlink/infrastructure/scraper"
)

// ShortLinkCache 定義快取操作介面，避免 Use Case 直接依賴 Redis 實作
type ShortLinkCache interface {
	SetShortLink(ctx context.Context, link *shortlink.ShortLink) error
}

// CreateShortLinkInput 建立短網址的輸入參數
type CreateShortLinkInput struct {
	URL             string
	ReferralOwnerID string // 若非空，同時建立推薦碼
}

// CreateShortLinkOutput 建立短網址的輸出結果
type CreateShortLinkOutput struct {
	ShortLink    *shortlink.ShortLink
	ReferralCode *referral.ReferralCode // 若有推薦碼則填入
}

// CreateShortLinkUseCase 實作「建立短網址」的業務邏輯
type CreateShortLinkUseCase struct {
	linkRepo     shortlink.Repository
	referralRepo referral.Repository
	cache        ShortLinkCache
	scraper      *scraper.OGScraper
}

func NewCreateShortLinkUseCase(
	linkRepo shortlink.Repository,
	referralRepo referral.Repository,
	cache ShortLinkCache,
	sc *scraper.OGScraper,
) *CreateShortLinkUseCase {
	return &CreateShortLinkUseCase{
		linkRepo:     linkRepo,
		referralRepo: referralRepo,
		cache:        cache,
		scraper:      sc,
	}
}

func (uc *CreateShortLinkUseCase) Execute(ctx context.Context, input CreateShortLinkInput) (*CreateShortLinkOutput, error) {
	// 1. 驗證 URL 格式
	if err := validateURL(input.URL); err != nil {
		return nil, fmt.Errorf("無效的 URL: %w", err)
	}

	// 2. 產生唯一短碼（理論上極少碰撞，簡化版本不做 retry）
	code, err := codegen.GenerateCode()
	if err != nil {
		return nil, fmt.Errorf("產生短碼失敗: %w", err)
	}

	// 3. 抓取 OG 資料（建立時同步抓取，Bot 來時直接讀 DB，不重複抓）
	ogData, err := uc.scraper.Scrape(ctx, input.URL)
	if err != nil {
		// OG 抓取失敗不中斷主流程，使用空值
		ogData = &scraper.OGData{}
	}

	// 4. 建立短網址 Domain 物件
	link := &shortlink.ShortLink{
		Code:          code,
		OriginalURL:   input.URL,
		OGTitle:       ogData.Title,
		OGDescription: ogData.Description,
		OGImage:       ogData.Image,
		CreatedAt:     time.Now(),
	}

	// 5. 存入 PostgreSQL（持久化）
	if err := uc.linkRepo.Save(ctx, link); err != nil {
		return nil, fmt.Errorf("儲存短網址失敗: %w", err)
	}

	// 6. 寫入 Redis 快取（降低後續 redirect 的 DB 查詢）
	// 快取失敗不中斷主流程，只影響效能
	_ = uc.cache.SetShortLink(ctx, link)

	out := &CreateShortLinkOutput{ShortLink: link}

	// 7. 若有推薦碼需求，同時建立推薦碼
	if input.ReferralOwnerID != "" {
		refCode := &referral.ReferralCode{
			Code:          fmt.Sprintf("%s-%s", input.ReferralOwnerID, code),
			OwnerID:       input.ReferralOwnerID,
			ShortLinkCode: code,
			CreatedAt:     time.Now(),
		}
		if err := uc.referralRepo.Save(ctx, refCode); err != nil {
			// 推薦碼建立失敗不中斷，短網址已成功建立
			return out, nil
		}
		out.ReferralCode = refCode
	}

	return out, nil
}

// validateURL 驗證 URL 必須為有效的 http/https 格式
func validateURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("僅支援 http/https 協定，收到: %s", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("URL 缺少 host")
	}
	return nil
}
