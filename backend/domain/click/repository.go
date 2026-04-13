package click

import "context"

// Repository 定義點擊事件的儲存介面
type Repository interface {
	Save(ctx context.Context, event *ClickEvent) error
	GetStatsByCode(ctx context.Context, shortLinkCode string) (*ClickStats, error)
}
