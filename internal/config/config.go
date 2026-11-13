package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPPort        string
	ProviderTimeout time.Duration

	CacheDriver string
	CacheTTL    time.Duration
	RedisAddr   string
}

func Load() (Config, error) {
	timeout, err := getDuration("PROVIDER_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}
	ttl, err := getDuration("CACHE_TTL", 2*time.Minute)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPPort:        getEnv("HTTP_PORT", "8080"),
		ProviderTimeout: timeout,
		CacheDriver:     getEnv("CACHE_DRIVER", "redis"),
		CacheTTL:        ttl,
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
	}
	if cfg.CacheDriver != "redis" && cfg.CacheDriver != "memory" {
		return Config{}, fmt.Errorf("CACHE_DRIVER: must be redis or memory, got %q", cfg.CacheDriver)
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q (examples: 2s, 500ms)", key, v)
	}
	return d, nil
}
