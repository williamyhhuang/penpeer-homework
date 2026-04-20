package bloom

import (
	"context"
	"hash/fnv"
	"log"
	"math"

	"github.com/redis/go-redis/v9"
)

const redisBloomKey = "bloom:shortcodes"

// RedisBloomFilter 以 Redis Bitmap 實作的分散式 Bloom Filter。
// 所有 pod 共用同一個 Redis key，解決 local memory bloom 各 pod 狀態不一致的問題。
//
// 使用 enhanced double hashing：gi(x) = (h1(x) + i·h2(x)) mod m
// Redis 故障時 MightExist 一律回傳 true（寬鬆策略，不誤殺正常請求）。
type RedisBloomFilter struct {
	client *redis.Client
	m      uint64 // bitmap 總 bit 數
	k      uint   // hash function 數量
}

// NewRedis 建立 Redis 分散式 Bloom Filter。
// capacity：預期短碼最大數量；fpRate：可接受的誤判率（例如 0.01 = 1%）。
func NewRedis(client *redis.Client, capacity uint, fpRate float64) *RedisBloomFilter {
	m, k := bloomParams(capacity, fpRate)
	return &RedisBloomFilter{client: client, m: m, k: k}
}

// Add 將短碼寫入 Redis Bitmap（建立短網址時呼叫）。
// 使用 Pipeline 一次送出 k 個 SETBIT，減少 RTT。
// 失敗時只記錄 log，不影響主流程（允許 false negative 略微增加）。
func (r *RedisBloomFilter) Add(code string) {
	ctx := context.Background()
	pipe := r.client.Pipeline()
	for _, offset := range r.hashOffsets(code) {
		pipe.SetBit(ctx, redisBloomKey, offset, 1)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("[bloom] Add 失敗 code=%s: %v", code, err)
	}
}

// MightExist 查詢短碼是否「可能存在」。
// 回傳 false → 短碼肯定不存在（k 個 bit 中有至少一個為 0）。
// 回傳 true  → 可能存在（繼續查 Redis/DB），或 Redis 故障時的寬鬆放行。
func (r *RedisBloomFilter) MightExist(code string) bool {
	ctx := context.Background()
	offsets := r.hashOffsets(code)
	pipe := r.client.Pipeline()
	cmds := make([]*redis.IntCmd, len(offsets))
	for i, offset := range offsets {
		cmds[i] = pipe.GetBit(ctx, redisBloomKey, offset)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		// Redis 故障：寬鬆放行，避免誤殺正常請求
		log.Printf("[bloom] MightExist 失敗 code=%s，放行: %v", code, err)
		return true
	}
	for _, cmd := range cmds {
		if cmd.Val() == 0 {
			return false
		}
	}
	return true
}

// hashOffsets 使用 enhanced double hashing 產生 k 個 bitmap 偏移量。
// gi(x) = (h1(x) + i·h2(x)) mod m
// h1 = FNV-1a 64bit，h2 = FNV-1 64bit（兩者獨立，分布良好）
func (r *RedisBloomFilter) hashOffsets(code string) []int64 {
	h1 := fnv.New64a()
	h1.Write([]byte(code))
	a := h1.Sum64()

	h2 := fnv.New64()
	h2.Write([]byte(code))
	b := h2.Sum64()

	offsets := make([]int64, r.k)
	for i := uint(0); i < r.k; i++ {
		offsets[i] = int64((a + uint64(i)*b) % r.m)
	}
	return offsets
}

// bloomParams 由容量與誤判率推算最優 bitmap 大小 m 與 hash 數量 k。
// m = -n·ln(p) / (ln2)²
// k = round(m/n · ln2)
func bloomParams(capacity uint, fpRate float64) (m uint64, k uint) {
	n := float64(capacity)
	ln2 := math.Log(2)
	m = uint64(math.Ceil(-n * math.Log(fpRate) / (ln2 * ln2)))
	k = uint(math.Round(float64(m) / n * ln2))
	if k < 1 {
		k = 1
	}
	return m, k
}
