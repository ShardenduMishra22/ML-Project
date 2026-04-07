INSERT INTO reports (id, created_at, risk_class, confidence, prediction, probability, report_json)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    NOW(),
    'Medium',
    0.75,
    1,
    0.71,
    '{"id":"00000000-0000-0000-0000-000000000001","riskClass":"Medium","confidence":0.75,"input":{"latitude":12.9716,"longitude":77.5946},"explanation":["Seed report for local validation"]}'::jsonb
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO api_traces (report_id, trace_json, created_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    '{"note":"seed trace"}'::jsonb,
    NOW()
)
ON CONFLICT (report_id) DO NOTHING;

INSERT INTO metrics (report_id, metric_name, metric_value, unit, created_at)
VALUES
    ('00000000-0000-0000-0000-000000000001', 'total_response_ms', 210.0, 'ms', NOW()),
    ('00000000-0000-0000-0000-000000000001', 'ml_inference_ms', 21.0, 'ms', NOW()),
    ('00000000-0000-0000-0000-000000000001', 'cache_hit_rate', 0.3, 'ratio', NOW());
