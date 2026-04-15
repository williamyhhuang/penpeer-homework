package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/penpeer/shortlink/infrastructure/metrics"
)

// PrometheusMiddleware 記錄每個 HTTP 請求的 Prometheus 指標。
// 需掛在所有業務 middleware 之前（gin.Default 之後），確保計時涵蓋完整請求週期。
// 使用 c.FullPath() 取得路由 pattern（如 "/:code"）而非實際 URL，避免 label 高基數問題。
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// FullPath() 回傳 Gin 路由 pattern，e.g. "/:code"、"/api/v1/links"
		// 對於未匹配任何路由的請求回傳空字串，標記為 "unmatched"
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		method := c.Request.Method

		metrics.HTTPRequestsInFlight.Inc()
		defer metrics.HTTPRequestsInFlight.Dec()

		c.Next()

		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(c.Writer.Status())

		metrics.HTTPRequestsTotal.WithLabelValues(method, path, statusCode).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
	}
}
