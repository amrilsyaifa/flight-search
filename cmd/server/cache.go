package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/amrilsyaifa/flight-search/internal/config"
	"github.com/amrilsyaifa/flight-search/internal/pkg/cache"
	"github.com/amrilsyaifa/flight-search/internal/services"
	"github.com/redis/go-redis/v9"
)

func buildCache(cfg config.Config, logger *slog.Logger) (services.Cache, func(), error) {
	if cfg.CacheDriver == "memory" {
		logger.Info("using in-memory cache")
		return cache.NewMemory(), func() {}, nil
	}

	client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("redis at %s: %w", cfg.RedisAddr, err)
	}

	logger.Info("using redis cache", "addr", cfg.RedisAddr)
	return cache.NewRedis(client), func() { _ = client.Close() }, nil
}
