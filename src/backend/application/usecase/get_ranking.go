package usecase

import (
	"context"
	"fmt"

	"github.com/penpeer/shortlink/domain/click"
)

// RankingItem 排行榜單一項目的輸出格式
type RankingItem struct {
	Rank        int    `json:"rank"`
	Code        string `json:"code"`
	OriginalURL string `json:"original_url"`
	TotalClicks int64  `json:"total_clicks"`
}

// GetRankingUseCase 取得所有短碼的點擊數排行榜
type GetRankingUseCase struct {
	clickRepo click.Repository
}

func NewGetRankingUseCase(clickRepo click.Repository) *GetRankingUseCase {
	return &GetRankingUseCase{clickRepo: clickRepo}
}

func (uc *GetRankingUseCase) Execute(ctx context.Context) ([]RankingItem, error) {
	rankings, err := uc.clickRepo.GetRanking(ctx)
	if err != nil {
		return nil, fmt.Errorf("查詢排行榜失敗: %w", err)
	}

	items := make([]RankingItem, len(rankings))
	for i, r := range rankings {
		items[i] = RankingItem{
			Rank:        r.Rank,
			Code:        r.Code,
			OriginalURL: r.OriginalURL,
			TotalClicks: r.TotalClicks,
		}
	}
	return items, nil
}
