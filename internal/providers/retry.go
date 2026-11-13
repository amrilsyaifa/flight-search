package providers

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"
)

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

type RetryFetcher struct {
	next   Fetcher
	policy RetryPolicy
	logger *slog.Logger
}

var _ Fetcher = (*RetryFetcher)(nil)

func NewRetryFetcher(next Fetcher, policy RetryPolicy, logger *slog.Logger) *RetryFetcher {
	if policy.MaxAttempts < 1 {
		policy.MaxAttempts = 1
	}
	return &RetryFetcher{next: next, policy: policy, logger: logger}
}

func (r *RetryFetcher) Fetch(ctx context.Context) ([]byte, error) {
	var err error
	for attempt := 1; attempt <= r.policy.MaxAttempts; attempt++ {
		var body []byte
		body, err = r.next.Fetch(ctx)
		if err == nil {
			return body, nil
		}

		if !errors.Is(err, ErrProviderUnavailable) || attempt == r.policy.MaxAttempts {
			return nil, err
		}

		wait := r.backoff(attempt)
		r.logger.WarnContext(ctx, "fetch failed, retrying", "attempt", attempt, "backoff_ms", wait.Milliseconds(), "error", err)
		timer := time.NewTimer(wait)

		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}

	}
	return nil, err
}

func (r *RetryFetcher) backoff(attempt int) time.Duration {
	delay := r.policy.BaseDelay << (attempt - 1) // base * 2^(attempt-1)
	if delay <= 0 || delay > r.policy.MaxDelay {
		delay = r.policy.MaxDelay
	}

	half := delay / 2
	if half <= 0 {
		return delay
	}
	return half + time.Duration(rand.Int64N(int64(half)+1))
}
