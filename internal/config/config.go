package config

import "os"

type Config struct {
	HTTPPort string
}

func Load() Config {
	return Config{HTTPPort: getEnv("PORT", "8080")}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
