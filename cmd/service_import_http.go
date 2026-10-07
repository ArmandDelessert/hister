package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

func (c *serviceAPIClient) getJSONWithRetry(ctx context.Context, endpoint string, query url.Values, target any, wait func(context.Context, time.Duration) error) error {
	for attempt := 0; ; attempt++ {
		err := c.getJSON(ctx, endpoint, query, target)
		var apiErr *serviceAPIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests || attempt >= 3 {
			return err
		}
		delay := serviceImportRetryDelay(apiErr.Header, time.Now())
		log.Info().Str("source", c.name).Dur("delay", delay).Msg("Service API rate limit reached, waiting before retrying")
		if err := wait(ctx, delay); err != nil {
			return fmt.Errorf("wait for %s API rate limit: %w", c.name, err)
		}
	}
}

func serviceImportRetryDelay(header http.Header, now time.Time) time.Duration {
	const maxDelay = 5 * time.Minute
	if retryAfter := header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil && seconds >= 0 {
			return time.Duration(min(seconds, int64(maxDelay/time.Second))) * time.Second
		}
		if until, err := http.ParseTime(retryAfter); err == nil {
			return min(max(until.Sub(now), 0), maxDelay)
		}
	}
	if seconds, err := strconv.ParseInt(header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		return min(max(time.Unix(seconds, 0).Sub(now)+time.Second, time.Second), maxDelay)
	}
	return time.Minute
}

func waitForServiceImport(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
