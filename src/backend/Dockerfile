# ── Build Stage ──────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /app

# 先複製 go.mod/go.sum 利用 Docker layer cache，避免每次都重新下載依賴
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 從 cmd/ 編譯，靜態連結避免 alpine libc 相容問題
RUN CGO_ENABLED=0 GOOS=linux go build -o /shortlink ./cmd/

# ── Runtime Stage ─────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /shortlink .

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=10s \
  CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["./shortlink"]
