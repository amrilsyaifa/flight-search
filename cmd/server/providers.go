package main

import (
	"log/slog"
	"time"

	"github.com/amrilsyaifa/flight-search/internal/providers"
	"github.com/amrilsyaifa/flight-search/internal/providers/garuda"
	"github.com/amrilsyaifa/flight-search/internal/providers/lion"
	"github.com/amrilsyaifa/flight-search/internal/providers/mockdata"
)

func buildProviders(logger *slog.Logger) []providers.Provider {
	garudaFetcher := providers.NewMockFetcher(mockdata.GarudaIndonesia, providers.MockConfig{
		MinDelay: 50 * time.Millisecond,
		MaxDelay: 100 * time.Millisecond,
	}, nil)

	lionFetcher := providers.NewMockFetcher(mockdata.LionAir, providers.MockConfig{
		MinDelay: 100 * time.Millisecond,
		MaxDelay: 200 * time.Millisecond,
	}, nil)

	return []providers.Provider{
		garuda.New(garudaFetcher, logger),
		lion.New(lionFetcher, logger),
	}
}
