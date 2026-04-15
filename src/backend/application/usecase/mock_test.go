package usecase_test

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/penpeer/shortlink/domain/click"
	"github.com/penpeer/shortlink/domain/referral"
	"github.com/penpeer/shortlink/domain/shortlink"
)

// ── Mock Repository ───────────────────────────────────────────────────────────

// mockShortLinkRepo 模擬 shortlink.Repository，不依賴真實 DB
type mockShortLinkRepo struct {
	store     map[string]*shortlink.ShortLink
	findCalls atomic.Int64 // 記錄 FindByCode 被呼叫的次數，用於驗證 singleflight 效果
}

func newMockShortLinkRepo() *mockShortLinkRepo {
	return &mockShortLinkRepo{store: make(map[string]*shortlink.ShortLink)}
}

func (m *mockShortLinkRepo) Save(_ context.Context, link *shortlink.ShortLink) error {
	m.store[link.Code] = link
	return nil
}

func (m *mockShortLinkRepo) FindByCode(_ context.Context, code string) (*shortlink.ShortLink, error) {
	m.findCalls.Add(1)
	// 模擬 DB 查詢延遲，確保 singleflight 測試中並發請求能被有效合併
	// 若無延遲，mock 瞬間返回，後續 goroutine 可能在 singleflight 完成後才到達，導致計數超過 1
	time.Sleep(10 * time.Millisecond)
	return m.store[code], nil
}

func (m *mockShortLinkRepo) FindAllCodes(_ context.Context) ([]string, error) {
	codes := make([]string, 0, len(m.store))
	for code := range m.store {
		codes = append(codes, code)
	}
	return codes, nil
}

func (m *mockShortLinkRepo) UpdateOG(_ context.Context, code, title, description, image string) error {
	if link, ok := m.store[code]; ok {
		link.OGTitle = title
		link.OGDescription = description
		link.OGImage = image
	}
	return nil
}

// mockReferralRepo 模擬 referral.Repository
type mockReferralRepo struct {
	store map[string]*referral.ReferralCode
}

func newMockReferralRepo() *mockReferralRepo {
	return &mockReferralRepo{store: make(map[string]*referral.ReferralCode)}
}

func (m *mockReferralRepo) Save(_ context.Context, ref *referral.ReferralCode) error {
	m.store[ref.Code] = ref
	return nil
}

func (m *mockReferralRepo) FindByCode(_ context.Context, code string) (*referral.ReferralCode, error) {
	return m.store[code], nil
}

// mockClickRepo 模擬 click.Repository
// 使用 mutex 保護 events slice，因為 asyncSaveClick 在獨立 goroutine 中呼叫 Save
type mockClickRepo struct {
	mu     sync.Mutex
	events []*click.ClickEvent
}

func (m *mockClickRepo) Save(_ context.Context, event *click.ClickEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

// Len 回傳目前已記錄的點擊事件筆數（thread-safe，供測試斷言使用）
func (m *mockClickRepo) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func (m *mockClickRepo) GetRanking(_ context.Context) ([]click.CodeRanking, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 彙整每個短碼的點擊總數
	counts := make(map[string]int64)
	for _, e := range m.events {
		counts[e.ShortLinkCode]++
	}
	rankings := make([]click.CodeRanking, 0, len(counts))
	rank := 1
	for code, total := range counts {
		rankings = append(rankings, click.CodeRanking{
			Rank:        rank,
			Code:        code,
			OriginalURL: "",
			TotalClicks: total,
		})
		rank++
	}
	return rankings, nil
}

func (m *mockClickRepo) GetStatsByCode(_ context.Context, code string) (*click.ClickStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stats := &click.ClickStats{
		ByPlatform:   make(map[click.Platform]int64),
		ByDeviceType: make(map[click.DeviceType]int64),
		ByRegion:     make(map[string]int64),
		ByReferral:   make(map[string]int64),
	}
	for _, e := range m.events {
		if e.ShortLinkCode != code {
			continue
		}
		stats.TotalClicks++
		stats.ByPlatform[e.Platform]++
		stats.ByDeviceType[e.DeviceType]++
		if e.Region != "" {
			stats.ByRegion[e.Region]++
		}
		if e.ReferralCode != "" {
			stats.ByReferral[e.ReferralCode]++
		}
	}
	return stats, nil
}

// mockCache 模擬 Redis Cache（同時實作 ShortLinkCache 與 RedirectCache 介面）
// 使用 mutex 保護所有 map，因為 asyncSaveClick goroutine 會並發呼叫 IsNewClick / SetShortLink
type mockCache struct {
	mu         sync.Mutex
	store      map[string]*shortlink.ShortLink
	nullSet    map[string]bool // 記錄哪些 code 被標記為不存在（null cache）
	dedupSet   map[string]bool // 記錄哪些 fingerprint+code 已在去重窗口內
	dedupError error           // 注入 IsNewClick 錯誤，測試 Redis 故障降級行為
}

func newMockCache() *mockCache {
	return &mockCache{
		store:    make(map[string]*shortlink.ShortLink),
		nullSet:  make(map[string]bool),
		dedupSet: make(map[string]bool),
	}
}

func (m *mockCache) SetShortLink(_ context.Context, link *shortlink.ShortLink) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[link.Code] = link
	return nil
}

func (m *mockCache) GetShortLink(_ context.Context, code string) (*shortlink.ShortLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.nullSet[code] {
		return nil, shortlink.ErrNullCache
	}
	return m.store[code], nil
}

func (m *mockCache) SetNullCache(_ context.Context, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nullSet[code] = true
	return nil
}

// IsNewClick 模擬 Redis SET NX 去重行為
// dedupError != nil 時模擬 Redis 故障，回傳 (true, err) 讓點擊通過（寬鬆策略）
func (m *mockCache) IsNewClick(_ context.Context, code, fingerprint string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dedupError != nil {
		return true, m.dedupError
	}
	key := fingerprint + ":" + code
	if m.dedupSet[key] {
		return false, nil // 已存在，重複點擊
	}
	m.dedupSet[key] = true
	return true, nil // 首次點擊
}
