# 社群短網址服務

前後端分離的短網址平台，支援：
- **社群預覽卡片**：建立時預先抓取 OG meta tags，讓 Facebook / Telegram / Discord 等平台正確顯示縮圖與摘要
- **點擊追蹤與分析**：記錄裝置類型、地理位置、推薦碼來源
- **推薦碼歸因**：短網址可附加推薦碼（`?ref=xxx`），追蹤各推薦來源的點擊比例
- **排行榜**：即時顯示所有短碼的點擊數排名
- **水平擴展**：Nginx Load Balancer + `--scale backend=N` 快速擴充後端副本

---

## Quick Start

### 前置需求

- Docker & Docker Compose

### 啟動所有服務

```bash
docker-compose up --build
```

水平擴充（3 個 backend 副本）：

```bash
docker-compose up --build --scale backend=3
```

### 服務入口

| 服務 | URL |
|------|-----|
| 前端（建立短網址） | http://localhost:3000 |
| 排行榜 Dashboard | http://localhost:3000/analytics |
| 單碼 Analytics | http://localhost:3000/analytics/{code} |
| 後端 API | http://localhost:8080 |
| Prometheus | http://localhost:9090 |
| Grafana（admin/admin） | http://localhost:3001 |

### 常用 API 指令

```bash
# 建立短網址
curl -X POST http://localhost:8080/api/v1/links \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.google.com"}'

# 302 跳轉測試
curl -I http://localhost:8080/aB3xYz7

# 點擊統計
curl http://localhost:8080/api/v1/links/aB3xYz7/analytics

# OG 預覽資料
curl http://localhost:8080/api/v1/links/aB3xYz7/preview

# 排行榜
curl http://localhost:8080/api/v1/links/ranking
```

### 本機單元測試

```bash
cd src/backend
go test ./application/usecase/...
```

### 整合測試

