package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/penpeer/shortlink/application/usecase"
	"github.com/penpeer/shortlink/domain/shortlink"
)

func TestGetPreview_Success(t *testing.T) {
	linkRepo := newMockShortLinkRepo()

	link := &shortlink.ShortLink{
		Code:          "prev123",
		OriginalURL:   "https://www.example.com",
		OGTitle:       "Example Title",
		OGDescription: "Example Description",
		OGImage:       "https://example.com/img.jpg",
		CreatedAt:     time.Now(),
	}
	_ = linkRepo.Save(context.Background(), link)

	uc := usecase.NewGetPreviewUseCase(linkRepo)

	out, err := uc.Execute(context.Background(), "prev123")
	if err != nil {
		t.Fatalf("預期成功但得到錯誤: %v", err)
	}
	if out.OGTitle != "Example Title" {
		t.Errorf("OG 標題不符：got %q", out.OGTitle)
	}
	if out.OGDescription != "Example Description" {
		t.Errorf("OG 描述不符：got %q", out.OGDescription)
	}
	if out.OGImage != "https://example.com/img.jpg" {
		t.Errorf("OG 圖片不符：got %q", out.OGImage)
	}
}

func TestGetPreview_NotFound(t *testing.T) {
	linkRepo := newMockShortLinkRepo()
	uc := usecase.NewGetPreviewUseCase(linkRepo)

	_, err := uc.Execute(context.Background(), "noexist")
	if err == nil {
		t.Error("短碼不存在時應回傳錯誤")
	}
}
