package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/penpeer/shortlink/application/usecase"
)

// RedirectHandler 處理短網址的核心重新導向邏輯
type RedirectHandler struct {
	redirectUC *usecase.RedirectShortLinkUseCase
}

func NewRedirectHandler(redirectUC *usecase.RedirectShortLinkUseCase) *RedirectHandler {
	return &RedirectHandler{redirectUC: redirectUC}
}

// GET /:code
// 這是系統的核心效能路徑，Bot 回傳 OG HTML，一般使用者 302 redirect
func (h *RedirectHandler) Redirect(c *gin.Context) {
	code := c.Param("code")

	out, err := h.redirectUC.Execute(c.Request.Context(), usecase.RedirectInput{
		Code:         code,
		UserAgent:    c.Request.UserAgent(),
		ReferralCode: c.Query("ref"),
		CFIPCountry:  c.GetHeader("CF-IPCountry"),
		XCountry:     c.GetHeader("X-Country"),
		ClientIP:     c.ClientIP(),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if out.IsBot {
		// 社群平台 Bot：回傳含 OG meta tags 的 HTML
		// 不加 Cache-Control，讓平台可以快取預覽
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, out.OGHTML)
		return
	}

	// 一般使用者：302 redirect（非 301，保留修改 URL 的彈性）
	c.Redirect(http.StatusFound, out.OriginalURL)
}
