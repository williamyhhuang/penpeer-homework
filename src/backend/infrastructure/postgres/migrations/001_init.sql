-- 短網址主表：儲存短碼、原始 URL 與 OG 預覽資料
CREATE TABLE IF NOT EXISTS short_links (
    code          VARCHAR(20)  PRIMARY KEY,
    original_url  TEXT         NOT NULL,
    og_title      TEXT         NOT NULL DEFAULT '',
    og_description TEXT        NOT NULL DEFAULT '',
    og_image      TEXT         NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ
);

-- 推薦碼表：記錄行銷推薦碼與關聯短碼
CREATE TABLE IF NOT EXISTS referral_codes (
    code            VARCHAR(100) PRIMARY KEY,
    owner_id        VARCHAR(200) NOT NULL,
    short_link_code VARCHAR(20)  NOT NULL REFERENCES short_links(code) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 點擊事件表：非同步寫入，記錄每次點擊的詳細資訊
CREATE TABLE IF NOT EXISTS click_events (
    id              BIGSERIAL    PRIMARY KEY,
    short_link_code VARCHAR(20)  NOT NULL REFERENCES short_links(code) ON DELETE CASCADE,
    clicked_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    platform        VARCHAR(50)  NOT NULL DEFAULT 'unknown',
    region          VARCHAR(100) NOT NULL DEFAULT '',
    device_type     VARCHAR(20)  NOT NULL DEFAULT 'desktop',
    referral_code   VARCHAR(100) NOT NULL DEFAULT ''
);

-- 加速統計查詢
CREATE INDEX IF NOT EXISTS idx_click_events_short_link_code ON click_events(short_link_code);
CREATE INDEX IF NOT EXISTS idx_click_events_clicked_at      ON click_events(clicked_at);
CREATE INDEX IF NOT EXISTS idx_referral_codes_short_link    ON referral_codes(short_link_code);
