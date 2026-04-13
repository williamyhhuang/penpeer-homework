package click

import "time"

// Platform 代表點擊來源的社群平台
type Platform string

const (
	PlatformFacebook Platform = "facebook"
	PlatformTwitter  Platform = "twitter"
	PlatformLinkedIn Platform = "linkedin"
	PlatformTelegram Platform = "telegram"
	PlatformWhatsApp Platform = "whatsapp"
	PlatformSlack    Platform = "slack"
	PlatformDiscord  Platform = "discord"
	PlatformUnknown  Platform = "unknown"
)

// DeviceType 裝置類型
type DeviceType string

const (
	DeviceBot     DeviceType = "bot"
	DeviceMobile  DeviceType = "mobile"
	DeviceDesktop DeviceType = "desktop"
)

// ClickEvent 記錄每次短網址點擊事件（不可變值物件）
type ClickEvent struct {
	ID            int64
	ShortLinkCode string
	ClickedAt     time.Time
	Platform      Platform   // 偵測到的社群平台
	Region        string     // 地區（從 IP 或 Header 推斷）
	DeviceType    DeviceType // Bot / 手機 / 桌機
	ReferralCode  string     // ?ref= 帶入的推薦碼，可為空
}

// ClickStats 點擊統計彙整，供 Analytics API 回傳
type ClickStats struct {
	TotalClicks  int64
	ByPlatform   map[Platform]int64
	ByDeviceType map[DeviceType]int64
	ByRegion     map[string]int64
	ByReferral   map[string]int64
}
