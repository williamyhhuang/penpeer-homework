package models

import "time"

// ClickEventModel 是 GORM 的 DB struct，對應 click_events 資料表
type ClickEventModel struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ShortLinkCode string    `gorm:"column:short_link_code;not null"`
	ClickedAt     time.Time `gorm:"column:clicked_at;not null"`
	Platform      string    `gorm:"column:platform;not null;default:'unknown'"`
	Region        string    `gorm:"column:region;not null;default:''"`
	DeviceType    string    `gorm:"column:device_type;not null;default:'desktop'"`
	ReferralCode  string    `gorm:"column:referral_code;not null;default:''"`
}

func (ClickEventModel) TableName() string { return "click_events" }

// 以下為純查詢結果 struct（非 table model），用於 GROUP BY / JOIN 查詢掃描

// PlatformCount 對應按平台分組的查詢結果
type PlatformCount struct {
	Platform string `gorm:"column:platform"`
	Count    int64  `gorm:"column:count"`
}

// DeviceCount 對應按裝置類型分組的查詢結果
type DeviceCount struct {
	DeviceType string `gorm:"column:device_type"`
	Count      int64  `gorm:"column:count"`
}

// RegionCount 對應按地區分組的查詢結果
type RegionCount struct {
	Region string `gorm:"column:region"`
	Count  int64  `gorm:"column:count"`
}

// ReferralCount 對應按推薦碼分組的查詢結果
type ReferralCount struct {
	ReferralCode string `gorm:"column:referral_code"`
	Count        int64  `gorm:"column:count"`
}

// RankingRow 對應排行榜 LEFT JOIN 查詢結果
type RankingRow struct {
	Code        string `gorm:"column:code"`
	OriginalURL string `gorm:"column:original_url"`
	TotalClicks int64  `gorm:"column:total_clicks"`
}
