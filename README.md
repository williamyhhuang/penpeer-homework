# 社群短網址服務

前後端分離的短網址平台，支援社群預覽（OG meta tags）、點擊追蹤與推薦碼歸因。

## 技術棧

| 層級 | 技術 | 版本 |
|------|------|------|
| 後端語言 | Go | 1.23 |
| API 框架 | Gin | 1.10.1 |
| ORM | GORM | 1.25.10 |
| GORM PG Driver | gorm.io/driver/postgres | 1.5.9 |
| PG 驅動 | pgx | v5.5.5 |
| 資料庫 | PostgreSQL | 18.0 |
| 快取 | Redis | 7.2.7 |
| Redis 客戶端 | go-redis/v9 | 9.7.0 |
| Bloom Filter | bits-and-blooms/bloom/v3 | 3.7.1 |
| 前端語言 | TypeScript | 6.0 |
| 前端框架 | React | 18.2 |
| 容器化 | Docker / Docker Compose | — |

## 專案架構

採用 **DDD + Hexagonal Architecture**，前後端完全分離。

```
src/
├── backend/                              # Go 後端
│   ├── cmd/
│   │   └── main.go                       # 進入點、DI 組裝、Graceful Shutdown
│   │
│   ├── domain/                           # 領域層（純業務邏輯，零外部依賴）
│   │   ├── shortlink/
│   │   │   ├── entity.go                 # ShortLink 聚合根實體
│   │   │   ├── repository.go             # Repository 介面定義
│   │   │   └── errors.go                 # ErrNullCache 等領域錯誤
│   │   ├── click/
│   │   │   ├── entity.go                 # ClickEvent 聚合根實體
│   │   │   └── repository.go             # Repository 介面定義
│   │   └── referral/
│   │       ├── entity.go                 # ReferralCode 聚合根實體
│   │       └── repository.go             # Repository 介面定義
│   │
│   ├── application/                      # 應用層（Use Cases）
│   │   ├── usecase/
│   │   │   ├── create_short_link.go      # 建立短網址（含 OG 抓取）
│   │   │   ├── redirect_short_link.go    # 短網址跳轉（含 singleflight）
│   │   │   ├── get_preview.go            # 取得 OG 預覽資料
│   │   │   ├── get_analytics.go          # 點擊統計與推薦歸因
│   │   │   ├── get_ranking.go            # 點擊數排行榜
│   │   │   ├── bloom.go                  # Bloom Filter 前置過濾
│   │   │   ├── mock_test.go              # Repository mock
│   │   │   ├── create_short_link_test.go
│   │   │   ├── redirect_short_link_test.go
│   │   │   ├── get_preview_test.go
│   │   │   ├── get_analytics_test.go
│   │   │   └── get_ranking_test.go
│   │   ├── codegen/
│   │   │   └── codegen.go                # 隨機短碼產生器（Base62）
│   │   └── uadetect/
│   │       └── uadetect.go               # User-Agent Bot 偵測
│   │
│   ├── infrastructure/                   # 基礎設施層（外部適配器）
│   │   ├── postgres/
│   │   │   ├── models/
│   │   │   │   ├── shortlink_model.go    # GORM ShortLink ORM 模型
│   │   │   │   ├── click_model.go        # GORM ClickEvent ORM 模型
│   │   │   │   └── referral_model.go     # GORM ReferralCode ORM 模型
│   │   │   ├── db.go                     # GORM 連線池初始化
│   │   │   ├── migrator.go               # Auto-migrate（GORM）
│   │   │   ├── shortlink_repo.go         # ShortLink Repository 實作
│   │   │   ├── click_repo.go             # ClickEvent Repository 實作
│   │   │   └── referral_repo.go          # ReferralCode Repository 實作
│   │   ├── redis/
│   │   │   └── cache.go                  # Redis 快取（含 jitter TTL、null cache）
│   │   ├── bloom/
│   │   │   └── filter.go                 # Bloom Filter（bits-and-blooms）
│   │   └── scraper/
│   │       └── og_scraper.go             # OG meta tag 抓取器
│   │
│   └── interfaces/
│       └── http/
│           ├── router.go                 # Gin 路由與中介層組裝
│           ├── handler/
│           │   ├── link_handler.go       # 建立 / 預覽 / 統計 / 排行 Handler
│           │   └── redirect_handler.go   # 跳轉 / Bot OG HTML Handler
│           └── middleware/
│               └── rate_limit.go         # 請求速率限制
│
└── frontend/                             # React 前端
    └── src/
        ├── App.tsx                       # 路由根元件
        ├── main.tsx                      # 應用進入點
        ├── api/
        │   └── client.ts                 # Axios API 客戶端封裝
        ├── components/
        │   ├── CreateLinkForm.tsx         # 建立短網址表單
        │   ├── LinkResult.tsx             # 短網址建立結果顯示
        │   ├── AnalyticsDashboard.tsx     # 單碼點擊統計儀表板
        │   └── RankingDashboard.tsx       # 全域排行榜儀表板
        └── types/
            └── index.ts                  # TypeScript 型別定義
```

## API 端點

| Method | Path | 說明 |
|--------|------|------|
| `POST` | `/api/v1/links` | 建立短網址（含 OG 抓取） |
| `GET` | `/:code` | 短網址跳轉（Bot 回傳 OG HTML，使用者 302 redirect） |
| `GET` | `/api/v1/links/:code/preview` | 取得 OG 預覽資料 |
| `GET` | `/api/v1/links/ranking` | 所有短碼點擊數排行榜 |
| `GET` | `/api/v1/links/:code/analytics` | 單碼點擊統計與推薦歸因 |
| `GET` | `/health` | 健康檢查 |

