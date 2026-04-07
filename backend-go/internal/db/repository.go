package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"landrisk/backend-go/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) SaveReport(ctx context.Context, report domain.Report) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(
		ctx,
		`INSERT INTO reports (id, created_at, risk_class, confidence, prediction, probability, report_json)
 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
 ON CONFLICT (id) DO UPDATE SET
   risk_class = EXCLUDED.risk_class,
   confidence = EXCLUDED.confidence,
   prediction = EXCLUDED.prediction,
   probability = EXCLUDED.probability,
   report_json = EXCLUDED.report_json`,
		report.ID,
		report.CreatedAt,
		report.RiskClass,
		report.Confidence,
		report.MLPrediction.Prediction,
		report.MLPrediction.Probability,
		payload,
	)
	return err
}

func (r *Repository) GetReport(ctx context.Context, id string) (domain.StoredReport, error) {
	var (
		createdAt  time.Time
		riskClass  string
		confidence float64
		payloadRaw []byte
	)
	row := r.pool.QueryRow(ctx, `SELECT created_at, risk_class, confidence, report_json FROM reports WHERE id = $1::uuid`, id)
	if err := row.Scan(&createdAt, &riskClass, &confidence, &payloadRaw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StoredReport{}, fmt.Errorf("report not found")
		}
		return domain.StoredReport{}, err
	}

	var payload map[string]any
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return domain.StoredReport{}, err
	}
	return domain.StoredReport{
		ID:         id,
		RiskClass:  riskClass,
		Confidence: confidence,
		CreatedAt:  createdAt,
		Payload:    payload,
	}, nil
}

func (r *Repository) SaveTrace(ctx context.Context, reportID string, trace map[string]any) error {
	payload, err := json.Marshal(trace)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(
		ctx,
		`INSERT INTO api_traces (report_id, trace_json, created_at)
 VALUES ($1::uuid, $2, NOW())
 ON CONFLICT (report_id) DO UPDATE SET trace_json = EXCLUDED.trace_json`,
		reportID,
		payload,
	)
	return err
}

func (r *Repository) GetTrace(ctx context.Context, reportID string) (map[string]any, error) {
	var payload []byte
	row := r.pool.QueryRow(ctx, `SELECT trace_json FROM api_traces WHERE report_id = $1::uuid`, reportID)
	if err := row.Scan(&payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("trace not found")
		}
		return nil, err
	}
	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func (r *Repository) SaveMetric(ctx context.Context, reportID, name string, value float64, unit string) error {
	_, err := r.pool.Exec(
		ctx,
		`INSERT INTO metrics (report_id, metric_name, metric_value, unit, created_at)
 VALUES ($1::uuid, $2, $3, $4, NOW())`,
		reportID,
		name,
		value,
		unit,
	)
	return err
}

func (r *Repository) GetMetrics(ctx context.Context, reportID string) (map[string]any, error) {
	rows, err := r.pool.Query(ctx, `SELECT metric_name, metric_value, unit FROM metrics WHERE report_id = $1::uuid`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]any{}
	for rows.Next() {
		var name, unit string
		var value float64
		if err := rows.Scan(&name, &value, &unit); err != nil {
			return nil, err
		}
		out[name] = map[string]any{"value": value, "unit": unit}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}
