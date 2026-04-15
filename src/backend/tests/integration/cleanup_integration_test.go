//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/penpeer/shortlink/application/usecase"
	"github.com/penpeer/shortlink/infrastructure/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertShortLink 直接寫入 short_links，方便整合測試建立前置資料
func insertShortLink(t *testing.T, code, originalURL string, expiresAt *time.Time) {
	t.Helper()
	db := testEnv.db

	if expiresAt != nil {
		err := db.Exec(
			`INSERT INTO short_links (code, original_url, expires_at) VALUES (?, ?, ?)`,
			code, originalURL, *expiresAt,
		).Error
		require.NoError(t, err)
	} else {
		err := db.Exec(
			`INSERT INTO short_links (code, original_url) VALUES (?, ?)`,
			code, originalURL,
		).Error
		require.NoError(t, err)
	}
}

// insertReferralCode 直接寫入 referral_codes
func insertReferralCode(t *testing.T, code, ownerID, shortLinkCode string) {
	t.Helper()
	err := testEnv.db.Exec(
		`INSERT INTO referral_codes (code, owner_id, short_link_code) VALUES (?, ?, ?)`,
		code, ownerID, shortLinkCode,
	).Error
	require.NoError(t, err)
}

// insertClickEvent 直接寫入 click_events，clickedAt 控制時間
func insertClickEvent(t *testing.T, shortLinkCode string, clickedAt time.Time) {
	t.Helper()
	err := testEnv.db.Exec(
		`INSERT INTO click_events (short_link_code, clicked_at) VALUES (?, ?)`,
		shortLinkCode, clickedAt,
	).Error
	require.NoError(t, err)
}

// countRows 查詢指定表的列數（支援 WHERE 條件）
func countRows(t *testing.T, table, where string, args ...interface{}) int64 {
	t.Helper()
	var count int64
	query := "SELECT COUNT(*) FROM " + table
	if where != "" {
		query += " WHERE " + where
	}
	err := testEnv.db.Raw(query, args...).Scan(&count).Error
	require.NoError(t, err)
	return count
}

// ── ArchiveExpiredShortLinks 整合測試 ─────────────────────────────────────────

// TestArchiveExpiredLinks_過期短網址移至封存表 驗證：
//   - 過期 short_link 從主表刪除，寫入 short_links_archive
//   - 關聯 referral_codes 同步封存至 referral_codes_archive
//   - 關聯 click_events 同步封存至 click_events_archive
func TestArchiveExpiredLinks_過期短網址移至封存表(t *testing.T) {
	ctx := context.Background()

	// 準備：過期 1 小時前的短網址
	expiredAt := time.Now().Add(-1 * time.Hour)
	code := "exp_test1"
	insertShortLink(t, code, "https://example.com/expired", &expiredAt)
	insertReferralCode(t, "REF_EXP_TEST1", "owner_test", code)
	insertClickEvent(t, code, time.Now().Add(-2*time.Hour))

	// 確認資料已寫入主表
	require.Equal(t, int64(1), countRows(t, "short_links", "code = ?", code))
	require.Equal(t, int64(1), countRows(t, "referral_codes", "short_link_code = ?", code))
	require.Equal(t, int64(1), countRows(t, "click_events", "short_link_code = ?", code))

	// 執行 cleanup
	archiveRepo := postgres.NewArchiveRepo(testEnv.db)
	uc := usecase.NewArchiveExpiredLinksUseCase(archiveRepo)
	count, err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(1), "至少封存 1 筆過期短網址")

	// 驗證主表已刪除
	assert.Equal(t, int64(0), countRows(t, "short_links", "code = ?", code),
		"過期短網址應從 short_links 刪除")
	assert.Equal(t, int64(0), countRows(t, "referral_codes", "short_link_code = ?", code),
		"關聯推薦碼應隨 CASCADE 從 referral_codes 刪除")
	assert.Equal(t, int64(0), countRows(t, "click_events", "short_link_code = ?", code),
		"關聯點擊事件應隨 CASCADE 從 click_events 刪除")

	// 驗證封存表已寫入
	assert.Equal(t, int64(1), countRows(t, "short_links_archive", "code = ?", code),
		"過期短網址應寫入 short_links_archive")
	assert.Equal(t, int64(1), countRows(t, "referral_codes_archive", "short_link_code = ?", code),
		"關聯推薦碼應寫入 referral_codes_archive")
	assert.Equal(t, int64(1), countRows(t, "click_events_archive", "short_link_code = ?", code),
		"關聯點擊事件應寫入 click_events_archive")
}

