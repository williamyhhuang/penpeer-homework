package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/penpeer/shortlink/domain/shortlink"
	"github.com/penpeer/shortlink/infrastructure/metrics"
)

const (
	// shortLinkBaseTTL 短網址快取基礎 TTL
	shortLinkBaseTTL = 24 * time.Hour
	// shortLinkJitter ±20% 隨機抖動，防止大量 key 在同一時刻集體過期（快取雪崩）
	shortLinkJitter = 288 * time.Minute // 24h × 20% = 4.8h

	// nullCacheTTL 不存在短碼的快取時間，較短避免佔用過多記憶體（防快取穿透）
	nullCacheTTL = 5 * time.Minute
	// nullValue Redis 中標記「此短碼不存在於 DB」的哨兵值
	nullValue = "__null__"

	keyPrefix = "shortlink:"
	// nullIndexKey sorted set：score=寫入時間戳(ns)，member=short code
	// 用於水位到達時能快速找到最舊的 null cache key 進行淘汰
	nullIndexKey = "nullcache:index"

	// dedupPrefix 點擊去重複 key 前綴，格式：dedup:click:{fingerprint}:{code}
	dedupPrefix = "dedup:click:"
	// dedupIndexKey 點擊去重複水位索引（sorted set），score=寫入時間戳(ns)
	// 僅在 DedupConfig.MaxKeys > 0 時啟用
	dedupIndexKey = "dedup:click:index"
)

// NullCacheConfig null cache 水位管控設定
type NullCacheConfig struct {
	// MaxKeys 水位閥值：null cache key 數量超過此值即觸發淘汰
	MaxKeys int64
	// EvictCount 每次觸發淘汰時，移除最舊的 key 數量
	EvictCount int64
}

// DedupConfig 點擊去重複設定
type DedupConfig struct {
	// WindowDuration 去重時間窗口，同一 fingerprint + code 在窗口內只計一次點擊
	WindowDuration time.Duration
	// MaxKeys 水位閥值：去重 key 超過此數量觸發淘汰。0 = 停用水位管控
	MaxKeys int64
	// EvictCount 每次淘汰的 key 數量（MaxKeys > 0 時生效）
	EvictCount int64
}

// Cache 封裝 Redis 操作，提供短網址的快取功能
type Cache struct {
	client       *redis.Client
	nullCfg      NullCacheConfig
	dedupCfg     DedupConfig
	// evictTrigger 水位淘汰觸發訊號（debounce：同時只跑一個淘汰任務）
	// 緩衝大小 1：最多等待一個任務排隊，避免每次 null cache miss 都開新 goroutine
	evictTrigger chan struct{}
}

func NewCache(host, port, password string, db int, nullCfg NullCacheConfig, dedupCfg DedupConfig) *Cache {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})
	c := &Cache{
		client:       client,
		nullCfg:      nullCfg,
		dedupCfg:     dedupCfg,
		evictTrigger: make(chan struct{}, 1),
	}
	go c.evictLoop()
	return c
}

// evictLoop 淘汰事件處理迴圈，保證同時只有一個 null cache 淘汰任務執行
// 由 NewCache 啟動，呼叫 Close() 後停止
func (c *Cache) evictLoop() {
	for range c.evictTrigger {
		c.evictIfOverWatermark(context.Background())
	}
}

// Close 關閉淘汰迴圈（優雅停機時呼叫，確保背景 goroutine 退出）
func (c *Cache) Close() {
	close(c.evictTrigger)
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Client 回傳底層 *redis.Client，供需要共用連線的元件使用（例如 Redis Bloom Filter）
func (c *Cache) Client() *redis.Client {
	return c.client
}

// jitteredTTL 在基礎 TTL 上加入 ±jitter 的隨機抖動
// 避免大量 key 同時到期造成快取雪崩
func jitteredTTL() time.Duration {
	offset := time.Duration(rand.Int63n(int64(shortLinkJitter)*2)) - shortLinkJitter
	return shortLinkBaseTTL + offset
}

// SetShortLink 將短網址寫入 Redis，TTL 帶隨機 jitter 防止雪崩
func (c *Cache) SetShortLink(ctx context.Context, link *shortlink.ShortLink) error {
	data, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("序列化短網址失敗: %w", err)
	}
	key := keyPrefix + link.Code
	if err := c.client.Set(ctx, key, data, jitteredTTL()).Err(); err != nil {
		metrics.CacheHitTotal.WithLabelValues("set", "error").Inc()
		return err
	}
	metrics.CacheHitTotal.WithLabelValues("set", "success").Inc()
	return nil
}

// GetShortLink 從 Redis 取得短網址
// 回傳 (nil, nil) 代表 cache miss；回傳 (nil, ErrNullCache) 代表此 code 已被標記不存在
func (c *Cache) GetShortLink(ctx context.Context, code string) (*shortlink.ShortLink, error) {
	key := keyPrefix + code
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // cache miss，由上層決定是否查 DB
	}
	if err != nil {
		return nil, fmt.Errorf("讀取 Redis 快取失敗: %w", err)
	}

	// 偵測 null cache 哨兵值：此 code 已確認不存在於 DB
	if string(data) == nullValue {
		return nil, shortlink.ErrNullCache
	}

	var link shortlink.ShortLink
	if err := json.Unmarshal(data, &link); err != nil {
		return nil, fmt.Errorf("反序列化短網址失敗: %w", err)
	}
	return &link, nil
}

