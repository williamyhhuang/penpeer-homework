-- ============================================================
-- 001_init.sql  短網址系統初始化 Schema
-- ============================================================

-- ──────────────────────────────────────────────────────────────
-- 短網址主表
-- ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS short_links (
    code            VARCHAR(20)   PRIMARY KEY,
    original_url    TEXT          NOT NULL,
    og_title        TEXT          NOT NULL DEFAULT '',
    og_description  TEXT          NOT NULL DEFAULT '',
    og_image        TEXT          NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,
    valid_flag      BOOLEAN       NOT NULL DEFAULT TRUE
);

-- 冪等補欄：若從舊版升上來（表已存在但缺少 valid_flag），補上欄位
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS valid_flag BOOLEAN NOT NULL DEFAULT TRUE;

COMMENT ON TABLE  short_links                 IS '短網址主表：儲存短碼、原始網址與 Open Graph 預覽資料';
COMMENT ON COLUMN short_links.code            IS '短網址唯一識別碼（主鍵），用於 redirect 路徑查詢，例如 /gh001';
COMMENT ON COLUMN short_links.original_url    IS '原始長網址，使用者點擊後 redirect 至此';
COMMENT ON COLUMN short_links.og_title        IS 'Open Graph 標題，用於社群分享預覽卡片標題';
COMMENT ON COLUMN short_links.og_description  IS 'Open Graph 描述，用於社群分享預覽卡片摘要文字';
COMMENT ON COLUMN short_links.og_image        IS 'Open Graph 預覽圖 URL，用於社群分享縮圖';
COMMENT ON COLUMN short_links.created_at      IS '短網址建立時間';
COMMENT ON COLUMN short_links.expires_at      IS '到期時間，NULL 表示永不過期；到期後 redirect 應回傳 410 Gone';
COMMENT ON COLUMN short_links.valid_flag      IS '邏輯刪除旗標：TRUE 為有效，FALSE 為已停用（軟刪除），保留歷史統計不受影響';

-- ──────────────────────────────────────────────────────────────
-- 推薦碼表
-- ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS referral_codes (
    code            VARCHAR(100)  PRIMARY KEY,
    owner_id        VARCHAR(200)  NOT NULL,
    short_link_code VARCHAR(20)   NOT NULL REFERENCES short_links(code) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    valid_flag      BOOLEAN       NOT NULL DEFAULT TRUE
);

-- 冪等補欄：若從舊版升上來缺少 valid_flag，補上欄位
ALTER TABLE referral_codes ADD COLUMN IF NOT EXISTS valid_flag BOOLEAN NOT NULL DEFAULT TRUE;

COMMENT ON TABLE  referral_codes                    IS '推薦碼表：記錄行銷人員（KOL / 業務）的推薦碼與對應短網址';
COMMENT ON COLUMN referral_codes.code               IS '推薦碼唯一識別碼（主鍵），附加於短網址後供來源追蹤，例如 ?ref=REF_GH_KOL1';
COMMENT ON COLUMN referral_codes.owner_id           IS '推薦碼擁有者識別碼，可為內部使用者 ID 或外部行銷人員 ID';
COMMENT ON COLUMN referral_codes.short_link_code    IS '關聯的短網址代碼（FK），指向 short_links.code';
COMMENT ON COLUMN referral_codes.created_at         IS '推薦碼建立時間';
COMMENT ON COLUMN referral_codes.valid_flag         IS '邏輯刪除旗標：TRUE 為有效，FALSE 為已停用；停用後該推薦碼不再計入新點擊統計';

-- ──────────────────────────────────────────────────────────────
-- 點擊事件表（append-only audit log，不設 valid_flag）
-- 此表為不可變歷史紀錄，軟刪除會破壞統計正確性
-- ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS click_events (
    id              BIGSERIAL     PRIMARY KEY,
    short_link_code VARCHAR(20)   NOT NULL REFERENCES short_links(code) ON DELETE CASCADE,
    clicked_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    platform        VARCHAR(50)   NOT NULL DEFAULT 'unknown',
    region          VARCHAR(100)  NOT NULL DEFAULT '',
    device_type     VARCHAR(20)   NOT NULL DEFAULT 'desktop',
    referral_code   VARCHAR(100)  NOT NULL DEFAULT ''
);

