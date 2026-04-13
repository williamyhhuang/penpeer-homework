package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/penpeer/shortlink/domain/shortlink"
)

// ShortLinkRepo 實作 domain/shortlink.Repository（Hexagonal Secondary Adapter）
type ShortLinkRepo struct {
	db *sqlx.DB
}

func NewShortLinkRepo(db *sqlx.DB) *ShortLinkRepo {
	return &ShortLinkRepo{db: db}
}

// dbShortLink 是對應資料庫欄位的內部結構，與 domain entity 分離
type dbShortLink struct {
	Code          string       `db:"code"`
	OriginalURL   string       `db:"original_url"`
	OGTitle       string       `db:"og_title"`
	OGDescription string       `db:"og_description"`
	OGImage       string       `db:"og_image"`
	CreatedAt     time.Time    `db:"created_at"`
	ExpiresAt     sql.NullTime `db:"expires_at"`
}

func (r *ShortLinkRepo) Save(ctx context.Context, link *shortlink.ShortLink) error {
	query := `
		INSERT INTO short_links (code, original_url, og_title, og_description, og_image, created_at, expires_at)
		VALUES (:code, :original_url, :og_title, :og_description, :og_image, :created_at, :expires_at)
		ON CONFLICT (code) DO UPDATE
		SET original_url   = EXCLUDED.original_url,
		    og_title       = EXCLUDED.og_title,
		    og_description = EXCLUDED.og_description,
		    og_image       = EXCLUDED.og_image
	`
	row := map[string]interface{}{
		"code":           link.Code,
		"original_url":   link.OriginalURL,
		"og_title":       link.OGTitle,
		"og_description": link.OGDescription,
		"og_image":       link.OGImage,
		"created_at":     link.CreatedAt,
		"expires_at":     link.ExpiresAt,
	}
	_, err := r.db.NamedExecContext(ctx, query, row)
	return err
}

func (r *ShortLinkRepo) FindByCode(ctx context.Context, code string) (*shortlink.ShortLink, error) {
	var row dbShortLink
	err := r.db.GetContext(ctx, &row, "SELECT * FROM short_links WHERE code = $1", code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // 回傳 nil 表示找不到，由上層決定如何處理
	}
	if err != nil {
		return nil, err
	}

	link := &shortlink.ShortLink{
		Code:          row.Code,
		OriginalURL:   row.OriginalURL,
		OGTitle:       row.OGTitle,
		OGDescription: row.OGDescription,
		OGImage:       row.OGImage,
		CreatedAt:     row.CreatedAt,
	}
	if row.ExpiresAt.Valid {
		link.ExpiresAt = &row.ExpiresAt.Time
	}
	return link, nil
}
