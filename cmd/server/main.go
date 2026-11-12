package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amrilsyaifa/flight-search/api"
	"github.com/amrilsyaifa/flight-search/internal/config"
	"github.com/amrilsyaifa/flight-search/internal/handlers"
	"github.com/amrilsyaifa/flight-search/internal/handlers/docs"
	"github.com/amrilsyaifa/flight-search/internal/handlers/flight"
	"github.com/amrilsyaifa/flight-search/internal/handlers/health"
	"github.com/amrilsyaifa/flight-search/internal/oapi"
	"github.com/amrilsyaifa/flight-search/internal/providers"
	"github.com/amrilsyaifa/flight-search/internal/providers/garuda"
	"github.com/amrilsyaifa/flight-search/internal/providers/mockdata"
	"github.com/amrilsyaifa/flight-search/internal/services"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	//Dependency
	garudaFetcher := providers.NewMockFetcher(mockdata.GarudaIndonesia, providers.MockConfig{
		MinDelay: 50 * time.Millisecond,
		MaxDelay: 100 * time.Millisecond,
	}, nil)

	providerList := []providers.Provider{
		garuda.New(garudaFetcher, logger),
		// nanti: lion, batik, airasia
	}

	searchService := services.NewSearchService(providerList, logger)
	server := handlers.NewServer(health.New(), flight.New(searchService, logger))
	strict := oapi.NewStrictHandler(server, nil)
	mux := http.NewServeMux()
	oapi.HandlerFromMux(strict, mux)

	docs.Register(mux, api.Spec)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// ctx cancel
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("gracefull shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}
