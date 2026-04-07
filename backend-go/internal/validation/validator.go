package validation

import (
	"fmt"
	"math"

	"landrisk/backend-go/internal/domain"
)

type Result struct {
	RiskClass      string
	Confidence     float64
	Summary        domain.ValidationSummary
	Explanation    []string
	ForcedHighRisk bool
}

func Evaluate(features domain.FeatureVector, satellite domain.SatelliteFeatures, prediction domain.MLPrediction) Result {
	riskClass := mapPredictionToRisk(prediction.Prediction)
	agreement := agreeOnWaterRisk(features, satellite)

	confidence := 0.5
	confidenceRule := "conflict"
	if agreement {
		confidence = 0.9
		confidenceRule = "high agreement"
	}

	overrides := []string{}
	forced := false
	if satellite.WaterOverlapRatio > 0.6 {
		riskClass = "High"
		forced = true
		overrides = append(overrides, "water_overlap_ratio > 0.6 forced High risk")
	}

	explanation := []string{
		fmt.Sprintf("Model predicted class %d with probability %.3f", prediction.Prediction, prediction.Probability),
		fmt.Sprintf("Satellite NDVI %.3f and vegetation density %.3f", satellite.NDVI, satellite.VegetationDensity),
		fmt.Sprintf("Water overlap ratio %.3f and distance to water %.2f m", satellite.WaterOverlapRatio, features.DistanceToWater),
		fmt.Sprintf("Cross-source agreement: %t", agreement),
	}
	if forced {
		explanation = append(explanation, "Rule override applied due to high water overlap")
	}

	return Result{
		RiskClass:      riskClass,
		Confidence:     math.Round(confidence*100) / 100,
		ForcedHighRisk: forced,
		Summary: domain.ValidationSummary{
			CrossSourceAgreement: agreement,
			ConfidenceRule:       confidenceRule,
			RuleOverrides:        overrides,
			Notes: []string{
				"Confidence is based on KGIS and satellite feature agreement.",
				"High overlap with water bodies triggers deterministic safety override.",
			},
		},
		Explanation: explanation,
	}
}

func mapPredictionToRisk(class int) string {
	switch class {
	case 2:
		return "High"
	case 1:
		return "Medium"
	default:
		return "Low"
	}
}

func agreeOnWaterRisk(features domain.FeatureVector, satellite domain.SatelliteFeatures) bool {
	kgisWaterRisk := features.DistanceToWater < 250 || features.WaterOverlapRatio > 0.35
	satWaterRisk := satellite.WaterOverlapRatio > 0.35
	return kgisWaterRisk == satWaterRisk
}
