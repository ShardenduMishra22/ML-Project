package preprocess

import (
	"encoding/json"
	"hash/fnv"
	"math"
	"strconv"
	"strings"

	"github.com/paulmach/orb/encoding/wkt"
	orbgeojson "github.com/paulmach/orb/geojson"
)

type Output struct {
	AdminHierarchy      map[string]any
	SurveyDetails       map[string]any
	Geometry            map[string]any
	Jurisdiction        map[string]any
	Zonation            map[string]any
	ZonationCode        int
	JurisdictionFlags   float64
	SurveyAreaSqMeters  float64
	DistanceToWater     float64
	NearbyAssetsDensity float64
	ForestProximity     float64
	Evidence            map[string]any
}

func Process(raw map[string]any) Output {
	adminHierarchy := mapOrEmpty(raw["kgisAdminHierarchy"])
	surveyDetails := mapOrEmpty(raw["surveyNo"])
	jurisdiction := mapOrEmpty(raw["jurisdictionBoundary"])
	zonation := mapOrEmpty(raw["locationDetails"])

	geometry := buildGeometry(raw["geomForSurveyNum"])
	normalizeGeometry(geometry)

	surveyArea := inferSurveyAreaSqMeters(surveyDetails)
	nearbyCount := inferCount(raw["nearbyAssets"])
	nearbyDensity := 0.0
	if surveyArea > 0 {
		nearbyDensity = nearbyCount / surveyArea
	}

	distanceToWater := inferDistance(raw["distanceBetweenPincode"])
	forestProximity := inferForestProximity(zonation)
	zonationCode := encodeCategory(strings.TrimSpace(strings.ToLower(stringify(zonation["zone"]))))
	jurisdictionFlags := inferJurisdictionFlags(jurisdiction)

	evidence := map[string]any{
		"surveyAreaSqMeters": surveyArea,
		"nearbyAssetsCount":  nearbyCount,
		"distanceToWater":    distanceToWater,
		"forestProximity":    forestProximity,
	}

	return Output{
		AdminHierarchy:      sanitizeMap(adminHierarchy),
		SurveyDetails:       sanitizeMap(surveyDetails),
		Geometry:            sanitizeMap(geometry),
		Jurisdiction:        sanitizeMap(jurisdiction),
		Zonation:            sanitizeMap(zonation),
		ZonationCode:        zonationCode,
		JurisdictionFlags:   jurisdictionFlags,
		SurveyAreaSqMeters:  surveyArea,
		DistanceToWater:     distanceToWater,
		NearbyAssetsDensity: nearbyDensity,
		ForestProximity:     forestProximity,
		Evidence:            evidence,
	}
}

func buildGeometry(rawGeom any) map[string]any {
	geomMap := mapOrEmpty(rawGeom)
	wktString := findWKTString(geomMap)
	if wktString == "" {
		return map[string]any{}
	}
	geometry, err := wkt.Unmarshal(wktString)
	if err != nil {
		return map[string]any{"invalidWKT": wktString}
	}
	g := orbgeojson.NewGeometry(geometry)
	payload, err := g.MarshalJSON()
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func normalizeGeometry(geometry map[string]any) {
	typeValue := strings.ToLower(stringify(geometry["type"]))
	coords, ok := geometry["coordinates"]
	if !ok {
		return
	}
	switch typeValue {
	case "polygon":
		geometry["coordinates"] = normalizePolygonCoords(coords)
	case "multipolygon":
		geometry["coordinates"] = normalizeMultiPolygonCoords(coords)
	}
}

func normalizePolygonCoords(coords any) any {
	rings, ok := coords.([]any)
	if !ok {
		return coords
	}
	for i := range rings {
		points, ok := rings[i].([]any)
		if !ok {
			continue
		}
		rings[i] = dedupePoints(points)
	}
	return rings
}

func normalizeMultiPolygonCoords(coords any) any {
	polygons, ok := coords.([]any)
	if !ok {
		return coords
	}
	for i := range polygons {
		polygons[i] = normalizePolygonCoords(polygons[i])
	}
	return polygons
}

func dedupePoints(points []any) []any {
	if len(points) == 0 {
		return points
	}
	out := make([]any, 0, len(points))
	var prev string
	for _, point := range points {
		current := stringify(point)
		if current == prev {
			continue
		}
		prev = current
		out = append(out, point)
	}
	if len(out) > 1 && stringify(out[0]) != stringify(out[len(out)-1]) {
		out = append(out, out[0])
	}
	return out
}

func sanitizeMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		if value == nil {
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			out[key] = sanitizeMap(typed)
		case []any:
			out[key] = sanitizeSlice(typed)
		default:
			out[key] = typed
		}
	}
	return out
}

