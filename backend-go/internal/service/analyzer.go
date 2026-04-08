package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
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
	cfg     config.Config
	repo    *db.Repository
	cache   *cache.RedisCache
	kgis    *kgis.Client
	ml      *ml.Client
	summary CitizenSummarizer
	logger  zerolog.Logger
	sf      singleflight.Group
}

type CitizenSummarizer interface {
	Summarize(ctx context.Context, report domain.Report) domain.CitizenSummary
}

func NewAnalyzer(cfg config.Config, repo *db.Repository, redisCache *cache.RedisCache, kgisClient *kgis.Client, mlClient *ml.Client, summarizer CitizenSummarizer, logger zerolog.Logger) *Analyzer {
	return &Analyzer{
		cfg:     cfg,
		repo:    repo,
		cache:   redisCache,
		kgis:    kgisClient,
		ml:      mlClient,
		summary: summarizer,
		logger:  logger.With().Str("component", "analyzer-service").Logger(),
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
		if a.summary != nil && strings.TrimSpace(cached.CitizenSummary.Overview) == "" {
			cached.CitizenSummary = a.summarizeCitizen(ctx, cached)
			if err := a.cache.SetJSON(ctx, cacheKey, cached, a.cfg.KGISCacheTTL); err != nil {
				a.logger.Warn().Err(err).Msg("failed to backfill citizen summary in cache")
			}
		}
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
		satelliteFallback := false
		satellite, satelliteLatency, err := a.ml.GetSatelliteFeatures(ctx, ml.SatelliteRequest{
			Latitude:  req.Latitude,
			Longitude: req.Longitude,
			Geometry:  pre.Geometry,
		})
		stepLatency["satelliteFeatures"] = time.Since(satStart).Milliseconds()
		if err != nil {
			if isTimeoutError(err) {
				satelliteFallback = true
				satellite = fallbackSatelliteFeatures(pre)
				a.logger.Warn().Err(err).Msg("satellite feature extraction timed out; using fallback values")
			} else {
				return nil, fmt.Errorf("satellite feature extraction failed: %w", err)
			}
		}

		features := buildFeatureVector(pre, satellite)
		prediction, mlLatency, predictionCacheHit, err := a.ml.Predict(ctx, features)
		stepLatency["mlPredict"] = mlLatency
		if err != nil {
			return nil, fmt.Errorf("ml prediction failed: %w", err)
		}

		validationResult := validation.Evaluate(features, satellite, prediction)
		if satelliteFallback {
			validationResult.Summary.Notes = append(validationResult.Summary.Notes, "Satellite feature service timed out; conservative fallback values were used.")
			validationResult.Explanation = append(validationResult.Explanation, "Satellite feature extraction timed out; fallback values were used for this run.")
		}
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
			domain.CitizenSummary{},
			validationResult.Explanation,
			apiTrace,
			latency,
		)
		if a.summary != nil {
			rep.CitizenSummary = a.summarizeCitizen(ctx, rep)
		}

		storeCtx, storeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer storeCancel()
		if err := a.repo.SaveReport(storeCtx, rep); err != nil {
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
		traceCtx, traceCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer traceCancel()
		if err := a.repo.SaveTrace(traceCtx, reportID, tracePayload); err != nil {
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

func (a *Analyzer) summarizeCitizen(reqCtx context.Context, rep domain.Report) domain.CitizenSummary {
	if a.summary == nil {
		return domain.CitizenSummary{}
	}

	const persistenceReserve = 1200 * time.Millisecond
	timeout := 1500 * time.Millisecond
	if a.cfg.OpenRouterTimeout > 0 && a.cfg.OpenRouterTimeout < timeout {
		timeout = a.cfg.OpenRouterTimeout
	}

	if deadline, ok := reqCtx.Deadline(); ok {
		remaining := time.Until(deadline) - persistenceReserve
		if remaining <= 0 {
			cancelledCtx, cancel := context.WithCancel(context.Background())
			cancel()
			return a.summary.Summarize(cancelledCtx, rep)
		}
		if remaining < timeout {
			timeout = remaining
		}
	}

	summaryCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return a.summary.Summarize(summaryCtx, rep)
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

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "timeout")
}

func fallbackSatelliteFeatures(pre preprocess.Output) domain.SatelliteFeatures {
	waterOverlap := 0.08
	switch {
	case pre.DistanceToWater > 0 && pre.DistanceToWater <= 100:
		waterOverlap = 0.55
	case pre.DistanceToWater > 0 && pre.DistanceToWater <= 250:
		waterOverlap = 0.35
	case pre.DistanceToWater > 0 && pre.DistanceToWater <= 500:
		waterOverlap = 0.2
	}

	ndvi := 0.22 + math.Min(0.35, pre.ForestProximity*0.16) - math.Min(0.1, waterOverlap*0.15)
	vegetation := 0.24 + math.Min(0.3, pre.ForestProximity*0.14)

	return domain.SatelliteFeatures{
		NDVI:              clampUnit(ndvi),
		VegetationDensity: clampUnit(vegetation),
		WaterOverlapRatio: clampUnit(waterOverlap),
	}
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
