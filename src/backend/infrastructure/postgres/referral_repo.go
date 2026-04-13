package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/penpeer/shortlink/domain/referral"
)

// ReferralRepo 實作 domain/referral.Repository
type ReferralRepo struct {
	db *sqlx.DB
}

func NewReferralRepo(db *sqlx.DB) *ReferralRepo {
	return &ReferralRepo{db: db}
}

type dbReferralCode struct {
	Code          string    `db:"code"`
	OwnerID       string    `db:"owner_id"`
	ShortLinkCode string    `db:"short_link_code"`
	CreatedAt     time.Time `db:"created_at"`
}

func (r *ReferralRepo) Save(ctx context.Context, ref *referral.ReferralCode) error {
	query := `
		INSERT INTO referral_codes (code, owner_id, short_link_code, created_at)
		VALUES (:code, :owner_id, :short_link_code, :created_at)
		ON CONFLICT (code) DO NOTHING
	`
	row := map[string]interface{}{
		"code":            ref.Code,
		"owner_id":        ref.OwnerID,
		"short_link_code": ref.ShortLinkCode,
		"created_at":      ref.CreatedAt,
	}
	_, err := r.db.NamedExecContext(ctx, query, row)
	return err
}

func (r *ReferralRepo) FindByCode(ctx context.Context, code string) (*referral.ReferralCode, error) {
	var row dbReferralCode
	err := r.db.GetContext(ctx, &row, "SELECT * FROM referral_codes WHERE code = $1", code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &referral.ReferralCode{
		Code:          row.Code,
		OwnerID:       row.OwnerID,
		ShortLinkCode: row.ShortLinkCode,
		CreatedAt:     row.CreatedAt,
	}, nil
}
