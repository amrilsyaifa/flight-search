package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort        string
	ProviderTimeout time.Duration

	StoreDriver string
	CacheTTL    time.Duration
	RedisAddr   string

	ProviderRateLimit int
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

	rateLimit, err := getInt("PROVIDER_RATE_LIMIT", 10)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPPort:          getEnv("HTTP_PORT", "8080"),
		ProviderTimeout:   timeout,
		StoreDriver:       getEnv("STORE_DRIVER", "redis"),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		CacheTTL:          ttl,
		ProviderRateLimit: rateLimit,
	}
	if cfg.StoreDriver != "redis" && cfg.StoreDriver != "memory" {
		return Config{}, fmt.Errorf("STORE_DRIVER: must be redis or memory, got %q", cfg.StoreDriver)
	}
	if cfg.ProviderRateLimit < 1 {
		return Config{}, fmt.Errorf("PROVIDER_RATE_LIMIT: must be at least 1, got %d", cfg.ProviderRateLimit)
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

func getInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid number %q", key, v)
	}
	return n, nil
}
