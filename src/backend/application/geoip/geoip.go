package geoip

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"
)

// ipAPIResponse ip-api.com 回應格式（只取 countryCode 欄位）
type ipAPIResponse struct {
	CountryCode string `json:"countryCode"`
	Status      string `json:"status"`
}

// httpClient 設定短 timeout，避免 goroutine 長時間等待
// GeoIP 查詢是非主路徑，查不到只是地區資料空白，不影響轉址正確性
var httpClient = &http.Client{Timeout: 2 * time.Second}

// LookupCountry 用 ip-api.com 將 IPv4/IPv6 轉換為 ISO 3166-1 alpha-2 國碼（e.g. "TW"）
// 查詢失敗或私有 IP 時回傳空字串（graceful degradation）
func LookupCountry(ctx context.Context, ipStr string) string {
	// 過濾私有、迴環 IP — 這類 IP 無法對外做地理查詢
	ip := net.ParseIP(ipStr)
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() {
		return ""
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://ip-api.com/json/"+ipStr+"?fields=status,countryCode", nil)
	if err != nil {
		return ""
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var result ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	if result.Status != "success" {
		return ""
	}
	return result.CountryCode
}
