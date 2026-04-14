package postgres

import (
	"context"

	"gorm.io/gorm"

	"github.com/penpeer/shortlink/domain/click"
	"github.com/penpeer/shortlink/infrastructure/postgres/models"
)

// ClickRepo 實作 domain/click.Repository
type ClickRepo struct {
	db *gorm.DB
}

func NewClickRepo(db *gorm.DB) *ClickRepo {
	return &ClickRepo{db: db}
}

func (r *ClickRepo) Save(ctx context.Context, event *click.ClickEvent) error {
	m := models.ClickEventModel{
		ShortLinkCode: event.ShortLinkCode,
		ClickedAt:     event.ClickedAt,
		Platform:      string(event.Platform),
		Region:        event.Region,
		DeviceType:    string(event.DeviceType),
		ReferralCode:  event.ReferralCode,
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

// GetStatsByCode 彙整指定短碼的點擊統計
// COUNT 用 GORM API，GROUP BY 保留 Raw SQL：語意複雜，Raw 比 GORM 鏈式語法更易讀
func (r *ClickRepo) GetStatsByCode(ctx context.Context, shortLinkCode string) (*click.ClickStats, error) {
	stats := &click.ClickStats{
		ByPlatform:   make(map[click.Platform]int64),
		ByDeviceType: make(map[click.DeviceType]int64),
		ByRegion:     make(map[string]int64),
		ByReferral:   make(map[string]int64),
	}

	// 總點擊數
	if err := r.db.WithContext(ctx).Model(&models.ClickEventModel{}).
		Where("short_link_code = ?", shortLinkCode).Count(&stats.TotalClicks).Error; err != nil {
		return nil, err
	}

	// 按平台分組
	var platformRows []models.PlatformCount
	if err := r.db.WithContext(ctx).Raw(
		"SELECT platform, COUNT(*) as count FROM click_events WHERE short_link_code = ? GROUP BY platform",
		shortLinkCode,
	).Scan(&platformRows).Error; err != nil {
		return nil, err
	}
	for _, row := range platformRows {
		stats.ByPlatform[click.Platform(row.Platform)] = row.Count
	}

	// 按裝置分組
	var deviceRows []models.DeviceCount
	if err := r.db.WithContext(ctx).Raw(
		"SELECT device_type, COUNT(*) as count FROM click_events WHERE short_link_code = ? GROUP BY device_type",
		shortLinkCode,
	).Scan(&deviceRows).Error; err != nil {
		return nil, err
	}
	for _, row := range deviceRows {
		stats.ByDeviceType[click.DeviceType(row.DeviceType)] = row.Count
	}

	// 按地區分組
	var regionRows []models.RegionCount
	if err := r.db.WithContext(ctx).Raw(
		"SELECT region, COUNT(*) as count FROM click_events WHERE short_link_code = ? GROUP BY region",
		shortLinkCode,
	).Scan(&regionRows).Error; err != nil {
		return nil, err
	}
	for _, row := range regionRows {
		if row.Region != "" {
			stats.ByRegion[row.Region] = row.Count
		}
	}

	// 按推薦碼分組
	var referralRows []models.ReferralCount
	if err := r.db.WithContext(ctx).Raw(
		"SELECT referral_code, COUNT(*) as count FROM click_events WHERE short_link_code = ? AND referral_code != '' GROUP BY referral_code",
		shortLinkCode,
	).Scan(&referralRows).Error; err != nil {
		return nil, err
	}
	for _, row := range referralRows {
		stats.ByReferral[row.ReferralCode] = row.Count
	}

	return stats, nil
}

// GetRanking 查詢所有短碼的點擊總數並依降冪排序
// LEFT JOIN 確保 0 點擊的短碼也出現在排行榜，保留 Raw SQL 以維持查詢語意清晰
func (r *ClickRepo) GetRanking(ctx context.Context) ([]click.CodeRanking, error) {
	var rows []models.RankingRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT sl.code, sl.original_url, COUNT(ce.id) AS total_clicks
		FROM short_links sl
		LEFT JOIN click_events ce ON sl.code = ce.short_link_code
		GROUP BY sl.code, sl.original_url
		ORDER BY total_clicks DESC, sl.code ASC
	`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	rankings := make([]click.CodeRanking, len(rows))
	for i, row := range rows {
		rankings[i] = click.CodeRanking{
			Rank:        i + 1,
			Code:        row.Code,
			OriginalURL: row.OriginalURL,
			TotalClicks: row.TotalClicks,
		}
	}
	return rankings, nil
}
