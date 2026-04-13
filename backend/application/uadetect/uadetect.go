package uadetect

import (
	"strings"

	"github.com/penpeer/shortlink/domain/click"
)

// 社群平台 Bot 的 User-Agent 關鍵字對應表
// 順序重要：越具體的放越前面避免誤判
var botSignatures = []struct {
	keyword  string
	platform click.Platform
}{
	{"facebookexternalhit", click.PlatformFacebook},
	{"facebot",             click.PlatformFacebook},
	{"twitterbot",          click.PlatformTwitter},
	{"linkedinbot",         click.PlatformLinkedIn},
	{"telegrambot",         click.PlatformTelegram},
	{"whatsapp",            click.PlatformWhatsApp},
	{"slackbot",            click.PlatformSlack},
	{"slack-imgproxy",      click.PlatformSlack},
	{"discordbot",          click.PlatformDiscord},
}

// DetectResult 使用者代理偵測結果
type DetectResult struct {
	IsBot      bool
	Platform   click.Platform
	DeviceType click.DeviceType
}

// Detect 解析 User-Agent 字串，判斷是否為社群 Bot 及裝置類型
func Detect(userAgent string) DetectResult {
	ua := strings.ToLower(userAgent)

	// 優先判斷是否為社群平台 Bot
	for _, sig := range botSignatures {
		if strings.Contains(ua, sig.keyword) {
			return DetectResult{
				IsBot:      true,
				Platform:   sig.platform,
				DeviceType: click.DeviceBot,
			}
		}
	}

	// 一般使用者：判斷手機或桌機
	deviceType := click.DeviceDesktop
	if strings.Contains(ua, "mobile") ||
		strings.Contains(ua, "android") ||
		strings.Contains(ua, "iphone") {
		deviceType = click.DeviceMobile
	}

	return DetectResult{
		IsBot:      false,
		Platform:   click.PlatformUnknown,
		DeviceType: deviceType,
	}
}

// ExtractRegion 從 HTTP Header 推斷地區
// 優先使用 Cloudflare CF-IPCountry，其次 X-Country，最後為空
func ExtractRegion(cfIPCountry, xCountry string) string {
	if cfIPCountry != "" && cfIPCountry != "XX" {
		return cfIPCountry
	}
	if xCountry != "" {
		return xCountry
	}
	return ""
}
