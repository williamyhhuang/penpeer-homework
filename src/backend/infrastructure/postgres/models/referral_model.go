package models

import "time"

// ReferralCodeModel 是 GORM 的 DB struct，對應 referral_codes 資料表
// 與 domain/referral/entity.go 嚴格隔離
type ReferralCodeModel struct {
	Code          string    `gorm:"column:code;primaryKey"`
	OwnerID       string    `gorm:"column:owner_id;not null"`
	ShortLinkCode string    `gorm:"column:short_link_code;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
}

func (ReferralCodeModel) TableName() string { return "referral_codes" }
