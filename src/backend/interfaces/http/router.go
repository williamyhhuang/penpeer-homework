package http

import (
	"github.com/gin-gonic/gin"
	"github.com/penpeer/shortlink/interfaces/http/handler"
	"github.com/penpeer/shortlink/interfaces/http/middleware"
)

// NewRouter 建立 Gin 路由，依照 API 設計掛載所有 handler
func NewRouter(
	linkHandler     *handler.LinkHandler,
	redirectHandler *handler.RedirectHandler,
	rlCfg           middleware.RateLimitConfig,
) *gin.Engine {
	r := gin.Default()

	// CORS：允許前端跨來源請求
	r.Use(corsMiddleware())

	// 短網址核心 redirect（效能關鍵路徑，per-IP rate limit 防止惡意掃碼）
	r.GET("/:code", middleware.RateLimitMiddleware(rlCfg), redirectHandler.Redirect)

	// REST API
	api := r.Group("/api/v1")
	{
		links := api.Group("/links")
		{
			links.POST("", linkHandler.CreateShortLink)
			// 靜態路由需在參數路由前宣告，否則 Gin 會將 "ranking" 解析為 :code
			links.GET("/ranking", linkHandler.GetRanking)
			links.GET("/:code/preview", linkHandler.GetPreview)
			links.GET("/:code/analytics", linkHandler.GetAnalytics)
		}
	}

	// 健康檢查端點（供 Docker healthcheck 使用）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}

// corsMiddleware 允許前端開發伺服器（localhost:3000）跨來源存取
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
