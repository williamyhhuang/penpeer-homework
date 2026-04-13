package shortlink

import "time"

// ShortLink 是短網址的核心聚合根，包含 OG 預覽資料
type ShortLink struct {
	Code          string     // 短碼（主鍵），例如 "aB3xYz"
	OriginalURL   string     // 原始長網址
	OGTitle       string     // 從原始頁面抓取的 OG 標題
	OGDescription string     // 從原始頁面抓取的 OG 描述
	OGImage       string     // 從原始頁面抓取的 OG 圖片 URL
	CreatedAt     time.Time
	ExpiresAt     *time.Time // nil 表示永不過期
}

// IsExpired 判斷短網址是否已過期
func (s *ShortLink) IsExpired() bool {
	if s.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*s.ExpiresAt)
}

// HasOGData 判斷是否有 OG 資料可用於社群預覽
func (s *ShortLink) HasOGData() bool {
	return s.OGTitle != "" || s.OGDescription != "" || s.OGImage != ""
}
