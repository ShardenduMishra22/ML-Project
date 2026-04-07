import { useMemo, useState } from "react";
import InputForm from "./components/InputForm";
import MapPanel from "./components/MapPanel";
import SummaryPanel from "./components/SummaryPanel";
import RawDataPanel from "./components/RawDataPanel";
import { analyzeLand, downloadReportJson, downloadReportPdf, getTrace } from "./services/api";

function saveBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}

function saveJsonFile(data, filename) {
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
  saveBlob(blob, filename);
}

export default function App() {
  const [loading, setLoading] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState("");
  const [report, setReport] = useState(null);
  const [trace, setTrace] = useState(null);
  const [input, setInput] = useState(null);

  const headerText = useMemo(
    () => "AI-powered geospatial risk assessment with satellite validation and traceable evidence.",
    []
  );

  async function handleAnalyze(payload) {
    setLoading(true);
    setError("");
    setInput(payload);

    try {
      const analyzed = await analyzeLand(payload);
      const reportPayload = analyzed.report;
      setReport(reportPayload);

      if (reportPayload?.id) {
        const tracePayload = await getTrace(reportPayload.id);
        setTrace(tracePayload);
      } else {
        setTrace(null);
      }
    } catch (err) {
      setError(err.message || "Failed to run analysis.");
      setReport(null);
      setTrace(null);
    } finally {
      setLoading(false);
    }
  }

  async function handleExportJson() {
    if (!report?.id) return;
    setExporting(true);
    setError("");
    try {
      const data = await downloadReportJson(report.id);
      saveJsonFile(data, `land-risk-report-${report.id}.json`);
    } catch (err) {
      setError(err.message || "JSON export failed.");
    } finally {
      setExporting(false);
    }
  }

  async function handleExportPdf() {
    if (!report?.id) return;
    setExporting(true);
    setError("");
    try {
      const blob = await downloadReportPdf(report.id);
      saveBlob(blob, `land-risk-report-${report.id}.pdf`);
    } catch (err) {
      setError(err.message || "PDF export failed.");
    } finally {
      setExporting(false);
    }
  }

  return (
    <div className="page">
      <header className="hero">
        <p className="eyebrow">Land Risk Intelligence System</p>
        <h1>Assessment Console</h1>
        <p>{headerText}</p>
      </header>

      {error && <div className="error-banner">{error}</div>}

      <section className="layout-main">
        <InputForm onSubmit={handleAnalyze} loading={loading} />
        <SummaryPanel
          report={report}
          onExportJson={handleExportJson}
          onExportPdf={handleExportPdf}
          exporting={exporting}
        />
      </section>

      <section className="layout-secondary">
        <MapPanel report={report} input={input} />
        <RawDataPanel trace={trace} />
      </section>
    </div>
  );
}
