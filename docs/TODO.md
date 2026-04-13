# 社群短網址服務 — 開發待辦清單

## 架構原則
- 遵循 DDD + Hexagonal Architecture（參考 `docs/ddd_arch.png`）
- 前後端分離，docker-compose 一鍵啟動

---

## 領域模型（Domain Layer）

| Aggregate | 欄位 |
|-----------|------|
| `ShortLink` | 短碼、原始URL、OG資料、建立時間、過期時間 |
| `ReferralCode` | 推薦碼、擁有者、關聯短碼 |
| `ClickEvent` | 點擊時間、來源平台、地區、裝置、推薦碼 |

---

## API 設計

| Method | Path | 說明 |
|--------|------|------|
| `POST` | `/api/v1/links` | 建立短網址（含推薦碼） |
| `GET` | `/:code` | 重新導向（核心 redirect） |
| `GET` | `/api/v1/links/:code/preview` | 回傳 OG 預覽資料 |
| `GET` | `/api/v1/links/:code/analytics` | 點擊統計與歸因 |

---

## 核心功能實作

### Phase 1 — 核心 Redirect（最小可用）
- [x] Domain Model 定義（ShortLink、ReferralCode、ClickEvent）
- [x] PostgreSQL schema 設計
- [x] `CreateShortLink` Use Case
- [x] `RedirectShortLink` Use Case
- [x] Gin HTTP Handler
- [x] Redis 快取短碼（降低 redirect 延遲）
- [x] Docker Compose 整合（PostgreSQL + Redis + Backend）

### Phase 2 — 社群功能
- [x] OG Scraper：建立短網址時抓取原始頁面的 og:title / og:description / og:image，存入 DB
- [x] User-Agent 偵測：區分社群 Bot 與一般使用者
  - Bot → 回傳含 OG meta tags 的 HTML（讓社群平台顯示預覽）
  - 一般使用者 → 302 redirect 到原始 URL
- [x] 推薦碼綁定與記錄（`?ref=user123`）
- [x] 支援的 Bot 清單：facebookexternalhit、Twitterbot、LinkedInBot、TelegramBot、WhatsApp、Slackbot、Discordbot

### Phase 3 — Analytics
- [x] ClickEvent 非同步寫入（goroutine，不阻塞 redirect）
- [x] 記錄：點擊次數、來源平台、地區、裝置
- [x] 歸因查詢 API（推薦者 / 行銷活動）

### Phase 4 — 前端
- [x] 建立短網址頁面（輸入長網址 → 取得短碼）
- [x] Analytics Dashboard（點擊統計視覺化）

### Phase 5 — 測試與收尾
- [x] 每個 Use Case 單元測試
- [x] docker-compose 整體驗證
- [x] README 撰寫（含 Quick Test 步驟）

---

## 效能策略

| 策略 | 說明 |
|------|------|
| Redis 快取 | redirect 優先查 Redis，miss 才查 PostgreSQL |
| 非同步寫入 | 302 回應後，goroutine 異步寫入 ClickEvent |
| OG 預先抓取 | 建立時抓好存 DB，Bot 來時直接讀，不重複抓 |

---

## 連結預覽（Link Preview）運作流程

```
使用者建立短網址
    └─→ 後端 Scraper 抓原始頁面 OG → 存 DB

有人點擊短網址
    ├─→ 社群 Bot → 回傳 OG HTML → 社群平台顯示預覽 ✅
    └─→ 一般使用者 → 302 redirect → 原始頁面 ✅
```

### OG HTML 範本結構
```html
<meta property="og:title"       content="...">
<meta property="og:description" content="...">
<meta property="og:image"       content="...">
<meta property="og:url"         content="...">
<meta http-equiv="refresh"      content="0;url={原始URL}">
```

---

## 本機測試方式

```bash
# 1. 啟動服務
docker-compose up

# 2. 建立短網址
curl -X POST http://localhost:8080/api/v1/links \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.google.com"}'

# 3. 測試 redirect（-I 只看 header）
curl -I http://localhost:8080/{short_code}
# 預期：HTTP/1.1 302 Found + Location: https://www.google.com

# 4. 瀏覽器開啟 http://localhost:8080/{short_code} 確認跳轉
```

---

## 社群平台真實預覽測試

本機無法被社群平台 Bot 存取，需透過以下其中一種方式：

| 方式 | 指令 |
|------|------|
| ngrok | `ngrok http 8080` |
| Cloudflare Tunnel | `cloudflared tunnel --url http://localhost:8080` |
| 部署（免費） | Fly.io / Railway |

取得公開 URL 後，將短網址貼至 Facebook / Telegram / Discord，確認預覽正常顯示。
