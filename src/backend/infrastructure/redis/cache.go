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
	shortLinkJitter = 288 * time.Minute // 24h * 20% = 4.8h ≈ 288 min

	// nullCacheTTL 不存在短碼的快取時間，較短避免佔用記憶體（防快取穿透）
	nullCacheTTL = 5 * time.Minute
	// nullValue Redis 中標記「此短碼不存在於 DB」的哨兵值
	nullValue = "__null__"

	keyPrefix = "shortlink:"
)

// Cache 封裝 Redis 操作，提供短網址的快取功能
type Cache struct {
	client *redis.Client
}

func NewCache(host, port, password string, db int) *Cache {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})
	return &Cache{client: client}
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

// SetNullCache 將不存在的短碼寫入快取作為哨兵，防止相同請求反覆查 DB（快取穿透防護）
func (c *Cache) SetNullCache(ctx context.Context, code string) error {
	key := keyPrefix + code
	return c.client.Set(ctx, key, nullValue, nullCacheTTL).Err()
}

// DeleteShortLink 從快取刪除短網址
func (c *Cache) DeleteShortLink(ctx context.Context, code string) error {
	return c.client.Del(ctx, keyPrefix+code).Err()
}
