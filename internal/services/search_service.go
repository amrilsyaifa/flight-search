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
	criteria := []models.SearchCriteria{req.Criteria}
	if req.Criteria.IsRoundTrip() {
		criteria = append(criteria, req.Criteria.ReturnLeg())
	}

	legs := s.searchLegs(ctx, criteria, req.Filters, req.Sort)

	result := models.SearchResult{
		Criteria: req.Criteria,
		Flights:  legs[0].flights,
		Metadata: combine(legs),
	}
	if req.Criteria.IsRoundTrip() {
		result.ReturnFlights = legs[1].flights
	}
	result.Metadata.SearchTime = time.Since(start)
	return result, nil
}

func (s *SearchService) searchLeg(ctx context.Context, c models.SearchCriteria, f models.SearchFilters, so models.SortOption) leg {
	agg, hit := s.fetch(ctx, c)

	flights := search.Filter(agg.Flights, c, f)
	flights = search.Dedupe(flights)
	search.Sort(flights, so)

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

func combine(legs []leg) models.SearchMetadata {
	m := models.SearchMetadata{CacheHit: true}
	for _, l := range legs {
		m.ProvidersQueried += l.agg.Queried
		m.ProvidersSucceeded += l.agg.Succeeded
		m.ProvidersFailed += l.agg.Failed
		m.CacheHit = m.CacheHit && l.cacheHit
	}
	return m
}

func (s *SearchService) SearchMultiCity(ctx context.Context, req models.MultiCityRequest) (models.MultiCityResult, error) {
	if err := validation.MultiCityRequest(req); err != nil {
		return models.MultiCityResult{}, err
	}

	start := time.Now()
	legs := s.searchLegs(ctx, req.Legs, req.Filters, req.Sort)

	result := models.MultiCityResult{
		Legs:     make([]models.LegResult, len(legs)),
		Metadata: combine(legs),
	}
	for i, l := range legs {
		result.Legs[i] = models.LegResult{Criteria: req.Legs[i], Flights: l.flights}
	}
	result.Metadata.SearchTime = time.Since(start)
	return result, nil
}

func (s *SearchService) searchLegs(ctx context.Context, criteria []models.SearchCriteria, f models.SearchFilters, so models.SortOption) []leg {
	out := make([]leg, len(criteria))

	var wg sync.WaitGroup
	for i, c := range criteria {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = s.searchLeg(ctx, c, f, so)
		}()
	}
	wg.Wait()
	return out
}
