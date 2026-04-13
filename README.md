# 社群短網址服務

前後端分離的短網址平台，支援社群預覽（OG meta tags）、點擊追蹤與推薦碼歸因。

## 技術棧

| 層級 | 技術 |
|------|------|
| 後端語言 | Go 1.21 |
| API 框架 | Gin 1.10 |
| 資料庫 | PostgreSQL 18.3 |
| 快取 | Redis 7.2.7 |
| 前端語言 | TypeScript 6.0 |
| 前端框架 | React 18.2 |
| 容器化 | Docker / Docker Compose |

## 專案架構

採用 **DDD + Hexagonal Architecture**，前後端完全分離。

```
src/
├── backend/                     # Go 後端
│   ├── cmd/
│   │   ├── main.go              # 進入點、DI 組裝、Graceful Shutdown
│   │   └── migrations/          # SQL migration 檔（embed 至二進位）
│   ├── domain/                  # 領域層（純業務邏輯，零外部依賴）
│   │   ├── shortlink/           # ShortLink 聚合根
│   │   ├── click/               # ClickEvent 聚合根
│   │   └── referral/            # ReferralCode 聚合根
│   ├── application/             # 應用層（Use Cases）
│   │   ├── usecase/             # 四個核心 Use Case
│   │   └── codegen/ uadetect/   # 短碼產生 / UA 偵測工具
│   ├── infrastructure/          # 基礎設施層（外部適配器）
│   │   ├── postgres/            # PostgreSQL repo 實作
│   │   ├── redis/               # Redis 快取實作
│   │   └── scraper/             # OG meta 抓取器
│   └── interfaces/
│       └── http/                # Gin Router + Handler（HTTP 適配器）
└── frontend/                    # React 前端
    ├── src/
    │   ├── api/                 # API client
    │   ├── components/          # UI 元件
    │   └── types/               # TypeScript 型別定義
    └── ...
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
