package usecase

import (
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/penpeer/shortlink/application/uadetect"
	"github.com/penpeer/shortlink/domain/click"
	"github.com/penpeer/shortlink/domain/shortlink"
)

// RedirectCache 定義 redirect 路徑所需的快取介面
type RedirectCache interface {
	GetShortLink(ctx context.Context, code string) (*shortlink.ShortLink, error)
	SetShortLink(ctx context.Context, link *shortlink.ShortLink) error
}

// RedirectInput redirect 請求的輸入資訊
type RedirectInput struct {
	Code         string
	UserAgent    string
	ReferralCode string // 從 ?ref= query param 取得
	CFIPCountry  string // Cloudflare header
	XCountry     string // 其他 CDN header
}

// RedirectOutput redirect 決策結果
type RedirectOutput struct {
	IsBot       bool
	OriginalURL string
	OGHTML      string // 社群 Bot 專用的 OG HTML
}

// RedirectShortLinkUseCase 實作「短網址重新導向」的核心業務邏輯
type RedirectShortLinkUseCase struct {
	linkRepo   shortlink.Repository
	clickRepo  click.Repository
	cache      RedirectCache
}

func NewRedirectShortLinkUseCase(
	linkRepo shortlink.Repository,
	clickRepo click.Repository,
	cache RedirectCache,
) *RedirectShortLinkUseCase {
	return &RedirectShortLinkUseCase{
		linkRepo:  linkRepo,
		clickRepo: clickRepo,
		cache:     cache,
	}
}

func (uc *RedirectShortLinkUseCase) Execute(ctx context.Context, input RedirectInput) (*RedirectOutput, error) {
	// 1. 優先查 Redis 快取（降低 redirect 延遲，這是效能關鍵路徑）
	link, err := uc.cache.GetShortLink(ctx, input.Code)
	if err != nil {
		// 快取讀取失敗不中斷，fallback 到 DB
		link = nil
	}

	// 2. Cache miss → 查 PostgreSQL
	if link == nil {
		link, err = uc.linkRepo.FindByCode(ctx, input.Code)
		if err != nil {
			return nil, fmt.Errorf("查詢短網址失敗: %w", err)
		}
		if link == nil {
			return nil, fmt.Errorf("短碼不存在: %s", input.Code)
		}
		// 回填快取，加速後續請求
		_ = uc.cache.SetShortLink(ctx, link)
	}

	// 3. 檢查是否過期
	if link.IsExpired() {
		return nil, fmt.Errorf("短網址已過期: %s", input.Code)
	}

	// 4. 偵測 User-Agent
	detected := uadetect.Detect(input.UserAgent)
	region := uadetect.ExtractRegion(input.CFIPCountry, input.XCountry)

	// 5. 非同步寫入 ClickEvent（不阻塞 redirect 回應）
	go uc.asyncSaveClick(link.Code, detected, region, input.ReferralCode)

	// 6. 社群 Bot → 回傳含 OG meta tags 的 HTML，讓平台顯示預覽
	if detected.IsBot {
		ogHTML := buildOGHTML(link)
		return &RedirectOutput{IsBot: true, OriginalURL: link.OriginalURL, OGHTML: ogHTML}, nil
	}

	// 7. 一般使用者 → 302 redirect 到原始 URL
	return &RedirectOutput{IsBot: false, OriginalURL: link.OriginalURL}, nil
}

// asyncSaveClick 在獨立 goroutine 中寫入點擊事件，避免阻塞 redirect
func (uc *RedirectShortLinkUseCase) asyncSaveClick(
	shortLinkCode string,
	detected uadetect.DetectResult,
	region, referralCode string,
) {
	// 使用獨立 context，避免 HTTP request context 被取消後 goroutine 無法寫入
	ctx := context.Background()
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
