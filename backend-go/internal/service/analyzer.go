package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/sync/singleflight"

	"landrisk/backend-go/internal/cache"
	"landrisk/backend-go/internal/config"
	"landrisk/backend-go/internal/db"
	"landrisk/backend-go/internal/domain"
	"landrisk/backend-go/internal/kgis"
	"landrisk/backend-go/internal/metrics"
	"landrisk/backend-go/internal/ml"
	"landrisk/backend-go/internal/preprocess"
	"landrisk/backend-go/internal/report"
	"landrisk/backend-go/internal/validation"
)

type Analyzer struct {
	cfg    config.Config
	repo   *db.Repository
	cache  *cache.RedisCache
	kgis   *kgis.Client
	ml     *ml.Client
	logger zerolog.Logger
	sf     singleflight.Group
}

func NewAnalyzer(cfg config.Config, repo *db.Repository, redisCache *cache.RedisCache, kgisClient *kgis.Client, mlClient *ml.Client, logger zerolog.Logger) *Analyzer {
	return &Analyzer{
		cfg:    cfg,
		repo:   repo,
		cache:  redisCache,
		kgis:   kgisClient,
		ml:     mlClient,
		logger: logger.With().Str("component", "analyzer-service").Logger(),
	}
}

func (a *Analyzer) Analyze(ctx context.Context, req domain.AnalyzeRequest) (domain.Report, bool, error) {
	if err := validateRequest(req); err != nil {
		return domain.Report{}, false, err
	}

	ctx, cancel := context.WithTimeout(ctx, a.cfg.RequestTimeout)
	defer cancel()

	requestHash := hashInput(req)
	cacheKey := "analyze:result:" + requestHash

	var cached domain.Report
	cachedOK, cacheErr := a.cache.GetJSON(ctx, cacheKey, &cached)
	if cacheErr == nil && cachedOK {
		return cached, true, nil
	}

	value, err, _ := a.sf.Do(cacheKey, func() (interface{}, error) {
		started := time.Now()
		stepLatency := map[string]int64{}

		kgisStart := time.Now()
		kgisData, apiTrace, err := a.kgis.FetchAll(ctx, req)
		stepLatency["kgisPull"] = time.Since(kgisStart).Milliseconds()
		if err != nil {
			return nil, err
		}

		prepStart := time.Now()
		pre := preprocess.Process(kgisData)
		stepLatency["preprocess"] = time.Since(prepStart).Milliseconds()

		satStart := time.Now()
		satellite, satelliteLatency, err := a.ml.GetSatelliteFeatures(ctx, ml.SatelliteRequest{
			Latitude:  req.Latitude,
			Longitude: req.Longitude,
			Geometry:  pre.Geometry,
		})
		stepLatency["satelliteFeatures"] = time.Since(satStart).Milliseconds()
		if err != nil {
			return nil, fmt.Errorf("satellite feature extraction failed: %w", err)
		}

		features := buildFeatureVector(pre, satellite)
		prediction, mlLatency, predictionCacheHit, err := a.ml.Predict(ctx, features)
		stepLatency["mlPredict"] = mlLatency
		if err != nil {
			return nil, fmt.Errorf("ml prediction failed: %w", err)
		}

		validationResult := validation.Evaluate(features, satellite, prediction)
		stepLatency["validation"] = 1

		reportID := uuid.NewString()
		latency := metrics.BuildLatencyMetrics(apiTrace, stepLatency, mlLatency+satelliteLatency, time.Since(started).Milliseconds())
		preMap := map[string]any{
			"adminHierarchy": pre.AdminHierarchy,
			"surveyDetails":  pre.SurveyDetails,
			"geometry":       pre.Geometry,
			"jurisdiction":   pre.Jurisdiction,
			"zonation":       pre.Zonation,
			"evidence":       pre.Evidence,
		}

		rep := report.BuildReport(
			reportID,
			req,
			preMap,
			satellite,
			prediction,
			validationResult.Summary,
			validationResult.RiskClass,
			validationResult.Confidence,
			validationResult.Explanation,
			apiTrace,
			latency,
		)

		if err := a.repo.SaveReport(ctx, rep); err != nil {
			return nil, fmt.Errorf("failed to store report: %w", err)
		}

		tracePayload := map[string]any{
			"inputHash": requestHash,
			"apiTrace":  apiTrace,
			"kgis":      kgisData,
			"features":  features,
			"satellite": satellite,
			"ml": map[string]any{
				"prediction":         prediction,
				"predictionCacheHit": predictionCacheHit,
			},
			"validation": validationResult.Summary,
			"latency":    latency,
		}
		if err := a.repo.SaveTrace(ctx, reportID, tracePayload); err != nil {
			a.logger.Warn().Err(err).Str("reportId", reportID).Msg("failed to store trace")
		}

		a.persistMetrics(ctx, reportID, latency)
		if err := a.cache.SetJSON(ctx, cacheKey, rep, a.cfg.KGISCacheTTL); err != nil {
			a.logger.Warn().Err(err).Msg("failed to cache final report")
		}
		return rep, nil
	})
	if err != nil {
		return domain.Report{}, false, err
	}
	return value.(domain.Report), false, nil
}

