package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"landrisk/backend-go/internal/domain"
)

type mockAnalyzer struct{}

func (m mockAnalyzer) Analyze(ctx context.Context, req domain.AnalyzeRequest) (domain.Report, bool, error) {
	return domain.Report{ID: "r1", CreatedAt: time.Now().UTC(), RiskClass: "Low", Confidence: 0.8}, false, nil
}

func (m mockAnalyzer) GetReport(ctx context.Context, id string) (domain.StoredReport, error) {
	return domain.StoredReport{ID: id, RiskClass: "Low", Confidence: 0.8, CreatedAt: time.Now().UTC(), Payload: map[string]any{"id": id}}, nil
}

func (m mockAnalyzer) GetReportPDF(ctx context.Context, id string) ([]byte, error) {
	return []byte("pdf-bytes"), nil
}

func (m mockAnalyzer) GetTrace(ctx context.Context, id string) (domain.TraceResponse, error) {
	return domain.TraceResponse{ReportID: id, Trace: map[string]any{"ok": true}, Metrics: map[string]any{}}, nil
}

func TestHealthEndpoint(t *testing.T) {
	app := fiber.New()
	handler := New(mockAnalyzer{}, zerolog.Nop())
	handler.Register(app)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
}

func TestAnalyzeEndpoint(t *testing.T) {
	app := fiber.New()
	handler := New(mockAnalyzer{}, zerolog.Nop())
	handler.Register(app)

	body, _ := json.Marshal(map[string]any{"latitude": 12.97, "longitude": 77.59})
	req := httptest.NewRequest(http.MethodPost, "/analyze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("analyze request failed: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
}
