package metrics

import "landrisk/backend-go/internal/domain"

func BuildLatencyMetrics(apiTrace []domain.APICallTrace, stepLatency map[string]int64, mlInference int64, total int64) domain.LatencyMetrics {
	cacheHits := 0
	apiLatencies := map[string]int64{}
	for _, trace := range apiTrace {
		apiLatencies[trace.Name] = trace.DurationMS
		if trace.CacheHit {
			cacheHits++
		}
	}
	cacheHitRate := 0.0
	if len(apiTrace) > 0 {
		cacheHitRate = float64(cacheHits) / float64(len(apiTrace))
	}
	return domain.LatencyMetrics{
		TotalResponseMS: total,
		MLInferenceMS:   mlInference,
		CacheHitRate:    cacheHitRate,
		APILatencyMS:    apiLatencies,
		StepLatencyMS:   stepLatency,
	}
}
