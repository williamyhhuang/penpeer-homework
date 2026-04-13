import { useState } from "react";
import { Routes, Route, useNavigate, useParams } from "react-router-dom";
import { CreateLinkForm } from "./components/CreateLinkForm";
import { LinkResult } from "./components/LinkResult";
import { AnalyticsDashboard } from "./components/AnalyticsDashboard";
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
  return <AnalyticsDashboard code={code} onBack={() => navigate("/")} />;
}

export function App() {
  const navigate = useNavigate();

  return (
    <div style={{ minHeight: "100vh" }}>
      <header style={{ background: "#1976d2", color: "#fff", padding: "12px 24px" }}>
        <h1
          style={{ margin: 0, fontSize: 22, cursor: "pointer", userSelect: "none" }}
          onClick={() => navigate("/")}
        >
          社群短網址
        </h1>
      </header>

      <main style={{ maxWidth: 640, margin: "32px auto", padding: "0 16px" }}>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/analytics/:code" element={<AnalyticsPage />} />
        </Routes>
      </main>
    </div>
  );
}
