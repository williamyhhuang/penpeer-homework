package referral

import "time"

// ReferralCode 追蹤行銷推薦碼，用於歸因分析
// 例如：短網址 + ?ref=user123 → 記錄哪個推薦者帶來的流量
type ReferralCode struct {
	Code          string    // 推薦碼，例如 "user123"
	OwnerID       string    // 擁有者識別碼（可為用戶 ID 或名稱）
	ShortLinkCode string    // 關聯的短碼
	CreatedAt     time.Time
}
