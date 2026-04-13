package usecase_test

import (
	"context"
	"testing"

	"github.com/penpeer/shortlink/application/usecase"
	"github.com/penpeer/shortlink/domain/click"
)

func TestGetRanking_Success(t *testing.T) {
	clickRepo := &mockClickRepo{}

	// 模擬 abc 有 3 次點擊，xyz 有 1 次點擊
	clickRepo.events = []*click.ClickEvent{
		{ShortLinkCode: "abc", Platform: click.PlatformFacebook, DeviceType: click.DeviceDesktop},
		{ShortLinkCode: "abc", Platform: click.PlatformUnknown, DeviceType: click.DeviceMobile},
		{ShortLinkCode: "abc", Platform: click.PlatformUnknown, DeviceType: click.DeviceDesktop},
		{ShortLinkCode: "xyz", Platform: click.PlatformUnknown, DeviceType: click.DeviceDesktop},
	}

	uc := usecase.NewGetRankingUseCase(clickRepo)
	items, err := uc.Execute(context.Background())

	if err != nil {
		t.Fatalf("預期成功但得到錯誤: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("預期 2 筆，got %d", len(items))
	}

	// 找出 abc 與 xyz 的結果（mock 不保證順序）
	counts := make(map[string]int64)
	for _, item := range items {
		counts[item.Code] = item.TotalClicks
	}
	if counts["abc"] != 3 {
		t.Errorf("abc 點擊數應為 3，got %d", counts["abc"])
	}
	if counts["xyz"] != 1 {
		t.Errorf("xyz 點擊數應為 1，got %d", counts["xyz"])
	}
}

func TestGetRanking_Empty(t *testing.T) {
	clickRepo := &mockClickRepo{}

	uc := usecase.NewGetRankingUseCase(clickRepo)
	items, err := uc.Execute(context.Background())

	if err != nil {
		t.Fatalf("預期成功但得到錯誤: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("無資料時應回傳空切片，got %d 筆", len(items))
	}
}
