package shortlink

import "context"

// Repository 定義短網址的儲存介面（Hexagonal Architecture Port）
type Repository interface {
	Save(ctx context.Context, link *ShortLink) error
	FindByCode(ctx context.Context, code string) (*ShortLink, error)
	// FindAllCodes 取得所有短碼，僅用於啟動時初始化 bloom filter
	FindAllCodes(ctx context.Context) ([]string, error)
	// UpdateOG 僅更新 OG 欄位，由非同步 scraper goroutine 呼叫
	// 設計成獨立方法避免 Save 的 ON CONFLICT DO UPDATE 覆蓋已有資料
	UpdateOG(ctx context.Context, code, title, description, image string) error
}
