package click

import "context"

// Repository 定義點擊事件的儲存介面
type Repository interface {
	Save(ctx context.Context, event *ClickEvent) error
	GetStatsByCode(ctx context.Context, shortLinkCode string) (*ClickStats, error)
	// GetRanking 回傳所有短碼依點擊數降冪排序的排行榜（含 0 點擊的短碼）
	GetRanking(ctx context.Context) ([]CodeRanking, error)
}
