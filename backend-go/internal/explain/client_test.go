package explain

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"landrisk/backend-go/internal/domain"
)

func TestSummarizeFallsBackWithoutAPIKey(t *testing.T) {
	client := NewClient("", "", "", 2*time.Second, zerolog.Nop())
	report := domain.Report{
		RiskClass:  "High",
		Confidence: 0.84,
		MLPrediction: domain.MLPrediction{
			Prediction:  2,
			Probability: 0.81,
		},
		SatelliteFeatures: domain.SatelliteFeatures{
			WaterOverlapRatio: 0.58,
		},
		ValidationSummary: domain.ValidationSummary{
			CrossSourceAgreement: false,
		},
	}

	summary := client.Summarize(context.Background(), report)
	if summary.Source != "fallback-rule-engine" {
		t.Fatalf("expected fallback source, got %q", summary.Source)
	}
	if summary.Overview == "" {
		t.Fatal("expected non-empty overview")
	}
	if len(summary.KeyPoints) == 0 {
		t.Fatal("expected non-empty key points")
	}
	if len(summary.NextSteps) == 0 {
		t.Fatal("expected non-empty next steps")
	}
	if summary.Disclaimer == "" {
		t.Fatal("expected non-empty disclaimer")
	}
}

func TestParseChatSummaryWithCodeFence(t *testing.T) {
	response := map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"content": "```json\n{\"overview\":\"Plain summary\",\"keyPoints\":[\"k1\",\"k2\"],\"nextSteps\":[\"n1\"],\"disclaimer\":\"d1\"}\n```",
				},
			},
		},
	}

	body, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	parsed, err := parseChatSummary(body)
	if err != nil {
		t.Fatalf("parseChatSummary failed: %v", err)
	}
	if parsed.Overview != "Plain summary" {
		t.Fatalf("unexpected overview: %q", parsed.Overview)
	}
	if len(parsed.KeyPoints) != 2 {
		t.Fatalf("expected 2 key points, got %d", len(parsed.KeyPoints))
	}
	if len(parsed.NextSteps) != 1 {
		t.Fatalf("expected 1 next step, got %d", len(parsed.NextSteps))
	}
}
