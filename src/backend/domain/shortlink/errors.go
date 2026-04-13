package shortlink

import "errors"

// ErrNullCache 代表 Redis 中已標記此短碼不存在於資料庫
// 快取層偵測到此標記時，應直接拒絕請求，不再查 DB（防止快取穿透）
var ErrNullCache = errors.New("快取標記為不存在")
