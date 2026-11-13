package validation

import (
	"fmt"
	"regexp"

	"github.com/amrilsyaifa/flight-search/internal/models"
)

var airportCode = regexp.MustCompile(`^[A-Z]{3}$`)

func SearchRequest(r models.SearchRequest) error {
	if err := SearchCriteria(r.Criteria); err != nil {
		return err
	}
	if err := searchFilters(r.Filters); err != nil {
		return err
	}
	return sortOption(r.Sort)
}

func searchFilters(f models.SearchFilters) error {
	switch {
	case f.MinPrice != nil && *f.MinPrice < 0:
		return invalid("filters.minPrice must be >= 0")
	case f.MaxPrice != nil && *f.MaxPrice < 0:
		return invalid("filters.maxPrice must be >= 0")
	case f.MinPrice != nil && f.MaxPrice != nil && *f.MinPrice > *f.MaxPrice:
		return invalid("filters.minPrice must not exceed maxPrice")
	case f.MaxStops != nil && *f.MaxStops < 0:
		return invalid("filters.maxStops must be >= 0")
	case f.MaxDurationMinutes != nil && *f.MaxDurationMinutes < 1:
		return invalid("filters.maxDurationMinutes must be >= 1")
	}
	return nil
}

func sortOption(s models.SortOption) error {
	switch s.By {
	case models.SortByBestValue, models.SortByPrice, models.SortByDuration, models.SortByDepartureTime, models.SortByArrivalTime:
	default:
		return invalid(fmt.Sprintf("unsupported sort.by %q", s.By))
	}
	if s.Order != models.SortAsc && s.Order != models.SortDesc {
		return invalid(fmt.Sprintf("sort.order must be asc or desc, got %q", s.Order))
	}
	return nil
}

func SearchCriteria(c models.SearchCriteria) error {
	switch {
	case !airportCode.MatchString(c.Origin):
		return invalid("origin must be a 3-letter IATA code")
	case !airportCode.MatchString(c.Destination):
		return invalid("destination must be a 3-letter IATA code")
	case c.Origin == c.Destination:
		return invalid("destination must differ from origin")
	case c.Passengers < 1 || c.Passengers > 9:
		return invalid("passengers must be between 1 and 9")
	}

	switch c.CabinClass {
	case models.CabinEconomy, models.CabinPremiumEconomy, models.CabinBusiness, models.CabinFirst:
	default:
		return invalid(fmt.Sprintf("unsupported cabin class %q", c.CabinClass))
	}
	return nil
}

func invalid(msg string) error {
	return fmt.Errorf("%w: %s", models.ErrInvalidRequest, msg)
}
