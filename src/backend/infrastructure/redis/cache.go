package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/penpeer/shortlink/domain/shortlink"
)

const (
	// 短網址快取 TTL：24 小時，redirect 效能關鍵路徑
	shortLinkTTL = 24 * time.Hour
	keyPrefix    = "shortlink:"
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

// SetShortLink 將短網址寫入 Redis
func (c *Cache) SetShortLink(ctx context.Context, link *shortlink.ShortLink) error {
	data, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("序列化短網址失敗: %w", err)
	}
	key := keyPrefix + link.Code
	return c.client.Set(ctx, key, data, shortLinkTTL).Err()
}

// GetShortLink 從 Redis 取得短網址，若不存在回傳 nil
func (c *Cache) GetShortLink(ctx context.Context, code string) (*shortlink.ShortLink, error) {
	key := keyPrefix + code
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Cache miss，由上層查 DB
	}
	if err != nil {
		return nil, fmt.Errorf("讀取 Redis 快取失敗: %w", err)
	}

	var link shortlink.ShortLink
	if err := json.Unmarshal(data, &link); err != nil {
		return nil, fmt.Errorf("反序列化短網址失敗: %w", err)
	}
	return &link, nil
}

// DeleteShortLink 從快取刪除短網址
func (c *Cache) DeleteShortLink(ctx context.Context, code string) error {
	return c.client.Del(ctx, keyPrefix+code).Err()
}
