import { useState } from "react";
import { CreateLinkForm } from "./components/CreateLinkForm";
import { LinkResult } from "./components/LinkResult";
import { AnalyticsDashboard } from "./components/AnalyticsDashboard";
import type { ShortLink } from "./types";

type View = "home" | "analytics";

export function App() {
  const [createdLink, setCreatedLink] = useState<ShortLink | null>(null);
  const [analyticsCode, setAnalyticsCode] = useState<string | null>(null);
  const [view, setView] = useState<View>("home");

  const handleViewAnalytics = (code: string) => {
    setAnalyticsCode(code);
    setView("analytics");
  };

  return (
    <div style={{ minHeight: "100vh" }}>
      <header style={{ background: "#1976d2", color: "#fff", padding: "12px 24px" }}>
        <h1
          style={{ margin: 0, fontSize: 22, cursor: "pointer", userSelect: "none" }}
          onClick={() => setView("home")}
        >
          社群短網址
        </h1>
      </header>

      <main style={{ maxWidth: 640, margin: "32px auto", padding: "0 16px" }}>
        {view === "home" && (
          <>
            <CreateLinkForm onCreated={setCreatedLink} />
            {createdLink && (
              <LinkResult link={createdLink} onViewAnalytics={handleViewAnalytics} />
            )}
          </>
        )}
        {view === "analytics" && analyticsCode && (
          <AnalyticsDashboard code={analyticsCode} onBack={() => setView("home")} />
        )}
      </main>
    </div>
  );
}
