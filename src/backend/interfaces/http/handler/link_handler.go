package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/penpeer/shortlink/application/usecase"
)

// LinkHandler 處理短網址 CRUD 與統計 API
type LinkHandler struct {
	createUC    *usecase.CreateShortLinkUseCase
	previewUC   *usecase.GetPreviewUseCase
	analyticsUC *usecase.GetAnalyticsUseCase
}

func NewLinkHandler(
	createUC *usecase.CreateShortLinkUseCase,
	previewUC *usecase.GetPreviewUseCase,
	analyticsUC *usecase.GetAnalyticsUseCase,
) *LinkHandler {
	return &LinkHandler{
		createUC:    createUC,
		previewUC:   previewUC,
		analyticsUC: analyticsUC,
	}
}

// CreateShortLinkRequest 建立短網址的 API 請求格式
type CreateShortLinkRequest struct {
	URL             string `json:"url"              binding:"required"`
	ReferralOwnerID string `json:"referral_owner_id"` // 選填：建立推薦碼
}

// POST /api/v1/links
func (h *LinkHandler) CreateShortLink(c *gin.Context) {
	var req CreateShortLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "請求格式錯誤: " + err.Error()})
		return
	}

	out, err := h.createUC.Execute(c.Request.Context(), usecase.CreateShortLinkInput{
		URL:             req.URL,
		ReferralOwnerID: req.ReferralOwnerID,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{
		"code":         out.ShortLink.Code,
		"original_url": out.ShortLink.OriginalURL,
		"og_title":     out.ShortLink.OGTitle,
		"og_image":     out.ShortLink.OGImage,
		"created_at":   out.ShortLink.CreatedAt,
	}
	if out.ReferralCode != nil {
		resp["referral_code"] = out.ReferralCode.Code
	}

	c.JSON(http.StatusCreated, resp)
}

// GET /api/v1/links/:code/preview
func (h *LinkHandler) GetPreview(c *gin.Context) {
	code := c.Param("code")
	out, err := h.previewUC.Execute(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":           out.Code,
		"original_url":   out.OriginalURL,
		"og_title":       out.OGTitle,
		"og_description": out.OGDescription,
		"og_image":       out.OGImage,
	})
}

// GET /api/v1/links/:code/analytics
func (h *LinkHandler) GetAnalytics(c *gin.Context) {
	code := c.Param("code")
	out, err := h.analyticsUC.Execute(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}
