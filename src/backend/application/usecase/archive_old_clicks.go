package usecase

import (
	"context"
	"fmt"
)

// ArchiveOldClicksUseCase 將超過保留天數的點擊事件搬移至封存表
// 應在 ArchiveExpiredLinksUseCase 執行完畢後再執行，
// 確保過期 short_links 的事件已在 Step 1 處理，此處只處理 active short_links 的舊事件
type ArchiveOldClicksUseCase struct {
	archiveRepo   ArchiveRepository
	retentionDays int
}

func NewArchiveOldClicksUseCase(archiveRepo ArchiveRepository, retentionDays int) *ArchiveOldClicksUseCase {
	return &ArchiveOldClicksUseCase{
		archiveRepo:   archiveRepo,
		retentionDays: retentionDays,
	}
}

// Execute 執行封存，回傳封存的點擊事件筆數
func (uc *ArchiveOldClicksUseCase) Execute(ctx context.Context) (int64, error) {
	count, err := uc.archiveRepo.ArchiveOldClickEvents(ctx, uc.retentionDays)
	if err != nil {
		return 0, fmt.Errorf("封存舊點擊事件失敗: %w", err)
	}
	return count, nil
}
