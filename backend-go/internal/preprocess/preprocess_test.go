package preprocess

import "testing"

func TestProcessBuildsGeometryAndMetrics(t *testing.T) {
	raw := map[string]any{
		"geomForSurveyNum": map[string]any{
			"wkt": "POLYGON((77.0 12.0,77.1 12.0,77.1 12.1,77.0 12.1,77.0 12.0))",
		},
		"surveyNo": map[string]any{
			"areaAcre": 2.0,
		},
		"nearbyAssets": map[string]any{
			"count": 5,
		},
		"distanceBetweenPincode": map[string]any{
			"distance": 0.8,
			"unit":     "km",
		},
		"locationDetails": map[string]any{
			"zone": "forest eco",
		},
	}

	out := Process(raw)
	if out.Geometry["type"] == nil {
		t.Fatalf("expected geojson type, got empty geometry: %#v", out.Geometry)
	}
	if out.SurveyAreaSqMeters <= 0 {
		t.Fatalf("expected survey area > 0, got %f", out.SurveyAreaSqMeters)
	}
	if out.DistanceToWater <= 0 {
		t.Fatalf("expected distance to water in meters > 0, got %f", out.DistanceToWater)
	}
	if out.ZonationCode == 0 {
		t.Fatalf("expected non-zero zonation code")
	}
}
