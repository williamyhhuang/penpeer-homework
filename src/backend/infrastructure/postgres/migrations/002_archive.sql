-- ============================================================
-- 002_archive.sql  封存表：保留過期與歷史資料供日後查閱
-- 由 cleanup container 定期將過期 short_links 及舊 click_events
-- 搬移至此，避免主要資料表無限成長
-- ============================================================

-- ──────────────────────────────────────────────────────────────
-- 短網址封存表
-- ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS short_links_archive (
    code            VARCHAR(20)   NOT NULL,
    original_url    TEXT          NOT NULL,
    og_title        TEXT          NOT NULL DEFAULT '',
    og_description  TEXT          NOT NULL DEFAULT '',
    og_image        TEXT          NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ   NOT NULL,
    expires_at      TIMESTAMPTZ,
    archived_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE  short_links_archive             IS '短網址封存表：存放已過期並從 short_links 搬出的歷史紀錄';
COMMENT ON COLUMN short_links_archive.code        IS '原短網址短碼（非主鍵，允許重複封存）';
COMMENT ON COLUMN short_links_archive.archived_at IS '封存時間，由 cleanup 任務寫入';

CREATE INDEX IF NOT EXISTS idx_short_links_archive_code        ON short_links_archive(code);
CREATE INDEX IF NOT EXISTS idx_short_links_archive_archived_at ON short_links_archive(archived_at);

-- ──────────────────────────────────────────────────────────────
-- 推薦碼封存表
-- ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS referral_codes_archive (
    code            VARCHAR(100)  NOT NULL,
    owner_id        VARCHAR(200)  NOT NULL,
    short_link_code VARCHAR(20)   NOT NULL,
    created_at      TIMESTAMPTZ   NOT NULL,
    archived_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE  referral_codes_archive             IS '推薦碼封存表：隨 short_links 封存一同搬入';
COMMENT ON COLUMN referral_codes_archive.archived_at IS '封存時間，與對應 short_links_archive 相同批次';

CREATE INDEX IF NOT EXISTS idx_referral_codes_archive_short_link ON referral_codes_archive(short_link_code);
CREATE INDEX IF NOT EXISTS idx_referral_codes_archive_archived_at ON referral_codes_archive(archived_at);

-- ──────────────────────────────────────────────────────────────
-- 點擊事件封存表
-- 兩種情境皆會寫入：
--   1. 隨過期 short_link 封存（Step 1）
--   2. 超過保留天數的舊事件封存（Step 2，active short_link 的舊資料）
-- ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS click_events_archive (
    id              BIGINT        NOT NULL,
    short_link_code VARCHAR(20)   NOT NULL,
    clicked_at      TIMESTAMPTZ   NOT NULL,
    platform        VARCHAR(50)   NOT NULL DEFAULT 'unknown',
    region          VARCHAR(100)  NOT NULL DEFAULT '',
    device_type     VARCHAR(20)   NOT NULL DEFAULT 'desktop',
    referral_code   VARCHAR(100)  NOT NULL DEFAULT '',
    archived_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE  click_events_archive             IS '點擊事件封存表：保留超過保留期限或隨 short_link 封存的歷史事件';
COMMENT ON COLUMN click_events_archive.id          IS '原 click_events.id，封存後不再自增';
COMMENT ON COLUMN click_events_archive.archived_at IS '封存時間，由 cleanup 任務寫入';

CREATE INDEX IF NOT EXISTS idx_click_events_archive_short_link_code ON click_events_archive(short_link_code);
CREATE INDEX IF NOT EXISTS idx_click_events_archive_clicked_at      ON click_events_archive(clicked_at);
CREATE INDEX IF NOT EXISTS idx_click_events_archive_archived_at     ON click_events_archive(archived_at);
