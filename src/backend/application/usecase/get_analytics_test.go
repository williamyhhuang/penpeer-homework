package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/penpeer/shortlink/application/usecase"
	"github.com/penpeer/shortlink/domain/click"
	"github.com/penpeer/shortlink/domain/shortlink"
)

func TestGetAnalytics_Success(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}

	link := &shortlink.ShortLink{
		Code:      "ana123",
		OriginalURL: "https://www.example.com",
		CreatedAt: time.Now(),
	}
	_ = linkRepo.Save(context.Background(), link)

	// 模擬 3 筆點擊事件
	clickRepo.events = []*click.ClickEvent{
		{ShortLinkCode: "ana123", Platform: click.PlatformFacebook, DeviceType: click.DeviceBot,     Region: "TW", ReferralCode: "ref1"},
		{ShortLinkCode: "ana123", Platform: click.PlatformUnknown,  DeviceType: click.DeviceDesktop, Region: "US"},
		{ShortLinkCode: "ana123", Platform: click.PlatformUnknown,  DeviceType: click.DeviceMobile,  Region: "TW"},
	}

	uc := usecase.NewGetAnalyticsUseCase(linkRepo, clickRepo)

	out, err := uc.Execute(context.Background(), "ana123")
	if err != nil {
		t.Fatalf("預期成功但得到錯誤: %v", err)
	}
	if out.TotalClicks != 3 {
		t.Errorf("總點擊數應為 3，got %d", out.TotalClicks)
	}
	if out.ByPlatform[click.PlatformFacebook] != 1 {
		t.Error("Facebook 點擊數應為 1")
	}
	if out.ByRegion["TW"] != 2 {
		t.Errorf("TW 點擊數應為 2，got %d", out.ByRegion["TW"])
	}
	if out.ByReferral["ref1"] != 1 {
		t.Error("推薦碼 ref1 應有 1 筆")
	}
}

func TestGetAnalytics_NotFound(t *testing.T) {
	linkRepo  := newMockShortLinkRepo()
	clickRepo := &mockClickRepo{}

	uc := usecase.NewGetAnalyticsUseCase(linkRepo, clickRepo)

	_, err := uc.Execute(context.Background(), "notexist")
	if err == nil {
		t.Error("短碼不存在時應回傳錯誤")
	}
}
