package config

import (
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed app.yaml
var rawConfig []byte

// App 非機敏應用程式設定，從 config/app.yaml 載入
type App struct {
	NullCache NullCache `yaml:"null_cache"`
	RateLimit RateLimit `yaml:"rate_limit"`
	Bloom     Bloom     `yaml:"bloom"`
	Cleanup   Cleanup   `yaml:"cleanup"`
}

// NullCache null cache 水位管控閾值
type NullCache struct {
	MaxKeys    int64 `yaml:"max_keys"`
	EvictCount int64 `yaml:"evict_count"`
}

// RateLimit per-IP 速率限制設定
type RateLimit struct {
	RPS   int `yaml:"rps"`
	Burst int `yaml:"burst"`
}

// Bloom bloom filter 容量設定
type Bloom struct {
	Capacity int `yaml:"capacity"`
}

// Cleanup 封存任務排程設定（非機敏，納入版本控制）
type Cleanup struct {
	ClickRetentionDays int `yaml:"click_retention_days"`
	IntervalHours      int `yaml:"interval_hours"`
}

// Load 解析內嵌的 app.yaml，回傳應用程式非機敏設定
// 使用 go:embed 將 YAML 編譯進 binary，部署時不需額外掛載設定檔
func Load() (App, error) {
	var cfg App
	if err := yaml.Unmarshal(rawConfig, &cfg); err != nil {
		return App{}, fmt.Errorf("解析 config/app.yaml 失敗: %w", err)
	}
	return cfg, nil
}