COMMENT ON TABLE  click_events                  IS '點擊事件表：append-only 不可變歷史日誌，每筆代表一次真實點擊行為';
COMMENT ON COLUMN click_events.id               IS '自增主鍵，唯一識別每一筆點擊事件';
COMMENT ON COLUMN click_events.short_link_code  IS '被點擊的短網址代碼（FK），指向 short_links.code';
COMMENT ON COLUMN click_events.clicked_at       IS '點擊發生的時間戳，為統計分析與趨勢圖表的時間軸依據';
COMMENT ON COLUMN click_events.platform         IS '流量來源平台，例如 facebook / instagram / twitter / line / direct';
COMMENT ON COLUMN click_events.region           IS '點擊者所在地區，由伺服器端 IP 反查取得，例如 台北 / 台中 / 高雄';
COMMENT ON COLUMN click_events.device_type      IS '裝置類型：mobile / desktop / tablet';
COMMENT ON COLUMN click_events.referral_code    IS '攜帶的推薦碼，空字串表示直接點擊（非推薦流量）';

-- ──────────────────────────────────────────────────────────────
-- 加速統計查詢用 Index
-- ──────────────────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_click_events_short_link_code ON click_events(short_link_code);
CREATE INDEX IF NOT EXISTS idx_click_events_clicked_at      ON click_events(clicked_at);
CREATE INDEX IF NOT EXISTS idx_referral_codes_short_link    ON referral_codes(short_link_code);
-- 加速「有效記錄」查詢，配合 valid_flag = TRUE 的 WHERE 條件
CREATE INDEX IF NOT EXISTS idx_short_links_valid_flag       ON short_links(valid_flag);
CREATE INDEX IF NOT EXISTS idx_referral_codes_valid_flag    ON referral_codes(valid_flag);

-- ============================================================
-- Demo Fake Data
-- 使用 DO 區塊確保冪等性：僅在資料表為空時插入，避免重複執行 migration 產生重複資料
-- ============================================================
DO $$
BEGIN

