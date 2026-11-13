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

	_ "time/tzdata" // Embeds the IANA time zone database

	"github.com/amrilsyaifa/flight-search/api"
	"github.com/amrilsyaifa/flight-search/internal/aggregator"
	"github.com/amrilsyaifa/flight-search/internal/config"
	"github.com/amrilsyaifa/flight-search/internal/handlers"
	"github.com/amrilsyaifa/flight-search/internal/handlers/docs"
	"github.com/amrilsyaifa/flight-search/internal/handlers/flight"
	"github.com/amrilsyaifa/flight-search/internal/handlers/health"
	"github.com/amrilsyaifa/flight-search/internal/oapi"
	"github.com/amrilsyaifa/flight-search/internal/services"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid config", "error", err)
		os.Exit(1)
	}

	logger.Info("config loaded",
		"http_port", cfg.HTTPPort,
		"store_driver", cfg.StoreDriver,
		"provider_timeout", cfg.ProviderTimeout.String(),
		"provider_rate_limit", cfg.ProviderRateLimit,
		"cache_ttl", cfg.CacheTTL.String(),
	)

	//Dependency
	st, err := buildStores(cfg, logger)
	if err != nil {
		logger.Error("store unavailable", "error", err)
		os.Exit(1)
	}
	defer st.close()

	providerList := buildProviders(providerDeps{
		limiter:   st.limiter,
		rateLimit: cfg.ProviderRateLimit,
		logger:    logger,
	})
	agg := aggregator.New(providerList, cfg.ProviderTimeout, logger)
	searchService := services.NewSearchService(agg, st.cache, cfg.CacheTTL, logger)
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
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}
