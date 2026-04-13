package postgres

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/penpeer/shortlink/domain/click"
)

// ClickRepo 實作 domain/click.Repository
type ClickRepo struct {
	db *sqlx.DB
}

func NewClickRepo(db *sqlx.DB) *ClickRepo {
	return &ClickRepo{db: db}
}

func (r *ClickRepo) Save(ctx context.Context, event *click.ClickEvent) error {
	query := `
		INSERT INTO click_events (short_link_code, clicked_at, platform, region, device_type, referral_code)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query,
		event.ShortLinkCode,
		event.ClickedAt,
		string(event.Platform),
		event.Region,
		string(event.DeviceType),
		event.ReferralCode,
	)
	return err
}

// GetStatsByCode 彙整指定短碼的點擊統計，使用單一 SQL 避免多次來回
func (r *ClickRepo) GetStatsByCode(ctx context.Context, shortLinkCode string) (*click.ClickStats, error) {
	stats := &click.ClickStats{
		ByPlatform:   make(map[click.Platform]int64),
		ByDeviceType: make(map[click.DeviceType]int64),
		ByRegion:     make(map[string]int64),
		ByReferral:   make(map[string]int64),
	}

	// 總點擊數
	err := r.db.GetContext(ctx, &stats.TotalClicks,
		"SELECT COUNT(*) FROM click_events WHERE short_link_code = $1", shortLinkCode)
	if err != nil {
		return nil, err
	}

	// 按平台分組
	var platformRows []struct {
		Platform string `db:"platform"`
		Count    int64  `db:"count"`
	}
	err = r.db.SelectContext(ctx, &platformRows,
		"SELECT platform, COUNT(*) as count FROM click_events WHERE short_link_code = $1 GROUP BY platform",
		shortLinkCode)
	if err != nil {
		return nil, err
	}
	for _, row := range platformRows {
		stats.ByPlatform[click.Platform(row.Platform)] = row.Count
	}

	// 按裝置分組
	var deviceRows []struct {
		DeviceType string `db:"device_type"`
		Count      int64  `db:"count"`
	}
	err = r.db.SelectContext(ctx, &deviceRows,
		"SELECT device_type, COUNT(*) as count FROM click_events WHERE short_link_code = $1 GROUP BY device_type",
		shortLinkCode)
	if err != nil {
		return nil, err
	}
	for _, row := range deviceRows {
		stats.ByDeviceType[click.DeviceType(row.DeviceType)] = row.Count
	}

	// 按地區分組
	var regionRows []struct {
		Region string `db:"region"`
		Count  int64  `db:"count"`
	}
	err = r.db.SelectContext(ctx, &regionRows,
		"SELECT region, COUNT(*) as count FROM click_events WHERE short_link_code = $1 GROUP BY region",
		shortLinkCode)
	if err != nil {
		return nil, err
	}
	for _, row := range regionRows {
		if row.Region != "" {
			stats.ByRegion[row.Region] = row.Count
		}
	}

	// 按推薦碼分組
	var referralRows []struct {
		ReferralCode string `db:"referral_code"`
		Count        int64  `db:"count"`
	}
	err = r.db.SelectContext(ctx, &referralRows,
		"SELECT referral_code, COUNT(*) as count FROM click_events WHERE short_link_code = $1 AND referral_code != '' GROUP BY referral_code",
		shortLinkCode)
	if err != nil {
		return nil, err
	}
	for _, row := range referralRows {
		stats.ByReferral[row.ReferralCode] = row.Count
	}

	return stats, nil
}

// GetRanking 查詢所有短碼的點擊總數並依降冪排序
// LEFT JOIN 確保 0 點擊的短碼也出現在排行榜
func (r *ClickRepo) GetRanking(ctx context.Context) ([]click.CodeRanking, error) {
	var rows []struct {
		Code        string `db:"code"`
		OriginalURL string `db:"original_url"`
		TotalClicks int64  `db:"total_clicks"`
	}
	err := r.db.SelectContext(ctx, &rows, `
		SELECT sl.code, sl.original_url, COUNT(ce.id) AS total_clicks
		FROM short_links sl
		LEFT JOIN click_events ce ON sl.code = ce.short_link_code
		GROUP BY sl.code, sl.original_url
		ORDER BY total_clicks DESC, sl.code ASC
	`)
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
