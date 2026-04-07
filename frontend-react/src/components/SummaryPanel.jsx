export default function SummaryPanel({ report, onExportJson, onExportPdf, exporting }) {
  if (!report) {
    return (
      <div className="card summary-card empty-state">
        <h2>Risk Summary</h2>
        <p className="muted">Run an analysis to generate the risk summary and confidence report.</p>
      </div>
    );
  }

  return (
    <div className="card summary-card">
      <h2>Risk Summary</h2>
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

      <h3>Explanation</h3>
      <ul className="explanation-list">
        {(report.explanation || []).map((line, idx) => (
          <li key={idx}>{line}</li>
        ))}
      </ul>

      <div className="actions">
        <button onClick={onExportJson} disabled={exporting}>Export JSON</button>
        <button onClick={onExportPdf} disabled={exporting}>Export PDF</button>
      </div>
    </div>
  );
}