整合測試使用 [testcontainers-go](https://golang.testcontainers.org/) 在本機自動啟動 PostgreSQL 與 Redis 容器，無需手動建置任何服務，測試結束後容器自動清除。

#### 前置需求

- Docker Desktop 已啟動（testcontainers 需要 Docker daemon）

#### 執行方式

```bash
cd src/backend
go test -v -tags integration ./tests/integration/...
```

#### 測試覆蓋範圍

**API 流程測試**

| 測試函式 | 驗證內容 |
|---|---|
| `TestCreateAndRedirect` | 建立短網址 → GET `/:code` → 302 redirect 到原始 URL |
| `TestRedirectRecordsClick_WithReferralCode` | 帶 `?ref=` 點擊後，推薦碼正確寫入 DB 並可從 analytics API 查到 |
| `TestRedirectRecordsClick_WithRegion` | 帶 `X-Country` header 點擊後，地區正確寫入 DB |
| `TestRedirectRecordsClick_BothReferralAndRegion` | 推薦碼 + 地區同時帶入，兩者都正確記錄 |
| `TestRedirectBot_ReturnsOGHTML` | 社群平台 Bot（`facebookexternalhit`）收到 200 + OG HTML，不 redirect |
| `TestRedirect_NotFound` | 不存在的短碼回傳 404 |
| `TestCreateWithReferral_ResponseContainsReferralCode` | 建立時帶 `referral_owner_id`，回應包含 `referral_code` |
| `TestMultipleClicks_CountAccumulates` | 多次點擊後 `total_clicks` 正確累加 |

**Cleanup Worker 封存測試**

| 測試函式 | 驗證內容 |
|---|---|
| `TestArchiveExpiredLinks_過期短網址移至封存表` | 過期短網址 + 關聯 referral_codes + click_events 全部移至封存表，主表清除 |
| `TestArchiveExpiredLinks_未過期短網址不受影響` | `expires_at` 在未來或 NULL 的短網址不被封存 |
| `TestArchiveExpiredLinks_無過期資料時回傳0` | 無過期資料時 count = 0 且不報錯 |
| `TestArchiveOldClicks_舊點擊事件移至封存表` | 超過保留天數的 click_events 移至封存表，保留期內點擊不動 |
| `TestArchiveOldClicks_近期點擊不受影響` | 保留期內全部點擊一律不封存 |
| `TestCleanupWorker_完整執行流程` | Step 1（封存過期短網址）→ Step 2（封存 active 短網址的舊點擊）完整流程驗證 |

#### 技術架構

```
整合測試流程
    │
    ├─→ TestMain：testcontainers-go 啟動 postgres:18.3 + redis:7.2.7-alpine
    │       └─→ 執行 SQL migration → 依賴注入組裝 Use Cases / Handlers
    │
    ├─→ httptest.NewServer(router) → 完整 HTTP 層，含 rate limit middleware
    │
    ├─→ 各 Test*：透過真實 HTTP 呼叫驗證 API 行為
    │       └─→ asyncSaveClick 為 goroutine，click 驗證使用 require.Eventually + DB 查詢
    │
    └─→ TestMain 結束：自動 Terminate 兩個容器
```

測試檔案位於 `src/backend/tests/integration/`，以 `//go:build integration` build tag 隔離，一般 `go test ./...` 不會執行，需明確加 `-tags integration`。

---

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
| 監控 | Prometheus + Grafana | 2.51.2 / 10.4.2 |

---

## 系統架構

```
                     外部使用者 / 社群 Bot
                              │
                           :3000
                              │
                              ▼
                 ┌────────────────────────┐
                 │       Frontend         │
                 │     React + Nginx      │  shortlink-frontend
                 └────────────┬───────────┘
                              │
                           :8080  proxy_pass → shortlink-lb
                              │
                              ▼
                 ┌────────────────────────┐
                 │     Load Balancer      │
                 │   Nginx  Round-Robin   │  shortlink-lb
                 └─────┬─────────────┬───┘
                       │             │
           ┌───────────┘             └───────────┐
           │       --scale backend=N             │
           ▼                                     ▼
┌──────────────────┐                  ┌──────────────────┐
│    Backend #1    │       ...        │    Backend #N    │
│    Go + Gin      │                  │    Go + Gin      │
└────────┬─────────┘                  └────────┬─────────┘
         └──────────────────┬──────────────────┘
                            │
              ┌─────────────┴──────────────┐
              │                            │
              ▼                            ▼
┌─────────────────────┐       ┌─────────────────────┐
│     PostgreSQL      │       │        Redis         │
│   shortlink-postgres│       │   shortlink-redis    │
│       :5432         │       │        :6379         │
└─────────────────────┘       └─────────────────────┘
              ▲
              │
┌─────────────┴───────────────┐   ┌─────────────────────┐
│       Cleanup Worker        │   │   Prometheus/Grafana │
│  （定期清理過期資料）        │   │   metrics 監控       │
└─────────────────────────────┘   └─────────────────────┘
```

### Docker Compose 服務清單

| Container | 映像檔 | 對外 Port | 說明 |
|-----------|--------|-----------|------|
| `shortlink-frontend` | React + Nginx（自建） | 3000 | 靜態資源 + 反向代理 API |
| `shortlink-lb` | nginx:alpine | 8080 | Round-Robin 負載均衡，分流到 backend 副本 |
| `backend` × N | Go + Gin（自建） | — | API 伺服器，`--scale backend=N` 水平擴充 |
| `shortlink-postgres` | postgres:18.3 | 5432 | 主資料庫 |
| `shortlink-redis` | redis:7.2.7-alpine | 6379 | 短網址快取、null cache、Bloom Filter |
| `cleanup` | Go（自建） | — | 定期清理過期資料的背景 Worker |
| `shortlink-prometheus` | prom/prometheus:v2.51.2 | 9090 | 指標收集 |
| `shortlink-grafana` | grafana/grafana:10.4.2 | 3001 | 儀表板（admin/admin） |

---

## 程式架構

採用 **DDD + Hexagonal Architecture（六角架構）**，前後端完全分離。

**依賴方向**：外層依賴內層，Domain 不得 import 任何框架。

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
│   │   │   └── bloom.go                  # Bloom Filter Port 介面
│   │   ├── codegen/
│   │   │   └── codegen.go                # 短碼產生器（Base62 + crypto/rand）
│   │   └── uadetect/
│   │       └── uadetect.go               # User-Agent Bot 偵測
│   │
│   ├── infrastructure/                   # 基礎設施層（外部適配器）
│   │   ├── postgres/
│   │   │   ├── migrations/               # 001_init.sql、002_archive.sql
│   │   │   ├── shortlink_repo.go         # ShortLink Repository 實作
│   │   │   ├── click_repo.go             # ClickEvent Repository 實作
│   │   │   └── referral_repo.go          # ReferralCode Repository 實作
│   │   ├── redis/
│   │   │   └── cache.go                  # Redis 快取（jitter TTL、null cache、點擊去重）
│   │   ├── bloom/
│   │   │   └── filter.go                 # Bloom Filter（bits-and-blooms）
│   │   ├── metrics/
│   │   │   └── metrics.go                # Prometheus 指標定義（HTTP、業務、快取層）
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
│               ├── rate_limit.go         # per-IP 固定窗口速率限制（防爆破 / 濫用）
│               └── prometheus_middleware.go  # HTTP 請求計數 / 延遲 / in-flight 埋點
│
└── frontend/                             # React 前端
    └── src/
        ├── App.tsx                       # 路由根元件
        ├── api/client.ts                 # Axios API 客戶端封裝
        └── components/
            ├── CreateLinkForm.tsx         # 建立短網址表單
            ├── LinkResult.tsx             # 短網址建立結果顯示
            ├── AnalyticsDashboard.tsx     # 單碼點擊統計儀表板
            └── RankingDashboard.tsx       # 全域排行榜儀表板
```

### API 端點

| Method | Path | 說明 |
|--------|------|------|
| `POST` | `/api/v1/links` | 建立短網址（含 OG 抓取） |
| `GET` | `/:code` | 短網址跳轉（Bot 回傳 OG HTML，使用者 302 redirect） |
| `GET` | `/api/v1/links/:code/preview` | 取得 OG 預覽資料 |
| `GET` | `/api/v1/links/ranking` | 所有短碼點擊數排行榜 |
| `GET` | `/api/v1/links/:code/analytics` | 單碼點擊統計與推薦歸因 |
| `GET` | `/health` | 健康檢查 |
| `GET` | `/metrics` | Prometheus scrape（不走 nginx） |

---

## 設計決策

### 1. Redis 快取策略

Redirect 為最高頻操作，目標是絕大多數請求不碰 PostgreSQL。

#### 快取流程

```
Redirect 請求
    │
    ├─→ [1] Bloom Filter（快速排除肯定不存在的短碼）
    │         ├─→ MightExist = false → 直接 404，不查 Redis / DB
    │         └─→ MightExist = true  → 繼續查 Redis
    │
    ├─→ [2] GetShortLink(code)
    │       ├─→ null cache 命中 → 直接回傳 404（不查 DB）
    │       ├─→ cache hit      → 回傳快取資料
    │       └─→ cache miss ──→ [3] singleflight.Do(code)
    │                               │  ← 同一 code 的並發請求在此等待 →
    │                               ├─→ FindByCode(DB)
    │                               │       ├─→ 找到 → SetShortLink（回填快取）
    │                               │       └─→ 找不到 → SetNullCache（寫入不存在標記）
    │                               └─→ 回傳結果給所有等待的 goroutine
    └─→ 302 redirect / OG HTML
```

#### 三層快取防護

| 防護 | 問題情境 | 實作方式 |
|------|---------|---------|
| **雪崩 (Avalanche)** | 大量 key 同時過期，DB 瞬間承受全部流量 | TTL 基礎 24h ± 20% 隨機 jitter（實際 19.2h～28.8h） |
| **擊穿 (Stampede)** | 熱點 key 過期瞬間，N 個並發請求同時 cache miss | `singleflight.Group`：同一 code 並發 miss 只有 1 個 goroutine 查 DB |
| **穿透 (Penetration)** | 不存在的 code 每次請求都打到 DB | DB 回 nil 時寫入 `__null__` 標記（TTL 5 分鐘） |

#### Bloom Filter 防穿透（第一道防線）

- 所有已建立的短碼在建立時加入 Redis-backed Bloom Filter
- 請求到達時先呼叫 `MightExist(code)`，回傳 `false` 代表**肯定不存在**，直接 404 拒絕，不碰 Redis / DB
- 誤判率（false positive）極低，不影響正常使用

#### Cache Key 設計

```
shortlink:{code}   ← 正常資料（TTL: 19.2h ～ 28.8h）
shortlink:{code}   ← 值為 "__null__" 代表此 code 不存在（TTL: 5 分鐘）
```

---

### 2. 短碼產生機制

**Base62 + `crypto/rand`**，長度固定 7 碼。

```
charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
容量    = 62^7 ≈ 35 億組不重複短碼
```

設計考量：

| 決策 | 理由 |
|------|------|
| 使用 `crypto/rand` 而非 `math/rand` | 防止短碼被預測，避免惡意枚舉 |
| 長度選 7 | 35 億組足夠一般規模；8 碼才 220 億，URL 卻多了 1 字元，邊際效益低 |
| 碰撞處理 | 建立時先查 Bloom Filter + DB，衝突則重試產生新碼 |
| URL 安全字元 | 字符集不含 `+`, `/`, `=` 等需要 URL encode 的字元 |

---

### 3. 定期封存資料

主資料表（`short_links`、`click_events`）若無限成長，查詢效能會隨時間劣化。採用獨立的 **Cleanup Worker** Container 定期執行封存，不影響 API 服務。

#### 封存策略

```
Cleanup Worker（定時執行）
    │
    ├─→ Step 1：封存過期短網址
    │       ├─→ 找出 expires_at < NOW() 的 short_links
    │       ├─→ 將其 click_events 搬至 click_events_archive
    │       ├─→ 將其 referral_codes 搬至 referral_codes_archive
    │       └─→ 將短網址本身搬至 short_links_archive（原表刪除）
    │
    └─→ Step 2：封存舊點擊事件（active 短碼的歷史資料）
            └─→ 將超過保留天數的 click_events 搬至 click_events_archive
```

#### 封存表設計

| 封存表 | 說明 |
|--------|------|
| `short_links_archive` | 已過期短網址的歷史紀錄 |
| `referral_codes_archive` | 隨短網址一同封存的推薦碼 |
| `click_events_archive` | 過期短網址的點擊事件，以及 active 短碼的超齡點擊事件 |

封存資料保留供日後稽核或分析，不直接刪除。

---

### 4. 防重複點擊策略

同一使用者在短時間內多次點擊同一短網址，若每次都寫入 DB 會造成點擊數虛高、統計失真。採用 **Redis + Fingerprint 時間窗口去重**。

#### 流程

```
使用者點擊 /:code
    │
    └─→ asyncSaveClick（goroutine，不阻塞 302 回應）
            │
            ├─→ 計算 fingerprint
            │       ├─→ 有 ClientIP  → SHA-256(clientIP)[:8 bytes] → 16 hex 字元
            │       └─→ 無 ClientIP  → SHA-256(userAgent[:32])[:8 bytes]
            │
            ├─→ Redis SET NX + EX（原子操作）
            │   key = dedup:click:{fingerprint}:{code}
            │   TTL = WindowDuration（可設定，預設 10 分鐘）
            │       ├─→ SET 成功（key 不存在） → 首次點擊 → 寫入 click_events ✅
            │       └─→ SET 失敗（key 已存在） → 重複點擊 → 靜默略過 🚫
            │
            └─→ Redis 故障 → 寬鬆策略：放行點擊（寧多計，不漏計）
```

#### 設計考量

| 決策 | 理由 |
|------|------|
| SHA-256 單向雜湊 fingerprint | IP 原文不落 Redis，保護使用者隱私；碰撞空間 2^64，實務不衝突 |
| SET NX + EX 原子操作 | 不需 Lua Script，單一指令保證「判斷 + 寫入」的原子性，避免 race condition |
| Redis 故障時放行 | 可觀測性優先：漏記真實點擊遠比誤殺正常點擊更難察覺，寬鬆策略保守計數 |
| 窗口式（非永久去重） | 允許使用者隔段時間再次點擊被計算，符合真實行為模式 |
| 水位淘汰機制 | 去重 key 數量超過 MaxKeys 時，批次清理最舊的記錄，防止 Redis 記憶體無限成長 |

#### Redis Key 設計

```
dedup:click:{fingerprint}:{code}   ← 去重標記（TTL = WindowDuration）
dedup:click:index                  ← Sorted Set，score=建立時間 ns，供水位淘汰排序
```

---

## 高可用、低延遲、可擴充性

### 高可用（High Availability）

| 機制 | 說明 |
|------|------|
| 多 backend 副本 | `--scale backend=N`，任意一副本掛掉 Nginx LB 自動略過 |
| Graceful Shutdown | `SIGTERM` 信號觸發，等待飛行中的請求完成再退出 |
| 健康檢查 | `/health` 端點，Docker / Nginx 用於存活探測 |
| Redis 容錯 | 快取 miss 時自動 fallback 查 PostgreSQL，不因 Redis 不可用而服務中斷 |

### 低延遲（Low Latency）

| 機制 | 說明 |
|------|------|
| Redis 快取 | Redirect 最常見操作，絕大多數請求在 Redis 層解決，不查 DB |
| singleflight | 熱點 key 瞬間 miss 時，只發一個 DB 查詢，其餘請求共用結果 |
| 非同步寫入 | 302 回應後，goroutine 非同步寫入 ClickEvent，不阻塞使用者請求 |
| OG 預先抓取 | 建立時就抓好存 DB，Bot 到來時直接讀取，不對外發 HTTP 請求 |
| Bloom Filter | 肯定不存在的短碼在記憶體層直接拒絕，連 Redis round-trip 都省掉 |

### 可擴充性（Scalability）

| 機制 | 說明 |
|------|------|
| 無狀態 backend | 所有狀態存 PostgreSQL / Redis，backend 可任意擴充副本數 |
| Nginx Round-Robin LB | 新副本啟動後自動加入分流，無需手動設定 |
| 獨立 Cleanup Worker | 封存任務不佔用 API 資源，可獨立調整執行頻率或資源配額 |
| Prometheus + Grafana | 監控各副本 QPS、延遲、錯誤率，提供擴容決策依據 |

---

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

---

## 本機社群平台真實預覽測試

本機無法被社群平台 Bot 存取，需透過以下工具暴露公開 URL：

| 工具 | 指令 |
|------|------|
| ngrok | `ngrok http 3000` |
| Cloudflare Tunnel | `cloudflared tunnel --url http://localhost:3000` |

取得公開 URL 後，將短網址貼至 Facebook / Telegram / Discord，確認預覽卡片正常顯示。

### 建立短網址 & 縮圖預覽示範

建立短網址後，將短連結貼至 Facebook / Telegram / Discord，平台 Bot 會抓取 OG 資料並顯示預覽卡片：

![短連結縮圖顯示正確](docs/result_2329.png)