-- ── short_links（15 筆）────────────────────────────────────────
IF NOT EXISTS (SELECT 1 FROM short_links LIMIT 1) THEN
  INSERT INTO short_links
    (code, original_url, og_title, og_description, og_image, created_at, expires_at, valid_flag)
  VALUES
    ('gh001', 'https://github.com',
     'GitHub', '全球最大程式碼托管平台，開發者協作中心',
     'https://github.githubassets.com/images/modules/site/social-cards/github-social.png',
     NOW() - INTERVAL '28 days', NULL, TRUE),

    ('yt001', 'https://youtube.com',
     'YouTube', '全球最大影音平台，每分鐘上傳超過 500 小時影片',
     'https://www.youtube.com/img/desktop/yt_1200.png',
     NOW() - INTERVAL '25 days', NULL, TRUE),

    ('tw001', 'https://twitter.com',
     'Twitter / X', '即時社群討論平台，全球輿論風向標',
     'https://abs.twimg.com/responsive-web/client-web/icon-ios.b1fc727a.png',
     NOW() - INTERVAL '22 days', NULL, TRUE),

    ('fb001', 'https://facebook.com',
     'Facebook', '全球最大社群媒體平台，月活躍用戶逾 30 億',
     'https://www.facebook.com/images/fb_icon_325x325.png',
     NOW() - INTERVAL '20 days', NULL, TRUE),

    ('ig001', 'https://instagram.com',
     'Instagram', '圖片與短影片社群平台，視覺內容行銷首選',
     'https://www.instagram.com/static/images/ico/favicon_200.png/ab6eff595bb1.png',
     NOW() - INTERVAL '18 days', NULL, TRUE),

    ('li001', 'https://linkedin.com',
     'LinkedIn', '全球最大職業社群平台，B2B 行銷主力渠道',
     'https://static.licdn.com/sc/h/al2o9zrvru7aqj8e1x2rzsrca',
     NOW() - INTERVAL '16 days', NULL, TRUE),

    ('am001', 'https://amazon.com',
     'Amazon', '全球最大電商平台，同時提供雲端服務 AWS',
     'https://images-na.ssl-images-amazon.com/images/G/01/social_share/amazon_logo.jpg',
     NOW() - INTERVAL '14 days', NULL, TRUE),

    ('nf001', 'https://netflix.com',
     'Netflix', '全球領先串流影音服務，原創內容產製力強',
     'https://assets.nflxext.com/ffe/siteui/vlv3/netflix-logo.png',
     NOW() - INTERVAL '12 days', NULL, TRUE),

    ('sp001', 'https://spotify.com',
     'Spotify', '全球最大音樂串流平台，Podcast 市場龍頭',
     'https://open.spotifycdn.com/cdn/images/spotify_logo.adb90efb.png',
     NOW() - INTERVAL '10 days', NOW() + INTERVAL '60 days', TRUE),

    ('tk001', 'https://tiktok.com',
     'TikTok', '短影音社群平台，演算法推播驅動病毒式傳播',
     'https://sf16-website-login.neutral.ttwstatic.com/obj/tiktok_web_login_static/tiktok/webapp/main/webapp-desktop/8152caf0c8e8bc67ae0d.png',
     NOW() - INTERVAL '9 days', NULL, TRUE),

    ('ms001', 'https://microsoft.com',
     'Microsoft', '全球最大軟體公司，企業雲端轉型夥伴',
     'https://img-prod-cms-rt-microsoft-com.akamaized.net/cms/api/am/imageFileData/RE1Mu3b',
     NOW() - INTERVAL '8 days', NULL, TRUE),

    ('ap001', 'https://apple.com',
     'Apple', '科技生態系品牌，軟硬整合標竿企業',
     'https://www.apple.com/ac/structured-data/images/knowledge_graph_logo.png',
     NOW() - INTERVAL '7 days', NOW() + INTERVAL '30 days', TRUE),

    ('go001', 'https://google.com',
     'Google', '全球最大搜尋引擎，數位廣告市場主導者',
     'https://www.google.com/images/branding/googleg/1x/googleg_standard_color_128dp.png',
     NOW() - INTERVAL '6 days', NULL, TRUE),

    ('wk001', 'https://wikipedia.org',
     'Wikipedia', '全球最大免費百科全書，超過 6000 萬篇條目',
     'https://upload.wikimedia.org/wikipedia/commons/thumb/8/80/Wikipedia-logo-v2.svg/1200px-Wikipedia-logo-v2.svg.png',
     NOW() - INTERVAL '5 days', NULL, TRUE),

    -- 刻意設 valid_flag = FALSE，示範軟刪除狀態
    ('rd001', 'https://reddit.com',
     'Reddit', '全球最大討論版社群，AMA 發源地',
     'https://www.redditstatic.com/shreddit/assets/favicon/192x192.png',
     NOW() - INTERVAL '4 days', NULL, FALSE);
END IF;

