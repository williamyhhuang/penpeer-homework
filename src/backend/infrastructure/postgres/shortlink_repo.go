package postgres

import (
	"context"
	"database/sql"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/penpeer/shortlink/domain/shortlink"
	"github.com/penpeer/shortlink/infrastructure/postgres/models"
)

// ShortLinkRepo 實作 domain/shortlink.Repository（Hexagonal Secondary Adapter）
type ShortLinkRepo struct {
	db *gorm.DB
}

func NewShortLinkRepo(db *gorm.DB) *ShortLinkRepo {
	return &ShortLinkRepo{db: db}
}

func (r *ShortLinkRepo) Save(ctx context.Context, link *shortlink.ShortLink) error {
	m := toShortLinkModel(link)
	// ON CONFLICT (code) DO UPDATE：語意與原本 INSERT ... ON CONFLICT DO UPDATE 相同
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"original_url", "og_title", "og_description", "og_image",
			}),
		}).
		Create(&m).Error
}

func (r *ShortLinkRepo) FindAllCodes(ctx context.Context) ([]string, error) {
	var codes []string
	if err := r.db.WithContext(ctx).Model(&models.ShortLinkModel{}).Pluck("code", &codes).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

func (r *ShortLinkRepo) FindByCode(ctx context.Context, code string) (*shortlink.ShortLink, error) {
	var m models.ShortLinkModel
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 找不到時回傳 nil，由上層決定如何處理
	}
	if err != nil {
		return nil, err
	}
	return toShortLinkDomain(&m), nil
}

// UpdateOG 僅更新指定短碼的 OG 欄位，供非同步 scraper 完成後回寫
func (r *ShortLinkRepo) UpdateOG(ctx context.Context, code, title, description, image string) error {
	return r.db.WithContext(ctx).
		Model(&models.ShortLinkModel{}).
		Where("code = ?", code).
		Updates(map[string]interface{}{
			"og_title":       title,
			"og_description": description,
			"og_image":       image,
		}).Error
}

// ── 轉換函式（infrastructure 層私有，嚴格隔離 domain entity）──────────────────

func toShortLinkModel(link *shortlink.ShortLink) models.ShortLinkModel {
	m := models.ShortLinkModel{
		Code:          link.Code,
		OriginalURL:   link.OriginalURL,
		OGTitle:       link.OGTitle,
		OGDescription: link.OGDescription,
		OGImage:       link.OGImage,
		CreatedAt:     link.CreatedAt,
	}
	if link.ExpiresAt != nil {
		m.ExpiresAt = sql.NullTime{Time: *link.ExpiresAt, Valid: true}
	}
	return m
}

func toShortLinkDomain(m *models.ShortLinkModel) *shortlink.ShortLink {
	link := &shortlink.ShortLink{
		Code:          m.Code,
		OriginalURL:   m.OriginalURL,
		OGTitle:       m.OGTitle,
		OGDescription: m.OGDescription,
		OGImage:       m.OGImage,
		CreatedAt:     m.CreatedAt,
	}
	if m.ExpiresAt.Valid {
		t := m.ExpiresAt.Time
		link.ExpiresAt = &t
	}
	return link
}
