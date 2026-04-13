import { useState } from "react";
import { api } from "../api/client";
import type { ShortLink } from "../types";

interface Props {
  onCreated: (link: ShortLink) => void;
}

export function CreateLinkForm({ onCreated }: Props) {
  const [url, setUrl] = useState("");
  const [referralOwner, setReferralOwner] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const link = await api.createLink(url, referralOwner || undefined);
      onCreated(link);
      setUrl("");
      setReferralOwner("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "建立失敗");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={styles.card}>
      <h2 style={styles.title}>建立短網址</h2>
      <form onSubmit={handleSubmit}>
        <div style={styles.field}>
          <label style={styles.label}>原始網址 *</label>
          <input
            style={styles.input}
            type="url"
            placeholder="https://www.example.com"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            required
          />
        </div>
        <div style={styles.field}>
          <label style={styles.label}>推薦碼擁有者（選填）</label>
          <input
            style={styles.input}
            type="text"
            placeholder="user123"
            value={referralOwner}
            onChange={(e) => setReferralOwner(e.target.value)}
          />
        </div>
        {error && <p style={styles.error}>{error}</p>}
        <button style={styles.button} type="submit" disabled={loading}>
          {loading ? "建立中..." : "建立短網址"}
        </button>
      </form>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  card:   { background: "#fff", borderRadius: 8, padding: 24, boxShadow: "0 2px 8px rgba(0,0,0,.1)" },
  title:  { margin: "0 0 20px", fontSize: 20, fontWeight: 600 },
  field:  { marginBottom: 16 },
  label:  { display: "block", marginBottom: 6, fontWeight: 500, fontSize: 14 },
  input:  { width: "100%", padding: "10px 12px", border: "1px solid #ddd", borderRadius: 6, fontSize: 14, boxSizing: "border-box" },
  error:  { color: "#d32f2f", fontSize: 14, marginBottom: 8 },
  button: { width: "100%", padding: "12px", background: "#1976d2", color: "#fff", border: "none", borderRadius: 6, fontSize: 16, cursor: "pointer", fontWeight: 600 },
};
