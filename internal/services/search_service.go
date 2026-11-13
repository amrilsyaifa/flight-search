package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/amrilsyaifa/flight-search/internal/aggregator"
	"github.com/amrilsyaifa/flight-search/internal/models"
	"github.com/amrilsyaifa/flight-search/internal/pkg/search"
	"github.com/amrilsyaifa/flight-search/internal/validation"
)

type FlightAggregator interface {
	Search(ctx context.Context) aggregator.Result
}

type SearchService struct {
	aggregator FlightAggregator
	logger     *slog.Logger
}

func NewSearchService(agg FlightAggregator, logger *slog.Logger) *SearchService {
	return &SearchService{aggregator: agg, logger: logger}
}

// Adapter each provider
func (s *SearchService) Search(ctx context.Context, req models.SearchRequest) (models.SearchResult, error) {
	if err := validation.SearchRequest(req); err != nil {
		return models.SearchResult{}, err
	}

	start := time.Now()
	agg := s.aggregator.Search(ctx)

	flights := search.Filter(agg.Flights, req.Criteria, req.Filters)
	search.Sort(flights, req.Sort)

	return models.SearchResult{
		Criteria: req.Criteria,
		Flights:  flights,
		Metadata: models.SearchMetadata{
			ProvidersQueried:   agg.Queried,
			ProvidersSucceeded: agg.Succeeded,
			ProvidersFailed:    agg.Failed,
			SearchTime:         time.Since(start),
		},
	}, nil
}