## 快速啟動

### 前置需求

- Docker & Docker Compose

### 啟動所有服務

```bash
docker-compose up --build
```

服務啟動後：

| 服務 | URL |
|------|-----|
| 前端（建立短網址） | http://localhost:3000 |
| 排行榜 Dashboard | http://localhost:3000/analytics |
| 單碼 Analytics | http://localhost:3000/analytics/{code} |
| 後端 API | http://localhost:8080 |
| PostgreSQL | localhost:5432 |
| Redis | localhost:6379 |

## 本機測試

### 建立短網址

```bash
curl -X POST http://localhost:8080/api/v1/links \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.google.com"}'
```

回傳範例：

```json
{
  "code": "aB3xYz",
  "short_url": "http://localhost:8080/aB3xYz",
  "original_url": "https://www.google.com"
}
```

### 測試 Redirect

```bash
# 只看 response header（確認 302）
curl -I http://localhost:8080/aB3xYz
# 預期：HTTP/1.1 302 Found
#        Location: https://www.google.com
```

### 查看點擊統計

```bash
curl http://localhost:8080/api/v1/links/aB3xYz/analytics
```

### 查看 OG 預覽

```bash
curl http://localhost:8080/api/v1/links/aB3xYz/preview
```

## 社群預覽運作原理

```
使用者建立短網址
    └─→ 後端 Scraper 抓取原始頁面 OG 資料 → 存入 DB

有人點擊短網址
    ├─→ 社群 Bot（facebookexternalhit、Twitterbot 等）
    │       └─→ 回傳含 OG meta 的 HTML → 社群平台顯示預覽卡片 ✅
    └─→ 一般使用者
            └─→ 302 redirect 到原始 URL ✅
```

支援偵測的 Bot：`facebookexternalhit`、`Twitterbot`、`LinkedInBot`、`TelegramBot`、`WhatsApp`、`Slackbot`、`Discordbot`

## 效能設計

| 策略 | 說明 |
|------|------|
| Redis 快取 | Redirect 優先查 Redis，cache miss 才查 PostgreSQL |
| 非同步寫入 | 302 回應後，goroutine 非同步寫入 ClickEvent，不阻塞使用者 |
| OG 預先抓取 | 建立時抓好存 DB，Bot 來時直接讀取，不重複請求外部頁面 |

## Redis 快取機制

### 快取流程

```
Redirect 請求
    │
    ├─→ [1] GetShortLink(code)
    │       ├─→ null cache 命中 → 直接回傳 404（不查 DB）
    │       ├─→ cache hit      → 回傳快取資料
    │       └─→ cache miss ──→ [2] singleflight.Do(code)
    │                               │  ← 同一 code 的並發請求在此等待 →
    │                               ├─→ FindByCode(DB)
    │                               │       ├─→ 找到 → SetShortLink（回填快取）
    │                               │       └─→ 找不到 → SetNullCache（寫入不存在標記）
    │                               └─→ 回傳結果給所有等待的 goroutine
    └─→ 302 redirect / OG HTML
```

### 三層快取防護

| 防護 | 問題情境 | 實作方式 | 位置 |
|------|---------|---------|------|
| **雪崩 (Avalanche)** | 大量 key 同時過期，DB 瞬間承受全部流量 | TTL 基礎 24h ± 20% 隨機 jitter，實際範圍 19.2h ～ 28.8h | `infrastructure/redis/cache.go` `jitteredTTL()` |
| **擊穿 (Stampede)** | 熱點 key 過期瞬間，N 個並發請求同時 cache miss | `singleflight.Group`：同一 code 並發 miss 只有 1 個 goroutine 查 DB，其餘共用結果 | `application/usecase/redirect_short_link.go` |
| **穿透 (Penetration)** | 不存在的 code 每次請求都打到 DB | DB 回 nil 時寫入 `__null__` 標記（TTL 5 分鐘），後續請求由 `ErrNullCache` 直接拒絕 | `infrastructure/redis/cache.go` `SetNullCache()` |

### Cache Key 設計

```
shortlink:{code}      ← 正常短網址資料（TTL: 19.2h ～ 28.8h）
shortlink:{code}      ← 值為 "__null__" 代表此 code 不存在（TTL: 5 分鐘）
```

### Sentinel Error 架構

`ErrNullCache` 定義於 `domain/shortlink/errors.go`，讓 `application` 與 `infrastructure` 雙層均可 import，不破壞 DDD 分層依賴方向。

## 後端單元測試

```bash
cd src/backend
go test ./application/usecase/...
```

## 社群平台真實預覽測試

本機無法被社群平台 Bot 存取，需透過以下方式暴露公開 URL：

| 工具 | 指令 |
|------|------|
| ngrok | `ngrok http 8080` |
| Cloudflare Tunnel | `cloudflared tunnel --url http://localhost:8080` |

取得公開 URL 後，將短網址貼至 Facebook / Telegram / Discord，確認預覽卡片正常顯示。

## 環境變數

| 變數 | 預設值 | 說明 |
|------|--------|------|
| `DB_HOST` | `localhost` | PostgreSQL 主機 |
| `DB_PORT` | `5432` | PostgreSQL 連接埠 |
| `DB_USER` | `postgres` | 資料庫使用者 |
| `DB_PASSWORD` | `postgres` | 資料庫密碼 |
| `DB_NAME` | `shortlink` | 資料庫名稱 |
| `REDIS_HOST` | `localhost` | Redis 主機 |
| `REDIS_PORT` | `6379` | Redis 連接埠 |
| `SERVER_PORT` | `8080` | 後端監聽連接埠 |
