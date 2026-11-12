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

// init providers
func buildProviders(logger *slog.Logger) []providers.Provider {
	garudaFetcher := providers.NewMockFetcher(mockdata.GarudaIndonesia, providers.MockConfig{
		MinDelay: 50 * time.Millisecond,
		MaxDelay: 100 * time.Millisecond,
	}, nil)

	lionFetcher := providers.NewMockFetcher(mockdata.LionAir, providers.MockConfig{
		MinDelay: 100 * time.Millisecond,
		MaxDelay: 200 * time.Millisecond,
	}, nil)

	batikFetcher := providers.NewMockFetcher(mockdata.BatikAir, providers.MockConfig{
		MinDelay: 200 * time.Millisecond,
		MaxDelay: 400 * time.Millisecond,
	}, nil)

	airAsiaFetcher := providers.NewMockFetcher(mockdata.AirAsia, providers.MockConfig{
		MinDelay:    50 * time.Millisecond,
		MaxDelay:    150 * time.Millisecond,
		FailureRate: 0.1, // 90% success rate
	}, nil)

	return []providers.Provider{
		garuda.New(garudaFetcher, logger),
		lion.New(lionFetcher, logger),
		batik.New(batikFetcher, logger),
		airasia.New(airAsiaFetcher, logger),
	}
}
