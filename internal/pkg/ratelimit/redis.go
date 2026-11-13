package ratelimit

import (
	"context"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	limiter *redis_rate.Limiter
}

func NewRedis(client *redis.Client) *Redis {
	return &Redis{limiter: redis_rate.NewLimiter(client)}
}

func (r *Redis) Allow(ctx context.Context, key string, perSecond int) (bool, time.Duration, error) {
	res, err := r.limiter.Allow(ctx, "provider:"+key, redis_rate.PerSecond(perSecond))
	if err != nil {
		return false, 0, err
	}
	return res.Allowed > 0, res.RetryAfter, nil
}
