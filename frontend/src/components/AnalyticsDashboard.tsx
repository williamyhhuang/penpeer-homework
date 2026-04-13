import { useEffect, useState } from "react";
import { api } from "../api/client";
import type { AnalyticsData } from "../types";

interface Props {
  code: string;
  onBack: () => void;
}

export function AnalyticsDashboard({ code, onBack }: Props) {
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.getAnalytics(code).then(setData).catch((e: Error) => setError(e.message));
  }, [code]);

  if (error) {
    return (
      <div style={styles.card}>
        <button onClick={onBack} style={styles.back}>← 返回</button>
        <p style={{ color: "#d32f2f" }}>載入失敗：{error}</p>
      </div>
    );
  }

  if (!data) {
    return <div style={styles.card}>載入中...</div>;
  }

  return (
    <div style={styles.card}>
      <button onClick={onBack} style={styles.back}>← 返回</button>
      <h2 style={styles.title}>點擊統計：/{code}</h2>
      <div style={styles.totalBox}>
        <span style={styles.totalNum}>{data.total_clicks}</span>
        <span style={styles.totalLabel}>總點擊數</span>
      </div>

      <div style={styles.grid}>
        <StatBlock title="來源平台" data={data.by_platform} />
        <StatBlock title="裝置類型" data={data.by_device} />
        <StatBlock title="地區分布" data={data.by_region} />
        <StatBlock title="推薦碼歸因" data={data.by_referral} />
      </div>
    </div>
  );
}

function StatBlock({ title, data }: { title: string; data: Record<string, number> }) {
  const entries = Object.entries(data).sort((a, b) => b[1] - a[1]);
  const total = entries.reduce((s, [, v]) => s + v, 0);

  return (
    <div style={styles.block}>
      <h4 style={styles.blockTitle}>{title}</h4>
      {entries.length === 0 ? (
        <p style={styles.empty}>尚無資料</p>
      ) : (
        entries.map(([key, count]) => (
          <div key={key} style={styles.barRow}>
            <span style={styles.barLabel}>{key}</span>
            <div style={styles.barTrack}>
              <div
                style={{
                  ...styles.barFill,
                  width: `${total > 0 ? (count / total) * 100 : 0}%`,
                }}
              />
            </div>
            <span style={styles.barCount}>{count}</span>
          </div>
        ))
      )}
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  card:       { background: "#fff", borderRadius: 8, padding: 24, boxShadow: "0 2px 8px rgba(0,0,0,.1)" },
  back:       { background: "none", border: "none", color: "#1976d2", cursor: "pointer", padding: "0 0 12px", fontSize: 14 },
  title:      { margin: "0 0 16px", fontSize: 20 },
  totalBox:   { display: "flex", flexDirection: "column", alignItems: "center", padding: "20px 0", marginBottom: 16, background: "#f0f7ff", borderRadius: 8 },
  totalNum:   { fontSize: 48, fontWeight: 700, color: "#1976d2" },
  totalLabel: { fontSize: 14, color: "#555", marginTop: 4 },
  grid:       { display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 },
  block:      { background: "#fafafa", borderRadius: 8, padding: 16 },
  blockTitle: { margin: "0 0 12px", fontSize: 14, fontWeight: 600, color: "#333" },
  empty:      { color: "#999", fontSize: 13, margin: 0 },
  barRow:     { display: "flex", alignItems: "center", gap: 8, marginBottom: 8 },
  barLabel:   { width: 80, fontSize: 12, color: "#555", flexShrink: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  barTrack:   { flex: 1, height: 8, background: "#e0e0e0", borderRadius: 4, overflow: "hidden" },
  barFill:    { height: "100%", background: "#1976d2", borderRadius: 4, transition: "width .3s" },
  barCount:   { width: 30, fontSize: 12, color: "#333", textAlign: "right", flexShrink: 0 },
};
