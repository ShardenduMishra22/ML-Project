const API_BASE = import.meta.env.VITE_API_BASE_URL || "http://localhost:8080";

export async function analyzeLand(payload) {
  const response = await fetch(`${API_BASE}/analyze`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({ error: "request failed" }));
    throw new Error(data.error || "request failed");
  }
  return response.json();
}

export async function getTrace(reportId) {
  const response = await fetch(`${API_BASE}/trace/${reportId}`);
  if (!response.ok) {
    const data = await response.json().catch(() => ({ error: "trace fetch failed" }));
    throw new Error(data.error || "trace fetch failed");
  }
  return response.json();
}

export async function downloadReportJson(reportId) {
  const response = await fetch(`${API_BASE}/report/${reportId}`);
  if (!response.ok) {
    const data = await response.json().catch(() => ({ error: "json export failed" }));
    throw new Error(data.error || "json export failed");
  }
  return response.json();
}

export async function downloadReportPdf(reportId) {
  const response = await fetch(`${API_BASE}/report/${reportId}?format=pdf`);
  if (!response.ok) {
    const data = await response.json().catch(() => ({ error: "pdf export failed" }));
    throw new Error(data.error || "pdf export failed");
  }
  return response.blob();
}
