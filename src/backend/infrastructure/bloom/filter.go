package bloom

import (
	"sync"

	gobloom "github.com/bits-and-blooms/bloom/v3"
)

// ShortCodeBloom 封裝 bloom filter，提供執行緒安全的短碼存在性篩查
// 特性：
//   - false negative 為 0（確定不存在的一定不存在）
//   - false positive 率可控（預設 1%，即最多 1% 的不存在 code 被誤判為存在）
type ShortCodeBloom struct {
	mu     sync.RWMutex
	filter *gobloom.BloomFilter
}

// New 建立 bloom filter
// capacity：預期短碼最大數量；fpRate：可接受的誤判率（0.0 ~ 1.0，建議 0.01）
func New(capacity uint, fpRate float64) *ShortCodeBloom {
	return &ShortCodeBloom{
		filter: gobloom.NewWithEstimates(capacity, fpRate),
	}
}

// Add 將短碼加入 bloom filter（建立短網址時呼叫）
func (b *ShortCodeBloom) Add(code string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.filter.AddString(code)
}

// MightExist 查詢短碼是否「可能存在」
//   - 回傳 false → 短碼肯定不存在，直接拒絕，無需查 Redis 或 DB
//   - 回傳 true  → 短碼可能存在，繼續走快取 / DB 查詢流程
func (b *ShortCodeBloom) MightExist(code string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.filter.TestString(code)
}
