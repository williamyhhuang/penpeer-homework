package scraper

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// OGData 代表從網頁抓取的 Open Graph 元資料
type OGData struct {
	Title       string
	Description string
	Image       string
}

// OGScraper 負責抓取網頁 OG 資料（建立短網址時執行一次，結果存 DB）
type OGScraper struct {
	client *http.Client
}

func NewOGScraper() *OGScraper {
	return &OGScraper{
		client: &http.Client{
			// OG 抓取為非同步背景工作，縮短 timeout 避免 goroutine 長時間洩漏
			Timeout: 5 * time.Second,
			// 不追蹤重新導向超過 3 次，避免無限循環
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("超過重新導向次數限制")
				}
				return nil
			},
		},
	}
}

// Scrape 抓取指定 URL 的 OG 資料
// 若抓取失敗不中斷主流程，回傳空 OGData
func (s *OGScraper) Scrape(ctx context.Context, targetURL string) (*OGData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return &OGData{}, fmt.Errorf("建立 HTTP 請求失敗: %w", err)
	}
	// 模擬瀏覽器 User-Agent，部分網站會封鎖爬蟲
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PenpeerBot/1.0; +https://penpeer.com)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.client.Do(req)
	if err != nil {
		return &OGData{}, fmt.Errorf("HTTP 請求失敗: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &OGData{}, fmt.Errorf("HTTP 狀態碼異常: %d", resp.StatusCode)
	}

	return parseOGFromHTML(resp)
}

// parseOGFromHTML 解析 HTML 的 <meta property="og:*"> 標籤
// 優化：利用 tokenizer 串流讀取，遇到 </head> 立即停止，不讀取整個 body
func parseOGFromHTML(resp *http.Response) (*OGData, error) {
	og := &OGData{}
	z := html.NewTokenizer(resp.Body)

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			// EOF 或讀取錯誤，直接回傳已蒐集的資料
			return og, nil

		case html.EndTagToken:
			name, _ := z.TagName()
			// 遇到 </head> 代表 meta 標籤已全部出現，提前結束掃描
			if strings.EqualFold(string(name), "head") {
				return og, nil
			}

		case html.SelfClosingTagToken, html.StartTagToken:
			name, hasAttr := z.TagName()
			if !hasAttr || !strings.EqualFold(string(name), "meta") {
				continue
			}
			var property, content string
			for {
				key, val, more := z.TagAttr()
				k := strings.ToLower(string(key))
				v := string(val)
				switch k {
				case "property":
					property = strings.ToLower(v)
				case "name":
					// 相容 name="description" 的寫法
					if property == "" {
						property = strings.ToLower(v)
					}
				case "content":
					content = v
				}
				if !more {
					break
				}
			}
			switch property {
			case "og:title":
				og.Title = content
			case "og:description":
				og.Description = content
			case "og:image":
				og.Image = content
			case "description":
				// 若沒有 og:description，fallback 到一般 description
				if og.Description == "" {
					og.Description = content
				}
			}
		}
	}
}
