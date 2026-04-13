package usecase

import (
	"context"
	"fmt"

	"github.com/penpeer/shortlink/domain/shortlink"
)

// PreviewOutput OG 預覽資料輸出
type PreviewOutput struct {
	Code          string
	OriginalURL   string
	OGTitle       string
	OGDescription string
	OGImage       string
}

// GetPreviewUseCase 取得短網址的 OG 預覽資料
type GetPreviewUseCase struct {
	linkRepo shortlink.Repository
}

func NewGetPreviewUseCase(linkRepo shortlink.Repository) *GetPreviewUseCase {
	return &GetPreviewUseCase{linkRepo: linkRepo}
}

func (uc *GetPreviewUseCase) Execute(ctx context.Context, code string) (*PreviewOutput, error) {
	link, err := uc.linkRepo.FindByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("查詢短網址失敗: %w", err)
	}
	if link == nil {
		return nil, fmt.Errorf("短碼不存在: %s", code)
	}
	return &PreviewOutput{
		Code:          link.Code,
		OriginalURL:   link.OriginalURL,
		OGTitle:       link.OGTitle,
		OGDescription: link.OGDescription,
		OGImage:       link.OGImage,
	}, nil
}
