package ml

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"landrisk/backend-go/internal/cache"
	"landrisk/backend-go/internal/domain"
	"landrisk/backend-go/internal/httpclient"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
	cacheTTL   time.Duration
	cache      *cache.RedisCache
	logger     zerolog.Logger
}

type SatelliteRequest struct {
	Latitude  *float64       `json:"latitude,omitempty"`
	Longitude *float64       `json:"longitude,omitempty"`
	Geometry  map[string]any `json:"geometry,omitempty"`
}

type predictRequest struct {
	Features []float64 `json:"features"`
}

func NewClient(baseURL string, timeout time.Duration, cacheTTL time.Duration, redisCache *cache.RedisCache, logger zerolog.Logger) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
		timeout:    timeout,
		cacheTTL:   cacheTTL,
		cache:      redisCache,
		logger:     logger.With().Str("component", "ml-client").Logger(),
	}
}

func (c *Client) GetSatelliteFeatures(ctx context.Context, req SatelliteRequest) (domain.SatelliteFeatures, int64, error) {
	start := time.Now()
	payload, err := json.Marshal(req)
	if err != nil {
		return domain.SatelliteFeatures{}, 0, err
	}

	body, _, err := httpclient.DoWithRetry(ctx, c.httpClient, httpclient.RequestConfig{
		Method:      http.MethodPost,
		URL:         c.baseURL + "/satellite-features",
		Headers:     map[string]string{"Content-Type": "application/json", "Accept": "application/json"},
		Body:        payload,
		RetryCount:  2,
		BaseBackoff: 250 * time.Millisecond,
		Timeout:     c.timeout,
	})
	if err != nil {
		return domain.SatelliteFeatures{}, time.Since(start).Milliseconds(), err
	}

	var out domain.SatelliteFeatures
	if err := json.Unmarshal(body, &out); err != nil {
		return domain.SatelliteFeatures{}, time.Since(start).Milliseconds(), err
	}
	return out, time.Since(start).Milliseconds(), nil
}

func (c *Client) Predict(ctx context.Context, features domain.FeatureVector) (domain.MLPrediction, int64, bool, error) {
	start := time.Now()
	cacheKey := predictionKey(features.AsArray())

	var cached domain.MLPrediction
	cachedOK, cacheErr := c.cache.GetJSON(ctx, cacheKey, &cached)
	if cacheErr == nil && cachedOK {
		return cached, time.Since(start).Milliseconds(), true, nil
	}

	payload, err := json.Marshal(predictRequest{Features: features.AsArray()})
	if err != nil {
		return domain.MLPrediction{}, 0, false, err
	}

	body, _, err := httpclient.DoWithRetry(ctx, c.httpClient, httpclient.RequestConfig{
		Method:      http.MethodPost,
		URL:         c.baseURL + "/predict",
		Headers:     map[string]string{"Content-Type": "application/json", "Accept": "application/json"},
		Body:        payload,
		RetryCount:  2,
		BaseBackoff: 250 * time.Millisecond,
		Timeout:     c.timeout,
	})
	if err != nil {
		return domain.MLPrediction{}, time.Since(start).Milliseconds(), false, err
	}

	var prediction domain.MLPrediction
	if err := json.Unmarshal(body, &prediction); err != nil {
		return domain.MLPrediction{}, time.Since(start).Milliseconds(), false, err
	}
	if err := c.cache.SetJSON(ctx, cacheKey, prediction, c.cacheTTL); err != nil {
		c.logger.Warn().Err(err).Msg("unable to cache prediction")
	}
	return prediction, time.Since(start).Milliseconds(), false, nil
}

func predictionKey(features []float64) string {
	payload, _ := json.Marshal(features)
	h := sha1.Sum(payload)
	return fmt.Sprintf("ml:prediction:%s", hex.EncodeToString(h[:]))
}
