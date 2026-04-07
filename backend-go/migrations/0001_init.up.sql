CREATE TABLE IF NOT EXISTS reports (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    risk_class TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL,
    prediction INTEGER NOT NULL,
    probability DOUBLE PRECISION NOT NULL,
    report_json JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS api_traces (
    report_id UUID PRIMARY KEY REFERENCES reports(id) ON DELETE CASCADE,
    trace_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS metrics (
    id BIGSERIAL PRIMARY KEY,
    report_id UUID NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    metric_name TEXT NOT NULL,
    metric_value DOUBLE PRECISION NOT NULL,
    unit TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reports_created_at ON reports(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_reports_risk_class ON reports(risk_class);
CREATE INDEX IF NOT EXISTS idx_metrics_report_id ON metrics(report_id);
CREATE INDEX IF NOT EXISTS idx_metrics_name ON metrics(metric_name);
