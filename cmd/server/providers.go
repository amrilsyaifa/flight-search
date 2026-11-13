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

type providerDeps struct {
	limiter   providers.Limiter
	rateLimit int // calls per second, per provider
	logger    *slog.Logger
}

func (d providerDeps) fetcher(name string, data []byte, mock providers.MockConfig) providers.Fetcher {
	logger := d.logger.With("provider", name)

	var f providers.Fetcher = providers.NewMockFetcher(data, mock, nil)
	// rate limiter
	f = providers.NewRateLimitedFetcher(f, d.limiter, name, d.rateLimit, logger)
	// retry exponential backoff
	f = providers.NewRetryFetcher(f, retryPolicy, logger)
	return f
}

// init providers
func buildProviders(d providerDeps) []providers.Provider {
	ms := time.Millisecond
	return []providers.Provider{
		garuda.New(d.fetcher("Garuda Indonesia", mockdata.GarudaIndonesia, providers.MockConfig{MinDelay: 50 * ms, MaxDelay: 100 * ms}), d.logger),
		lion.New(d.fetcher("Lion Air", mockdata.LionAir, providers.MockConfig{MinDelay: 100 * ms, MaxDelay: 200 * ms}), d.logger),
		batik.New(d.fetcher("Batik Air", mockdata.BatikAir, providers.MockConfig{MinDelay: 200 * ms, MaxDelay: 400 * ms}), d.logger),
		airasia.New(d.fetcher("AirAsia", mockdata.AirAsia, providers.MockConfig{MinDelay: 50 * ms, MaxDelay: 150 * ms, FailureRate: 0.1}), d.logger),
	}
}
