package models

import (
	"database/sql"
	"time"
)

// ShortLinkModel 是 GORM 的 DB struct，屬於 Infrastructure Layer
// 與 domain/shortlink/entity.go 嚴格隔離，不含任何業務邏輯
type ShortLinkModel struct {
	Code          string       `gorm:"column:code;primaryKey"`
	OriginalURL   string       `gorm:"column:original_url;not null"`
	OGTitle       string       `gorm:"column:og_title;not null;default:''"`
	OGDescription string       `gorm:"column:og_description;not null;default:''"`
	OGImage       string       `gorm:"column:og_image;not null;default:''"`
	CreatedAt     time.Time    `gorm:"column:created_at;not null"`
	ExpiresAt     sql.NullTime `gorm:"column:expires_at"`
}

func (ShortLinkModel) TableName() string { return "short_links" }
