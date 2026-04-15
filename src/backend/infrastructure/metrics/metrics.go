// Package metrics 定義應用程式所有 Prometheus 指標。
// 放在 infrastructure 層，讓 application（usecase）與 infrastructure（redis）皆可引用，
// 同時避免 usecase 反向依賴 interfaces/http 層。
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ── HTTP 層指標 ─────────────────────────────────────────────────────────

	// HTTPRequestsTotal HTTP 請求總量，依 method / path pattern / status_code 分類。
	// path 使用 Gin 的 FullPath()（如 "/:code"）而非實際 URL，避免 label 高基數問題。
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "shortlink_http_requests_total",
			Help: "Total number of HTTP requests processed",
		},
		[]string{"method", "path", "status_code"},
	)

	// HTTPRequestDuration HTTP 請求延遲分佈（Histogram），用於計算 P50/P95/P99。
	// buckets 針對短網址服務調整：Redis 命中約 5ms，DB 查詢約 50ms。
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "shortlink_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5},
		},
		[]string{"method", "path"},
	)

	// HTTPRequestsInFlight 當前正在處理中的請求數（Gauge）。
	// 高峰值代表服務壓力大，搭配 P99 延遲一起觀察。
	HTTPRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "shortlink_http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)

	// ── 業務層指標 ──────────────────────────────────────────────────────────

	// RedirectTotal 短網址轉導請求量，依查詢路徑分類。
	// result labels：
	//   bloom_miss  → Bloom Filter 確定不存在，直接拒絕（防穿透）
	//   redis_hit   → Redis 快取命中，最快路徑
	//   db_hit      → Redis miss，查 DB 成功
	//   not_found   → DB 查無此碼
	//   expired     → 短網址已過期
	RedirectTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "shortlink_redirect_total",
			Help: "Total number of redirect requests by result",
		},
		[]string{"result"},
	)

	// CacheHitTotal Redis 快取操作計數，用於計算命中率。
	// labels: operation(get/set), result(hit/miss/null_cache/error)
	CacheHitTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "shortlink_cache_operations_total",
			Help: "Total number of Redis cache operations by operation and result",
		},
		[]string{"operation", "result"},
	)

	// BloomFilterChecks Bloom Filter 查詢計數。
	// result labels：pass（MightExist=true，繼續查快取）/ reject（確定不存在，直接拒絕）
	BloomFilterChecks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "shortlink_bloom_filter_checks_total",
			Help: "Total number of bloom filter checks",
		},
		[]string{"result"},
	)

	// LinksCreatedTotal 短網址建立總數（DB save 成功才計）。
	LinksCreatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "shortlink_links_created_total",
			Help: "Total number of short links successfully created",
		},
	)

	// ClicksRecordedTotal 點擊事件寫入量（非同步寫入，反映實際存入 DB 的結果）。
	// result labels：success / error
	ClicksRecordedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "shortlink_clicks_recorded_total",
			Help: "Total number of click events recorded to database",
		},
		[]string{"result"},
	)

	// NullCacheEvictions null cache 水位觸發淘汰次數。
	// 頻繁淘汰代表無效短碼請求量大，或 MaxKeys 設定過低。
	NullCacheEvictions = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "shortlink_null_cache_evictions_total",
			Help: "Total number of null cache batch evictions triggered by watermark",
		},
	)

	// SingleflightDedup singleflight 去重計數（同一 code 的並發 cache miss 被合併為一次 DB 查詢）。
	// 高值代表有大量對同一短碼的並發請求（可能是熱點短碼 cache 剛過期）。
	SingleflightDedup = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "shortlink_singleflight_dedup_total",
			Help: "Total number of duplicate DB queries prevented by singleflight",
		},
	)
)
