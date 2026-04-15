# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) 
when working with code in this repository.

始終用繁體中文說明
依照 馬斯克 第一性原理

## Project Overview
- 這是一個社群短網址專案，前後端分離
- 可以使用 docker-compose 在本機建置站台
- 詳細功能需求參考 `docs/request.pdf`

## Technology Stack
- **Golang 1.21**  for backend
- **PostgreSQL 18.0** for database access
- **gin 1.11.0** for API framework
- **Redis 7.2.7** for cache
- **TypeScript 6.0.0** for frontend
- **React 18.2.0** for frontend
- **docker docker-compose** for containerized DevOps

## Architecture
always follow docs/DDD_architecture.png

### DDD 六角架構（Hexagonal / Ports & Adapters）摘要

圖示為標準六角架構，分三層同心圓：

| 層次 | 職責 | 本專案對應路徑 |
|------|------|---------------|
| **Domain Layer**（最內層） | 純業務規則、Entity、Repository 介面 | `src/backend/domain/` |
| **Application Layer** | Use Case 編排，不含框架相依 | `src/backend/application/usecase/` + `application/codegen/` + `application/geoip/` + `application/uadetect/` |
| **Infrastructure / Adapters**（最外層） | 資料庫、Redis、HTTP、Metrics 等實作 | `src/backend/infrastructure/` + `src/backend/interfaces/` |

**依賴方向**：外層依賴內層，Domain 不得 import 任何框架。

#### Domain 模組
- `domain/shortlink` — ShortLink Entity、Repository 介面、錯誤定義
- `domain/click` — Click Entity（點擊紀錄）、Repository 介面
- `domain/referral` — Referral Entity（推薦碼）、Repository 介面

#### Application Use Cases
| Use Case 檔案 | 功能 |
|---|---|
| `create_short_link.go` | 建立短網址（含推薦碼） |
| `redirect_short_link.go` | 短碼轉導、記錄點擊 |
| `get_preview.go` | 取得 OG 預覽資訊 |
| `get_analytics.go` | 取得點擊分析 |
| `get_ranking.go` | 取得點擊排行 |
| `archive_expired_links.go` | 封存過期短網址（cleanup worker） |
| `archive_old_clicks.go` | 封存舊點擊資料（cleanup worker） |
| `bloom.go` | Bloom Filter 防重複短碼 |

#### Infrastructure 實作
- `infrastructure/postgres/` — PostgreSQL GORM Repo 實作 + Migration（`migrations/001_init.sql`, `002_archive.sql`）
- `infrastructure/redis/` — Redis 快取（短網址轉導快取）
- `infrastructure/bloom/` — Bloom Filter（Redis 為後端）
- `infrastructure/metrics/` — Prometheus 指標定義
- `infrastructure/scraper/` — OG Tag 爬蟲

#### Interfaces（Primary Adapters）
- `interfaces/http/handler/link_handler.go` — REST API Handler
- `interfaces/http/handler/redirect_handler.go` — 短碼轉導 Handler
- `interfaces/http/middleware/` — Rate Limit + Prometheus Middleware
- `interfaces/http/router.go` — Gin 路由定義

### docker-compose 服務清單

| 服務 container | Image / Build | 對外 Port | 說明 |
|---|---|---|---|
| `shortlink-postgres` | `postgres:18.3` | 5432 | 主資料庫 |
| `shortlink-redis` | `redis:7.2.7-alpine` | 6379 | 快取、Bloom Filter |
| `backend`（可 --scale） | `./src/backend` | expose 9091（metrics） | Gin API，支援水平擴展 |
| `shortlink-lb` | `nginx:alpine` | **8080** | Nginx Load Balancer，設定於 `nginx.conf` |
| `shortlink-frontend` | `./src/frontend` | **3000** | React SPA，內建 nginx，proxy_pass → shortlink-lb:8080 |
| `cleanup` | `./src/backend Dockerfile.cleanup` | — | 定期封存過期資料的 Worker |
| `shortlink-prometheus` | `prom/prometheus:v2.51.2` | 9090 | 指標收集 |
| `shortlink-grafana` | `grafana/grafana:10.4.2` | 3001 | 儀表板（admin/admin） |
| `shortlink-postgres-exporter` | postgres-exporter:v0.15.0 | expose 9187 | PostgreSQL → Prometheus |
| `shortlink-redis-exporter` | redis_exporter:v1.62.0 | expose 9121 | Redis → Prometheus |
| `shortlink-cadvisor` | cadvisor:v0.49.1 | expose 8080 | 容器資源監控 |

**啟動多副本 backend：**
```bash
docker-compose up --scale backend=3
```

### API 路由總覽

| Method | Path | 功能 |
|---|---|---|
| GET | `/:code` | 短碼轉導（rate limited） |
| POST | `/api/v1/links` | 建立短網址 |
| GET | `/api/v1/links/ranking` | 點擊排行 |
| GET | `/api/v1/links/:code/preview` | OG 預覽 |
| GET | `/api/v1/links/:code/analytics` | 點擊分析 |
| GET | `/health` | 健康檢查 |
| GET | `/metrics` | Prometheus scrape（不走 nginx） |

### Frontend 元件
- `App.tsx` — 路由進入點
- `components/CreateLinkForm.tsx` — 建立短網址表單
- `components/LinkResult.tsx` — 顯示短網址結果（含推薦碼組合）
- `components/AnalyticsDashboard.tsx` — 點擊分析圖表
- `components/RankingDashboard.tsx` — 排行榜
- `api/client.ts` — Axios API 客戶端（base URL: `/api/v1`）

## Important Documentation

**CRITICAL**: Always read relevant documentation before 
implementing features and update documentation after making changes.

### Documentation Structure
- `README.md` - Project overview and navigation
- `CLAUDE.md` - **MUST READ** before development
- `docs/ddd_arch.png` - **MUST READ** before development

## Code Development Principles

### Version and API Management
- **ALWAYS check package versions** before writing code
- **NEVER use deprecated methods or APIs**

### Traditional Chinese Comments
- Add Traditional Chinese comments in places that are difficult to understand
- Focus on explaining "why" rather than "what"

### Rules Should be Followed
- Always git commit after implementing codes
- Always implement unit tests for codes
- Always run unit tests after implementing codes, if unit tests failed, fix it
- every config params added to .env file must be added to .env.example file