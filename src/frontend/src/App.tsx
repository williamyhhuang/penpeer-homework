import { useState } from "react";
import { Routes, Route, useNavigate, useParams } from "react-router-dom";
import { CreateLinkForm } from "./components/CreateLinkForm";
import { LinkResult } from "./components/LinkResult";
import { AnalyticsDashboard } from "./components/AnalyticsDashboard";
import { RankingDashboard } from "./components/RankingDashboard";
import type { ShortLink } from "./types";

function HomePage() {
  const [createdLink, setCreatedLink] = useState<ShortLink | null>(null);
  const navigate = useNavigate();

  return (
    <>
      <CreateLinkForm onCreated={setCreatedLink} />
      {createdLink && (
        <LinkResult
          link={createdLink}
          onViewAnalytics={(code) => navigate(`/analytics/${code}`)}
        />
      )}
    </>
  );
}

function AnalyticsPage() {
  const { code } = useParams<{ code: string }>();
  const navigate = useNavigate();

  if (!code) return null;
  return <AnalyticsDashboard code={code} onBack={() => navigate("/analytics")} />;
}

export function App() {
  const navigate = useNavigate();

  return (
    <div style={{ minHeight: "100vh" }}>
      <header style={styles.header}>
        <h1 style={styles.logo} onClick={() => navigate("/")}>
          社群短網址
        </h1>
        <nav>
          <button style={styles.navBtn} onClick={() => navigate("/analytics")}>
            排行榜
          </button>
        </nav>
      </header>

      <main style={{ maxWidth: 760, margin: "32px auto", padding: "0 16px" }}>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/analytics" element={<RankingDashboard />} />
          <Route path="/analytics/:code" element={<AnalyticsPage />} />
        </Routes>
      </main>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  header:  { background: "#1976d2", color: "#fff", padding: "12px 24px", display: "flex", alignItems: "center", justifyContent: "space-between" },
  logo:    { margin: 0, fontSize: 22, cursor: "pointer", userSelect: "none" },
  navBtn:  { background: "rgba(255,255,255,0.15)", color: "#fff", border: "1px solid rgba(255,255,255,0.4)", borderRadius: 6, padding: "6px 14px", cursor: "pointer", fontSize: 14 },
};
