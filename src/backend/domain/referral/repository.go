package referral

import "context"

// Repository 定義推薦碼的儲存介面
type Repository interface {
	Save(ctx context.Context, ref *ReferralCode) error
	FindByCode(ctx context.Context, code string) (*ReferralCode, error)
}
