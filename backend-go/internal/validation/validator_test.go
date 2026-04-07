package validation

import (
	"testing"

	"landrisk/backend-go/internal/domain"
)

func TestEvaluateForcesHighRiskOnWaterOverlap(t *testing.T) {
	result := Evaluate(
		domain.FeatureVector{DistanceToWater: 500, WaterOverlapRatio: 0.2},
		domain.SatelliteFeatures{NDVI: 0.2, VegetationDensity: 0.3, WaterOverlapRatio: 0.72},
		domain.MLPrediction{Prediction: 0, Probability: 0.51},
	)

	if result.RiskClass != "High" {
		t.Fatalf("expected High risk due to override, got %s", result.RiskClass)
	}
	if !result.ForcedHighRisk {
		t.Fatalf("expected forced high risk override")
	}
}

func TestEvaluateConfidenceHighOnAgreement(t *testing.T) {
	result := Evaluate(
		domain.FeatureVector{DistanceToWater: 120, WaterOverlapRatio: 0.4},
		domain.SatelliteFeatures{WaterOverlapRatio: 0.45},
		domain.MLPrediction{Prediction: 2, Probability: 0.82},
	)

	if result.Confidence != 0.9 {
		t.Fatalf("expected 0.9 confidence, got %.2f", result.Confidence)
	}
}
