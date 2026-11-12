package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/amrilsyaifa/flight-search/internal/models"
	"github.com/amrilsyaifa/flight-search/internal/providers"
	"github.com/amrilsyaifa/flight-search/internal/validation"
)

type SearchService struct {
	providers []providers.Provider
	logger    *slog.Logger
}

func NewSearchService(ps []providers.Provider, logger *slog.Logger) *SearchService {
	return &SearchService{providers: ps, logger: logger}
}

func (s *SearchService) Search(ctx context.Context, c models.SearchCriteria) (models.SearchResult, error) {
	if err := validation.SearchCriteria(c); err != nil {
		return models.SearchResult{}, err
	}

	start := time.Now()
	result := models.SearchResult{Criteria: c, Flights: []models.Flight{}}

	for _, p := range s.providers {
		result.Metadata.ProvidersQueried++
		flights, err := p.Search(ctx)
		if err != nil {
			s.logger.WarnContext(ctx, "provider failed", "provider", p.Name(), "error", err)
			result.Metadata.ProvidersFailed++
			continue
		}

		result.Metadata.ProvidersSucceeded++
		result.Flights = append(result.Flights, flights...)
	}
	result.Metadata.SearchTime = time.Since(start)
	return result, nil
}
