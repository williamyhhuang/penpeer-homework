package config

import (
	"testing"
)

func TestLoad_defaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 不應回傳錯誤，實際: %v", err)
	}

	if cfg.NullCache.MaxKeys <= 0 {
		t.Errorf("NullCache.MaxKeys 應 > 0，實際: %d", cfg.NullCache.MaxKeys)
	}
	if cfg.NullCache.EvictCount <= 0 {
		t.Errorf("NullCache.EvictCount 應 > 0，實際: %d", cfg.NullCache.EvictCount)
	}
	if cfg.RateLimit.RPS <= 0 {
		t.Errorf("RateLimit.RPS 應 > 0，實際: %d", cfg.RateLimit.RPS)
	}
	if cfg.RateLimit.Burst <= 0 {
		t.Errorf("RateLimit.Burst 應 > 0，實際: %d", cfg.RateLimit.Burst)
	}
	if cfg.Bloom.Capacity <= 0 {
		t.Errorf("Bloom.Capacity 應 > 0，實際: %d", cfg.Bloom.Capacity)
	}
}

func TestLoad_values(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 不應回傳錯誤，實際: %v", err)
	}

	// 驗證 app.yaml 中的預設值
	if cfg.NullCache.MaxKeys != 10000 {
		t.Errorf("NullCache.MaxKeys 期望 10000，實際: %d", cfg.NullCache.MaxKeys)
	}
	if cfg.NullCache.EvictCount != 1000 {
		t.Errorf("NullCache.EvictCount 期望 1000，實際: %d", cfg.NullCache.EvictCount)
	}
	if cfg.RateLimit.RPS != 30 {
		t.Errorf("RateLimit.RPS 期望 30，實際: %d", cfg.RateLimit.RPS)
	}
	if cfg.RateLimit.Burst != 60 {
		t.Errorf("RateLimit.Burst 期望 60，實際: %d", cfg.RateLimit.Burst)
	}
	if cfg.Bloom.Capacity != 1000000 {
		t.Errorf("Bloom.Capacity 期望 1000000，實際: %d", cfg.Bloom.Capacity)
	}
}
