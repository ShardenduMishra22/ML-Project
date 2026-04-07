package kgis

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"landrisk/backend-go/internal/cache"
	"landrisk/backend-go/internal/domain"
	"landrisk/backend-go/internal/httpclient"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	retryCount int
	timeout    time.Duration
	cacheTTL   time.Duration
	cache      *cache.RedisCache
	logger     zerolog.Logger
	sf         singleflight.Group
}

type endpointDef struct {
	Name      string
	PathFn    func(req domain.AnalyzeRequest) string
	QueryFn   func(req domain.AnalyzeRequest) url.Values
	ShouldRun func(req domain.AnalyzeRequest) bool
}

type fetchResult struct {
	Body   []byte
	Status int
	Err    string
}

func NewClient(baseURL string, timeout time.Duration, retryCount int, cacheTTL time.Duration, redisCache *cache.RedisCache, logger zerolog.Logger) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
		retryCount: retryCount,
		timeout:    timeout,
		cacheTTL:   cacheTTL,
		cache:      redisCache,
		logger:     logger.With().Str("component", "kgis-client").Logger(),
	}
}

func (c *Client) FetchAll(ctx context.Context, req domain.AnalyzeRequest) (map[string]any, []domain.APICallTrace, error) {
	endpoints := buildEndpoints(req)
	result := make(map[string]any, len(endpoints))
	traces := make([]domain.APICallTrace, 0, len(endpoints))

	var mu sync.Mutex
	group, gctx := errgroup.WithContext(ctx)
	group.SetLimit(8)

	for _, endpoint := range endpoints {
		ep := endpoint
		group.Go(func() error {
			if ep.ShouldRun != nil && !ep.ShouldRun(req) {
				return nil
			}

			path := ep.PathFn(req)
			query := ep.QueryFn(req)
			fullURL := c.baseURL + path
			if q := query.Encode(); q != "" {
				fullURL += "?" + q
			}

			trace := domain.APICallTrace{Name: ep.Name, Endpoint: path}
			started := time.Now()
			cacheKey := "kgis:" + keyHash(fullURL)

			var cached map[string]any
			cachedOK, cacheErr := c.cache.GetJSON(gctx, cacheKey, &cached)
			if cacheErr == nil && cachedOK {
				trace.Success = true
				trace.CacheHit = true
				trace.StatusCode = http.StatusOK
				trace.DurationMS = time.Since(started).Milliseconds()
				mu.Lock()
				result[ep.Name] = cached
				traces = append(traces, trace)
				mu.Unlock()
				return nil
			}

			body, statusCode, err := c.fetchWithDedupe(gctx, fullURL)
			trace.StatusCode = statusCode
			trace.DurationMS = time.Since(started).Milliseconds()
			if err != nil {
				trace.Success = false
				trace.Error = err.Error()
				mu.Lock()
				traces = append(traces, trace)
				mu.Unlock()
				c.logger.Warn().Err(err).Str("endpoint", ep.Name).Str("url", fullURL).Msg("kgis call failed")
				return nil
			}

			var parsed map[string]any
			if err := json.Unmarshal(body, &parsed); err != nil {
				parsed = map[string]any{"raw": string(body)}
			}
			_ = c.cache.SetJSON(gctx, cacheKey, parsed, c.cacheTTL)

			trace.Success = true
			mu.Lock()
			result[ep.Name] = parsed
			traces = append(traces, trace)
			mu.Unlock()
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, nil, err
	}
	sort.Slice(traces, func(i, j int) bool { return traces[i].Name < traces[j].Name })
	return result, traces, nil
}

func (c *Client) fetchWithDedupe(ctx context.Context, fullURL string) ([]byte, int, error) {
	value, _, _ := c.sf.Do(fullURL, func() (interface{}, error) {
		body, statusCode, err := httpclient.DoWithRetry(ctx, c.httpClient, httpclient.RequestConfig{
			Method:      http.MethodGet,
			URL:         fullURL,
			Headers:     map[string]string{"Accept": "application/json"},
			RetryCount:  c.retryCount,
			BaseBackoff: 300 * time.Millisecond,
			Timeout:     c.timeout,
		})
		res := fetchResult{Body: body, Status: statusCode}
		if err != nil {
			res.Err = err.Error()
		}
		return res, nil
	})
	if value == nil {
		return nil, 0, errors.New("empty response")
	}
	res := value.(fetchResult)
	if res.Err != "" {
		return res.Body, res.Status, errors.New(res.Err)
	}
	return res.Body, res.Status, nil
}

func buildEndpoints(req domain.AnalyzeRequest) []endpointDef {
	queryWithLocation := func(r domain.AnalyzeRequest) url.Values {
		q := url.Values{}
		if r.Latitude != nil {
			v := strconv.FormatFloat(*r.Latitude, 'f', 6, 64)
			q.Set("lat", v)
			q.Set("latitude", v)
		}
		if r.Longitude != nil {
			v := strconv.FormatFloat(*r.Longitude, 'f', 6, 64)
			q.Set("lon", v)
			q.Set("longitude", v)
		}
		if r.VillageID != "" {
			q.Set("villageId", r.VillageID)
		}
		if r.SurveyNumber != "" {
			q.Set("surveyNo", r.SurveyNumber)
		}
		if r.Pincode != "" {
			q.Set("pincode", r.Pincode)
		}
		return q
	}

	always := func(domain.AnalyzeRequest) bool { return true }
	requiresSurvey := func(r domain.AnalyzeRequest) bool { return r.VillageID != "" && r.SurveyNumber != "" }

	return []endpointDef{
		{Name: "kgisAdminHierarchy", PathFn: constantPath("/genericwebservices/ws/kgisadminhierarchy"), QueryFn: queryWithLocation, ShouldRun: always},
		{Name: "nearbyAdminHierarchy", PathFn: constantPath("/genericwebservices/ws/nearbyadminhierarchy"), QueryFn: queryWithLocation, ShouldRun: always},
		{Name: "surveyNo", PathFn: constantPath("/genericwebservices/ws/surveyno"), QueryFn: queryWithLocation, ShouldRun: always},
		{Name: "districtCode", PathFn: constantPath("/genericwebservices/ws/districtcode"), QueryFn: queryWithLocation, ShouldRun: always},
		{Name: "talukCode", PathFn: constantPath("/genericwebservices/ws/talukcode"), QueryFn: queryWithLocation, ShouldRun: always},
		{Name: "hobliCode", PathFn: constantPath("/genericwebservices/ws/hoblicode"), QueryFn: queryWithLocation, ShouldRun: always},
		{
			Name: "geomForSurveyNum",
			PathFn: func(r domain.AnalyzeRequest) string {
				t := r.SurveyType
				if t == "" {
					t = "parcel"
				}
				return fmt.Sprintf("/genericwebservices/ws/geomForSurveyNum/%s/%s/%s", url.PathEscape(r.VillageID), url.PathEscape(r.SurveyNumber), url.PathEscape(t))
			},
			QueryFn:   func(domain.AnalyzeRequest) url.Values { return url.Values{} },
			ShouldRun: requiresSurvey,
		},
		{Name: "nearbyAssets", PathFn: constantPath("/NearbyAssets/ws/getNearbyAssetData"), QueryFn: queryWithLocation, ShouldRun: always},
		{Name: "distanceBetweenPincode", PathFn: constantPath("/genericwebservices/ws/getDistanceBtwPincode"), QueryFn: queryWithLocation, ShouldRun: always},
		{Name: "jurisdictionBoundary", PathFn: constantPath("/boundarywebservices/ws/getJurisdictionBoundary"), QueryFn: queryWithLocation, ShouldRun: always},
		{Name: "locationDetails", PathFn: constantPath("/genericwebservices/ws/getlocationdetails"), QueryFn: queryWithLocation, ShouldRun: always},
		{Name: "kgisAdminCodes2", PathFn: constantPath("/genericwebservices/ws/getKGISAdminCodes2"), QueryFn: queryWithLocation, ShouldRun: always},
	}
}

func constantPath(path string) func(domain.AnalyzeRequest) string {
	return func(domain.AnalyzeRequest) string { return path }
}

func keyHash(value string) string {
	h := sha1.Sum([]byte(value))
	return hex.EncodeToString(h[:])
}
