function formatDate(dateValue) {
  if (!dateValue) return "--";
  const date = new Date(dateValue);
  if (Number.isNaN(date.getTime())) return "--";
  return date.toLocaleString();
}

export default function SummaryPanel({ report, onExportJson, onExportPdf, exporting, loading, activity }) {
  if (!report) {
    return (
      <div className="card summary-card empty-state">
        <div className="section-head">
          <div>
            <p className="section-kicker">Outcome</p>
            <h2>Risk Summary</h2>
          </div>
          <span className={`status-chip status-${activity?.status || "idle"}`}>
            {loading ? "Analyzing" : "Waiting"}
          </span>
        </div>
        <p className="muted">
          {loading
            ? "Assessment is currently running. Risk summary will appear as soon as computation completes."
            : "Run an analysis to generate the risk summary and confidence report."}
        </p>
        <div className={`summary-placeholder ${loading ? "is-loading" : ""}`}>
          <div className="skeleton-block" />
          <div className="skeleton-grid">
            <div className="skeleton-block" />
            <div className="skeleton-block" />
            <div className="skeleton-block" />
            <div className="skeleton-block" />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="card summary-card">
      <div className="section-head">
        <div>
          <p className="section-kicker">Outcome</p>
          <h2>Risk Summary</h2>
        </div>
        <span className="summary-meta">Generated {formatDate(report.createdAt)}</span>
      </div>
      <div className="risk-badges">
        <span className={`pill risk-${report.riskClass?.toLowerCase()}`}>{report.riskClass}</span>
        <span className="pill confidence">Confidence {Number(report.confidence || 0).toFixed(2)}</span>
      </div>

      <div className="stats-grid">
        <div>
          <strong>Prediction</strong>
          <p>{report.mlPrediction?.prediction}</p>
        </div>
        <div>
          <strong>Probability</strong>
          <p>{Number(report.mlPrediction?.probability || 0).toFixed(3)}</p>
        </div>
        <div>
          <strong>NDVI</strong>
          <p>{Number(report.satelliteFeatures?.ndvi || 0).toFixed(3)}</p>
        </div>
        <div>
          <strong>Water Overlap</strong>
          <p>{Number(report.satelliteFeatures?.water_overlap_ratio || 0).toFixed(3)}</p>
        </div>
      </div>

      {report.citizenSummary?.overview && (
        <>
          <h3>Simple Explanation</h3>
          <p className="citizen-overview">{report.citizenSummary.overview}</p>

          {(report.citizenSummary.keyPoints || []).length > 0 && (
            <ul className="explanation-list citizen-points">
              {(report.citizenSummary.keyPoints || []).map((point, idx) => (
                <li key={`point-${idx}`}>{point}</li>
              ))}
            </ul>
          )}

          {(report.citizenSummary.nextSteps || []).length > 0 && (
            <>
              <h3>Suggested Next Steps</h3>
              <ul className="explanation-list citizen-next-steps">
                {(report.citizenSummary.nextSteps || []).map((step, idx) => (
                  <li key={`step-${idx}`}>{step}</li>
                ))}
              </ul>
            </>
          )}

          {report.citizenSummary.disclaimer && (
            <p className="citizen-disclaimer">{report.citizenSummary.disclaimer}</p>
          )}
        </>
      )}

      <h3>Technical Explanation</h3>
      <ul className="explanation-list">
        {(report.explanation || []).map((line, idx) => (
          <li key={idx}>{line}</li>
        ))}
      </ul>

      <div className="actions">
        <button onClick={onExportJson} disabled={exporting}>Export JSON</button>
        <button className="btn-secondary" onClick={onExportPdf} disabled={exporting}>Export PDF</button>
      </div>
    </div>
  );
}
