package providers

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Limiter interface {
	Allow(ctx context.Context, key string, perSecond int) (allowed bool, retryAfter time.Duration, err error)
}

type RateLimitedFetcher struct {
	next      Fetcher
	limiter   Limiter
	key       string
	perSecond int
	logger    *slog.Logger
}

var _ Fetcher = (*RateLimitedFetcher)(nil)

func NewRateLimitedFetcher(next Fetcher, limiter Limiter, key string, perSecond int, logger *slog.Logger) *RateLimitedFetcher {
	return &RateLimitedFetcher{next: next, limiter: limiter, key: key, perSecond: perSecond, logger: logger}
}

func (r *RateLimitedFetcher) Fetch(ctx context.Context) ([]byte, error) {
	for {
		allowed, retryAfter, err := r.limiter.Allow(ctx, r.key, r.perSecond)
		if err != nil {

			r.logger.WarnContext(ctx, "rate limiter unavailable, allowing call", "error", err)
			return r.next.Fetch(ctx)
		}
		if allowed {
			return r.next.Fetch(ctx)
		}

		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < retryAfter {
			return nil, fmt.Errorf("%w (next slot in %s)", ErrRateLimited, retryAfter)
		}

		r.logger.DebugContext(ctx, "rate limited, waiting", "wait_ms", retryAfter.Milliseconds())
		timer := time.NewTimer(retryAfter)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
