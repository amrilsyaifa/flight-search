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
