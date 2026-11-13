package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPPort        string
	ProviderTimeout time.Duration
}

func Load() (Config, error) {
	timeout, err := getDuration("PROVIDER_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPPort:        getEnv("HTTP_PORT", "8080"),
		ProviderTimeout: timeout,
	}, nil
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
