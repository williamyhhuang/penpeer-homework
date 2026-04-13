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
			Timeout: 10 * time.Second,
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
func parseOGFromHTML(resp *http.Response) (*OGData, error) {
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return &OGData{}, fmt.Errorf("解析 HTML 失敗: %w", err)
	}

	og := &OGData{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// 只需要掃描 <head> 內的 <meta> 標籤，找到所有 OG 資料後即可停止
		if n.Type == html.ElementNode && n.Data == "meta" {
			var property, content string
			for _, attr := range n.Attr {
				switch strings.ToLower(attr.Key) {
				case "property":
					property = strings.ToLower(attr.Val)
				case "name":
					// 相容 name="description" 的寫法
					if property == "" {
						property = strings.ToLower(attr.Val)
					}
				case "content":
					content = attr.Val
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
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return og, nil
}
