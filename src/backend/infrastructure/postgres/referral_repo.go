package postgres

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/penpeer/shortlink/domain/referral"
	"github.com/penpeer/shortlink/infrastructure/postgres/models"
)

// ReferralRepo 實作 domain/referral.Repository（Hexagonal Secondary Adapter）
type ReferralRepo struct {
	db *gorm.DB
}

func NewReferralRepo(db *gorm.DB) *ReferralRepo {
	return &ReferralRepo{db: db}
}

func (r *ReferralRepo) Save(ctx context.Context, ref *referral.ReferralCode) error {
	m := models.ReferralCodeModel{
		Code:          ref.Code,
		OwnerID:       ref.OwnerID,
		ShortLinkCode: ref.ShortLinkCode,
		CreatedAt:     ref.CreatedAt,
	}
	// ON CONFLICT (code) DO NOTHING：推薦碼已存在時靜默略過
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&m).Error
}

func (r *ReferralRepo) FindByCode(ctx context.Context, code string) (*referral.ReferralCode, error) {
	var m models.ReferralCodeModel
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 找不到時回傳 nil，由上層決定如何處理
	}
	if err != nil {
		return nil, err
	}
	return &referral.ReferralCode{
		Code:          m.Code,
		OwnerID:       m.OwnerID,
		ShortLinkCode: m.ShortLinkCode,
		CreatedAt:     m.CreatedAt,
	}, nil
}
