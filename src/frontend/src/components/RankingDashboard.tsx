import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";
import type { RankingItem } from "../types";

export function RankingDashboard() {
  const [items, setItems] = useState<RankingItem[]>([]);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    api.getRanking()
      .then((res) => setItems(res.ranking))
      .catch((e: Error) => setError(e.message));
  }, []);

  const maxClicks = items[0]?.total_clicks ?? 1;

  return (
    <div style={styles.card}>
      <h2 style={styles.title}>點擊數排行榜</h2>

      {error && <p style={styles.error}>載入失敗：{error}</p>}

      {!error && items.length === 0 && (
        <p style={styles.empty}>尚無短網址資料</p>
      )}

      {items.length > 0 && (
        <table style={styles.table}>
          <thead>
            <tr>
              <th style={styles.th}>#</th>
              <th style={styles.th}>短碼</th>
              <th style={styles.th}>原始網址</th>
              <th style={{ ...styles.th, width: 200 }}>點擊數</th>
              <th style={styles.th}></th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.code} style={styles.tr}>
                <td style={styles.tdRank}>{item.rank}</td>
                <td style={styles.tdCode}>
                  <code style={styles.code}>{item.code}</code>
                </td>
                <td style={styles.tdUrl}>
                  <a
                    href={item.original_url}
                    target="_blank"
                    rel="noreferrer"
                    style={styles.urlLink}
                  >
                    {item.original_url}
                  </a>
                </td>
                <td style={styles.tdBar}>
                  <div style={styles.barTrack}>
                    <div
                      style={{
                        ...styles.barFill,
                        width: `${maxClicks > 0 ? (item.total_clicks / maxClicks) * 100 : 0}%`,
                      }}
                    />
                  </div>
                  <span style={styles.clickCount}>{item.total_clicks}</span>
                </td>
                <td style={styles.tdAction}>
                  <button
                    style={styles.detailBtn}
                    onClick={() => navigate(`/analytics/${item.code}`)}
                  >
                    詳情
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  card:       { background: "#fff", borderRadius: 8, padding: 24, boxShadow: "0 2px 8px rgba(0,0,0,.1)" },
  title:      { margin: "0 0 20px", fontSize: 20, fontWeight: 600 },
  error:      { color: "#d32f2f", fontSize: 14 },
  empty:      { color: "#999", textAlign: "center", padding: "32px 0" },
  table:      { width: "100%", borderCollapse: "collapse", fontSize: 14 },
  th:         { textAlign: "left", padding: "8px 12px", borderBottom: "2px solid #e0e0e0", color: "#555", fontWeight: 600, whiteSpace: "nowrap" },
  tr:         { borderBottom: "1px solid #f0f0f0" },
  tdRank:     { padding: "10px 12px", fontWeight: 700, color: "#1976d2", width: 36 },
  tdCode:     { padding: "10px 12px", whiteSpace: "nowrap" },
  code:       { background: "#f5f5f5", padding: "2px 6px", borderRadius: 4, fontFamily: "monospace" },
  tdUrl:      { padding: "10px 12px", maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  urlLink:    { color: "#555", textDecoration: "none" },
  tdBar:      { padding: "10px 12px", width: 200 },
  barTrack:   { display: "inline-block", width: 120, height: 8, background: "#e0e0e0", borderRadius: 4, overflow: "hidden", verticalAlign: "middle", marginRight: 8 },
  barFill:    { height: "100%", background: "#1976d2", borderRadius: 4 },
  clickCount: { fontSize: 13, fontWeight: 600, color: "#333" },
  tdAction:   { padding: "10px 12px", whiteSpace: "nowrap" },
  detailBtn:  { padding: "4px 10px", background: "transparent", color: "#1976d2", border: "1px solid #1976d2", borderRadius: 4, cursor: "pointer", fontSize: 12 },
};
