package main

import (
	"log/slog"
	"time"

	"github.com/amrilsyaifa/flight-search/internal/providers"
	"github.com/amrilsyaifa/flight-search/internal/providers/airasia"
	"github.com/amrilsyaifa/flight-search/internal/providers/batik"
	"github.com/amrilsyaifa/flight-search/internal/providers/garuda"
	"github.com/amrilsyaifa/flight-search/internal/providers/lion"
	"github.com/amrilsyaifa/flight-search/internal/providers/mockdata"
)

var retryPolicy = providers.RetryPolicy{
	MaxAttempts: 3,
	BaseDelay:   50 * time.Millisecond,
	MaxDelay:    200 * time.Millisecond,
}

func fetcher(name string, data []byte, cfg providers.MockConfig, logger *slog.Logger) providers.Fetcher {
	mock := providers.NewMockFetcher(data, cfg, nil)
	// retry exponential backoff
	return providers.NewRetryFetcher(mock, retryPolicy, logger.With("provider", name))
}

// init providers
func buildProviders(logger *slog.Logger) []providers.Provider {
	ms := time.Millisecond
	return []providers.Provider{
		garuda.New(fetcher("Garuda Indonesia", mockdata.GarudaIndonesia, providers.MockConfig{MinDelay: 50 * ms, MaxDelay: 100 * ms}, logger), logger),
		lion.New(fetcher("Lion Air", mockdata.LionAir, providers.MockConfig{MinDelay: 100 * ms, MaxDelay: 200 * ms}, logger), logger),
		batik.New(fetcher("Batik Air", mockdata.BatikAir, providers.MockConfig{MinDelay: 200 * ms, MaxDelay: 400 * ms}, logger), logger),
		airasia.New(fetcher("AirAsia", mockdata.AirAsia, providers.MockConfig{MinDelay: 50 * ms, MaxDelay: 150 * ms, FailureRate: 0.1}, logger), logger),
	}
}