-- ── referral_codes（45 筆，每個短網址 3 個）────────────────────
IF NOT EXISTS (SELECT 1 FROM referral_codes LIMIT 1) THEN
  INSERT INTO referral_codes (code, owner_id, short_link_code, created_at, valid_flag) VALUES
    ('REF_GH_KOL1',  'influencer_001', 'gh001', NOW() - INTERVAL '27 days', TRUE),
    ('REF_GH_KOL2',  'influencer_002', 'gh001', NOW() - INTERVAL '26 days', TRUE),
    ('REF_GH_KOL3',  'influencer_003', 'gh001', NOW() - INTERVAL '25 days', FALSE),  -- 已停用示範

    ('REF_YT_KOL1',  'influencer_001', 'yt001', NOW() - INTERVAL '24 days', TRUE),
    ('REF_YT_KOL2',  'influencer_002', 'yt001', NOW() - INTERVAL '23 days', TRUE),
    ('REF_YT_KOL3',  'influencer_004', 'yt001', NOW() - INTERVAL '22 days', TRUE),

    ('REF_TW_KOL1',  'influencer_003', 'tw001', NOW() - INTERVAL '21 days', TRUE),
    ('REF_TW_KOL2',  'influencer_005', 'tw001', NOW() - INTERVAL '20 days', TRUE),
    ('REF_TW_KOL3',  'influencer_001', 'tw001', NOW() - INTERVAL '19 days', FALSE),  -- 已停用示範

    ('REF_FB_KOL1',  'influencer_002', 'fb001', NOW() - INTERVAL '19 days', TRUE),
    ('REF_FB_KOL2',  'influencer_003', 'fb001', NOW() - INTERVAL '18 days', TRUE),
    ('REF_FB_KOL3',  'influencer_004', 'fb001', NOW() - INTERVAL '17 days', TRUE),

    ('REF_IG_KOL1',  'influencer_005', 'ig001', NOW() - INTERVAL '17 days', TRUE),
    ('REF_IG_KOL2',  'influencer_001', 'ig001', NOW() - INTERVAL '16 days', TRUE),
    ('REF_IG_KOL3',  'influencer_002', 'ig001', NOW() - INTERVAL '15 days', TRUE),

    ('REF_LI_KOL1',  'influencer_003', 'li001', NOW() - INTERVAL '15 days', TRUE),
    ('REF_LI_KOL2',  'influencer_004', 'li001', NOW() - INTERVAL '14 days', FALSE),  -- 已停用示範
    ('REF_LI_KOL3',  'influencer_005', 'li001', NOW() - INTERVAL '13 days', TRUE),

    ('REF_AM_KOL1',  'influencer_001', 'am001', NOW() - INTERVAL '13 days', TRUE),
    ('REF_AM_KOL2',  'influencer_002', 'am001', NOW() - INTERVAL '12 days', TRUE),
    ('REF_AM_KOL3',  'influencer_003', 'am001', NOW() - INTERVAL '11 days', TRUE),

    ('REF_NF_KOL1',  'influencer_004', 'nf001', NOW() - INTERVAL '11 days', TRUE),
    ('REF_NF_KOL2',  'influencer_005', 'nf001', NOW() - INTERVAL '10 days', TRUE),
    ('REF_NF_KOL3',  'influencer_001', 'nf001', NOW() - INTERVAL '9 days',  TRUE),

    ('REF_SP_KOL1',  'influencer_002', 'sp001', NOW() - INTERVAL '9 days',  TRUE),
    ('REF_SP_KOL2',  'influencer_003', 'sp001', NOW() - INTERVAL '8 days',  TRUE),
    ('REF_SP_KOL3',  'influencer_004', 'sp001', NOW() - INTERVAL '7 days',  FALSE),  -- 已停用示範

    ('REF_TK_KOL1',  'influencer_005', 'tk001', NOW() - INTERVAL '8 days',  TRUE),
    ('REF_TK_KOL2',  'influencer_001', 'tk001', NOW() - INTERVAL '7 days',  TRUE),
    ('REF_TK_KOL3',  'influencer_002', 'tk001', NOW() - INTERVAL '6 days',  TRUE),

    ('REF_MS_KOL1',  'influencer_003', 'ms001', NOW() - INTERVAL '7 days',  TRUE),
    ('REF_MS_KOL2',  'influencer_004', 'ms001', NOW() - INTERVAL '6 days',  TRUE),
    ('REF_MS_KOL3',  'influencer_005', 'ms001', NOW() - INTERVAL '5 days',  TRUE),

    ('REF_AP_KOL1',  'influencer_001', 'ap001', NOW() - INTERVAL '6 days',  TRUE),
    ('REF_AP_KOL2',  'influencer_002', 'ap001', NOW() - INTERVAL '5 days',  TRUE),
    ('REF_AP_KOL3',  'influencer_003', 'ap001', NOW() - INTERVAL '4 days',  TRUE),

    ('REF_GO_KOL1',  'influencer_004', 'go001', NOW() - INTERVAL '5 days',  TRUE),
    ('REF_GO_KOL2',  'influencer_005', 'go001', NOW() - INTERVAL '4 days',  TRUE),
    ('REF_GO_KOL3',  'influencer_001', 'go001', NOW() - INTERVAL '3 days',  FALSE),  -- 已停用示範

    ('REF_WK_KOL1',  'influencer_002', 'wk001', NOW() - INTERVAL '4 days',  TRUE),
    ('REF_WK_KOL2',  'influencer_003', 'wk001', NOW() - INTERVAL '3 days',  TRUE),
    ('REF_WK_KOL3',  'influencer_004', 'wk001', NOW() - INTERVAL '2 days',  TRUE),

    ('REF_RD_KOL1',  'influencer_005', 'rd001', NOW() - INTERVAL '3 days',  TRUE),
    ('REF_RD_KOL2',  'influencer_001', 'rd001', NOW() - INTERVAL '2 days',  TRUE),
    ('REF_RD_KOL3',  'influencer_002', 'rd001', NOW() - INTERVAL '1 day',   TRUE);
