//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createLinkResp POST /api/v1/links 的回應結構
type createLinkResp struct {
	Code         string `json:"code"`
	OriginalURL  string `json:"original_url"`
	ReferralCode string `json:"referral_code"`
}

// analyticsResp GET /api/v1/links/:code/analytics 的回應結構
type analyticsResp struct {
	TotalClicks int64            `json:"total_clicks"`
	ByPlatform  map[string]int64 `json:"by_platform"`
	ByDevice    map[string]int64 `json:"by_device"`
	ByRegion    map[string]int64 `json:"by_region"`
	ByReferral  map[string]int64 `json:"by_referral"`
}

// createShortLink 輔助函式：建立短網址，回傳 code 與完整回應
func createShortLink(t *testing.T, originalURL, referralOwnerID string) createLinkResp {
	t.Helper()

	body := map[string]string{"url": originalURL}
	if referralOwnerID != "" {
		body["referral_owner_id"] = referralOwnerID
	}
	b, _ := json.Marshal(body)

	resp, err := http.Post(
		testEnv.server.URL+"/api/v1/links",
		"application/json",
		bytes.NewReader(b),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result createLinkResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.NotEmpty(t, result.Code, "回應應包含短碼")
	return result
}

// doRedirect 發送 GET /:code 請求，不跟隨 redirect，回傳原始 response
func doRedirect(t *testing.T, code, refCode, userAgent, countryHeader string) *http.Response {
	t.Helper()

	url := fmt.Sprintf("%s/%s", testEnv.server.URL, code)
	if refCode != "" {
		url += "?ref=" + refCode
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if countryHeader != "" {
		// 透過 X-Country header 模擬地區，避免依賴外部 GeoIP API
		req.Header.Set("X-Country", countryHeader)
	}

	// 不跟隨 redirect，才能驗證 302 Location header
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

// waitForClick 等待 async goroutine 完成 click 寫入 DB
// asyncSaveClick 是 goroutine，Execute() 返回後 click 可能還沒寫入 DB
// 使用 Eventually + DB 查詢，避免修改 production code
func waitForClick(t *testing.T, code string, minCount int64) {
	t.Helper()
	require.Eventually(t, func() bool {
		stats, err := testEnv.pgClickRepo.GetStatsByCode(context.Background(), code)
		if err != nil {
			return false
		}
		return stats.TotalClicks >= minCount
	}, 300*time.Millisecond, 10*time.Millisecond,
		"等待 click 事件寫入 DB 超時（code=%s, minCount=%d）", code, minCount)
}

// ── 測試案例 ──────────────────────────────────────────────────────────────────

// TestCreateAndRedirect 驗證核心流程：建立短網址 → GET /:code → 302 redirect 到原始 URL
func TestCreateAndRedirect(t *testing.T) {
	originalURL := "https://www.example.com/integration-test"
	resp := createShortLink(t, originalURL, "")

	// 發送 redirect 請求（一般桌機 UA）
	r := doRedirect(t, resp.Code, "", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", "")
	defer r.Body.Close()

	assert.Equal(t, http.StatusFound, r.StatusCode, "一般使用者應收到 302")
	assert.Equal(t, originalURL, r.Header.Get("Location"), "Location header 應指向原始 URL")
}

// TestRedirectRecordsClick_WithReferralCode 驗證：帶 ?ref= 點擊後，推薦碼確實寫入 DB
func TestRedirectRecordsClick_WithReferralCode(t *testing.T) {
	originalURL := "https://www.example.com/ref-test"
	referralOwnerID := "user_alice"

	// 建立短網址時同時建立推薦碼
	created := createShortLink(t, originalURL, referralOwnerID)
	require.NotEmpty(t, created.ReferralCode, "建立時帶 referral_owner_id 應回傳 referral_code")

	// 帶推薦碼點擊
	r := doRedirect(t, created.Code, created.ReferralCode, "Mozilla/5.0", "")
	defer r.Body.Close()
	assert.Equal(t, http.StatusFound, r.StatusCode)

	// 等待 async goroutine 寫入完成
	waitForClick(t, created.Code, 1)

	// 查 analytics API 驗證推薦碼有被記錄
	resp, err := http.Get(
		fmt.Sprintf("%s/api/v1/links/%s/analytics", testEnv.server.URL, created.Code),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var analytics analyticsResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&analytics))

	assert.Equal(t, int64(1), analytics.TotalClicks, "應記錄 1 次點擊")
	assert.Equal(t, int64(1), analytics.ByReferral[created.ReferralCode],
		"推薦碼 %q 應有 1 次點擊", created.ReferralCode)
}

// TestRedirectRecordsClick_WithRegion 驗證：帶 X-Country header 點擊後，地區確實寫入 DB
func TestRedirectRecordsClick_WithRegion(t *testing.T) {
	originalURL := "https://www.example.com/region-test"
	created := createShortLink(t, originalURL, "")

	// 帶地區 header 點擊，模擬來自台灣的請求
	r := doRedirect(t, created.Code, "", "Mozilla/5.0", "TW")
	defer r.Body.Close()
	assert.Equal(t, http.StatusFound, r.StatusCode)

	waitForClick(t, created.Code, 1)

	resp, err := http.Get(
		fmt.Sprintf("%s/api/v1/links/%s/analytics", testEnv.server.URL, created.Code),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var analytics analyticsResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&analytics))

	assert.Equal(t, int64(1), analytics.TotalClicks)
	assert.Equal(t, int64(1), analytics.ByRegion["TW"], "地區 TW 應有 1 次點擊")
}

// TestRedirectRecordsClick_BothReferralAndRegion 驗證：推薦碼與地區同時帶入，兩者都正確記錄
func TestRedirectRecordsClick_BothReferralAndRegion(t *testing.T) {
	originalURL := "https://www.example.com/combined-test"
	created := createShortLink(t, originalURL, "user_bob")
	require.NotEmpty(t, created.ReferralCode)

	// 帶推薦碼 + 地區同時點擊
	r := doRedirect(t, created.Code, created.ReferralCode, "Mozilla/5.0", "JP")
	defer r.Body.Close()
	assert.Equal(t, http.StatusFound, r.StatusCode)

	waitForClick(t, created.Code, 1)

	resp, err := http.Get(
		fmt.Sprintf("%s/api/v1/links/%s/analytics", testEnv.server.URL, created.Code),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var analytics analyticsResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&analytics))

	assert.Equal(t, int64(1), analytics.TotalClicks)
	assert.Equal(t, int64(1), analytics.ByReferral[created.ReferralCode])
	assert.Equal(t, int64(1), analytics.ByRegion["JP"])
}

// TestRedirectBot_ReturnsOGHTML 驗證：社群平台 Bot 收到 OG HTML，不 redirect
func TestRedirectBot_ReturnsOGHTML(t *testing.T) {
	originalURL := "https://www.example.com/og-test"
	created := createShortLink(t, originalURL, "")

	// Facebook Bot UA
	r := doRedirect(t, created.Code, "",
		"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uagent.php)", "")
	defer r.Body.Close()

	// Bot 應收到 200 + HTML，不是 302
	assert.Equal(t, http.StatusOK, r.StatusCode, "Bot 應收到 200 OK")
	ct := r.Header.Get("Content-Type")
	assert.True(t, strings.HasPrefix(ct, "text/html"), "Content-Type 應為 text/html，got: %s", ct)
}

// TestRedirect_NotFound 驗證：不存在的短碼回傳 404
func TestRedirect_NotFound(t *testing.T) {
	r := doRedirect(t, "xxxxxxx", "", "Mozilla/5.0", "")
	defer r.Body.Close()
	assert.Equal(t, http.StatusNotFound, r.StatusCode)
}

// TestCreateWithReferral_ResponseContainsReferralCode 驗證：建立時帶 referral_owner_id，回應包含 referral_code
func TestCreateWithReferral_ResponseContainsReferralCode(t *testing.T) {
	created := createShortLink(t, "https://www.example.com/create-ref", "user_carol")
	assert.NotEmpty(t, created.ReferralCode, "回應應包含 referral_code")
}

// TestMultipleClicks_CountAccumulates 驗證：多次點擊後 TotalClicks 正確累加
func TestMultipleClicks_CountAccumulates(t *testing.T) {
	originalURL := "https://www.example.com/multi-click"
	created := createShortLink(t, originalURL, "")

	const clickCount = 3
	for i := 0; i < clickCount; i++ {
		r := doRedirect(t, created.Code, "", "Mozilla/5.0", "")
		r.Body.Close()
	}

	// 等待所有 async goroutine 完成
	waitForClick(t, created.Code, clickCount)

	resp, err := http.Get(
		fmt.Sprintf("%s/api/v1/links/%s/analytics", testEnv.server.URL, created.Code),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var analytics analyticsResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&analytics))
	assert.Equal(t, int64(clickCount), analytics.TotalClicks, "點擊次數應正確累加")
}
