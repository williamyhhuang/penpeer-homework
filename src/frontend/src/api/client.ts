import type { ShortLink, PreviewData, AnalyticsData, RankingItem } from "../types";

const BASE_URL = "/api/v1";

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, options);
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error ?? `HTTP ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  /** 建立短網址 */
  createLink: (url: string, referralOwnerID?: string): Promise<ShortLink> =>
    request("/links", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url, referral_owner_id: referralOwnerID ?? "" }),
    }),

  /** 取得 OG 預覽資料 */
  getPreview: (code: string): Promise<PreviewData> =>
    request(`/links/${code}/preview`),

  /** 取得點擊統計 */
  getAnalytics: (code: string): Promise<AnalyticsData> =>
    request(`/links/${code}/analytics`),

  /** 取得所有短碼點擊數排行榜 */
  getRanking: (): Promise<{ ranking: RankingItem[] }> =>
    request("/links/ranking"),
};
