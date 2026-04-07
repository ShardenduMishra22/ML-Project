package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"
)

type RequestConfig struct {
	Method      string
	URL         string
	Headers     map[string]string
	Body        []byte
	RetryCount  int
	BaseBackoff time.Duration
	Timeout     time.Duration
}

func DoWithRetry(ctx context.Context, client *http.Client, cfg RequestConfig) ([]byte, int, error) {
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = 250 * time.Millisecond
	}

	var lastErr error
	statusCode := 0

	for attempt := 0; attempt <= cfg.RetryCount; attempt++ {
		attemptCtx := ctx
		var cancel context.CancelFunc
		if cfg.Timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		}

		req, err := http.NewRequestWithContext(attemptCtx, cfg.Method, cfg.URL, bytes.NewReader(cfg.Body))
		if cancel != nil {
			defer cancel()
		}
		if err != nil {
			return nil, 0, err
		}
		for key, value := range cfg.Headers {
			req.Header.Set(key, value)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			statusCode = resp.StatusCode
			responseBytes, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
			} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if cancel != nil {
					cancel()
				}
				return responseBytes, resp.StatusCode, nil
			} else {
				lastErr = fmt.Errorf("status=%d body=%s", resp.StatusCode, string(responseBytes))
				if resp.StatusCode >= 400 && resp.StatusCode < 500 {
					if cancel != nil {
						cancel()
					}
					return responseBytes, resp.StatusCode, lastErr
				}
			}
		}

		if cancel != nil {
			cancel()
		}
		if attempt == cfg.RetryCount {
			break
		}
		backoff := time.Duration(float64(cfg.BaseBackoff) * math.Pow(2, float64(attempt)))
		jitter := time.Duration(rand.Int63n(int64(100 * time.Millisecond)))
		select {
		case <-time.After(backoff + jitter):
		case <-ctx.Done():
			return nil, statusCode, ctx.Err()
		}
	}
	return nil, statusCode, lastErr
}
