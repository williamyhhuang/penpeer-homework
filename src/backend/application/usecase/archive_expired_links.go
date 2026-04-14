package usecase

import (
	"context"
	"fmt"
)

// ArchiveRepository 封存操作的 Repository 介面（由 infrastructure 層實作）
type ArchiveRepository interface {
	ArchiveExpiredShortLinks(ctx context.Context) (int64, error)
	ArchiveOldClickEvents(ctx context.Context, retentionDays int) (int64, error)
}

// ArchiveExpiredLinksUseCase 將過期的短網址（含推薦碼與點擊事件）搬移至封存表
type ArchiveExpiredLinksUseCase struct {
	archiveRepo ArchiveRepository
}

func NewArchiveExpiredLinksUseCase(archiveRepo ArchiveRepository) *ArchiveExpiredLinksUseCase {
	return &ArchiveExpiredLinksUseCase{archiveRepo: archiveRepo}
}

// Execute 執行封存，回傳封存的短網址筆數
func (uc *ArchiveExpiredLinksUseCase) Execute(ctx context.Context) (int64, error) {
	count, err := uc.archiveRepo.ArchiveExpiredShortLinks(ctx)
	if err != nil {
		return 0, fmt.Errorf("封存過期短網址失敗: %w", err)
	}
	return count, nil
}
