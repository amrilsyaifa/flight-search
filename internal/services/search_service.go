package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/amrilsyaifa/flight-search/internal/aggregator"
	"github.com/amrilsyaifa/flight-search/internal/models"
	"github.com/amrilsyaifa/flight-search/internal/pkg/helper/search"
	"github.com/amrilsyaifa/flight-search/internal/validation"
)

type FlightAggregator interface {
	Search(ctx context.Context) aggregator.Result
}

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

type SearchService struct {
	aggregator FlightAggregator
	cache      Cache
	cacheTTL   time.Duration
	logger     *slog.Logger
}

func NewSearchService(agg FlightAggregator, c Cache, cacheTTL time.Duration, logger *slog.Logger) *SearchService {
	return &SearchService{aggregator: agg, cache: c, cacheTTL: cacheTTL, logger: logger}
}

// Adapter each provider
func (s *SearchService) Search(ctx context.Context, req models.SearchRequest) (models.SearchResult, error) {
	if err := validation.SearchRequest(req); err != nil {
		return models.SearchResult{}, err
	}

	start := time.Now()
	agg, cacheHit := s.fetch(ctx, req.Criteria)

	flights := search.Filter(agg.Flights, req.Criteria, req.Filters)
	flights = search.Dedupe(flights)
	search.Sort(flights, req.Sort)

	return models.SearchResult{
		Criteria: req.Criteria,
		Flights:  flights,
		Metadata: models.SearchMetadata{
			ProvidersQueried:   agg.Queried,
			ProvidersSucceeded: agg.Succeeded,
			ProvidersFailed:    agg.Failed,
			SearchTime:         time.Since(start),
			CacheHit:           cacheHit,
		},
	}, nil
}

func (s *SearchService) fetch(ctx context.Context, c models.SearchCriteria) (aggregator.Result, bool) {
	key := cacheKey(c)

	if res, ok := s.readCache(ctx, key); ok {
		return res, true
	}

	res := s.aggregator.Search(ctx)

	if res.Failed == 0 {
		s.writeCache(ctx, key, res)
	}
	return res, false
}

func cacheKey(c models.SearchCriteria) string {
	return fmt.Sprintf("flights:v1:%s:%s:%s:%s", c.Origin, c.Destination, c.DepartureDate, c.CabinClass)
}

func (s *SearchService) readCache(ctx context.Context, key string) (aggregator.Result, bool) {
	raw, ok, err := s.cache.Get(ctx, key)
	if err != nil {
		s.logger.WarnContext(ctx, "cache read failed", "key", key, "error", err)
		return aggregator.Result{}, false
	}
	if !ok {
		return aggregator.Result{}, false
	}

	var res aggregator.Result
	if err := json.Unmarshal(raw, &res); err != nil {
		s.logger.WarnContext(ctx, "cache entry unreadable", "key", key, "error", err)
		return aggregator.Result{}, false
	}
	return res, true
}

func (s *SearchService) writeCache(ctx context.Context, key string, res aggregator.Result) {
	raw, err := json.Marshal(res)
	if err != nil {
		s.logger.WarnContext(ctx, "cache encode failed", "key", key, "error", err)
		return
	}
	if err := s.cache.Set(ctx, key, raw, s.cacheTTL); err != nil {
		s.logger.WarnContext(ctx, "cache write failed", "key", key, "error", err)
	}
}