func sanitizeSlice(in []any) []any {
	out := make([]any, 0, len(in))
	seen := map[string]struct{}{}
	for _, value := range in {
		if value == nil {
			continue
		}
		if mapped, ok := value.(map[string]any); ok {
			clean := sanitizeMap(mapped)
			key := stringify(clean)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, clean)
			continue
		}
		key := stringify(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func mapOrEmpty(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}
	return map[string]any{"value": value}
}

func findWKTString(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, val := range typed {
			keyLower := strings.ToLower(key)
			if keyLower == "wkt" || keyLower == "geom" || strings.Contains(keyLower, "geometry") {
				if str := stringify(val); strings.Contains(str, "(") && strings.Contains(str, ")") {
					return str
				}
			}
			if nested := findWKTString(val); nested != "" {
				return nested
			}
		}
	case []any:
		for _, item := range typed {
			if nested := findWKTString(item); nested != "" {
				return nested
			}
		}
	case string:
		if strings.Contains(typed, "(") && strings.Contains(typed, ")") {
			return typed
		}
	}
	return ""
}

func inferSurveyAreaSqMeters(surveyDetails map[string]any) float64 {
	if areaSqM, ok := pickFloat(surveyDetails, "areaSqM", "area_sqm", "area"); ok {
		return areaSqM
	}
	if areaAcre, ok := pickFloat(surveyDetails, "areaAcre", "area_acre", "acres"); ok {
		return areaAcre * 4046.8564224
	}
	if areaSqFt, ok := pickFloat(surveyDetails, "areaSqFt", "area_sqft", "sqft"); ok {
		return areaSqFt * 0.092903
	}
	return 0
}

func inferCount(value any) float64 {
	mapped := mapOrEmpty(value)
	if count, ok := pickFloat(mapped, "count", "assetCount", "total"); ok {
		return count
	}
	if list, ok := mapped["assets"].([]any); ok {
		return float64(len(list))
	}
	return 0
}

func inferDistance(value any) float64 {
	mapped := mapOrEmpty(value)
	if distance, ok := pickFloat(mapped, "distance", "distanceKm", "distance_km"); ok {
		unit := strings.ToLower(stringify(mapped["unit"]))
		if strings.Contains(unit, "km") {
			return distance * 1000
		}
		return distance
	}
	return 0
}

func inferForestProximity(zonation map[string]any) float64 {
	zone := strings.ToLower(stringify(zonation["zone"]))
	score := 0.0
	if strings.Contains(zone, "forest") {
		score += 1.0
	}
	if strings.Contains(zone, "eco") {
		score += 0.5
	}
	if distance, ok := pickFloat(zonation, "forestDistance", "forest_distance_m"); ok && distance > 0 {
		score += math.Max(0, 1.0-(distance/5000.0))
	}
	return score
}

func inferJurisdictionFlags(jurisdiction map[string]any) float64 {
	tokens := []string{"restricted", "water", "forest", "heritage", "eco"}
	raw := strings.ToLower(stringify(jurisdiction))
	score := 0.0
	for _, token := range tokens {
		if strings.Contains(raw, token) {
			score += 1.0
		}
	}
	return score
}

func encodeCategory(input string) int {
	if input == "" {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(input))
	return int(h.Sum32()%1000) + 1
}

func pickFloat(input map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, exists := input[key]; exists {
			if f, ok := toFloat(value); ok {
				return f, true
			}
		}
	}
	return 0, false
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		v, err := typed.Float64()
		return v, err == nil
	case string:
		trimmed := strings.TrimSpace(strings.TrimSuffix(typed, "m"))
		if trimmed == "" {
			return 0, false
		}
		v, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return v, true
	default:
		return 0, false
	}
}

func stringify(value any) string {
	if value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(payload)
}
