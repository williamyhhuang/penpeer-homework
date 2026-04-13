package usecase

import (
	"context"
	"fmt"

	"github.com/penpeer/shortlink/domain/click"
	"github.com/penpeer/shortlink/domain/shortlink"
)

// AnalyticsOutput 點擊統計輸出，供 API 序列化為 JSON
type AnalyticsOutput struct {
	Code        string                       `json:"code"`
	TotalClicks int64                        `json:"total_clicks"`
	ByPlatform  map[click.Platform]int64     `json:"by_platform"`
	ByDevice    map[click.DeviceType]int64   `json:"by_device"`
	ByRegion    map[string]int64             `json:"by_region"`
	ByReferral  map[string]int64             `json:"by_referral"`
}

// GetAnalyticsUseCase 取得短網址的點擊統計與歸因分析
type GetAnalyticsUseCase struct {
	linkRepo  shortlink.Repository
	clickRepo click.Repository
}

func NewGetAnalyticsUseCase(
	linkRepo shortlink.Repository,
	clickRepo click.Repository,
) *GetAnalyticsUseCase {
	return &GetAnalyticsUseCase{
		linkRepo:  linkRepo,
		clickRepo: clickRepo,
	}
}

func (uc *GetAnalyticsUseCase) Execute(ctx context.Context, code string) (*AnalyticsOutput, error) {
	// 先確認短碼存在
	link, err := uc.linkRepo.FindByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("查詢短網址失敗: %w", err)
	}
	if link == nil {
		return nil, fmt.Errorf("短碼不存在: %s", code)
	}

	stats, err := uc.clickRepo.GetStatsByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("查詢點擊統計失敗: %w", err)
	}

	return &AnalyticsOutput{
		Code:        code,
		TotalClicks: stats.TotalClicks,
		ByPlatform:  stats.ByPlatform,
		ByDevice:    stats.ByDeviceType,
		ByRegion:    stats.ByRegion,
		ByReferral:  stats.ByReferral,
	}, nil
}