func (a *Analyzer) GetReport(ctx context.Context, id string) (domain.StoredReport, error) {
	return a.repo.GetReport(ctx, id)
}

func (a *Analyzer) GetReportPDF(ctx context.Context, id string) ([]byte, error) {
	stored, err := a.repo.GetReport(ctx, id)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(stored.Payload)
	if err != nil {
		return nil, err
	}
	var parsed domain.Report
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, err
	}
	return report.GeneratePDF(parsed)
}

func (a *Analyzer) GetTrace(ctx context.Context, id string) (domain.TraceResponse, error) {
	trace, err := a.repo.GetTrace(ctx, id)
	if err != nil {
		return domain.TraceResponse{}, err
	}
	metricPayload, err := a.repo.GetMetrics(ctx, id)
	if err != nil {
		metricPayload = map[string]any{}
	}
	return domain.TraceResponse{ReportID: id, Trace: trace, Metrics: metricPayload}, nil
}

func (a *Analyzer) persistMetrics(ctx context.Context, reportID string, latency domain.LatencyMetrics) {
	_ = a.repo.SaveMetric(ctx, reportID, "total_response_ms", float64(latency.TotalResponseMS), "ms")
	_ = a.repo.SaveMetric(ctx, reportID, "ml_inference_ms", float64(latency.MLInferenceMS), "ms")
	_ = a.repo.SaveMetric(ctx, reportID, "cache_hit_rate", latency.CacheHitRate, "ratio")
	for name, value := range latency.APILatencyMS {
		_ = a.repo.SaveMetric(ctx, reportID, "api_latency_"+name, float64(value), "ms")
	}
	for name, value := range latency.StepLatencyMS {
		_ = a.repo.SaveMetric(ctx, reportID, "step_latency_"+name, float64(value), "ms")
	}
}

func validateRequest(req domain.AnalyzeRequest) error {
	hasLatLon := req.Latitude != nil && req.Longitude != nil
	hasSurvey := req.SurveyNumber != "" && req.VillageID != ""
	if !hasLatLon && !hasSurvey {
		return fmt.Errorf("either latitude/longitude or villageId/surveyNumber is required")
	}
	if req.Latitude != nil && (*req.Latitude < -90 || *req.Latitude > 90) {
		return fmt.Errorf("latitude out of range")
	}
	if req.Longitude != nil && (*req.Longitude < -180 || *req.Longitude > 180) {
		return fmt.Errorf("longitude out of range")
	}
	return nil
}

func buildFeatureVector(pre preprocess.Output, sat domain.SatelliteFeatures) domain.FeatureVector {
	anomaly := 0.0
	if pre.SurveyAreaSqMeters == 0 {
		anomaly += 0.35
	}
	if len(pre.Geometry) == 0 {
		anomaly += 0.25
	}
	if pre.DistanceToWater == 0 {
		anomaly += 0.15
	}
	if sat.WaterOverlapRatio > 0.7 && sat.NDVI > 0.6 {
		anomaly += 0.25
	}
	anomaly = math.Min(1.0, anomaly)

	return domain.FeatureVector{
		DistanceToWater:     pre.DistanceToWater,
		WaterOverlapRatio:   sat.WaterOverlapRatio,
		NDVI:                sat.NDVI,
		VegetationDensity:   sat.VegetationDensity,
		ForestProximity:     pre.ForestProximity,
		JurisdictionFlags:   pre.JurisdictionFlags,
		ZonationCode:        float64(pre.ZonationCode),
		NearbyAssetsDensity: pre.NearbyAssetsDensity,
		SurveyArea:          pre.SurveyAreaSqMeters,
		AnomalyScore:        anomaly,
	}
}

func hashInput(req domain.AnalyzeRequest) string {
	payload, _ := json.Marshal(req)
	h := sha1.Sum(payload)
	return hex.EncodeToString(h[:])
}
