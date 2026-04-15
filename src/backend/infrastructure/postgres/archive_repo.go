package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// ArchiveRepo 封存資料庫操作
// 每個方法都在單一 transaction 內完成 INSERT + DELETE，確保原子性
type ArchiveRepo struct {
	db *gorm.DB
}

func NewArchiveRepo(db *gorm.DB) *ArchiveRepo {
	return &ArchiveRepo{db: db}
}

// ArchiveExpiredShortLinks 將過期的短網址（含推薦碼與點擊事件）搬移至封存表
// 執行順序：先 INSERT archive 表，再 DELETE 主表（CASCADE 自動清除關聯資料）
// 回傳封存的短網址筆數
func (r *ArchiveRepo) ArchiveExpiredShortLinks(ctx context.Context) (int64, error) {
	var archivedCount int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Step 1a：先取得所有過期短碼清單，後續 INSERT 用
		type codeRow struct{ Code string }
		var expired []codeRow
		if err := tx.Raw(`
			SELECT code FROM short_links
			WHERE expires_at IS NOT NULL AND expires_at < NOW()
		`).Scan(&expired).Error; err != nil {
			return fmt.Errorf("查詢過期短碼失敗: %w", err)
		}
		if len(expired) == 0 {
			// 沒有過期資料，直接回傳
			return nil
		}

		codes := make([]string, len(expired))
		for i, r := range expired {
			codes[i] = r.Code
		}

		// Step 1b：封存 short_links
		if err := tx.Exec(`
			INSERT INTO short_links_archive
				(code, original_url, og_title, og_description, og_image, created_at, expires_at, archived_at)
			SELECT code, original_url, og_title, og_description, og_image, created_at, expires_at, NOW()
			FROM short_links
			WHERE expires_at IS NOT NULL AND expires_at < NOW()
		`).Error; err != nil {
			return fmt.Errorf("封存 short_links 失敗: %w", err)
		}

		// Step 1c：封存 referral_codes（依過期短碼）
		// 使用 IN (?) 而非 ANY(?)，GORM 會將 []string 自動展開為 IN ('a','b',...)
		if err := tx.Exec(`
			INSERT INTO referral_codes_archive
				(code, owner_id, short_link_code, created_at, archived_at)
			SELECT code, owner_id, short_link_code, created_at, NOW()
			FROM referral_codes
			WHERE short_link_code IN (?)
		`, codes).Error; err != nil {
			return fmt.Errorf("封存 referral_codes 失敗: %w", err)
		}

		// Step 1d：封存 click_events（依過期短碼）
		if err := tx.Exec(`
			INSERT INTO click_events_archive
				(id, short_link_code, clicked_at, platform, region, device_type, referral_code, archived_at)
			SELECT id, short_link_code, clicked_at, platform, region, device_type, referral_code, NOW()
			FROM click_events
			WHERE short_link_code IN (?)
		`, codes).Error; err != nil {
			return fmt.Errorf("封存 click_events 失敗: %w", err)
		}

		// Step 1e：刪除過期 short_links（CASCADE 自動刪 referral_codes 與 click_events）
		result := tx.Exec(`
			DELETE FROM short_links
			WHERE expires_at IS NOT NULL AND expires_at < NOW()
		`)
		if result.Error != nil {
			return fmt.Errorf("刪除過期 short_links 失敗: %w", result.Error)
		}
		archivedCount = result.RowsAffected

		return nil
	})

	if err != nil {
		return 0, err
	}
	return archivedCount, nil
}

// ArchiveOldClickEvents 將超過保留天數的舊點擊事件封存（針對仍有效的 short_links）
// 因 Step 1 已先處理過期 short_links，此處的事件均屬 active short_links
// 回傳封存的點擊事件筆數
func (r *ArchiveRepo) ArchiveOldClickEvents(ctx context.Context, retentionDays int) (int64, error) {
	var archivedCount int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Step 2a：封存超過保留天數的舊點擊事件
		if err := tx.Exec(`
			INSERT INTO click_events_archive
				(id, short_link_code, clicked_at, platform, region, device_type, referral_code, archived_at)
			SELECT id, short_link_code, clicked_at, platform, region, device_type, referral_code, NOW()
			FROM click_events
			WHERE clicked_at < NOW() - (? * INTERVAL '1 day')
		`, retentionDays).Error; err != nil {
			return fmt.Errorf("封存舊點擊事件失敗: %w", err)
		}

		// Step 2b：刪除已封存的舊點擊事件
		result := tx.Exec(`
			DELETE FROM click_events
			WHERE clicked_at < NOW() - (? * INTERVAL '1 day')
		`, retentionDays)
		if result.Error != nil {
			return fmt.Errorf("刪除舊點擊事件失敗: %w", result.Error)
		}
		archivedCount = result.RowsAffected

		return nil
	})

	if err != nil {
		return 0, err
	}
	return archivedCount, nil
}