END IF;

-- ── click_events（2000 筆，generate_series 批次產生）───────────
-- 模擬真實流量特徵：
--   熱門連結（gh001/yt001/ig001）佔較多流量
--   行動裝置佔約 60%
--   60% 流量攜帶推薦碼
--   時間分布在最近 30 天
IF NOT EXISTS (SELECT 1 FROM click_events LIMIT 1) THEN
  INSERT INTO click_events
    (short_link_code, clicked_at, platform, region, device_type, referral_code)
  SELECT
    -- 熱門度加權：重複出現次數越多代表流量越高
    -- 陣列共 30 個元素（索引 1-30），乘以 29 後四捨五入最大得 29，加 1 = 30，不越界
    (ARRAY[
      'gh001','gh001','gh001','gh001',
      'yt001','yt001','yt001','yt001',
      'ig001','ig001','ig001',
      'tw001','tw001','tw001',
      'fb001','fb001','fb001',
      'tk001','tk001',
      'sp001','sp001',
      'nf001','nf001',
      'li001','go001','am001','ms001','ap001','wk001','rd001'
    ])[1 + (random() * 29)::int],

    -- 時間隨機分布在最近 30 天
    NOW() - (random() * INTERVAL '30 days'),

    -- 流量來源平台（direct 佔比最高，模擬直接搜尋流量）
    (ARRAY[
      'direct','direct','direct',
      'facebook','facebook',
      'instagram','instagram',
      'twitter',
      'line','line'
    ])[1 + (random() * 9)::int],

    -- 台灣地區分布（北部都會區佔多數）
    (ARRAY[
      '台北','台北','台北',
      '新北','新北',
      '桃園','桃園',
      '台中','台中',
      '高雄','高雄',
      '台南','嘉義','台東','花蓮','基隆','宜蘭','新竹','苗栗','彰化'
    ])[1 + (random() * 19)::int],

    -- 裝置類型（行動裝置佔約 60%）
    (ARRAY[
      'mobile','mobile','mobile',
      'desktop','desktop',
      'tablet'
    ])[1 + (random() * 5)::int],

    -- 推薦碼（60% 機率攜帶）
    CASE
      WHEN random() > 0.40 THEN
        (ARRAY[
          'REF_GH_KOL1','REF_GH_KOL2',
          'REF_YT_KOL1','REF_YT_KOL2','REF_YT_KOL3',
          'REF_TW_KOL1','REF_TW_KOL2',
          'REF_FB_KOL1','REF_FB_KOL2','REF_FB_KOL3',
          'REF_IG_KOL1','REF_IG_KOL2','REF_IG_KOL3',
          'REF_LI_KOL1','REF_LI_KOL3',
          'REF_AM_KOL1','REF_AM_KOL2','REF_AM_KOL3',
          'REF_NF_KOL1','REF_NF_KOL2','REF_NF_KOL3',
          'REF_SP_KOL1','REF_SP_KOL2',
          'REF_TK_KOL1','REF_TK_KOL2','REF_TK_KOL3',
          'REF_MS_KOL1','REF_MS_KOL2','REF_MS_KOL3',
          'REF_AP_KOL1','REF_AP_KOL2','REF_AP_KOL3',
          'REF_GO_KOL1','REF_GO_KOL2',
          'REF_WK_KOL1','REF_WK_KOL2','REF_WK_KOL3'
        ])[1 + (random() * 36)::int]
      ELSE ''
    END

  FROM generate_series(1, 2000);
END IF;

END $$;
