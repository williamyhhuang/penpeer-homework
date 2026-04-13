export interface ShortLink {
  code: string;
  original_url: string;
  og_title: string;
  og_image: string;
  created_at: string;
  referral_code?: string;
}

export interface PreviewData {
  code: string;
  original_url: string;
  og_title: string;
  og_description: string;
  og_image: string;
}

export interface AnalyticsData {
  code: string;
  total_clicks: number;
  by_platform: Record<string, number>;
  by_device: Record<string, number>;
  by_region: Record<string, number>;
  by_referral: Record<string, number>;
}

export interface RankingItem {
  rank: number;
  code: string;
  original_url: string;
  total_clicks: number;
}