// SetNullCache 將不存在的短碼寫入快取作為哨兵，並同步更新 sorted set 索引
// 當 sorted set 大小超過水位閥值，非同步淘汰最舊的 key（防記憶體爆炸）
func (c *Cache) SetNullCache(ctx context.Context, code string) error {
	key := keyPrefix + code

	// 用 pipeline 同時寫入 null 值與 sorted set 索引（原子性提升）
	pipe := c.client.Pipeline()
	pipe.Set(ctx, key, nullValue, nullCacheTTL)
	pipe.ZAdd(ctx, nullIndexKey, redis.Z{
		Score:  float64(time.Now().UnixNano()),
		Member: code,
	})
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	// 非同步觸發水位檢查，不阻塞寫入主流程
	// 使用 debounce channel：同時只允許一個淘汰任務排隊，避免每次 miss 都開新 goroutine
	select {
	case c.evictTrigger <- struct{}{}:
	default: // 已有淘汰任務排隊或執行中，跳過
	}
	return nil
}

// evictIfOverWatermark 當 null cache 索引超過水位閥值時，淘汰最舊的 EvictCount 個 key
// 透過 sorted set 的 ZPOPMIN 取得最舊 code 並批次刪除，保持 O(log N) 效率
func (c *Cache) evictIfOverWatermark(ctx context.Context) {
	count, err := c.client.ZCard(ctx, nullIndexKey).Result()
	if err != nil || count <= c.nullCfg.MaxKeys {
		return
	}
	// 觸發淘汰，記錄水位事件（頻繁淘汰代表無效短碼請求量大）
	metrics.NullCacheEvictions.Inc()

	// ZPOPMIN：原子地取出並移除 sorted set 中分數最小（最舊）的 EvictCount 個 member
	items, err := c.client.ZPopMin(ctx, nullIndexKey, c.nullCfg.EvictCount).Result()
	if err != nil || len(items) == 0 {
		return
	}

	// 批次刪除對應的 shortlink key
	keys := make([]string, 0, len(items))
	for _, item := range items {
		if code, ok := item.Member.(string); ok {
			keys = append(keys, keyPrefix+code)
		}
	}
	if len(keys) > 0 {
		c.client.Del(ctx, keys...)
	}
}

// DeleteShortLink 從快取刪除短網址
func (c *Cache) DeleteShortLink(ctx context.Context, code string) error {
	return c.client.Del(ctx, keyPrefix+code).Err()
}

// IsNewClick 判斷此次點擊是否為去重窗口內的首次點擊。
// 使用 Redis SET NX + EX 原子操作：key 不存在時寫入並回傳 true（首次點擊）；
// key 已存在時回傳 false（重複點擊，應跳過寫入 DB）。
// 若 Redis 操作失敗，回傳 (true, err)：寬鬆策略——寧可多計，不可漏計。
// WindowDuration == 0 時停用去重，直接放行（測試環境或停用場景使用）。
func (c *Cache) IsNewClick(ctx context.Context, code, fingerprint string) (bool, error) {
	if c.dedupCfg.WindowDuration == 0 {
		return true, nil
	}
	key := dedupPrefix + fingerprint + ":" + code
	ok, err := c.client.SetNX(ctx, key, "1", c.dedupCfg.WindowDuration).Result()
	if err != nil {
		// Redis 故障時放行，避免去重失效導致漏計真實點擊
		return true, fmt.Errorf("dedup SetNX 失敗: %w", err)
	}
	// 首次點擊且啟用水位管控時，非同步更新索引
	if ok && c.dedupCfg.MaxKeys > 0 {
		go c.trackDedupKey(context.Background(), fingerprint+":"+code)
	}
	return ok, nil
}

// trackDedupKey 將去重 key 加入 sorted set 索引，供水位淘汰使用
func (c *Cache) trackDedupKey(ctx context.Context, member string) {
	c.client.ZAdd(ctx, dedupIndexKey, redis.Z{
		Score:  float64(time.Now().UnixNano()),
		Member: member,
	})
	c.evictDedupIfOverWatermark(ctx)
}

// evictDedupIfOverWatermark 當去重索引超過水位閥值時，淘汰最舊的 EvictCount 個 key
func (c *Cache) evictDedupIfOverWatermark(ctx context.Context) {
	count, err := c.client.ZCard(ctx, dedupIndexKey).Result()
	if err != nil || count <= c.dedupCfg.MaxKeys {
		return
	}

	items, err := c.client.ZPopMin(ctx, dedupIndexKey, c.dedupCfg.EvictCount).Result()
	if err != nil || len(items) == 0 {
		return
	}

	keys := make([]string, 0, len(items))
	for _, item := range items {
		if member, ok := item.Member.(string); ok {
			keys = append(keys, dedupPrefix+member)
		}
	}
	if len(keys) > 0 {
		c.client.Del(ctx, keys...)
	}
}
