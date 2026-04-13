package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/penpeer/shortlink/domain/shortlink"
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

	keyPrefix    = "shortlink:"
	// nullIndexKey sorted set：score=寫入時間戳(ns)，member=short code
	// 用於水位到達時能快速找到最舊的 null cache key 進行淘汰
	nullIndexKey = "nullcache:index"
)

// NullCacheConfig null cache 水位管控設定
type NullCacheConfig struct {
	// MaxKeys 水位閥值：null cache key 數量超過此值即觸發淘汰
	MaxKeys int64
	// EvictCount 每次觸發淘汰時，移除最舊的 key 數量
	EvictCount int64
}

// Cache 封裝 Redis 操作，提供短網址的快取功能
type Cache struct {
	client  *redis.Client
	nullCfg NullCacheConfig
}

func NewCache(host, port, password string, db int, nullCfg NullCacheConfig) *Cache {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})
	return &Cache{client: client, nullCfg: nullCfg}
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
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
	return c.client.Set(ctx, key, data, jitteredTTL()).Err()
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
	go c.evictIfOverWatermark(context.Background())
	return nil
}

// evictIfOverWatermark 當 null cache 索引超過水位閥值時，淘汰最舊的 EvictCount 個 key
// 透過 sorted set 的 ZPOPMIN 取得最舊 code 並批次刪除，保持 O(log N) 效率
func (c *Cache) evictIfOverWatermark(ctx context.Context) {
	count, err := c.client.ZCard(ctx, nullIndexKey).Result()
	if err != nil || count <= c.nullCfg.MaxKeys {
		return
	}

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
