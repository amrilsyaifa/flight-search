package flight

import (
	"fmt"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/amrilsyaifa/flight-search/internal/models"
	"github.com/amrilsyaifa/flight-search/internal/oapi"
)

// ---------- request: API -> domain ----------
func toCriteria(b oapi.SearchRequest) models.SearchCriteria {
	c := models.SearchCriteria{
		Origin:        strings.ToUpper(strings.TrimSpace(b.Origin)),
		Destination:   strings.ToUpper(strings.TrimSpace(b.Destination)),
		DepartureDate: models.DateOf(b.DepartureDate.Time),
		Passengers:    b.Passengers,
		CabinClass:    models.CabinClass(b.CabinClass),
	}
	if b.ReturnDate != nil {
		d := models.DateOf(b.ReturnDate.Time)
		c.ReturnDate = &d
	}
	return c
}

// ---------- response: domain -> API ----------
func toResponse(r models.SearchResult) oapi.SearchResponse {
	flights := make([]oapi.Flight, 0, len(r.Flights))
	for _, f := range r.Flights {
		flights = append(flights, toAPIFlight(f))
	}

	return oapi.SearchResponse{
		SearchCriteria: oapi.SearchCriteria{
			Origin:        r.Criteria.Origin,
			Destination:   r.Criteria.Destination,
			DepartureDate: toAPIDate(r.Criteria.DepartureDate),
			Passengers:    r.Criteria.Passengers,
			CabinClass:    oapi.CabinClass(r.Criteria.CabinClass),
		},
		Metadata: oapi.SearchMetadata{
			TotalResults:       len(flights),
			ProvidersQueried:   r.Metadata.ProvidersQueried,
			ProvidersSucceeded: r.Metadata.ProvidersSucceeded,
			ProvidersFailed:    r.Metadata.ProvidersFailed,
			SearchTimeMs:       r.Metadata.SearchTime.Milliseconds(),
			CacheHit:           r.Metadata.CacheHit,
		},
		Flights: flights,
	}
}

func toAPIFlight(f models.Flight) oapi.Flight {
	var aircraft *string // nil -> JSON null
	if f.Aircraft != "" {
		aircraft = &f.Aircraft
	}
	amenities := f.Amenities
	if amenities == nil {
		amenities = []string{} // JSON [] instead of null
	}

	return oapi.Flight{
		Id:           f.ID,
		Provider:     f.Provider,
		Airline:      oapi.Airline{Name: f.Airline.Name, Code: f.Airline.Code},
		FlightNumber: f.FlightNumber,
		Departure:    toAPIPoint(f.Departure),
		Arrival:      toAPIPoint(f.Arrival),
		Duration: oapi.Duration{
			TotalMinutes: f.DurationMinutes,
			Formatted:    formatDuration(f.DurationMinutes),
		},
		Stops:          f.Stops(),
		Price:          oapi.Price{Amount: f.Price.Amount, Currency: f.Price.Currency},
		AvailableSeats: f.AvailableSeats,
		CabinClass:     oapi.CabinClass(f.CabinClass),
		Aircraft:       aircraft,
		Amenities:      amenities,
		Baggage:        oapi.Baggage{CarryOn: f.Baggage.CarryOn, Checked: f.Baggage.Checked},
	}
}

func toAPIPoint(p models.FlightPoint) oapi.FlightPoint {
	return oapi.FlightPoint{
		Airport:   p.Airport,
		City:      p.City,
		Datetime:  p.Time,        // keeps the local offset, e.g. +07:00
		Timestamp: p.Time.Unix(), // derived here, never stored
	}
}

func toAPIDate(d models.Date) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)}
}

// formatDuration renders minutes as "4h 20m", "55m" or "2h".
func formatDuration(minutes int) string {
	h, m := minutes/60, minutes%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dh %dm", h, m)
	}
}

func toSearchRequest(b oapi.SearchRequest) (models.SearchRequest, error) {
	filters, err := toFilters(b.Filters)
	if err != nil {
		return models.SearchRequest{}, err
	}
	return models.SearchRequest{
		Criteria: toCriteria(b),
		Filters:  filters,
		Sort:     toSort(b.Sort),
	}, nil
}

func toFilters(f *oapi.SearchFilters) (models.SearchFilters, error) {
	if f == nil {
		return models.SearchFilters{}, nil
	}
	out := models.SearchFilters{
		MinPrice:           f.MinPrice,
		MaxPrice:           f.MaxPrice,
		MaxStops:           f.MaxStops,
		MaxDurationMinutes: f.MaxDurationMinutes,
	}
	if f.Airlines != nil {
		for _, a := range *f.Airlines {
			out.Airlines = append(out.Airlines, strings.ToUpper(strings.TrimSpace(a)))
		}
	}

	var err error
	if out.DepartureFrom, err = parseTimeFilter("departureTimeFrom", f.DepartureTimeFrom); err != nil {
		return models.SearchFilters{}, err
	}
	if out.DepartureTo, err = parseTimeFilter("departureTimeTo", f.DepartureTimeTo); err != nil {
		return models.SearchFilters{}, err
	}
	if out.ArrivalFrom, err = parseTimeFilter("arrivalTimeFrom", f.ArrivalTimeFrom); err != nil {
		return models.SearchFilters{}, err
	}
	if out.ArrivalTo, err = parseTimeFilter("arrivalTimeTo", f.ArrivalTimeTo); err != nil {
		return models.SearchFilters{}, err
	}
	return out, nil
}

func parseTimeFilter(field string, v *string) (*models.TimeOfDay, error) {
	if v == nil {
		return nil, nil
	}
	t, err := models.ParseTimeOfDay(*v)
	if err != nil {
		return nil, fmt.Errorf("%w: filters.%s: %v", models.ErrInvalidRequest, field, err)
	}
	return &t, nil
}

func toSort(s *oapi.SortOption) models.SortOption {
	out := models.DefaultSort
	if s == nil {
		return out
	}
	if s.By != nil {
		out.By = models.SortField(*s.By)
	}
	if s.Order != nil {
		out.Order = models.SortOrder(*s.Order)
	}
	return out
}
