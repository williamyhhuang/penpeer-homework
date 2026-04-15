package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/penpeer/shortlink/application/geoip"
	"github.com/penpeer/shortlink/application/uadetect"
	"github.com/penpeer/shortlink/domain/click"
	"github.com/penpeer/shortlink/domain/shortlink"
	"github.com/penpeer/shortlink/infrastructure/metrics"
)

// RedirectCache 定義 redirect 路徑所需的快取介面
// GetShortLink 回傳 (nil, shortlink.ErrNullCache) 代表此 code 已被標記不存在，不需查 DB
type RedirectCache interface {
	GetShortLink(ctx context.Context, code string) (*shortlink.ShortLink, error)
	SetShortLink(ctx context.Context, link *shortlink.ShortLink) error
	SetNullCache(ctx context.Context, code string) error
	// IsNewClick 判斷此次點擊是否為去重窗口內的首次點擊。
	// 首次點擊回傳 true 並設立去重標記；重複點擊回傳 false。
	// Redis 故障時回傳 (true, err)：寬鬆策略，讓點擊通過以避免漏計。
	IsNewClick(ctx context.Context, code, fingerprint string) (bool, error)
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

// clickJob click worker pool 的任務資料包
type clickJob struct {
	shortLinkCode string
	detected      uadetect.DetectResult
	region        string
	referralCode  string
	clientIP      string
	userAgent     string
}

// ClickWorkerConfig click worker pool 設定
type ClickWorkerConfig struct {
	// Workers worker goroutine 數量（固定大小 pool）
	Workers int
	// QueueSize 任務緩衝佇列大小，超過時丟棄並計入 dropped 指標
	QueueSize int
}

// RedirectShortLinkUseCase 實作「短網址重新導向」的核心業務邏輯
type RedirectShortLinkUseCase struct {
	linkRepo  shortlink.Repository
	clickRepo click.Repository
	cache     RedirectCache
	bloom     CodeBloom // 第一道篩查：確定不存在的 code 直接拒絕，不查 Redis 也不查 DB
	// sfGroup 防止快取擊穿：同一 code 的並發 miss 只允許一個 goroutine 查 DB
	sfGroup singleflight.Group
	// clickChan / clickWg：有界 worker pool，避免每次轉導都開新 goroutine
	clickChan chan clickJob
	clickWg   sync.WaitGroup
}

func NewRedirectShortLinkUseCase(
	linkRepo shortlink.Repository,
	clickRepo click.Repository,
	cache RedirectCache,
	bloom CodeBloom,
	opts ...ClickWorkerConfig,
) *RedirectShortLinkUseCase {
	cfg := ClickWorkerConfig{Workers: 10, QueueSize: 500} // 預設值
	if len(opts) > 0 {
		if opts[0].Workers > 0 {
			cfg.Workers = opts[0].Workers
		}
		if opts[0].QueueSize > 0 {
			cfg.QueueSize = opts[0].QueueSize
		}
	}
	uc := &RedirectShortLinkUseCase{
		linkRepo:  linkRepo,
		clickRepo: clickRepo,
		cache:     cache,
		bloom:     bloom,
		clickChan: make(chan clickJob, cfg.QueueSize),
	}
	for i := 0; i < cfg.Workers; i++ {
		uc.clickWg.Add(1)
		go func() {
			defer uc.clickWg.Done()
			for job := range uc.clickChan {
				uc.asyncSaveClick(job.shortLinkCode, job.detected, job.region, job.referralCode, job.clientIP, job.userAgent)
			}
		}()
	}
	return uc
}

// Shutdown 優雅停機：關閉任務佇列，等待所有 worker 處理完佇列中的任務後退出
func (uc *RedirectShortLinkUseCase) Shutdown() {
	close(uc.clickChan)
	uc.clickWg.Wait()
}

func (uc *RedirectShortLinkUseCase) Execute(ctx context.Context, input RedirectInput) (*RedirectOutput, error) {
	// 0. Bloom filter 第一道篩查：確定不存在的 code 直接拒絕
	// false → 肯定不在 DB，無需查 Redis 或 DB（防快取穿透 + 降低無效請求成本）
	if uc.bloom != nil && !uc.bloom.MightExist(input.Code) {
		metrics.BloomFilterChecks.WithLabelValues("reject").Inc()
		metrics.RedirectTotal.WithLabelValues("bloom_miss").Inc()
		return nil, fmt.Errorf("短碼不存在: %s", input.Code)
	}
	metrics.BloomFilterChecks.WithLabelValues("pass").Inc()

	// 1. 優先查 Redis 快取（降低 redirect 延遲，這是效能關鍵路徑）
	link, err := uc.cache.GetShortLink(ctx, input.Code)

	// 快取穿透防護：此 code 已被標記不存在，直接拒絕不查 DB
	if errors.Is(err, shortlink.ErrNullCache) {
		metrics.CacheHitTotal.WithLabelValues("get", "null_cache").Inc()
		metrics.RedirectTotal.WithLabelValues("not_found").Inc()
		return nil, fmt.Errorf("短碼不存在: %s", input.Code)
	}
	if err != nil {
		// 其他快取讀取錯誤不中斷，fallback 到 DB
		metrics.CacheHitTotal.WithLabelValues("get", "error").Inc()
		link = nil
	} else if link != nil {
		metrics.CacheHitTotal.WithLabelValues("get", "hit").Inc()
		metrics.RedirectTotal.WithLabelValues("redis_hit").Inc()
	} else {
		metrics.CacheHitTotal.WithLabelValues("get", "miss").Inc()
	}

	// 2. Cache miss → 用 singleflight 查 PostgreSQL
	// 防止快取擊穿：同一 code 的並發 miss 只觸發一次 DB 查詢，其餘等待共享結果
	if link == nil {
		val, sfErr, shared := uc.sfGroup.Do(input.Code, func() (any, error) {
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
		if shared {
			// 此請求共享了其他 goroutine 的 DB 查詢結果（singleflight 去重生效）
			metrics.SingleflightDedup.Inc()
		}
		if sfErr != nil {
			metrics.RedirectTotal.WithLabelValues("not_found").Inc()
			return nil, sfErr
		}
		metrics.RedirectTotal.WithLabelValues("db_hit").Inc()
		link = val.(*shortlink.ShortLink)
	}

	// 3. 檢查是否過期
	if link.IsExpired() {
		metrics.RedirectTotal.WithLabelValues("expired").Inc()
		return nil, fmt.Errorf("短網址已過期: %s", input.Code)
	}

	// 4. 偵測 User-Agent
	detected := uadetect.Detect(input.UserAgent)
	region := uadetect.ExtractRegion(input.CFIPCountry, input.XCountry)

	// 5. 非同步寫入 ClickEvent（不阻塞 redirect 回應）
	// 送入有界 worker pool；佇列已滿時丟棄並計入 dropped 指標（高流量保護）
	job := clickJob{
		shortLinkCode: link.Code,
		detected:      detected,
		region:        region,
		referralCode:  input.ReferralCode,
		clientIP:      input.ClientIP,
		userAgent:     input.UserAgent,
	}
	select {
	case uc.clickChan <- job:
	default:
		metrics.ClicksRecordedTotal.WithLabelValues("dropped").Inc()
	}

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
	region, referralCode, clientIP, userAgent string,
) {
	// 使用獨立 context，避免 HTTP request context 被取消後 goroutine 無法寫入
	ctx := context.Background()

	// 去重判斷：同一 fingerprint 在窗口內對同一短碼只計一次點擊
	fingerprint := fingerprintKey(clientIP, userAgent)
	isNew, err := uc.cache.IsNewClick(ctx, shortLinkCode, fingerprint)
	if err != nil {
		// Redis 故障記錄 log，但採寬鬆策略繼續寫入（寧多計不漏計）
		log.Printf("[dedup] IsNewClick 失敗，採寬鬆策略放行 code=%s: %v", shortLinkCode, err)
	}
	if !isNew {
		// 重複點擊，靜默跳過，不寫 DB
		return
	}

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
	if err := uc.clickRepo.Save(ctx, event); err != nil {
		metrics.ClicksRecordedTotal.WithLabelValues("error").Inc()
	} else {
		metrics.ClicksRecordedTotal.WithLabelValues("success").Inc()
	}
}

// fingerprintKey 將訪客身份轉換為去重用的不可逆 fingerprint。
// 優先使用 clientIP；Bot 常無 IP，改用 userAgent 前 32 字元作為 seed。
// SHA-256 截前 8 bytes（16 hex 字元）：單向不可逆，碰撞空間 2^64，key 長度短。
func fingerprintKey(clientIP, userAgent string) string {
	seed := clientIP
	if seed == "" {
		if len(userAgent) > 32 {
			seed = userAgent[:32]
		} else {
			seed = userAgent
		}
	}
	h := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(h[:8])
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
