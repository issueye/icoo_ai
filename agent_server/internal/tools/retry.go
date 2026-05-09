package tools

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

const (
	defaultExternalRetryAttempts = 3
	defaultExternalRetryDelay    = 200 * time.Millisecond
)

func retryHTTP(ctx context.Context, attempts int, fn func() (*http.Response, error)) (*http.Response, int, error) {
	if attempts <= 0 {
		attempts = defaultExternalRetryAttempts
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, attempt - 1, err
		}
		resp, err := fn()
		if err == nil {
			if !shouldRetryStatus(resp.StatusCode) || attempt == attempts {
				return resp, attempt, nil
			}
			_ = resp.Body.Close()
			if err := sleepRetry(ctx, attempt); err != nil {
				return nil, attempt, err
			}
			continue
		}
		lastErr = err
		if !shouldRetryErr(err) || attempt == attempts {
			return nil, attempt, err
		}
		if err := sleepRetry(ctx, attempt); err != nil {
			return nil, attempt, err
		}
	}
	return nil, attempts, lastErr
}

func shouldRetryStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

func shouldRetryErr(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

func sleepRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(attempt) * defaultExternalRetryDelay
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
