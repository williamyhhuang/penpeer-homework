package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/penpeer/shortlink/application/usecase"
)

// ── Mock ArchiveRepository ────────────────────────────────────────────────────

type mockArchiveRepo struct {
	// 控制回傳值
	expiredCount    int64
	expiredErr      error
	oldClicksCount  int64
	oldClicksErr    error
	// 記錄呼叫參數
	lastRetentionDays int
}

func (m *mockArchiveRepo) ArchiveExpiredShortLinks(_ context.Context) (int64, error) {
	return m.expiredCount, m.expiredErr
}

func (m *mockArchiveRepo) ArchiveOldClickEvents(_ context.Context, retentionDays int) (int64, error) {
	m.lastRetentionDays = retentionDays
	return m.oldClicksCount, m.oldClicksErr
}

// ── ArchiveExpiredLinksUseCase 測試 ───────────────────────────────────────────

func TestArchiveExpiredLinks_成功封存(t *testing.T) {
	repo := &mockArchiveRepo{expiredCount: 3}
	uc := usecase.NewArchiveExpiredLinksUseCase(repo)

	count, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("預期無錯誤，但得到: %v", err)
	}
	if count != 3 {
		t.Errorf("預期封存 3 筆，但得到 %d", count)
	}
}

func TestArchiveExpiredLinks_無過期資料(t *testing.T) {
	repo := &mockArchiveRepo{expiredCount: 0}
	uc := usecase.NewArchiveExpiredLinksUseCase(repo)

	count, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("無過期資料不應回傳錯誤，但得到: %v", err)
	}
	if count != 0 {
		t.Errorf("預期封存 0 筆，但得到 %d", count)
	}
}

func TestArchiveExpiredLinks_repo錯誤時回傳錯誤(t *testing.T) {
	repo := &mockArchiveRepo{expiredErr: errors.New("db error")}
	uc := usecase.NewArchiveExpiredLinksUseCase(repo)

	_, err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("預期回傳錯誤，但得到 nil")
	}
}

// ── ArchiveOldClicksUseCase 測試 ──────────────────────────────────────────────

func TestArchiveOldClicks_成功封存(t *testing.T) {
	repo := &mockArchiveRepo{oldClicksCount: 150}
	uc := usecase.NewArchiveOldClicksUseCase(repo, 90)

	count, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("預期無錯誤，但得到: %v", err)
	}
	if count != 150 {
		t.Errorf("預期封存 150 筆，但得到 %d", count)
	}
}

func TestArchiveOldClicks_正確傳遞保留天數(t *testing.T) {
	repo := &mockArchiveRepo{oldClicksCount: 0}
	uc := usecase.NewArchiveOldClicksUseCase(repo, 30)

	_, _ = uc.Execute(context.Background())

	if repo.lastRetentionDays != 30 {
		t.Errorf("預期保留天數為 30，但得到 %d", repo.lastRetentionDays)
	}
}

func TestArchiveOldClicks_repo錯誤時回傳錯誤(t *testing.T) {
	repo := &mockArchiveRepo{oldClicksErr: errors.New("db error")}
	uc := usecase.NewArchiveOldClicksUseCase(repo, 90)

	_, err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("預期回傳錯誤，但得到 nil")
	}
}