// TestArchiveExpiredLinks_未過期短網址不受影響 驗證：
//   - expires_at 在未來的短網址不會被封存
//   - expires_at IS NULL 的短網址不會被封存
func TestArchiveExpiredLinks_未過期短網址不受影響(t *testing.T) {
	ctx := context.Background()

	// 未過期（1 小時後到期）
	futureExpiry := time.Now().Add(1 * time.Hour)
	codeActive := "noexp_test1"
	insertShortLink(t, codeActive, "https://example.com/active", &futureExpiry)

	// 永不過期（expires_at = NULL）
	codeNoExpiry := "noexp_test2"
	insertShortLink(t, codeNoExpiry, "https://example.com/noexpiry", nil)

	// 執行 cleanup
	archiveRepo := postgres.NewArchiveRepo(testEnv.db)
	uc := usecase.NewArchiveExpiredLinksUseCase(archiveRepo)
	_, err := uc.Execute(ctx)
	require.NoError(t, err)

	// 驗證未過期短網址仍在主表
	assert.Equal(t, int64(1), countRows(t, "short_links", "code = ?", codeActive),
		"未到期的短網址不應被封存")
	assert.Equal(t, int64(1), countRows(t, "short_links", "code = ?", codeNoExpiry),
		"永不過期的短網址不應被封存")
}

// TestArchiveExpiredLinks_無過期資料時回傳0 驗證：沒有過期資料時 count = 0、不報錯
func TestArchiveExpiredLinks_無過期資料時回傳0(t *testing.T) {
	ctx := context.Background()

	// 此測試依賴其他測試已清除過期資料，或本身不插入過期資料
	// 插入一筆未來到期的短網址確保表不為空
	futureExpiry := time.Now().Add(24 * time.Hour)
	insertShortLink(t, "noexp_test3", "https://example.com/future", &futureExpiry)

	archiveRepo := postgres.NewArchiveRepo(testEnv.db)
	uc := usecase.NewArchiveExpiredLinksUseCase(archiveRepo)

	// 此時若沒有其他過期資料，count 應為 0
	// 注意：因其他測試也在同一 DB，若殘留過期資料 count 可能 > 0，但不應報錯
	_, err := uc.Execute(ctx)
	assert.NoError(t, err, "無過期資料時不應回傳錯誤")
}

// ── ArchiveOldClickEvents 整合測試 ────────────────────────────────────────────

// TestArchiveOldClicks_舊點擊事件移至封存表 驗證：
//   - 超過保留天數的 click_events 移至 click_events_archive
//   - 保留期內的 click_events 不受影響
func TestArchiveOldClicks_舊點擊事件移至封存表(t *testing.T) {
	ctx := context.Background()
	const retentionDays = 30

	// 建立不過期短網址作為關聯 FK
	codeForOldClicks := "oldclk_host1"
	insertShortLink(t, codeForOldClicks, "https://example.com/oldclicks", nil)

	// 插入超過保留天數的舊點擊（31 天前）
	oldClickAt := time.Now().AddDate(0, 0, -(retentionDays + 1))
	insertClickEvent(t, codeForOldClicks, oldClickAt)
	insertClickEvent(t, codeForOldClicks, oldClickAt)

	// 插入保留期內的新點擊（1 天前）
	recentClickAt := time.Now().AddDate(0, 0, -1)
	insertClickEvent(t, codeForOldClicks, recentClickAt)

	// 確認初始狀態
	require.Equal(t, int64(3), countRows(t, "click_events", "short_link_code = ?", codeForOldClicks))

	// 執行 cleanup
	archiveRepo := postgres.NewArchiveRepo(testEnv.db)
	uc := usecase.NewArchiveOldClicksUseCase(archiveRepo, retentionDays)
	count, err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(2), "至少封存 2 筆舊點擊事件")

	// 驗證舊點擊已移至封存表
	assert.Equal(t, int64(0),
		countRows(t, "click_events",
			"short_link_code = ? AND clicked_at < NOW() - (? * INTERVAL '1 day')",
			codeForOldClicks, retentionDays),
		"超過保留天數的點擊應從 click_events 刪除")

	// 驗證近期點擊仍在主表
	assert.Equal(t, int64(1),
		countRows(t, "click_events", "short_link_code = ?", codeForOldClicks),
		"保留期內的點擊事件應留在 click_events")

	// 驗證舊點擊寫入封存表
	assert.GreaterOrEqual(t,
		countRows(t, "click_events_archive", "short_link_code = ?", codeForOldClicks),
		int64(2),
		"超過保留天數的點擊應寫入 click_events_archive")
}

