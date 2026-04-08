package service

import (
	"context"
	"fmt"
	"testing"

	"landrisk/backend-go/internal/preprocess"
)

func TestIsTimeoutError(t *testing.T) {
	if !isTimeoutError(context.DeadlineExceeded) {
		t.Fatal("expected context deadline exceeded to be treated as timeout")
	}

	err := fmt.Errorf("post satellite: context deadline exceeded")
	if !isTimeoutError(err) {
		t.Fatal("expected wrapped timeout message to be treated as timeout")
	}

	if isTimeoutError(fmt.Errorf("validation failed")) {
		t.Fatal("did not expect non-timeout error to be treated as timeout")
	}
}

func TestFallbackSatelliteFeaturesInRange(t *testing.T) {
	features := fallbackSatelliteFeatures(preprocess.Output{
		DistanceToWater: 50,
		ForestProximity: 1.3,
	})

	if features.NDVI < 0 || features.NDVI > 1 {
		t.Fatalf("ndvi out of range: %f", features.NDVI)
	}
	if features.VegetationDensity < 0 || features.VegetationDensity > 1 {
		t.Fatalf("vegetation density out of range: %f", features.VegetationDensity)
	}
	if features.WaterOverlapRatio < 0 || features.WaterOverlapRatio > 1 {
		t.Fatalf("water overlap out of range: %f", features.WaterOverlapRatio)
	}
	if features.WaterOverlapRatio < 0.5 {
		t.Fatalf("expected high overlap for near-water fallback, got: %f", features.WaterOverlapRatio)
	}
}
