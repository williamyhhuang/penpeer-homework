package usecase

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/penpeer/shortlink/application/geoip"
	"github.com/penpeer/shortlink/application/uadetect"
	"github.com/penpeer/shortlink/domain/click"
	"github.com/penpeer/shortlink/domain/shortlink"
)

// RedirectCache 定義 redirect 路徑所需的快取介面
// GetShortLink 回傳 (nil, shortlink.ErrNullCache) 代表此 code 已被標記不存在，不需查 DB
type RedirectCache interface {
	GetShortLink(ctx context.Context, code string) (*shortlink.ShortLink, error)
	SetShortLink(ctx context.Context, link *shortlink.ShortLink) error
	SetNullCache(ctx context.Context, code string) error
}

// RedirectInput redirect 請求的輸入資訊
type RedirectInput struct {
	Code         string
	UserAgent    string
	ReferralCode string // 從 ?ref= query param 取得
	CFIPCountry  string // Cloudflare header
	XCountry     string // 其他 CDN header
	ClientIP     string // 用戶端真實 IP（CDN Header 缺席時的 GeoIP fallback）
}

// RedirectOutput redirect 決策結果
type RedirectOutput struct {
	IsBot       bool
	OriginalURL string
	OGHTML      string // 社群 Bot 專用的 OG HTML
}

// RedirectShortLinkUseCase 實作「短網址重新導向」的核心業務邏輯
type RedirectShortLinkUseCase struct {
	linkRepo  shortlink.Repository
	clickRepo click.Repository
	cache     RedirectCache
	bloom     CodeBloom // 第一道篩查：確定不存在的 code 直接拒絕，不查 Redis 也不查 DB
	// sfGroup 防止快取擊穿：同一 code 的並發 miss 只允許一個 goroutine 查 DB
	sfGroup singleflight.Group
}

func NewRedirectShortLinkUseCase(
	linkRepo shortlink.Repository,
	clickRepo click.Repository,
	cache RedirectCache,
	bloom CodeBloom,
) *RedirectShortLinkUseCase {
	return &RedirectShortLinkUseCase{
		linkRepo:  linkRepo,
		clickRepo: clickRepo,
		cache:     cache,
		bloom:     bloom,
	}
}

func (uc *RedirectShortLinkUseCase) Execute(ctx context.Context, input RedirectInput) (*RedirectOutput, error) {
	// 0. Bloom filter 第一道篩查：確定不存在的 code 直接拒絕
	// false → 肯定不在 DB，無需查 Redis 或 DB（防快取穿透 + 降低無效請求成本）
	if uc.bloom != nil && !uc.bloom.MightExist(input.Code) {
		return nil, fmt.Errorf("短碼不存在: %s", input.Code)
	}

	// 1. 優先查 Redis 快取（降低 redirect 延遲，這是效能關鍵路徑）
	link, err := uc.cache.GetShortLink(ctx, input.Code)

	// 快取穿透防護：此 code 已被標記不存在，直接拒絕不查 DB
	if errors.Is(err, shortlink.ErrNullCache) {
		return nil, fmt.Errorf("短碼不存在: %s", input.Code)
	}
	if err != nil {
		// 其他快取讀取錯誤不中斷，fallback 到 DB
		link = nil
	}

	// 2. Cache miss → 用 singleflight 查 PostgreSQL
	// 防止快取擊穿：同一 code 的並發 miss 只觸發一次 DB 查詢，其餘等待共享結果
	if link == nil {
		val, sfErr, _ := uc.sfGroup.Do(input.Code, func() (any, error) {
			l, e := uc.linkRepo.FindByCode(ctx, input.Code)
			if e != nil {
				return nil, fmt.Errorf("查詢短網址失敗: %w", e)
			}
			if l == nil {
				// 短碼不存在，寫入 null cache 防止後續請求重複查 DB
				_ = uc.cache.SetNullCache(ctx, input.Code)
				return nil, fmt.Errorf("短碼不存在: %s", input.Code)
			}
			// 回填快取，加速後續請求
			_ = uc.cache.SetShortLink(ctx, l)
			return l, nil
		})
		if sfErr != nil {
			return nil, sfErr
		}
		link = val.(*shortlink.ShortLink)
	}

	// 3. 檢查是否過期
	if link.IsExpired() {
		return nil, fmt.Errorf("短網址已過期: %s", input.Code)
	}

	// 4. 偵測 User-Agent
	detected := uadetect.Detect(input.UserAgent)
	region := uadetect.ExtractRegion(input.CFIPCountry, input.XCountry)

	// 5. 非同步寫入 ClickEvent（不阻塞 redirect 回應）
	go uc.asyncSaveClick(link.Code, detected, region, input.ReferralCode, input.ClientIP)

	// 6. 社群 Bot → 回傳含 OG meta tags 的 HTML，讓平台顯示預覽
	if detected.IsBot {
		ogHTML := buildOGHTML(link)
		return &RedirectOutput{IsBot: true, OriginalURL: link.OriginalURL, OGHTML: ogHTML}, nil
	}

	// 7. 一般使用者 → 302 redirect 到原始 URL
	return &RedirectOutput{IsBot: false, OriginalURL: link.OriginalURL}, nil
}

// asyncSaveClick 在獨立 goroutine 中寫入點擊事件，避免阻塞 redirect
// 當 CDN Header 沒有帶 region 時（本機開發 / 非 Cloudflare 環境），以 clientIP 呼叫
// ip-api.com GeoIP 取得國碼作為 fallback，確保地區分布統計有資料
func (uc *RedirectShortLinkUseCase) asyncSaveClick(
	shortLinkCode string,
	detected uadetect.DetectResult,
	region, referralCode, clientIP string,
) {
	// 使用獨立 context，避免 HTTP request context 被取消後 goroutine 無法寫入
	ctx := context.Background()

	// CDN Header 未帶地區（非 Cloudflare / 本機環境），嘗試用 IP 查詢國碼
	if region == "" && clientIP != "" {
		region = geoip.LookupCountry(ctx, clientIP)
	}

	event := &click.ClickEvent{
		ShortLinkCode: shortLinkCode,
		ClickedAt:     time.Now(),
		Platform:      detected.Platform,
		Region:        region,
		DeviceType:    detected.DeviceType,
		ReferralCode:  referralCode,
	}
	_ = uc.clickRepo.Save(ctx, event)
}

// ogHTMLTmpl 社群 Bot 回傳的 HTML 模板
// 包含 OG meta tags 與即時 redirect，讓社群平台顯示預覽，一般使用者自動跳轉
const ogHTMLTmpl = `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta property="og:title"       content="{{.Title}}">
  <meta property="og:description" content="{{.Description}}">
  <meta property="og:image"       content="{{.Image}}">
  <meta property="og:url"         content="{{.URL}}">
  <meta http-equiv="refresh"      content="0;url={{.URL}}">
  <title>{{.Title}}</title>
</head>
<body>
  <p>正在跳轉至 <a href="{{.URL}}">{{.URL}}</a>...</p>
</body>
</html>`

var ogTmpl = template.Must(template.New("og").Parse(ogHTMLTmpl))

func buildOGHTML(link *shortlink.ShortLink) string {
	data := struct {
		Title       string
		Description string
		Image       string
		URL         string
	}{
		Title:       link.OGTitle,
		Description: link.OGDescription,
		Image:       link.OGImage,
		URL:         link.OriginalURL,
	}
	var sb strings.Builder
	_ = ogTmpl.Execute(&sb, data)
	return sb.String()
}
