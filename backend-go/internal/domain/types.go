package domain

import "time"

type AnalyzeRequest struct {
	RequestID    string   `json:"requestId,omitempty"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	SurveyNumber string   `json:"surveyNumber,omitempty"`
	VillageID    string   `json:"villageId,omitempty"`
	Pincode      string   `json:"pincode,omitempty"`
	SurveyType   string   `json:"surveyType,omitempty"`
}

type SatelliteFeatures struct {
	NDVI              float64 `json:"ndvi"`
	VegetationDensity float64 `json:"vegetation_density"`
	WaterOverlapRatio float64 `json:"water_overlap_ratio"`
}

type MLPrediction struct {
	Prediction  int     `json:"prediction"`
	Probability float64 `json:"probability"`
}

type FeatureVector struct {
	DistanceToWater     float64 `json:"distance_to_water"`
	WaterOverlapRatio   float64 `json:"water_overlap_ratio"`
	NDVI                float64 `json:"ndvi"`
	VegetationDensity   float64 `json:"vegetation_density"`
	ForestProximity     float64 `json:"forest_proximity"`
	JurisdictionFlags   float64 `json:"jurisdiction_flags"`
	ZonationCode        float64 `json:"zonation_codes_encoded"`
	NearbyAssetsDensity float64 `json:"nearby_assets_density"`
	SurveyArea          float64 `json:"survey_area"`
	AnomalyScore        float64 `json:"anomaly_score"`
}

func (f FeatureVector) AsArray() []float64 {
	return []float64{
		f.DistanceToWater,
		f.WaterOverlapRatio,
		f.NDVI,
		f.VegetationDensity,
		f.ForestProximity,
		f.JurisdictionFlags,
		f.ZonationCode,
		f.NearbyAssetsDensity,
		f.SurveyArea,
		f.AnomalyScore,
	}
}

type ValidationSummary struct {
	CrossSourceAgreement bool     `json:"crossSourceAgreement"`
	ConfidenceRule       string   `json:"confidenceRule"`
	RuleOverrides        []string `json:"ruleOverrides"`
	Notes                []string `json:"notes"`
}

type APICallTrace struct {
	Name       string `json:"name"`
	Endpoint   string `json:"endpoint"`
	StatusCode int    `json:"statusCode"`
	Success    bool   `json:"success"`
	CacheHit   bool   `json:"cacheHit"`
	DurationMS int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

type LatencyMetrics struct {
	TotalResponseMS int64            `json:"totalResponseMs"`
	MLInferenceMS   int64            `json:"mlInferenceMs"`
	CacheHitRate    float64          `json:"cacheHitRate"`
	APILatencyMS    map[string]int64 `json:"apiLatencyMs"`
	StepLatencyMS   map[string]int64 `json:"stepLatencyMs"`
}

type CitizenSummary struct {
	Overview   string   `json:"overview"`
	KeyPoints  []string `json:"keyPoints"`
	NextSteps  []string `json:"nextSteps"`
	Disclaimer string   `json:"disclaimer"`
	Source     string   `json:"source"`
}

type Report struct {
	ID                string            `json:"id"`
	CreatedAt         time.Time         `json:"createdAt"`
	Input             AnalyzeRequest    `json:"input"`
	AdminHierarchy    map[string]any    `json:"adminHierarchy"`
	SurveyDetails     map[string]any    `json:"surveyDetails"`
	Geometry          map[string]any    `json:"geometry"`
	Jurisdiction      map[string]any    `json:"jurisdiction"`
	Zonation          map[string]any    `json:"zonation"`
	SatelliteFeatures SatelliteFeatures `json:"satelliteFeatures"`
	MLPrediction      MLPrediction      `json:"mlPrediction"`
	RiskClass         string            `json:"riskClass"`
	Confidence        float64           `json:"confidence"`
	CitizenSummary    CitizenSummary    `json:"citizenSummary"`
	Explanation       []string          `json:"explanation"`
	ValidationSummary ValidationSummary `json:"validationSummary"`
	Evidence          map[string]any    `json:"evidence"`
	APITrace          []APICallTrace    `json:"apiTrace"`
	LatencyMetrics    LatencyMetrics    `json:"latencyMetrics"`
}

type StoredReport struct {
	ID         string         `json:"id"`
	RiskClass  string         `json:"riskClass"`
	Confidence float64        `json:"confidence"`
	CreatedAt  time.Time      `json:"createdAt"`
	Payload    map[string]any `json:"payload"`
}

type TraceResponse struct {
	ReportID string         `json:"reportId"`
	Trace    map[string]any `json:"trace"`
	Metrics  map[string]any `json:"metrics"`
}
