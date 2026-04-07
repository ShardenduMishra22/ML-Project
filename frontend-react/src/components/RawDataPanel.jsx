export default function RawDataPanel({ trace }) {
  return (
    <div className="card raw-card">
      <h2>Trace + Raw API Data</h2>
      <pre>{trace ? JSON.stringify(trace, null, 2) : "No trace loaded yet."}</pre>
    </div>
  );
}
