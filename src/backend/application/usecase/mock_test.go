package usecase_test

import (
	"context"

	"github.com/penpeer/shortlink/domain/click"
	"github.com/penpeer/shortlink/domain/referral"
	"github.com/penpeer/shortlink/domain/shortlink"
)

// ── Mock Repository ───────────────────────────────────────────────────────────

// mockShortLinkRepo 模擬 shortlink.Repository，不依賴真實 DB
type mockShortLinkRepo struct {
	store map[string]*shortlink.ShortLink
}

func newMockShortLinkRepo() *mockShortLinkRepo {
	return &mockShortLinkRepo{store: make(map[string]*shortlink.ShortLink)}
}

func (m *mockShortLinkRepo) Save(_ context.Context, link *shortlink.ShortLink) error {
	m.store[link.Code] = link
	return nil
}

func (m *mockShortLinkRepo) FindByCode(_ context.Context, code string) (*shortlink.ShortLink, error) {
	return m.store[code], nil
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
type mockClickRepo struct {
	events []*click.ClickEvent
}

func (m *mockClickRepo) Save(_ context.Context, event *click.ClickEvent) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockClickRepo) GetStatsByCode(_ context.Context, code string) (*click.ClickStats, error) {
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
type mockCache struct {
	store map[string]*shortlink.ShortLink
}

func newMockCache() *mockCache {
	return &mockCache{store: make(map[string]*shortlink.ShortLink)}
}

func (m *mockCache) SetShortLink(_ context.Context, link *shortlink.ShortLink) error {
	m.store[link.Code] = link
	return nil
}

func (m *mockCache) GetShortLink(_ context.Context, code string) (*shortlink.ShortLink, error) {
	return m.store[code], nil
}