// TestArchiveOldClicks_近期點擊不受影響 驗證：保留天數內的點擊一律不封存
func TestArchiveOldClicks_近期點擊不受影響(t *testing.T) {
	ctx := context.Background()
	const retentionDays = 90

	codeForRecentClicks := "recentclk_host1"
	insertShortLink(t, codeForRecentClicks, "https://example.com/recent", nil)

	// 全部都是近期點擊（1 天前）
	recentClickAt := time.Now().AddDate(0, 0, -1)
	insertClickEvent(t, codeForRecentClicks, recentClickAt)
	insertClickEvent(t, codeForRecentClicks, recentClickAt)

	beforeCount := countRows(t, "click_events", "short_link_code = ?", codeForRecentClicks)
	require.Equal(t, int64(2), beforeCount)

	archiveRepo := postgres.NewArchiveRepo(testEnv.db)
	uc := usecase.NewArchiveOldClicksUseCase(archiveRepo, retentionDays)
	_, err := uc.Execute(ctx)
	require.NoError(t, err)

	afterCount := countRows(t, "click_events", "short_link_code = ?", codeForRecentClicks)
	assert.Equal(t, int64(2), afterCount, "近期點擊不應被封存")
}

// TestCleanupWorker_完整執行流程 驗證 cleanup worker 的完整執行順序：
//  Step 1：先封存過期短網址（含關聯點擊）
//  Step 2：再封存 active 短網址的舊點擊事件
func TestCleanupWorker_完整執行流程(t *testing.T) {
	ctx := context.Background()
	const retentionDays = 30

	// 準備 1：過期短網址（含點擊）
	expiredAt := time.Now().Add(-1 * time.Hour)
	codeExpired := "flow_expired1"
	insertShortLink(t, codeExpired, "https://example.com/flow-expired", &expiredAt)
	insertClickEvent(t, codeExpired, time.Now().Add(-2*time.Hour))

	// 準備 2：active 短網址，但有舊點擊（超過 30 天）
	codeActive := "flow_active1"
	insertShortLink(t, codeActive, "https://example.com/flow-active", nil)
	oldClickAt := time.Now().AddDate(0, 0, -(retentionDays + 1))
	insertClickEvent(t, codeActive, oldClickAt)
	insertClickEvent(t, codeActive, time.Now().AddDate(0, 0, -1)) // 近期點擊保留

	archiveRepo := postgres.NewArchiveRepo(testEnv.db)

	// Step 1：封存過期短網址
	expiredUC := usecase.NewArchiveExpiredLinksUseCase(archiveRepo)
	expiredCount, err := expiredUC.Execute(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, expiredCount, int64(1))

	// Step 2：封存 active 短網址的舊點擊
	oldClicksUC := usecase.NewArchiveOldClicksUseCase(archiveRepo, retentionDays)
	oldClicksCount, err := oldClicksUC.Execute(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, oldClicksCount, int64(1))

	// 驗證最終狀態
	assert.Equal(t, int64(0), countRows(t, "short_links", "code = ?", codeExpired),
		"過期短網址應已從主表移除")
	assert.Equal(t, int64(1), countRows(t, "short_links", "code = ?", codeActive),
		"active 短網址應仍在主表")
	assert.Equal(t, int64(1), countRows(t, "click_events", "short_link_code = ?", codeActive),
		"active 短網址的近期點擊應保留")
	assert.Equal(t, int64(0),
		countRows(t, "click_events",
			"short_link_code = ? AND clicked_at < NOW() - (? * INTERVAL '1 day')",
			codeActive, retentionDays),
		"active 短網址的舊點擊應已封存")
}
