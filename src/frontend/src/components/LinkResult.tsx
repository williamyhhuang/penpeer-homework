import { useState } from "react";
import type { ShortLink } from "../types";

interface Props {
  link: ShortLink;
  onViewAnalytics: (code: string) => void;
}

export function LinkResult({ link, onViewAnalytics }: Props) {
  const shortURL = `${window.location.origin}/${link.code}`;
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    await navigator.clipboard.writeText(shortURL);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div style={styles.card}>
      <h3 style={styles.title}>短網址建立成功！</h3>
      <div style={styles.row}>
        <a href={shortURL} target="_blank" rel="noreferrer" style={styles.link}>
          {shortURL}
        </a>
        <button style={styles.copyBtn} onClick={copy}>
          {copied ? "已複製" : "複製"}
        </button>
      </div>
      {link.og_title && (
        <div style={styles.og}>
          {link.og_image && (
            <img src={link.og_image} alt="OG" style={styles.ogImg} />
          )}
          <p style={styles.ogTitle}>{link.og_title}</p>
        </div>
      )}
      {link.referral_code && (
        <p style={styles.ref}>推薦碼：<code>{link.referral_code}</code></p>
      )}
      <button style={styles.analyticsBtn} onClick={() => onViewAnalytics(link.code)}>
        查看點擊統計
      </button>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  card:        { background: "#f0f7ff", borderRadius: 8, padding: 24, marginTop: 16 },
  title:       { margin: "0 0 12px", color: "#1976d2" },
  row:         { display: "flex", alignItems: "center", gap: 8, marginBottom: 12 },
  link:        { color: "#1976d2", fontWeight: 600, fontSize: 16, wordBreak: "break-all" },
  copyBtn:     { padding: "6px 14px", background: "#1976d2", color: "#fff", border: "none", borderRadius: 4, cursor: "pointer" },
  og:          { display: "flex", alignItems: "center", gap: 12, margin: "12px 0", padding: 12, background: "#fff", borderRadius: 6 },
  ogImg:       { width: 60, height: 60, objectFit: "cover", borderRadius: 4 },
  ogTitle:     { margin: 0, fontWeight: 500 },
  ref:         { margin: "8px 0", fontSize: 14, color: "#555" },
  analyticsBtn:{ padding: "8px 16px", background: "transparent", color: "#1976d2", border: "1px solid #1976d2", borderRadius: 6, cursor: "pointer", fontSize: 14 },
};
