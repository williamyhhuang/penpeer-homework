package usecase

import (
	"context"
	"fmt"
	"log"
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
	bloom        CodeBloom // 建立成功後更新 bloom filter，確保後續 redirect 不誤判
}

func NewCreateShortLinkUseCase(
	linkRepo shortlink.Repository,
	referralRepo referral.Repository,
	cache ShortLinkCache,
	sc *scraper.OGScraper,
	bloom CodeBloom,
) *CreateShortLinkUseCase {
	return &CreateShortLinkUseCase{
		linkRepo:     linkRepo,
		referralRepo: referralRepo,
		cache:        cache,
		scraper:      sc,
		bloom:        bloom,
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

	// 3. 先建立短網址（OG 欄位暫為空），立刻持久化並回應，不等外部 HTTP 抓取
	link := &shortlink.ShortLink{
		Code:        code,
		OriginalURL: input.URL,
		CreatedAt:   time.Now(),
	}

	// 4. 存入 PostgreSQL（持久化）
	if err := uc.linkRepo.Save(ctx, link); err != nil {
		return nil, fmt.Errorf("儲存短網址失敗: %w", err)
	}

	// 5. 寫入 Redis 快取（降低後續 redirect 的 DB 查詢）
	// 快取失敗不中斷主流程，只影響效能
	_ = uc.cache.SetShortLink(ctx, link)

	// 6. 非同步抓取 OG 資料：不阻塞當前請求，背景完成後回寫 DB
	// 設計理由：OG 資料只用於 preview（Bot 訪問），不影響短網址轉導核心功能
	go uc.fetchAndUpdateOG(code, input.URL)

	// 7. 更新 bloom filter，確保後續 redirect 不會因 filter 未知此 code 而誤判為不存在
	if uc.bloom != nil {
		uc.bloom.Add(link.Code)
	}

	out := &CreateShortLinkOutput{ShortLink: link}

	// 8. 若有推薦碼需求，同時建立推薦碼
	if input.ReferralOwnerID != "" {
		refCode := &referral.ReferralCode{
			Code:          input.ReferralOwnerID,
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

// fetchAndUpdateOG 在背景 goroutine 中抓取 OG 資料並回寫 DB
// 使用獨立的 context（不繼承 request context），避免 HTTP 請求結束後 context 被取消
func (uc *CreateShortLinkUseCase) fetchAndUpdateOG(code, targetURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ogData, err := uc.scraper.Scrape(ctx, targetURL)
	if err != nil {
		log.Printf("[OG] 抓取失敗 code=%s url=%s err=%v", code, targetURL, err)
		return
	}
	if err := uc.linkRepo.UpdateOG(ctx, code, ogData.Title, ogData.Description, ogData.Image); err != nil {
		log.Printf("[OG] 回寫 DB 失敗 code=%s err=%v", code, err)
	}
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
