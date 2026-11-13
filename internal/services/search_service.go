package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

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
	inflight   singleflight.Group
}

type leg struct {
	flights  []models.Flight
	agg      aggregator.Result
	cacheHit bool
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
	result := models.SearchResult{Criteria: req.Criteria}

	if !req.Criteria.IsRoundTrip() {
		out := s.searchLeg(ctx, req.Criteria, req)
		result.Flights = out.flights
		result.Metadata = out.metadata()
	} else {
		// Each goroutine writes only its own variable, so no mutex is needed.
		var out, back leg
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); out = s.searchLeg(ctx, req.Criteria, req) }()
		go func() { defer wg.Done(); back = s.searchLeg(ctx, req.Criteria.ReturnLeg(), req) }()
		wg.Wait()

		result.Flights = out.flights
		result.ReturnFlights = back.flights
		result.Metadata = combine(out.metadata(), back.metadata())
	}

	result.Metadata.SearchTime = time.Since(start)
	return result, nil
}

func (s *SearchService) searchLeg(ctx context.Context, c models.SearchCriteria, req models.SearchRequest) leg {
	agg, hit := s.fetch(ctx, c)

	flights := search.Filter(agg.Flights, c, req.Filters)
	flights = search.Dedupe(flights)
	search.Sort(flights, req.Sort)

	return leg{flights: flights, agg: agg, cacheHit: hit}
}

func (l leg) metadata() models.SearchMetadata {
	return models.SearchMetadata{
		ProvidersQueried:   l.agg.Queried,
		ProvidersSucceeded: l.agg.Succeeded,
		ProvidersFailed:    l.agg.Failed,
		CacheHit:           l.cacheHit,
	}
}

func (s *SearchService) fetch(ctx context.Context, c models.SearchCriteria) (aggregator.Result, bool) {
	key := cacheKey(c)

	if res, ok := s.readCache(ctx, key); ok {
		return res, true
	}

	v, _, _ := s.inflight.Do(key, func() (any, error) {
		shared := context.WithoutCancel(ctx)

		res := s.aggregator.Search(shared)

		if res.Failed == 0 {
			s.writeCache(shared, key, res)
		}
		return res, nil
	})

	return v.(aggregator.Result), false
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

func combine(a, b models.SearchMetadata) models.SearchMetadata {
	return models.SearchMetadata{
		ProvidersQueried:   a.ProvidersQueried + b.ProvidersQueried,
		ProvidersSucceeded: a.ProvidersSucceeded + b.ProvidersSucceeded,
		ProvidersFailed:    a.ProvidersFailed + b.ProvidersFailed,
		CacheHit:           a.CacheHit && b.CacheHit,
	}
}
