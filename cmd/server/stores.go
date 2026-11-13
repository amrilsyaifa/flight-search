package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/amrilsyaifa/flight-search/internal/config"
	"github.com/amrilsyaifa/flight-search/internal/pkg/cache"
	"github.com/amrilsyaifa/flight-search/internal/pkg/ratelimit"
	"github.com/amrilsyaifa/flight-search/internal/providers"
	"github.com/amrilsyaifa/flight-search/internal/services"
	"github.com/redis/go-redis/v9"
)

type stores struct {
	cache   services.Cache
	limiter providers.Limiter
	close   func()
}

func buildStores(cfg config.Config, logger *slog.Logger) (stores, error) {
	if cfg.StoreDriver == "memory" {
		logger.Info("using in-memory cache and rate limiter")
		return stores{cache: cache.NewMemory(), limiter: ratelimit.NewMemory(), close: func() {}}, nil
	}

	client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return stores{}, fmt.Errorf("redis at %s: %w", cfg.RedisAddr, err)
	}

	logger.Info("using redis cache and rate limiter", "addr", cfg.RedisAddr)
	return stores{
		cache:   cache.NewRedis(client),
		limiter: ratelimit.NewRedis(client),
		close:   func() { _ = client.Close() },
	}, nil
}
