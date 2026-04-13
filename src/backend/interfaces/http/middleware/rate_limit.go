package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimitConfig per-IP rate limit 設定
type RateLimitConfig struct {
	// RPS 每秒允許的請求數（固定窗口算法，窗口大小 = 1 秒）
	RPS int
	// Burst 短時間突發上限：單一窗口內最多允許 Burst 個請求（≥ RPS）
	Burst int
}

// ipEntry 記錄單一 IP 在當前窗口內的請求狀況
type ipEntry struct {
	mu       sync.Mutex
	count    int       // 當前窗口已計數
	resetAt  time.Time // 窗口重置時間點
	lastSeen time.Time // 最後一次請求時間（用於清理長時間不活躍的 entry）
}

// RateLimitMiddleware 建立 per-IP 固定窗口速率限制 middleware
// 超過 Burst 上限時回傳 429 Too Many Requests
// 背景協程每 5 分鐘清理 10 分鐘未活躍的 IP，防止記憶體無限增長
func RateLimitMiddleware(cfg RateLimitConfig) gin.HandlerFunc {
	var limiters sync.Map

	// 背景清理：移除長時間未活躍的 IP entry，防止 sync.Map 無限膨脹
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			limiters.Range(func(k, v any) bool {
				e := v.(*ipEntry)
				e.mu.Lock()
				idle := now.Sub(e.lastSeen)
				e.mu.Unlock()
				if idle > 10*time.Minute {
					limiters.Delete(k)
				}
				return true
			})
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		// LoadOrStore 保證同一 IP 只有一個 entry，並發安全
		val, _ := limiters.LoadOrStore(ip, &ipEntry{
			resetAt:  now.Add(time.Second),
			lastSeen: now,
		})
		e := val.(*ipEntry)

		e.mu.Lock()
		// 窗口過期：重置計數
		if now.After(e.resetAt) {
			e.count = 0
			e.resetAt = now.Add(time.Second)
		}
		e.count++
		e.lastSeen = now
		over := e.count > cfg.Burst
		e.mu.Unlock()

		if over {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "請求頻率過高，請稍後再試",
			})
			return
		}
		c.Next()
	}
}
